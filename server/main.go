// SERVER/main.go
package main

import (
	"bufio"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log"
	mrand "math/rand"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"golang.org/x/crypto/ssh"
)

const (
	hostKeyFile           = "server_ed25519"
	instanceKeyFile       = "instance_key.pem"
	nickDBFile            = "nicks.db"
	configFile            = "config.json"
	defaultSshListenAddr  = "0.0.0.0:2222"
	defaultHttpListenAddr = "0.0.0.0:8080"
	serverVersion         = "1.0.3"
	gossipFanout          = 3 // The number of peers to gossip with in each cycle
)

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
func (dsm *DataStreamManager) GetAndRemovePending(key string) (ssh.Channel, bool) {
	dsm.mu.Lock()
	defer dsm.mu.Unlock()
	ch, ok := dsm.pending[key]
	if ok {
		delete(dsm.pending, key)
	}
	return ch, ok
}

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
func ensureInstanceKey(path string) (crypto.Signer, error) {
	keyBytes, err := os.ReadFile(path)
	if err == nil {
		block, _ := pem.Decode(keyBytes)
		if block == nil {
			return nil, fmt.Errorf("failed to parse PEM block containing the private key")
		}
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
		return key.(crypto.Signer), nil
	}

	log.Printf("Instance key %s not found, generating a new one...", path)
	_, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}

	privBytes, err := x509.MarshalPKCS8PrivateKey(privKey)
	if err != nil {
		return nil, err
	}

	pemBlock := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privBytes,
	}

	if err := os.WriteFile(path, pem.EncodeToMemory(pemBlock), 0600); err != nil {
		return nil, fmt.Errorf("failed to write new instance key: %w", err)
	}

	return privKey, nil
}

func startFileRegistryCleanup(registry *FileRegistry) {
	log.Println("Starting stale file registry cleanup routine...")
	ticker := time.NewTicker(5 * time.Minute)
	go func() {
		for range ticker.C {
			registry.CleanupStaleEntries(15 * time.Minute)
		}
	}()
}

func startGossipProtocol(hub *ChatHub) {
	log.Println("Gossip protocol starting...")
	ticker := time.NewTicker(5 * time.Minute)
	go gossip(hub)

	for range ticker.C {
		go gossip(hub)
	}
}

func gossip(hub *ChatHub) {
	hub.mu.Lock()
	hub.adminConfig.mu.RLock()

	allowedPeers := []string{}
	blockedMap := make(map[string]bool)
	for _, p := range hub.adminConfig.BlockedPeers {
		blockedMap[p] = true
	}
	for _, p := range hub.config.Peers {
		domain := strings.Split(p, ":")[0]
		if !blockedMap[domain] {
			allowedPeers = append(allowedPeers, p)
		}
	}
	hub.adminConfig.mu.RUnlock()

	if len(allowedPeers) == 0 {
		hub.mu.Unlock()
		log.Println("Gossip: No non-blocked peers to gossip with.")
		return
	}
	hub.mu.Unlock() // Unlock early as we don't need the hub lock for S2S calls

	// --- START OF IMPROVED GOSSIP LOGIC ---

	// 1. Shuffle the list of peers to ensure we talk to different ones each time.
	mrand.Shuffle(len(allowedPeers), func(i, j int) {
		allowedPeers[i], allowedPeers[j] = allowedPeers[j], allowedPeers[i]
	})

	// 2. Determine how many peers to contact in this cycle.
	numToContact := gossipFanout
	if len(allowedPeers) < gossipFanout {
		numToContact = len(allowedPeers)
	}

	// 3. Contact the first few peers from the shuffled list.
	for i := 0; i < numToContact; i++ {
		targetPeer := allowedPeers[i]

		// Use a goroutine so we can contact multiple peers in parallel.
		go func(peerAddress string) {
			log.Printf("Gossip: Gossiping with peer %s", peerAddress)
			discoveredPeers, err := hub.s2sClient.FetchPeers(peerAddress)
			if err != nil {
				log.Printf("Gossip: Failed to fetch peers from %s: %v", peerAddress, err)
				return
			}

			hub.mu.Lock()
			defer hub.mu.Unlock()
			hub.adminConfig.mu.RLock()
			defer hub.adminConfig.mu.RUnlock()

			listenAddrParts := strings.Split(hub.config.HttpListenAddr, ":")
			listenPort := "8080"
			if len(listenAddrParts) > 1 {
				listenPort = listenAddrParts[len(listenAddrParts)-1]
			}
			selfAddress := fmt.Sprintf("%s:%s", hub.config.Domain, listenPort)

			existingPeers := make(map[string]bool)
			for _, p := range hub.config.Peers {
				existingPeers[p] = true
			}

			newlyAdded := 0
			for _, discoveredPeer := range discoveredPeers {
				discoveredDomain := strings.Split(discoveredPeer, ":")[0]
				if discoveredPeer != selfAddress && !existingPeers[discoveredPeer] && !blockedMap[discoveredDomain] {
					hub.config.Peers = append(hub.config.Peers, discoveredPeer)
					existingPeers[discoveredPeer] = true
					newlyAdded++
				}
			}
			if newlyAdded > 0 {
				log.Printf("Gossip: Discovered and added %d new peer(s) from %s", newlyAdded, peerAddress)
			}
		}(targetPeer)
	}
	// --- END OF IMPROVED GOSSIP LOGIC ---
}

