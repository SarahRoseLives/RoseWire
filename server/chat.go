// SERVER/chat.go
package main

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

type TransferInfo struct {
	ID       string
	FileName string
	Size     int64
	FromUser string
	ToUser   string
}

// federatedTransferState stores the target domain for an upload that needs to be forwarded.
type federatedTransferState struct {
	targetDomain string
}

type ChatHub struct {
	mu                 sync.Mutex
	clients            map[string]*ChatClient
	fileRegistry       *FileRegistry
	transfers          map[string]*TransferInfo
	federatedTransfers sync.Map // transferID -> *federatedTransferState
	totalTransfers     int
	config             *Config
	s2sClient          *S2SClient
	adminConfig        *AdminConfig // Added admin config
}

type ChatClient struct {
	nickname      string
	federatedName string
	channel       ssh.Channel
	outgoing      chan []byte
	done          chan struct{}
	hub           *ChatHub
	fileRegistry  *FileRegistry
	once          sync.Once
}

func NewChatHub(registry *FileRegistry, config *Config, s2sClient *S2SClient, adminConfig *AdminConfig) *ChatHub {
	return &ChatHub{
		clients:      make(map[string]*ChatClient),
		fileRegistry: registry,
		transfers:    make(map[string]*TransferInfo),
		config:       config,
		s2sClient:    s2sClient,
		adminConfig:  adminConfig, // Initialize admin config
	}
}

func (hub *ChatHub) federateActivity(activity Activity) {
	if hub.s2sClient == nil || len(hub.config.Peers) == 0 {
		return
	}

	hub.adminConfig.mu.RLock()
	blockedPeers := make(map[string]bool)
	for _, p := range hub.adminConfig.BlockedPeers {
		blockedPeers[p] = true
	}
	hub.adminConfig.mu.RUnlock()

	log.Printf("Federating activity type '%s' from '%s' to %d peers.", activity.Type, activity.Actor, len(hub.config.Peers))
	for _, peer := range hub.config.Peers {
		peerDomain := strings.Split(peer, ":")[0]
		if blockedPeers[peerDomain] {
			log.Printf("Federation: Skipping blocked peer %s", peerDomain)
			continue
		}
		go func(peerAddress string) {
			err := hub.s2sClient.PushActivity(peerAddress, activity)
			if err != nil {
				log.Printf("Failed to federate to peer %s: %v", peerAddress, err)
			}
		}(peer)
	}
}

// Helper to send a system message back to the SYSTEM user only.
func (c *ChatClient) sendSystemMessage(text string) {
	if c.nickname != "SYSTEM" {
		return
	}
	payload := ChatBroadcastPayload{
		Timestamp: time.Now().Format("15:04"),
		Text:      text,
		IsSystem:  true,
	}
	c.send("system_broadcast", payload)
}

