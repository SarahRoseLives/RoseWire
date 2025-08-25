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

type ChatHub struct {
	mu             sync.Mutex
	clients        map[string]*ChatClient
	fileRegistry   *FileRegistry
	transfers      map[string]*TransferInfo
	totalTransfers int
	config         *Config
	s2sClient      *S2SClient
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

func NewChatHub(registry *FileRegistry, config *Config, s2sClient *S2SClient) *ChatHub {
	return &ChatHub{
		clients:      make(map[string]*ChatClient),
		fileRegistry: registry,
		transfers:    make(map[string]*TransferInfo),
		config:       config,
		s2sClient:    s2sClient,
	}
}

func (hub *ChatHub) federateActivity(activity Activity) {
	if hub.s2sClient == nil || len(hub.config.Peers) == 0 {
		return
	}

	log.Printf("Federating activity type '%s' from '%s' to %d peers.", activity.Type, activity.Actor, len(hub.config.Peers))
	for _, peer := range hub.config.Peers {
		go func(peerDomain string) {
			err := hub.s2sClient.PushActivity(peerDomain, activity)
			if err != nil {
				log.Printf("Failed to federate to peer %s: %v", peerDomain, err)
			}
		}(peer)
	}
}

func (c *ChatClient) handleMessage(msg InboundMessage) {
	switch msg.Type {
	case "share":
		var p SharePayload
		if err := json.Unmarshal(msg.Payload, &p); err == nil {
			c.fileRegistry.UpdateUserFiles(c.federatedName, p.Files)
			shareObject, _ := json.Marshal(ShareActivityObject{Files: p.Files})
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
				localResults := c.fileRegistry.Search(p.Query)
				if len(localResults) > 0 {
					resultsChan <- localResults
				}
			}()

			for _, peer := range c.hub.config.Peers {
				wg.Add(1)
				go func(peerDomain string) {
					defer wg.Done()
					peerResults, err := c.hub.s2sClient.SearchPeer(peerDomain, p.Query)
					if err != nil {
						log.Printf("Error searching peer %s: %v", peerDomain, err)
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
		totalTransfers := c.hub.totalTransfers
		c.hub.mu.Unlock()
		stats := NetworkStatsPayload{
			Users:           users,
			RelayServers:    1,
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

	case "upload_data":
		var p UploadDataPayload
		if err := json.Unmarshal(msg.Payload, &p); err == nil {
			log.Printf("handleMessage: got 'upload_data' from '%s' for transfer %s", c.federatedName, p.TransferID)
			c.relayTransferMessage("upload_data", p, p.TransferID)
		}

	case "upload_done":
		var p UploadDonePayload
		if err := json.Unmarshal(msg.Payload, &p); err == nil {
			log.Printf("handleMessage: got 'upload_done' from '%s' for transfer %s", c.federatedName, p.TransferID)
			c.relayTransferMessage("upload_done", p, p.TransferID)
			c.hub.mu.Lock()
			delete(c.hub.transfers, p.TransferID)
			c.hub.totalTransfers++
			c.hub.mu.Unlock()
		}

	case "upload_error":
		var p UploadErrorPayload
		if err := json.Unmarshal(msg.Payload, &p); err == nil {
			log.Printf("handleMessage: got 'upload_error' from '%s' for transfer %s: %s", c.federatedName, p.TransferID, p.Message)
			c.relayTransferMessage("transfer_error", TransferErrorPayload(p), p.TransferID)
			c.hub.mu.Lock()
			delete(c.hub.transfers, p.TransferID)
			c.hub.mu.Unlock()
		}

	default:
		log.Printf("Unknown message type '%s' from %s", msg.Type, c.federatedName)
	}
}

// A helper function to extract the domain from a federated name.
func domainFromFederatedName(name string) (string, bool) {
	parts := strings.Split(name, "@")
	if len(parts) == 3 {
		return parts[2], true
	}
	return "", false
}

// FIX: This entire method was missing and has now been restored.
func (c *ChatClient) initiateFileTransfer(filename, peer string) {
	if peer == c.federatedName {
		c.send("transfer_error", TransferErrorPayload{Message: "You cannot download your own file."})
		return
	}

	fileInfo, found := c.fileRegistry.FindFile(filename, peer)
	if !found {
		c.send("transfer_error", TransferErrorPayload{Message: fmt.Sprintf("File not found or peer '%s' does not own it.", peer)})
		return
	}

	isRemote := !strings.HasSuffix(peer, "@"+c.hub.config.Domain)

	if isRemote {
		// --- FEDERATED TRANSFER ---
		peerDomain, ok := domainFromFederatedName(peer)
		if !ok {
			c.send("transfer_error", TransferErrorPayload{Message: "Invalid peer address format."})
			return
		}

		transferID, err := generateTransferID()
		if err != nil {
			log.Printf("Failed to generate transfer ID: %v", err)
			c.send("transfer_error", TransferErrorPayload{Message: "Server error creating transfer."})
			return
		}

		// 1. Tell the local client the transfer is starting.
		c.send("transfer_start", TransferStartPayload{
			TransferID: transferID,
			FileName:   filename,
			Size:       fileInfo.Size,
			FromUser:   peer,
		})
		log.Printf("Federated Transfer %s: Notified local client %s to start download.", transferID, c.federatedName)

		// 2. Make an S2S request to the peer's server to start the upload.
		go func() {
			s2sReq := S2STransferRequest{
				TransferID:    transferID,
				FileName:      filename,
				FileOwner:     peer,
				RequesterPeer: c.federatedName,
			}
			err := c.hub.s2sClient.RequestTransfer(peerDomain, s2sReq)
			if err != nil {
				log.Printf("Federated Transfer %s: S2S request to %s failed: %v", transferID, peerDomain, err)
				// Inform the client that the federated request failed.
				c.send("transfer_error", TransferErrorPayload{
					TransferID: transferID,
					Message:    fmt.Sprintf("Peer server %s rejected the transfer.", peerDomain),
				})
			}
		}()

	} else {
		// --- LOCAL TRANSFER (Existing logic) ---
		transferID, err := generateTransferID()
		if err != nil {
			log.Printf("Failed to generate transfer ID: %v", err)
			c.send("transfer_error", TransferErrorPayload{Message: "Server error creating transfer."})
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

		log.Printf("Local Transfer %s initiated: %s wants '%s' from %s", transferID, c.federatedName, filename, peer)

		c.send("transfer_start", TransferStartPayload{
			TransferID: transferID,
			FileName:   filename,
			Size:       fileInfo.Size,
			FromUser:   peer,
		})
		c.hub.unicast("upload_request", UploadRequestPayload{
			TransferID: transferID,
			FileName:   filename,
		}, peer)
	}
}

// --- The rest of chat.go remains unchanged ---
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