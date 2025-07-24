package home

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// ChatClient manages the SSH session for chat.
type ChatClient struct {
	Nickname   string
	KeyPath    string
	ServerAddr string

	sshClient *ssh.Client
	session   *ssh.Session
	stdin     io.WriteCloser
	stdout    io.Reader

	Incoming chan string
	Outgoing chan string
	Done     chan struct{}

	once sync.Once
}

func NewChatClient(nickname, keyPath, serverAddr string) *ChatClient {
	return &ChatClient{
		Nickname:   nickname,
		KeyPath:    keyPath,
		ServerAddr: serverAddr,
		Incoming:   make(chan string, 64),
		Outgoing:   make(chan string, 8),
		Done:       make(chan struct{}),
	}
}

// Connect establishes the SSH "chat" session.
func (c *ChatClient) Connect() error {
	priv := strings.TrimSuffix(c.KeyPath, ".pub")
	key, err := os.ReadFile(priv)
	if err != nil {
		return fmt.Errorf("read key: %w", err)
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return fmt.Errorf("parse key: %w", err)
	}
	config := &ssh.ClientConfig{
		User: c.Nickname,
		Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout: 4 * time.Second,
	}

	client, err := ssh.Dial("tcp", c.ServerAddr, config)
	if err != nil {
		return fmt.Errorf("ssh dial: %w", err)
	}
	c.sshClient = client
	session, err := client.NewSession()
	if err != nil {
		client.Close()
		return fmt.Errorf("session: %w", err)
	}
	stdin, err := session.StdinPipe()
	if err != nil {
		client.Close()
		return fmt.Errorf("stdin: %w", err)
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		client.Close()
		return fmt.Errorf("stdout: %w", err)
	}

	// Start a "chat" subsystem (you must implement this on the server side)
	if err := session.RequestSubsystem("chat"); err != nil {
		client.Close()
		return fmt.Errorf("request subsystem: %w", err)
	}

	c.session = session
	c.stdin = stdin
	c.stdout = stdout

	go c.readLoop()
	go c.writeLoop()

	return nil
}

func (c *ChatClient) readLoop() {
	scanner := bufio.NewScanner(c.stdout)
	for scanner.Scan() {
		select {
		case c.Incoming <- scanner.Text():
		case <-c.Done:
			return
		}
	}
	c.Close()
}

func (c *ChatClient) writeLoop() {
	for {
		select {
		case msg := <-c.Outgoing:
			fmt.Fprintln(c.stdin, msg)
		case <-c.Done:
			return
		}
	}
}

func (c *ChatClient) Send(msg string) {
	if msg = strings.TrimSpace(msg); msg != "" {
		c.Outgoing <- msg
	}
}

func (c *ChatClient) Receive() <-chan string {
	return c.Incoming
}

func (c *ChatClient) Close() {
	c.once.Do(func() {
		close(c.Done)
		if c.session != nil {
			c.session.Close()
		}
		if c.sshClient != nil {
			c.sshClient.Close()
		}
	})
}

// ChatLogEntry represents a single chat message for the UI.
type ChatLogEntry struct {
	Time    string
	Sender  string
	Message string
}

// ParseChatLine parses "[14:35] alice: hello" or just "alice: hi"
func ParseChatLine(line string) ChatLogEntry {
	ts := time.Now().Format("[15:04]")
	sender := "???"
	msg := line
	if i := strings.Index(line, "] "); i > 0 {
		ts = line[:i+1]
		rest := line[i+2:]
		if j := strings.Index(rest, ": "); j > 0 {
			sender = rest[:j]
			msg = rest[j+2:]
		} else {
			msg = rest
		}
	} else if j := strings.Index(line, ": "); j > 0 {
		sender = line[:j]
		msg = line[j+2:]
	}
	return ChatLogEntry{Time: ts, Sender: sender, Message: msg}
}