func (c *ChatClient) handleSystemCommand(text string) {
	parts := strings.Fields(text)
	if len(parts) < 2 {
		c.sendSystemMessage("Invalid command format. Use !kick <user|@user@domain>.")
		return
	}
	command := parts[0]
	targetUser := parts[1]

	// The SYSTEM user cannot be targeted.
	systemFederatedName := fmt.Sprintf("@system@%s", c.hub.config.Domain)
	if strings.EqualFold(targetUser, "SYSTEM") || strings.EqualFold(targetUser, systemFederatedName) {
		c.sendSystemMessage("The SYSTEM user cannot be targeted.")
		return
	}

	var federatedName string
	var targetDomain string
	var isRemote bool

	// Check if the user provided a full federated name
	if strings.HasPrefix(targetUser, "@") && strings.Count(targetUser, "@") == 2 {
		federatedName = strings.ToLower(targetUser)
		domainParts := strings.Split(federatedName, "@")
		targetDomain = domainParts[2]
	} else {
		// Assume it's a local user if no domain is provided
		federatedName = fmt.Sprintf("@%s@%s", strings.ToLower(targetUser), c.hub.config.Domain)
		targetDomain = c.hub.config.Domain
	}

	isRemote = (targetDomain != c.hub.config.Domain)
	usernameToBan := strings.Split(strings.TrimPrefix(federatedName, "@"), "@")[0]

	switch command {
	case "!kick":
		if isRemote {
			c.sendSystemMessage(fmt.Sprintf("Cannot moderate remote user %s. This action can only be performed by an administrator on %s.", federatedName, targetDomain))
			return
		}

		c.hub.mu.Lock()
		targetClient, ok := c.hub.clients[federatedName]
		c.hub.mu.Unlock()

		if !ok {
			c.sendSystemMessage(fmt.Sprintf("Local user '%s' not found or is not online.", federatedName))
			return
		}

		log.Printf("SYSTEM command: Kicking user %s", federatedName)
		c.sendSystemMessage(fmt.Sprintf("Kicking user %s...", federatedName))
		go targetClient.Close()

		kickMsg := fmt.Sprintf("User %s has been kicked by an administrator.", federatedName)
		c.hub.broadcast("system_broadcast", ChatBroadcastPayload{
			Timestamp: time.Now().Format("15:04"),
			Text:      kickMsg,
			IsSystem:  true,
		}, "")

	case "!ban":
		if isRemote {
			c.sendSystemMessage(fmt.Sprintf("Cannot ban a remote user. The ban list is local to this server. Please contact an admin on %s.", targetDomain))
			return
		}

		c.hub.adminConfig.mu.Lock()
		found := false
		for _, bannedUser := range c.hub.adminConfig.BannedUsers {
			if bannedUser == usernameToBan {
				found = true
				break
			}
		}
		if !found {
			c.hub.adminConfig.BannedUsers = append(c.hub.adminConfig.BannedUsers, usernameToBan)
			if err := c.hub.adminConfig.Save(adminConfigFile); err != nil {
				log.Printf("ERROR: Failed to save admin config after ban: %v", err)
				c.sendSystemMessage("Error: Failed to save ban list.")
				c.hub.adminConfig.mu.Unlock()
				return
			}
			c.sendSystemMessage(fmt.Sprintf("User '%s' has been added to the local ban list.", usernameToBan))
		} else {
			c.sendSystemMessage(fmt.Sprintf("User '%s' is already banned.", usernameToBan))
		}
		c.hub.adminConfig.mu.Unlock()

		c.hub.mu.Lock()
		targetClient, ok := c.hub.clients[federatedName]
		c.hub.mu.Unlock()

		if ok {
			log.Printf("SYSTEM command: Banning and kicking user %s", federatedName)
			go targetClient.Close()
		}

		banMsg := fmt.Sprintf("User %s has been banned by an administrator.", federatedName)
		c.hub.broadcast("system_broadcast", ChatBroadcastPayload{
			Timestamp: time.Now().Format("15:04"),
			Text:      banMsg,
			IsSystem:  true,
		}, "")

	default:
		c.sendSystemMessage(fmt.Sprintf("Unknown command: %s", command))
	}
}

