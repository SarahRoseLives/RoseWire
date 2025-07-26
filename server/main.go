package main

import (
	"bufio"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
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

var (
	statusHTTPListen = "127.0.0.1:8080" // set to "0.0.0.0:8080" for public access
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
	// On unix, ensure the file is private
	// os.Chmod(tmp, 0600)
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

func ensureHostKey(path string) (ssh.Signer, error) {
	keyBytes, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Host key %s not found. Generate with:\n\n    ssh-keygen -t ed25519 -f %s\n\n", path, path)
		return nil, err
	}
	return ssh.ParsePrivateKey(keyBytes)
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

	// Start the status web server at web root ("/")
	statusSvc := NewStatusService(chatHub, statusHTTPListen)
	go func() {
		log.Printf("Status web server listening at http://%s/", statusHTTPListen)
		http.Handle("/", statusSvc)
		http.Handle("/api/status", statusSvc)
		http.ListenAndServe(statusHTTPListen, nil)
	}()

	config := &ssh.ServerConfig{
		PublicKeyCallback: func(meta ssh.ConnMetadata, pubKey ssh.PublicKey) (*ssh.Permissions, error) {
			nick := meta.User()
			if nick == "" {
				return nil, fmt.Errorf("nickname missing")
			}
			err := nickDB.Register(nick, pubKey)
			if err != nil {
				return nil, err
			}
			if err := nickDB.Save(nickDBFile); err != nil {
				// Log the save error but don't fail the login
				log.Printf("Error saving nick DB: %v", err)
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
		go handleSessionRequests(channel, requests, nickname, chatHub)
	}
}

// Helper struct for parsing "exec" request payloads.
type execPayload struct {
	Command string
}

func handleSessionRequests(channel ssh.Channel, requests <-chan *ssh.Request, nickname string, chatHub *ChatHub) {
	// A session can have multiple requests. We only care about the first one
	// that establishes the subsystem/shell.
	for req := range requests {
		isChatSubsystem := false
		switch req.Type {
		case "exec":
			var payload execPayload
			ssh.Unmarshal(req.Payload, &payload)
			// dartssh2 sends 'execute("subsystem:chat")' which becomes an exec request
			if payload.Command == "subsystem:chat" {
				isChatSubsystem = true
			}
		case "subsystem":
			// Standard SSH clients use this
			if string(req.Payload[4:]) == "chat" {
				isChatSubsystem = true
			}
		case "shell":
			// We don't support a shell, but we can reply gracefully
			req.Reply(true, nil)
			io.WriteString(channel, "RoseWire shell not implemented. Closing session.\n")
			channel.Close()
			return // End this goroutine
		}

		if isChatSubsystem {
			log.Printf("User '%s' approved for 'chat' subsystem (type: %s)", nickname, req.Type)
			req.Reply(true, nil)
			// --- THE FIX ---
			// Get the client instance from Join and wait for it to be done.
			client := chatHub.Join(nickname, channel)
			<-client.Done() // This line blocks until the client disconnects.
			// --- END FIX ---
			return // Now we can safely return.
		}

		// If it's not a request we handle, reject it.
		if req.WantReply {
			req.Reply(false, nil)
		}
	}
}