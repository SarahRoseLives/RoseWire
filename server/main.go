package main

import (
	"bufio"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"
)

const (
	serverHost  = "0.0.0.0"
	serverPort  = 2222
	hostKeyFile = "server_ed25519"
	nickDBFile  = "nicks.db"
)

type NickDB struct {
	sync.Mutex
	NickToKey map[string]string // nickname -> base64 public key
}

func LoadNickDB(path string) (*NickDB, error) {
	db := &NickDB{NickToKey: make(map[string]string)}
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return db, nil
		}
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		parts := strings.SplitN(scanner.Text(), " ", 2)
		if len(parts) != 2 {
			continue
		}
		db.NickToKey[parts[0]] = parts[1]
	}
	return db, scanner.Err()
}

func (db *NickDB) Save(path string) error {
	db.Lock()
	defer db.Unlock()
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	defer f.Close()
	for nick, key := range db.NickToKey {
		fmt.Fprintf(f, "%s %s\n", nick, key)
	}
	return os.Rename(tmp, path)
}

func (db *NickDB) Register(nick string, pubkey ssh.PublicKey) error {
	db.Lock()
	defer db.Unlock()
	keyStr := base64.StdEncoding.EncodeToString(pubkey.Marshal())
	if old, ok := db.NickToKey[nick]; ok {
		if old != keyStr {
			return errors.New("nickname already taken with different key")
		}
	} else {
		db.NickToKey[nick] = keyStr
	}
	return nil
}

func (db *NickDB) Check(nick string, pubkey ssh.PublicKey) bool {
	db.Lock()
	defer db.Unlock()
	keyStr := base64.StdEncoding.EncodeToString(pubkey.Marshal())
	known, ok := db.NickToKey[nick]
	return ok && known == keyStr
}

func ensureHostKey(path string) (ssh.Signer, error) {
	keyBytes, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Host key %s not found. Generate with:\n\n    ssh-keygen -t ed25519 -f %s\n\n", path, path)
		return nil, err
	}
	return ssh.ParsePrivateKey(keyBytes)
}

func parseNickname(conn ssh.ConnMetadata) string {
	return conn.User()
}

func main() {
	fmt.Printf("Starting RoseWire relay server on %s:%d ...\n", serverHost, serverPort)
	hostSigner, err := ensureHostKey(hostKeyFile)
	if err != nil {
		log.Fatalf("Failed to load host key: %v", err)
	}

	nickDB, err := LoadNickDB(nickDBFile)
	if err != nil {
		log.Fatalf("Failed to load nick DB: %v", err)
	}

	fileRegistry := NewFileRegistry()
	chatHub := NewChatHub(fileRegistry)

	config := &ssh.ServerConfig{
		NoClientAuth: false,
		PublicKeyCallback: func(meta ssh.ConnMetadata, pubKey ssh.PublicKey) (*ssh.Permissions, error) {
			nick := parseNickname(meta)
			if nick == "" {
				return nil, fmt.Errorf("nickname missing")
			}
			err := nickDB.Register(nick, pubKey)
			if err != nil {
				return nil, err
			}
			if err := nickDB.Save(nickDBFile); err != nil {
				return nil, err
			}
			return &ssh.Permissions{
				Extensions: map[string]string{
					"nickname": nick,
				},
			}, nil
		},
	}
	config.AddHostKey(hostSigner)

	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", serverHost, serverPort))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	defer listener.Close()

	for {
		nConn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept: %v", err)
			continue
		}
		go handleConn(nConn, config, chatHub)
	}
}

func handleConn(nConn net.Conn, config *ssh.ServerConfig, chatHub *ChatHub) {
	defer nConn.Close()
	sshConn, chans, reqs, err := ssh.NewServerConn(nConn, config)
	if err != nil {
		log.Printf("SSH handshake failed: %v", err)
		return
	}
	defer sshConn.Close()
	nickname := sshConn.Permissions.Extensions["nickname"]
	log.Printf("User '%s' logged in from %s", nickname, sshConn.RemoteAddr())

	go ssh.DiscardRequests(reqs)

	for newChannel := range chans {
		if newChannel.ChannelType() != "session" {
			newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}
		channel, requests, err := newChannel.Accept()
		if err != nil {
			log.Printf("Could not accept channel: %v", err)
			continue
		}
		go handleSessionChannel(channel, requests, nickname, chatHub)
	}
}

// Helper struct for parsing "exec" request payloads.
type execPayload struct {
	Command string
}

func handleSessionChannel(channel ssh.Channel, requests <-chan *ssh.Request, nickname string, chatHub *ChatHub) {
	req, ok := <-requests
	if !ok {
		return
	}

	switch req.Type {
	case "shell":
		req.Reply(false, nil)
		io.WriteString(channel, "RoseWire relay shell not implemented.\n")
		channel.Close()

	// --------------------- FIX START ---------------------
	// Handle 'exec' requests, which is how dartssh2 sends subsystem requests.
	case "exec":
		var payload execPayload
		if err := ssh.Unmarshal(req.Payload, &payload); err != nil {
			log.Printf("Warning: malformed exec payload from %s", nickname)
			req.Reply(false, nil)
			channel.Close()
			return
		}

		if payload.Command == "subsystem:chat" {
			log.Printf("User '%s' approved for 'chat' via exec request", nickname)
			req.Reply(true, nil)
			chatHub.Join(nickname, channel)

			// Drain remaining requests to keep the session alive.
			for req := range requests {
				if req.WantReply {
					req.Reply(false, nil)
				}
			}
			return // Exit when the client disconnects.
		}
		// Fallthrough for any other exec command.

	// ---------------------- FIX END ----------------------

	case "subsystem":
		// This case is kept for compatibility with other clients (e.g., OpenSSH).
		if string(req.Payload[4:]) == "chat" {
			log.Printf("User '%s' approved for 'chat' subsystem", nickname)
			req.Reply(true, nil)
			chatHub.Join(nickname, channel)
			for req := range requests {
				if req.WantReply {
					req.Reply(false, nil)
				}
			}
			return // Exits when the client disconnects.
		}
		// Fallthrough for unknown subsystems.
		fallthrough

	default:
		log.Printf("User '%s' requested unknown request type: %s", nickname, req.Type)
		req.Reply(false, nil)
		channel.Close()
	}
}