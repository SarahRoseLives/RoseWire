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
	"time"

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

// DataStreamManager handles pairing data channels for parallel transfers.
type DataStreamManager struct {
	mu      sync.Mutex
	pending map[string]ssh.Channel // Key: "transferID:streamIndex", Value: the first channel that connected
}

// NewDataStreamManager creates a new manager instance.
func NewDataStreamManager() *DataStreamManager {
	return &DataStreamManager{
		pending: make(map[string]ssh.Channel),
	}
}

// pipeStreams bi-directionally copies data between two channels using a deadlock-safe pattern.
func pipeStreams(c1, c2 ssh.Channel) {
	var once sync.Once
	// The close function will be called exactly once by the first goroutine to finish.
	closeFunc := func() {
		c1.Close()
		c2.Close()
		// Removed RemoteAddr as it's not available on ssh.Channel
		log.Printf("Finished piping streams.")
	}

	// Copy from c1 to c2
	go func() {
		io.Copy(c1, c2)
		once.Do(closeFunc)
	}()

	// Copy from c2 to c1
	go func() {
		io.Copy(c2, c1)
		once.Do(closeFunc)
	}()
}

// Pair finds the peer for the given key and pipes them together.
// If the peer is not found, it stores newChan and waits.
func (dsm *DataStreamManager) Pair(key string, newChan ssh.Channel) {
	dsm.mu.Lock()
	peerChan, ok := dsm.pending[key]
	if ok {
		// Peer was waiting. Pair them and remove from map.
		delete(dsm.pending, key)
		dsm.mu.Unlock()

		log.Printf("Pairing streams for key %s", key)
		go pipeStreams(newChan, peerChan)
		return
	}

	// We are the first. Add to map and wait for peer.
	dsm.pending[key] = newChan
	dsm.mu.Unlock()
	log.Printf("Stream for key %s is pending a peer", key)

	// Add a timeout to prevent dangling channels.
	// The newChan.Context() method does not exist, so we rely only on the timer.
	// If a client disconnects, this entry will leak for 30 seconds before being cleaned up.
	go func() {
		<-time.After(30 * time.Second) // 30 second timeout to connect
		dsm.mu.Lock()
		// Check if we are still pending after the timeout
		if ch, stillPending := dsm.pending[key]; stillPending && ch == newChan {
			log.Printf("Timed out waiting for peer for key %s. Closing channel.", key)
			delete(dsm.pending, key)
			newChan.Close()
		}
		dsm.mu.Unlock()
	}()
}

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
	dataManager := NewDataStreamManager()

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
		go handleConn(nConn, config, chatHub, dataManager)
	}
}

func handleConn(nConn net.Conn, config *ssh.ServerConfig, chatHub *ChatHub, dataManager *DataStreamManager) {
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
		go handleSessionRequests(channel, requests, nickname, chatHub, dataManager)
	}
}

type execPayload struct {
	Command string
}

func handleSessionRequests(channel ssh.Channel, requests <-chan *ssh.Request, nickname string, chatHub *ChatHub, dataManager *DataStreamManager) {
	for req := range requests {
		isChatSubsystem := false
		isDataSubsystem := false
		var dataKey string

		switch req.Type {
		case "exec":
			var payload execPayload
			ssh.Unmarshal(req.Payload, &payload)
			if payload.Command == "subsystem:chat" {
				isChatSubsystem = true
			} else if strings.HasPrefix(payload.Command, "subsystem:data-transfer:") {
				subsystem := strings.TrimPrefix(payload.Command, "subsystem:")
				parts := strings.Split(subsystem, ":")
				if len(parts) == 3 && parts[0] == "data-transfer" {
					isDataSubsystem = true
					dataKey = fmt.Sprintf("%s:%s", parts[1], parts[2]) // transferID:streamIndex
				}
			}
		case "subsystem":
			subsystem := string(req.Payload[4:])
			if subsystem == "chat" {
				isChatSubsystem = true
			} else if strings.HasPrefix(subsystem, "data-transfer:") {
				parts := strings.Split(subsystem, ":")
				if len(parts) == 3 && parts[0] == "data-transfer" {
					isDataSubsystem = true
					dataKey = fmt.Sprintf("%s:%s", parts[1], parts[2]) // transferID:streamIndex
				}
			}
		case "shell":
			req.Reply(true, nil)
			io.WriteString(channel, "RoseWire shell not implemented. Closing session.\n")
			channel.Close()
			return
		}

		if isChatSubsystem {
			log.Printf("User '%s' approved for 'chat' subsystem (type: %s)", nickname, req.Type)
			req.Reply(true, nil)
			client := chatHub.Join(nickname, channel)
			<-client.Done()
			return
		}

		if isDataSubsystem {
			log.Printf("User '%s' approved for data subsystem on key '%s'", nickname, dataKey)
			req.Reply(true, nil)
			dataManager.Pair(dataKey, channel)
			return
		}

		if req.WantReply {
			req.Reply(false, nil)
		}
	}
}