func startHttpServer(listenAddr string, cfg *Config, nickDB *NickDB, chatHub *ChatHub, dataManager *DataStreamManager, instanceSigner crypto.Signer, adminCfg *AdminConfig, store *sessions.CookieStore) {
	statusSvc := NewStatusService(chatHub, listenAddr)
	webfingerHandler := &WebFingerHandler{Cfg: cfg, NickDB: nickDB}
	s2sHandler := NewS2SHandler(cfg, chatHub, dataManager, chatHub.s2sClient, adminCfg)
	adminHandler := NewAdminHandler(store, adminCfg, chatHub)

	router := mux.NewRouter()

	router.HandleFunc("/actor", func(w http.ResponseWriter, r *http.Request) {
		pubBytes, err := x509.MarshalPKIXPublicKey(instanceSigner.Public())
		if err != nil {
			http.Error(w, "Failed to marshal public key", http.StatusInternalServerError)
			return
		}
		pemBlock := &pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: pubBytes,
		}
		actor := InstanceActor{
			ID:        fmt.Sprintf("https://%s/actor", cfg.Domain),
			Type:      "Service",
			PublicKey: string(pem.EncodeToMemory(pemBlock)),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(actor)
	}).Methods("GET")

	router.Handle("/", statusSvc)
	router.Handle("/api/status", statusSvc)
	router.Handle("/.well-known/webfinger", webfingerHandler)
	router.HandleFunc("/api/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"version": serverVersion})
	}).Methods("GET")

	router.HandleFunc("/admin", adminHandler.ServeAdminPanel).Methods("GET")
	router.HandleFunc("/admin/login", adminHandler.HandleLogin).Methods("POST")
	router.HandleFunc("/admin/logout", adminHandler.HandleLogout).Methods("POST")

	adminAPIRouter := router.PathPrefix("/api/admin").Subrouter()
	adminAPIRouter.Use(adminHandler.AuthMiddleware)
	adminAPIRouter.HandleFunc("/config", adminHandler.HandleGetConfig).Methods("GET")
	adminAPIRouter.HandleFunc("/update", adminHandler.HandleUpdateConfig).Methods("POST")

	s2sRouter := router.PathPrefix("/api/s2s").Subrouter()
	s2sRouter.Use(s2sHandler.authMiddleware)
	s2sRouter.HandleFunc("/inbox", s2sHandler.Inbox).Methods("POST")
	s2sRouter.HandleFunc("/search", s2sHandler.Search).Methods("GET")
	s2sRouter.HandleFunc("/transfers", s2sHandler.InitiateTransfer).Methods("POST")
	s2sRouter.HandleFunc("/data/{tid}/{idx}", s2sHandler.RelayData).Methods("POST")
	s2sRouter.HandleFunc("/peers", s2sHandler.Peers).Methods("GET")

	// Start the server using the certificate files specified in the config
	log.Printf("HTTPS services listening at https://%s %s/", cfg.Domain, listenAddr)
	if err := http.ListenAndServeTLS(listenAddr, cfg.CertFile, cfg.KeyFile, router); err != nil {
		log.Fatalf("Failed to start HTTPS server: %v", err)
	}
}

