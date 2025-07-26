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

// TransferInfo now represents the server's state for an active transfer.
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
	transfers      map[string]*TransferInfo // Keyed by unique transfer ID
	totalTransfers int                      // <-- Add this field for total transfer count
}

type ChatClient struct {
	nickname     string
	channel      ssh.Channel
	outgoing     chan []byte // Changed to byte slice for JSON
	done         chan struct{}
	hub          *ChatHub
	fileRegistry *FileRegistry
	once         sync.Once
}

func NewChatHub(registry *FileRegistry) *ChatHub {
	return &ChatHub{
		clients:      make(map[string]*ChatClient),
		fileRegistry: registry,
		transfers:    make(map[string]*TransferInfo), // Initialize the new transfers map
	}
}

// Generates a new unique ID for a transfer.
func generateTransferID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// Join now returns the client it creates.
func (hub *ChatHub) Join(nickname string, channel ssh.Channel) *ChatClient {
	client := &ChatClient{
		nickname:     nickname,
		channel:      channel,
		outgoing:     make(chan []byte, 16),
		done:         make(chan struct{}),
		hub:          hub,
		fileRegistry: hub.fileRegistry,
	}
	hub.mu.Lock()
	hub.clients[nickname] = client
	hub.mu.Unlock()

	go client.readLoop()
	go client.writeLoop()

	// Broadcast join message
	joinMsg := ChatBroadcastPayload{
		Timestamp: time.Now().Format("15:04"),
		Text:      fmt.Sprintf("%s joined the chat.", nickname),
		IsSystem:  true,
	}
	hub.broadcast("system_broadcast", joinMsg, "")
	return client
}

func (c *ChatClient) Done() <-chan struct{} {
	return c.done
}

// broadcast sends a structured message to clients.
func (hub *ChatHub) broadcast(msgType string, payload interface{}, from string) {
	hub.mu.Lock()
	defer hub.mu.Unlock()

	msg, err := json.Marshal(OutboundMessage{Type: msgType, Payload: payload})
	if err != nil {
		log.Printf("Error marshalling broadcast message: %v", err)
		return
	}

	for nick, client := range hub.clients {
		if nick == from {
			continue
		}
		select {
		case client.outgoing <- msg:
		default:
		}
	}
}

// unicast sends a structured message to a single client.
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

func (hub *ChatHub) part(nickname string) {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	delete(hub.clients, nickname)
}

func (c *ChatClient) send(msgType string, payload interface{}) {
	msg, err := json.Marshal(OutboundMessage{Type: msgType, Payload: payload})
	if err != nil {
		log.Printf("Error marshalling message for %s: %v", c.nickname, err)
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
			log.Printf("Error unmarshalling message from %s: %v", c.nickname, err)
			continue
		}
		log.Printf("readLoop: received message type '%s' from %s", msg.Type, c.nickname)
		c.handleMessage(msg)
	}
}

func (c *ChatClient) handleMessage(msg InboundMessage) {
	switch msg.Type {
	case "share":
		var p SharePayload
		if err := json.Unmarshal(msg.Payload, &p); err == nil {
			c.fileRegistry.UpdateUserFiles(c.nickname, p.Files)
		}

	case "search":
		var p SearchPayload
		if err := json.Unmarshal(msg.Payload, &p); err == nil {
			results := c.fileRegistry.Search(p.Query)
			c.send("search_results", SearchResultsPayload{Results: results})
		}

	case "top_files":
		results := c.fileRegistry.TopFiles(50)
		c.send("search_results", SearchResultsPayload{Results: results})

	case "get_stats":
		c.hub.mu.Lock()
		var users []map[string]string
		for nick := range c.hub.clients {
			users = append(users, map[string]string{"nickname": nick, "status": "Online"})
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
			log.Printf("handleMessage: '%s' requested file '%s' from peer '%s'", c.nickname, p.FileName, p.Peer)
			c.initiateFileTransfer(p.FileName, p.Peer)
		}

	case "chat_message":
		var p ChatMessagePayload
		if err := json.Unmarshal(msg.Payload, &p); err == nil {
			broadcastPayload := ChatBroadcastPayload{
				Timestamp: time.Now().Format("15:04"),
				Nickname:  c.nickname,
				Text:      p.Text,
				IsSystem:  false,
			}
			c.hub.broadcast("chat_broadcast", broadcastPayload, "")
		}

	case "upload_data":
		var p UploadDataPayload
		if err := json.Unmarshal(msg.Payload, &p); err == nil {
			log.Printf("handleMessage: got 'upload_data' from '%s' for transfer %s (data size: %d)", c.nickname, p.TransferID, len(p.Data))
			c.relayTransferMessage("upload_data", p, p.TransferID)
		}

	case "upload_done":
		var p UploadDonePayload
		if err := json.Unmarshal(msg.Payload, &p); err == nil {
			log.Printf("handleMessage: got 'upload_done' from '%s' for transfer %s", c.nickname, p.TransferID)
			c.relayTransferMessage("upload_done", p, p.TransferID)
			c.hub.mu.Lock()
			delete(c.hub.transfers, p.TransferID)
			c.hub.totalTransfers++
			c.hub.mu.Unlock()
		}

	case "upload_error":
		var p UploadErrorPayload
		if err := json.Unmarshal(msg.Payload, &p); err == nil {
			log.Printf("handleMessage: got 'upload_error' from '%s' for transfer %s: %s", c.nickname, p.TransferID, p.Message)
			c.relayTransferMessage("transfer_error", TransferErrorPayload(p), p.TransferID)
			c.hub.mu.Lock()
			delete(c.hub.transfers, p.TransferID)
			c.hub.mu.Unlock()
		}

	default:
		log.Printf("Unknown message type '%s' from %s", msg.Type, c.nickname)
	}
}