func (c *ChatClient) handleMessage(msg InboundMessage) {
	switch msg.Type {
	case "share":
		var p SharePayload
		if err := json.Unmarshal(msg.Payload, &p); err == nil {
			c.hub.adminConfig.mu.RLock()
			blockedExts := make(map[string]bool)
			for _, ext := range c.hub.adminConfig.BlockedFileTypes {
				blockedExts[ext] = true
			}
			c.hub.adminConfig.mu.RUnlock()

			var allowedFiles []SharedFile
			for _, file := range p.Files {
				isBlocked := false
				for ext := range blockedExts {
					if strings.HasSuffix(strings.ToLower(file.Name), ext) {
						log.Printf("SHARE BLOCKED: User '%s' tried to share blocked filetype: %s", c.federatedName, file.Name)
						isBlocked = true
						break
					}
				}
				if !isBlocked {
					allowedFiles = append(allowedFiles, file)
				}
			}

			c.fileRegistry.UpdateUserFiles(c.federatedName, allowedFiles)
			shareObject, _ := json.Marshal(ShareActivityObject{Files: allowedFiles})
			activity := Activity{
				Type:   "Share",
				Actor:  c.federatedName,
				Object: shareObject,
			}
			go c.hub.federateActivity(activity)
		}

	case "search":
		var p SearchPayload
		if err := json.Unmarshal(msg.Payload, &p); err == nil {
			var wg sync.WaitGroup
			resultsChan := make(chan []SearchResult)

			wg.Add(1)
			go func() {
				defer wg.Done()
				localResults := c.fileRegistry.Search(p.Query, c.federatedName)
				if len(localResults) > 0 {
					resultsChan <- localResults
				}
			}()

			c.hub.adminConfig.mu.RLock()
			blockedPeers := make(map[string]bool)
			for _, peer := range c.hub.adminConfig.BlockedPeers {
				blockedPeers[peer] = true
			}
			c.hub.adminConfig.mu.RUnlock()

			for _, peer := range c.hub.config.Peers {
				wg.Add(1)
				go func(peerAddress string) {
					defer wg.Done()
					peerDomain := strings.Split(peerAddress, ":")[0]
					if blockedPeers[peerDomain] {
						log.Printf("SEARCH: Skipping blocked peer %s", peerDomain)
						return
					}
					peerResults, err := c.hub.s2sClient.SearchPeer(peerAddress, p.Query, c.federatedName)
					if err != nil {
						log.Printf("Error searching peer %s: %v", peerAddress, err)
						return
					}
					if len(peerResults) > 0 {
						resultsChan <- peerResults
					}
				}(peer)
			}

			go func() {
				wg.Wait()
				close(resultsChan)
			}()

			var finalResults []SearchResult
			for results := range resultsChan {
				finalResults = append(finalResults, results...)
			}

			log.Printf("Total federated search results for '%s': %d", p.Query, len(finalResults))
			c.send("search_results", SearchResultsPayload{Results: finalResults})
		}

	case "top_files":
		results := c.fileRegistry.TopFiles(50)
		c.send("search_results", SearchResultsPayload{Results: results})

	case "get_stats":
		c.hub.mu.Lock()
		var users []map[string]string
		for name := range c.hub.clients {
			users = append(users, map[string]string{"nickname": name, "status": "Online"})
		}
		activeTransfers := len(c.hub.transfers)
		c.hub.federatedTransfers.Range(func(key, value interface{}) bool {
			activeTransfers++
			return true
		})
		totalTransfers := c.hub.totalTransfers
		relayServers := 1 + len(c.hub.config.Peers)
		c.hub.mu.Unlock()

		stats := NetworkStatsPayload{
			Users:           users,
			RelayServers:    relayServers,
			TotalUsers:      len(users),
			ActiveTransfers: activeTransfers,
			TotalTransfers:  totalTransfers,
		}
		c.send("network_stats", stats)

	case "get_file":
		var p GetFilePayload
		if err := json.Unmarshal(msg.Payload, &p); err == nil {
			log.Printf("handleMessage: '%s' requested file '%s' from peer '%s'", c.federatedName, p.FileName, p.Peer)
			c.initiateFileTransfer(p.FileName, p.Peer)
		}

	case "chat_message":
		var p ChatMessagePayload
		if err := json.Unmarshal(msg.Payload, &p); err == nil {
			// Check if the message is from the SYSTEM user and is a command.
			if c.nickname == "SYSTEM" && strings.HasPrefix(p.Text, "!") {
				c.handleSystemCommand(p.Text)
				return // Command is handled, don't broadcast it as a regular message.
			}

			// If not a command, proceed as a regular chat message.
			broadcastPayload := ChatBroadcastPayload{
				Timestamp: time.Now().Format("15:04"),
				Nickname:  c.federatedName,
				Text:      p.Text,
				IsSystem:  false,
			}
			c.hub.broadcast("chat_broadcast", broadcastPayload, "")
			chatObject, _ := json.Marshal(ChatActivityObject{Content: p.Text})
			activity := Activity{
				Type:   "Create",
				Actor:  c.federatedName,
				Object: chatObject,
			}
			go c.hub.federateActivity(activity)
		}

	case "upload_done":
		var p UploadDonePayload
		if err := json.Unmarshal(msg.Payload, &p); err == nil {
			log.Printf("handleMessage: got 'upload_done' from '%s' for transfer %s", c.federatedName, p.TransferID)
			if _, isFederated := c.hub.federatedTransfers.Load(p.TransferID); !isFederated {
				c.relayTransferMessage("upload_done", p, p.TransferID)
				c.hub.mu.Lock()
				delete(c.hub.transfers, p.TransferID)
				c.hub.totalTransfers++
				c.hub.mu.Unlock()
			}
		}

	case "upload_error":
		var p UploadErrorPayload
		if err := json.Unmarshal(msg.Payload, &p); err == nil {
			log.Printf("handleMessage: got 'upload_error' from '%s' for transfer %s: %s", c.federatedName, p.TransferID, p.Message)
			if _, isFederated := c.hub.federatedTransfers.Load(p.TransferID); isFederated {
				// TODO: Federate transfer errors back to the requesting server.
				c.hub.federatedTransfers.Delete(p.TransferID)
			} else {
				c.relayTransferMessage("transfer_error", TransferErrorPayload(p), p.TransferID)
				c.hub.mu.Lock()
				delete(c.hub.transfers, p.TransferID)
				c.hub.mu.Unlock()
			}
		}

	default:
		log.Printf("Unknown message type '%s' from %s", msg.Type, c.federatedName)
	}
}

func domainFromFederatedName(name string) (string, bool) {
	parts := strings.Split(name, "@")
	if len(parts) == 3 {
		return parts[2], true
	}
	return "", false
}

