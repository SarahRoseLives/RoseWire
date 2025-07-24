package main

import (
	"bufio"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// ChatHub manages all chat subsystem sessions.
type ChatHub struct {
	mu      sync.Mutex
	clients map[string]*ChatClient
}

// ChatClient represents a connected chat user.
type ChatClient struct {
	nickname string
	channel  ssh.Channel
	outgoing chan string
	done     chan struct{}
	hub      *ChatHub
}

func NewChatHub() *ChatHub {
	return &ChatHub{
		clients: make(map[string]*ChatClient),
	}
}

func (hub *ChatHub) Join(nickname string, channel ssh.Channel) {
	client := &ChatClient{
		nickname: nickname,
		channel:  channel,
		outgoing: make(chan string, 16),
		done:     make(chan struct{}),
		hub:      hub,
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
		if text != "" {
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
	c.hub.part(c.nickname)
	close(c.done)
	c.channel.Close()
	log.Printf("%s left chat", c.nickname)
}