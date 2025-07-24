package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// ChatHub manages all chat subsystem sessions.
type ChatHub struct {
	mu           sync.Mutex
	clients      map[string]*ChatClient
	fileRegistry *FileRegistry
}

// ChatClient represents a connected chat user.
type ChatClient struct {
	nickname     string
	channel      ssh.Channel
	outgoing     chan string
	done         chan struct{}
	hub          *ChatHub
	fileRegistry *FileRegistry // Client needs access to the registry
	once         sync.Once
}

func NewChatHub(registry *FileRegistry) *ChatHub {
	return &ChatHub{
		clients:      make(map[string]*ChatClient),
		fileRegistry: registry,
	}
}

func (hub *ChatHub) Join(nickname string, channel ssh.Channel) {
	client := &ChatClient{
		nickname:     nickname,
		channel:      channel,
		outgoing:     make(chan string, 16),
		done:         make(chan struct{}),
		hub:          hub,
		fileRegistry: hub.fileRegistry, // Pass registry to client
	}
	hub.mu.Lock()
	hub.clients[nickname] = client
	hub.mu.Unlock()

	go client.readLoop()
	go client.writeLoop()
	hub.broadcast(fmt.Sprintf("[%s] %s joined the chat.", time.Now().Format("15:04"), nickname), nickname)
}

func (hub *ChatHub) broadcast(msg, from string) {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	for nick, client := range hub.clients {
		if nick == from {
			continue
		}
		select {
		case client.outgoing <- msg:
		default:
			// drop if full
		}
	}
}

func (hub *ChatHub) part(nickname string) {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	delete(hub.clients, nickname)
}

// ChatClient methods

func (c *ChatClient) readLoop() {
	scanner := bufio.NewScanner(c.channel)
	for scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}

		// Handle /share command
		if strings.HasPrefix(text, "/share ") {
			payload := strings.TrimPrefix(text, "/share ")
			files, err := ParseShareCommand(payload)
			if err != nil {
				log.Printf("Error parsing share command from %s: %v", c.nickname, err)
			} else {
				c.fileRegistry.UpdateUserFiles(c.nickname, files)
			}
		// Handle /search command
		} else if strings.HasPrefix(text, "/search ") {
			query := strings.TrimPrefix(text, "/search ")
			results := c.fileRegistry.Search(query)

			var parts []string
			for _, res := range results {
				// Format: name:raw_size_bytes:peer
				part := fmt.Sprintf("%s:%d:%s", res.FileName, res.Size, res.Peer)
				parts = append(parts, part)
			}
			payload := strings.Join(parts, "|")
			// Send results back to the originating client only
			c.outgoing <- "[SEARCH] " + payload
		// Handle /top command
		} else if strings.HasPrefix(text, "/top") {
			// Optional: parse limit, e.g. "/top 10"
			limit := 10
			parts := strings.Fields(text)
			if len(parts) == 2 {
				if v, err := strconv.Atoi(parts[1]); err == nil && v > 0 {
					limit = v
				}
			}
			results := c.fileRegistry.TopFiles(limit)

			var resultParts []string
			for _, res := range results {
				part := fmt.Sprintf("%s:%d:%s", res.FileName, res.Size, res.Peer)
				resultParts = append(resultParts, part)
			}
			payload := strings.Join(resultParts, "|")
			c.outgoing <- "[SEARCH] " + payload
		// Handle /stats command
		} else if strings.HasPrefix(text, "/stats") {
			// Provide network stats as JSON
			c.hub.mu.Lock()
			var users []map[string]string
			for nick := range c.hub.clients {
				users = append(users, map[string]string{
					"nickname": nick,
					"status":   "Online",
				})
			}
			c.hub.mu.Unlock()
			stats := map[string]interface{}{
				"users":           users,
				"relayServers":    1,
				"totalUsers":      len(users),
				"totalTransfers":  0,
				"activeTransfers": 0,
			}
			jsonPayload, err := json.Marshal(stats)
			if err != nil {
				c.outgoing <- "[STATS] {}"
			} else {
				c.outgoing <- "[STATS] " + string(jsonPayload)
			}
		} else {
			// Broadcast regular chat message to everyone
			msg := fmt.Sprintf("[%s] %s: %s", time.Now().Format("15:04"), c.nickname, text)
			c.hub.broadcast(msg, "")
		}
	}
	c.Close()
}

func (c *ChatClient) writeLoop() {
	for {
		select {
		case msg := <-c.outgoing:
			fmt.Fprintln(c.channel, msg)
		case <-c.done:
			return
		}
	}
}

func (c *ChatClient) Close() {
	c.once.Do(func() {
		// Clean up user files on disconnect
		c.fileRegistry.RemoveUser(c.nickname)
		c.hub.part(c.nickname)
		close(c.done)
		c.channel.Close()
		log.Printf("%s left chat", c.nickname)
		c.hub.broadcast(fmt.Sprintf("[%s] %s left the chat.", time.Now().Format("15:04"), c.nickname), "")
	})
}