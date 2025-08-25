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
	hostKeyFile = "server_ed25519"
	nickDBFile  = "nicks.db"
	configFile  = "config.json"
	// Default listen addresses are used if not specified in the config.
	defaultSshListenAddr  = "0.0.0.0:2222"
	defaultHttpListenAddr = "0.0.0.0:8080"
)

// DataStreamManager remains the same...
type DataStreamManager struct {
	mu      sync.Mutex
	pending map[string]ssh.Channel
}

func NewDataStreamManager() *DataStreamManager {
	return &DataStreamManager{
		pending: make(map[string]ssh.Channel),
	}
}
func pipeStreams(c1, c2 ssh.Channel) {
	var once sync.Once
	closeFunc := func() {
		c1.Close()
		c2.Close()
		log.Printf("Finished piping streams.")
	}
	go func() {
		io.Copy(c1, c2)
		once.Do(closeFunc)
	}()
	go func() {
		io.Copy(c2, c1)
		once.Do(closeFunc)
	}()
}
func (dsm *DataStreamManager) Pair(key string, newChan ssh.Channel) {
	dsm.mu.Lock()
	peerChan, ok := dsm.pending[key]
	if ok {
		delete(dsm.pending, key)
		dsm.mu.Unlock()
		log.Printf("Pairing streams for key %s", key)
		go pipeStreams(newChan, peerChan)
		return
	}
	dsm.pending[key] = newChan
	dsm.mu.Unlock()
	log.Printf("Stream for key %s is pending a peer", key)

	go func() {
		<-time.After(30 * time.Second)
		dsm.mu.Lock()
		if ch, stillPending := dsm.pending[key]; stillPending && ch == newChan {
			log.Printf("Timed out waiting for peer for key %s. Closing channel.", key)
			delete(dsm.pending, key)
			newChan.Close()
		}
		dsm.mu.Unlock()
	}()
}

// NickDB remains the same...
type NickDB struct {
	sync.Mutex
	NickToKey map[string]string
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
		if len(parts) == 2 {
			db.NickToKey[parts[0]] = parts[1]
		}
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

func startHttpServer(listenAddr string, cfg *Config, nickDB *NickDB, chatHub *ChatHub) {
	statusSvc := NewStatusService(chatHub, listenAddr)
	webfingerHandler := &WebFingerHandler{Cfg: cfg, NickDB: nickDB}
	s2sHandler := NewS2SHandler(cfg, chatHub)

	mux := http.NewServeMux()

	mux.Handle("/", statusSvc)
	mux.Handle("/api/status", statusSvc)
	mux.Handle("/.well-known/webfinger", webfingerHandler)

	s2sRouter := http.NewServeMux()
	s2sRouter.HandleFunc("/api/s2s/inbox", s2sHandler.Inbox)
	mux.Handle("/api/s2s/", s2sHandler.authMiddleware(s2sRouter))

	log.Printf("HTTP services (Status, WebFinger, S2S) listening at http://%s/", listenAddr)
	if err := http.ListenAndServe(listenAddr, mux); err != nil {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}
}

func main() {
	fmt.Printf("Starting RoseWire server...\n")

	cfg, err := LoadConfig(configFile)
	if err != nil {
		log.Fatalf("Failed to load %s: %v. Please create it.", configFile, err)
	}
	log.Printf("Server domain configured as: %s", cfg.Domain)

	hostSigner, err := ensureHostKey(hostKeyFile)
	if err != nil {
		log.Fatalf("Failed to load host key: %v", err)
	}

	nickDB, err := LoadNickDB(nickDBFile)
	if err != nil {
		log.Fatalf("Failed to load nick DB: %v", err)
	}

	fileRegistry := NewFileRegistry()
	s2sClient := NewS2SClient(cfg.SharedSecret)
	chatHub := NewChatHub(fileRegistry, cfg, s2sClient)
	dataManager := NewDataStreamManager()

	// Use listen addresses from config, or defaults if not provided.
	httpAddr := cfg.HttpListenAddr
	if httpAddr == "" {
		httpAddr = defaultHttpListenAddr
	}
	go startHttpServer(httpAddr, cfg, nickDB, chatHub)

	sshAddr := cfg.SshListenAddr
	if sshAddr == "" {
		sshAddr = defaultSshListenAddr
	}

	sshConfig := &ssh.ServerConfig{
		PublicKeyCallback: func(meta ssh.ConnMetadata, pubKey ssh.PublicKey) (*ssh.Permissions, error) {
			nick := meta.User()
			if nick == "" || strings.Contains(nick, "@") {
				return nil, fmt.Errorf("invalid nickname format; must not be empty or contain '@'")
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
	sshConfig.AddHostKey(hostSigner)

	// FIX: Use the configured or default SSH address.
	listener, err := net.Listen("tcp", sshAddr)
	if err != nil {
		log.Fatalf("Failed to listen for SSH on %s: %v", sshAddr, err)
	}
	defer listener.Close()
	log.Printf("SSH server listening on %s", sshAddr)

	for {
		nConn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			continue
		}
		go handleConn(nConn, sshConfig, chatHub, dataManager)
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
					dataKey = fmt.Sprintf("%s:%s", parts[1], parts[2])
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