func (c *ChatClient) initiateFileTransfer(filename, peer string) {
	c.hub.adminConfig.mu.RLock()
	isBlocked := false
	for _, ext := range c.hub.adminConfig.BlockedFileTypes {
		if strings.HasSuffix(strings.ToLower(filename), ext) {
			isBlocked = true
			break
		}
	}
	c.hub.adminConfig.mu.RUnlock()

	if isBlocked {
		log.Printf("DOWNLOAD BLOCKED: User '%s' tried to download blocked filetype: %s", c.federatedName, filename)
		c.send("transfer_error", TransferErrorPayload{Message: "This file type is blocked by the server administrator."})
		return
	}

	if peer == c.federatedName {
		c.send("transfer_error", TransferErrorPayload{Message: "You cannot download your own file."})
		return
	}

	fileInfo, found := c.hub.fileRegistry.FindFile(filename, peer)
	if !found {
		c.send("transfer_error", TransferErrorPayload{Message: fmt.Sprintf("File not found or peer '%s' does not own it.", peer)})
		return
	}

	isRemote := !strings.HasSuffix(peer, "@"+c.hub.config.Domain)
	transferID, err := generateTransferID()
	if err != nil {
		log.Printf("Failed to generate transfer ID: %v", err)
		c.send("transfer_error", TransferErrorPayload{Message: "Server error creating transfer."})
		return
	}

	if isRemote {
		peerDomain, ok := domainFromFederatedName(peer)
		if !ok {
			c.send("transfer_error", TransferErrorPayload{TransferID: transferID, Message: "Invalid peer address format."})
			return
		}

		c.send("transfer_start", TransferStartPayload{
			TransferID: transferID,
			FileName:   filename,
			Size:       fileInfo.Size,
			FromUser:   peer,
		})

		fullPeerAddress := peerDomain
		for _, p := range c.hub.config.Peers {
			if strings.HasPrefix(p, peerDomain) {
				fullPeerAddress = p
				break
			}
		}

		log.Printf("Federated Transfer %s: %s requesting '%s' from %s", transferID, c.federatedName, filename, peer)

		go func() {
			s2sReq := S2STransferRequest{
				TransferID:          transferID,
				FileName:            filename,
				FileOwner:           peer,
				RequesterPeer:       c.federatedName,
				RequesterPeerDomain: c.hub.config.Domain,
			}
			err := c.hub.s2sClient.RequestTransfer(fullPeerAddress, s2sReq)
			if err != nil {
				log.Printf("Federated Transfer %s: S2S request to %s failed: %v", transferID, fullPeerAddress, err)
				c.send("transfer_error", TransferErrorPayload{
					TransferID: transferID,
					Message:    fmt.Sprintf("Peer server %s rejected the transfer.", peerDomain),
				})
			}
		}()

	} else {
		log.Printf("Local Transfer %s initiated: %s wants '%s' from %s", transferID, c.federatedName, filename, peer)

		ok := c.hub.unicast("upload_request", UploadRequestPayload{
			TransferID: transferID,
			FileName:   filename,
		}, peer)

		if !ok {
			log.Printf("Local Transfer %s FAILED: Uploader %s is not online.", transferID, peer)
			c.send("transfer_error", TransferErrorPayload{
				TransferID: transferID,
				Message:    fmt.Sprintf("User '%s' is offline.", peer),
			})
			return
		}

		transfer := &TransferInfo{
			ID:       transferID,
			FileName: filename,
			Size:     fileInfo.Size,
			FromUser: peer,
			ToUser:   c.federatedName,
		}
		c.hub.mu.Lock()
		c.hub.transfers[transferID] = transfer
		c.hub.mu.Unlock()

		c.send("transfer_start", TransferStartPayload{
			TransferID: transferID,
			FileName:   filename,
			Size:       fileInfo.Size,
			FromUser:   peer,
		})
	}
}

func generateTransferID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func (hub *ChatHub) Join(nickname string, channel ssh.Channel) *ChatClient {
	federatedName := fmt.Sprintf("@%s@%s", nickname, hub.config.Domain)
	client := &ChatClient{
		nickname:      nickname,
		federatedName: federatedName,
		channel:       channel,
		outgoing:      make(chan []byte, 16),
		done:          make(chan struct{}),
		hub:           hub,
		fileRegistry:  hub.fileRegistry,
	}
	hub.mu.Lock()
	hub.clients[federatedName] = client
	hub.mu.Unlock()

	// Unicast a welcome message with the user's confirmed identity.
	client.send("welcome", map[string]string{"identity": federatedName})

	go client.readLoop()
	go client.writeLoop()
	joinMsg := ChatBroadcastPayload{
		Timestamp: time.Now().Format("15:04"),
		Text:      fmt.Sprintf("%s joined the chat.", federatedName),
		IsSystem:  true,
	}
	hub.broadcast("system_broadcast", joinMsg, "")
	return client
}