func (c *ChatClient) initiateFileTransfer(filename, peer string) {
	if peer == c.nickname {
		c.send("transfer_error", TransferErrorPayload{Message: "You cannot download your own file."})
		return
	}

	fileInfo, found := c.fileRegistry.FindFile(filename, peer)
	if !found {
		c.send("transfer_error", TransferErrorPayload{Message: fmt.Sprintf("File not found or peer '%s' does not own it.", peer)})
		return
	}

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
		ToUser:   c.nickname,
	}
	c.hub.mu.Lock()
	c.hub.transfers[transferID] = transfer
	c.hub.mu.Unlock()

	log.Printf("Transfer %s initiated: %s wants '%s' from %s", transferID, c.nickname, filename, peer)

	// Tell the downloader the transfer is starting
	c.send("transfer_start", TransferStartPayload{
		TransferID: transferID,
		FileName:   filename,
		Size:       fileInfo.Size,
		FromUser:   peer,
	})

	// Tell the uploader to start sending the file
	ok := c.hub.unicast("upload_request", UploadRequestPayload{
		TransferID: transferID,
		FileName:   filename,
	}, peer)
	log.Printf("initiateFileTransfer: sent 'upload_request' to '%s' for transfer %s (ok=%v)", peer, transferID, ok)
}

func (c *ChatClient) relayTransferMessage(msgType string, payload interface{}, transferID string) {
	c.hub.mu.Lock()
	transfer, ok := c.hub.transfers[transferID]
	c.hub.mu.Unlock()

	if !ok {
		log.Printf("SECURITY: Received data for unknown transfer ID '%s' from %s", transferID, c.nickname)
		return
	}
	if transfer.FromUser != c.nickname {
		log.Printf("SECURITY: Mismatched user for transfer ID '%s'. Expected %s, got %s", transferID, transfer.FromUser, c.nickname)
		return
	}

	okSend := c.hub.unicast(msgType, payload, transfer.ToUser)
	log.Printf("relayTransferMessage: relayed '%s' for transfer %s from '%s' to '%s' (ok=%v)", msgType, transferID, c.nickname, transfer.ToUser, okSend)
}

func (c *ChatClient) writeLoop() {
	for {
		select {
		case msg := <-c.outgoing:
			// Ensure message ends with a newline for the client scanner
			if !strings.HasSuffix(string(msg), "\n") {
				msg = append(msg, '\n')
			}
			c.channel.Write(msg)
		case <-c.done:
			return
		}
	}
}

func (c *ChatClient) Close() {
	c.once.Do(func() {
		c.fileRegistry.RemoveUser(c.nickname)
		c.hub.part(c.nickname)
		close(c.done)
		c.channel.Close()
		log.Printf("%s left chat", c.nickname)

		leaveMsg := ChatBroadcastPayload{
			Timestamp: time.Now().Format("15:04"),
			Text:      fmt.Sprintf("%s left the chat.", c.nickname),
			IsSystem:  true,
		}
		c.hub.broadcast("system_broadcast", leaveMsg, "")
	})
}