func main() {
	mrand.Seed(time.Now().UnixNano())
	fmt.Printf("Starting RoseWire server...\n")

	cfg, err := LoadConfig(configFile)
	if err != nil {
		log.Fatalf("Failed to load %s: %v. Please create it.", configFile, err)
	}
	if cfg.Domain == "" {
		log.Fatalf("The 'domain' field in %s cannot be empty.", configFile)
	}
	// Validate that certificate paths are provided
	if cfg.CertFile == "" || cfg.KeyFile == "" {
		log.Fatalf("The 'cert_file' and 'key_file' fields must be set in %s for manual TLS.", configFile)
	}
	log.Printf("Server domain configured as: %s", cfg.Domain)

	adminCfg, err := LoadAdminConfig(adminConfigFile)
	if err != nil {
		log.Fatalf("Failed to load admin config: %v", err)
	}

	if adminCfg.PasswordHash == "" {
		if err := SetupAdminPassword(adminCfg); err != nil {
			log.Fatalf("Admin password setup failed: %v", err)
		}
	}

	nickDB, err := LoadNickDB(nickDBFile)
	if err != nil {
		log.Fatalf("Failed to load nick DB: %v", err)
	}

	// Check for SYSTEM user registration on startup.
	nickDB.Lock()
	_, systemUserExists := nickDB.NickToKey["SYSTEM"]
	nickDB.Unlock()
	if !systemUserExists {
		fmt.Println()
		log.Println("*****************************************************************")
		log.Println("* *")
		log.Println("* ACTION REQUIRED                        *")
		log.Println("* *")
		log.Println("* The SYSTEM user has not been registered. This user is the     *")
		log.Println("* administrator for this RoseWire instance.                     *")
		log.Println("* *")
		log.Println("* To register the admin, please log in from another terminal:   *")
		log.Println("* *")
		log.Printf("* ssh SYSTEM@%s                                        *", cfg.Domain)
		log.Println("* *")
		log.Println("* The first public key used will be permanently associated      *")
		log.Println("* with the SYSTEM account. The server will continue to run.     *")
		log.Println("* *")
		log.Println("*****************************************************************")
		fmt.Println()
	}

	instanceSigner, err := ensureInstanceKey(instanceKeyFile)
	if err != nil {
		log.Fatalf("Failed to load or generate instance key: %v", err)
	}

	hostSigner, err := ensureHostKey(hostKeyFile)
	if err != nil {
		log.Fatalf("Failed to load host key: %v", err)
	}

	fileRegistry := NewFileRegistry()
	s2sClient := NewS2SClient(instanceSigner, cfg.Domain)
	chatHub := NewChatHub(fileRegistry, cfg, s2sClient, adminCfg)
	dataManager := NewDataStreamManager()

	go startFileRegistryCleanup(fileRegistry)

	sessionKey := make([]byte, 32)
	_, err = rand.Read(sessionKey)
	if err != nil {
		log.Fatalf("Could not generate session key: %v", err)
	}
	sessionStore := sessions.NewCookieStore(sessionKey)

	go startGossipProtocol(chatHub)

	httpAddr := cfg.HttpListenAddr
	if httpAddr == "" {
		httpAddr = defaultHttpListenAddr
	}
	go startHttpServer(httpAddr, cfg, nickDB, chatHub, dataManager, instanceSigner, adminCfg, sessionStore)

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

			adminCfg.mu.RLock()
			isBanned := false
			// The SYSTEM user can never be banned.
			if nick != "SYSTEM" {
				for _, bannedUser := range adminCfg.BannedUsers {
					if bannedUser == nick {
						isBanned = true
						break
					}
				}
			}
			adminCfg.mu.RUnlock()

			if isBanned {
				log.Printf("Connection rejected for banned user: %s", nick)
				return nil, errors.New("this account has been suspended")
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
		go handleConn(nConn, sshConfig, chatHub, dataManager, s2sClient)
	}
}
func handleConn(nConn net.Conn, config *ssh.ServerConfig, chatHub *ChatHub, dataManager *DataStreamManager, s2sClient *S2SClient) {
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
		go handleSessionRequests(channel, requests, nickname, chatHub, dataManager, s2sClient)
	}
}

type execPayload struct {
	Command string
}

func handleSessionRequests(channel ssh.Channel, requests <-chan *ssh.Request, nickname string, chatHub *ChatHub, dataManager *DataStreamManager, s2sClient *S2SClient) {
	for req := range requests {
		isChatSubsystem := false
		isDataSubsystem := false
		var transferID, streamIndex string

		var subsystem string
		switch req.Type {
		case "exec":
			var payload execPayload
			ssh.Unmarshal(req.Payload, &payload)
			subsystem = payload.Command
		case "subsystem":
			subsystem = string(req.Payload[4:])
		}

		if subsystem == "chat" || subsystem == "subsystem:chat" {
			isChatSubsystem = true
		} else if strings.HasPrefix(subsystem, "data-transfer:") || strings.HasPrefix(subsystem, "subsystem:data-transfer:") {
			trimmed := strings.TrimPrefix(subsystem, "subsystem:")
			parts := strings.Split(trimmed, ":")
			if len(parts) == 3 && parts[0] == "data-transfer" {
				isDataSubsystem = true
				transferID = parts[1]
				streamIndex = parts[2]
			}
		}

		if isChatSubsystem {
			log.Printf("User '%s' approved for 'chat' subsystem (type: %s)", nickname, req.Type)
			req.Reply(true, nil)
			client := chatHub.Join(nickname, channel)
			<-client.Done()
			return
		}

		if isDataSubsystem {
			log.Printf("User '%s' connected for data subsystem on %s:%s", nickname, transferID, streamIndex)
			req.Reply(true, nil)
			streamKey := fmt.Sprintf("%s:%s", transferID, streamIndex)

			if val, ok := chatHub.federatedTransfers.Load(transferID); ok {
				state := val.(*federatedTransferState)
				targetPeerAddress := state.targetDomain
				for _, p := range chatHub.config.Peers {
					if strings.HasPrefix(p, state.targetDomain) {
						targetPeerAddress = p
						break
					}
				}

				log.Printf("Forwarding federated upload stream %s to %s", streamKey, targetPeerAddress)
				go func() {
					err := s2sClient.RelayStream(targetPeerAddress, transferID, streamIndex, channel)
					if err != nil {
						log.Printf("Error relaying stream %s: %v", streamKey, err)
					}
					channel.Close()
				}()
			} else {
				dataManager.Pair(streamKey, channel)
			}
			return
		}

		if req.WantReply {
			req.Reply(false, nil)
		}
	}
}