func (c *ChatClient) Done() <-chan struct{} {
	return c.done
}

func (hub *ChatHub) broadcast(msgType string, payload interface{}, from string) {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	msg, err := json.Marshal(OutboundMessage{Type: msgType, Payload: payload})
	if err != nil {
		log.Printf("Error marshalling broadcast message: %v", err)
		return
	}
	for name, client := range hub.clients {
		if name == from {
			continue
		}
		select {
		case client.outgoing <- msg:
		default:
		}
	}
}

func (hub *ChatHub) unicast(msgType string, payload interface{}, to string) bool {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	client, ok := hub.clients[to]
	if !ok {
		log.Printf("unicast: target client '%s' not found for message type '%s'", to, msgType)
		return false
	}
	msg, err := json.Marshal(OutboundMessage{Type: msgType, Payload: payload})
	if err != nil {
		log.Printf("Error marshalling unicast message: %v", err)
		return false
	}
	select {
	case client.outgoing <- msg:
		log.Printf("unicast: sent message type '%s' to '%s'", msgType, to)
		return true
	default:
		log.Printf("unicast: outgoing channel full for '%s', dropped message type '%s'", to, msgType)
		return false
	}
}

func (hub *ChatHub) part(federatedName string) {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	delete(hub.clients, federatedName)
}

func (c *ChatClient) send(msgType string, payload interface{}) {
	msg, err := json.Marshal(OutboundMessage{Type: msgType, Payload: payload})
	if err != nil {
		log.Printf("Error marshalling message for %s: %v", c.federatedName, err)
		return
	}
	c.outgoing <- msg
}

func (c *ChatClient) readLoop() {
	defer c.Close()
	scanner := bufio.NewScanner(c.channel)
	for scanner.Scan() {
		var msg InboundMessage
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		if err := json.Unmarshal(line, &msg); err != nil {
			log.Printf("Error unmarshalling message from %s: %v", c.federatedName, err)
			continue
		}
		log.Printf("readLoop: received message type '%s' from %s", msg.Type, c.federatedName)
		c.handleMessage(msg)
	}
}

func (c *ChatClient) Close() {
	c.once.Do(func() {
		c.fileRegistry.RemoveUser(c.federatedName)

		// --- START FIX ---
		// Federate an empty share list to notify peers that this user's files are gone.
		log.Printf("Federating unshare for %s", c.federatedName)
		shareObject, _ := json.Marshal(ShareActivityObject{Files: []SharedFile{}}) // Empty file list
		activity := Activity{
			Type:   "Share",
			Actor:  c.federatedName,
			Object: shareObject,
		}
		go c.hub.federateActivity(activity)
		// --- END FIX ---

		c.hub.part(c.federatedName)
		close(c.done)
		c.channel.Close()
		log.Printf("%s left chat", c.federatedName)
		leaveMsg := ChatBroadcastPayload{
			Timestamp: time.Now().Format("15:04"),
			Text:      fmt.Sprintf("%s left the chat.", c.federatedName),
			IsSystem:  true,
		}
		c.hub.broadcast("system_broadcast", leaveMsg, "")
	})
}

func (c *ChatClient) writeLoop() {
	for {
		select {
		case msg := <-c.outgoing:
			if !strings.HasSuffix(string(msg), "\n") {
				msg = append(msg, '\n')
			}
			c.channel.Write(msg)
		case <-c.done:
			return
		}
	}
}

func (c *ChatClient) relayTransferMessage(msgType string, payload interface{}, transferID string) {
	c.hub.mu.Lock()
	transfer, ok := c.hub.transfers[transferID]
	c.hub.mu.Unlock()
	if !ok {
		log.Printf("SECURITY: Received data for unknown transfer ID '%s' from %s", transferID, c.federatedName)
		return
	}
	if transfer.FromUser != c.federatedName {
		log.Printf("SECURITY: Mismatched user for transfer ID '%s'. Expected %s, got %s", transferID, transfer.FromUser, c.federatedName)
		return
	}
	okSend := c.hub.unicast(msgType, payload, transfer.ToUser)
	log.Printf("relayTransferMessage: relayed '%s' for transfer %s from '%s' to '%s' (ok=%v)", msgType, transferID, c.federatedName, transfer.ToUser, okSend)
}