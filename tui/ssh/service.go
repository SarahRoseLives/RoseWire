// ssh/service.go
package ssh

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"rosetui/profile"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/crypto/ssh"
)

// --- Custom Message Types ---

// Parsed message types sent from the service to the TUI.
type WelcomeMsg struct{ Identity string }
type SystemBroadcastMsg struct{ Text string }
type ChatBroadcastMsg struct {
	Nickname string
	Text     string
}
type SearchResultsMsg struct{ Results []SearchResult }

// A message indicating a change in the connection status.
type StatusMsg struct{ Message string }

// A message indicating a fatal connection error.
type ErrorMsg struct{ Err error }

func (e ErrorMsg) Error() string { return e.Err.Error() }

// **NEW:** A message indicating the connection has been terminated.
type DisconnectedMsg struct{}

// --- Internal JSON parsing structs ---

type serverMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type welcomePayload struct {
	Identity string `json:"identity"`
}

type broadcastPayload struct {
	Timestamp string `json:"timestamp"`
	Nickname  string `json:"nickname"`
	Text      string `json:"text"`
}

type SearchResult struct {
	FileName string `json:"fileName"`
	Size     int64  `json:"size"`
	Peer     string `json:"peer"`
	Hash     string `json:"Hash"`
}

type searchResultsPayload struct {
	Results []SearchResult `json:"results"`
}

// --- Service ---

type Service struct {
	client        *ssh.Client
	chatSession   *ssh.Session
	profile       profile.Profile
	serverAddr    string
	msgChan       chan tea.Msg // Channel to send messages back to the TUI
	stopKeepAlive context.CancelFunc
	mu            sync.Mutex
}

func NewService(p profile.Profile, serverAddr string, msgChan chan tea.Msg) *Service {
	return &Service{
		profile:    p,
		serverAddr: serverAddr,
		msgChan:    msgChan,
	}
}

// Connect returns a tea.Cmd that handles the SSH connection and lifecycle.
func (s *Service) Connect() tea.Cmd {
	return func() tea.Msg {
		s.mu.Lock()
		// Stop any previous keep-alive goroutine before making a new connection.
		if s.stopKeepAlive != nil {
			s.stopKeepAlive()
		}
		ctx, cancel := context.WithCancel(context.Background())
		s.stopKeepAlive = cancel
		s.mu.Unlock() // Unlock early so other methods aren't blocked during dial

		s.msgChan <- StatusMsg{Message: "Authenticating..."}
		key, err := os.ReadFile(s.profile.KeyPath)
		if err != nil {
			s.msgChan <- ErrorMsg{fmt.Errorf("unable to read private key: %w", err)}
			return DisconnectedMsg{}
		}

		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			s.msgChan <- ErrorMsg{fmt.Errorf("unable to parse private key: %w", err)}
			return DisconnectedMsg{}
		}

		config := &ssh.ClientConfig{
			User: s.profile.Nickname,
			Auth: []ssh.AuthMethod{
				ssh.PublicKeys(signer),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			Timeout:         10 * time.Second,
		}

		s.msgChan <- StatusMsg{Message: fmt.Sprintf("Connecting to %s...", s.serverAddr)}
		client, err := ssh.Dial("tcp", net.JoinHostPort(s.serverAddr, "2222"), config)
		if err != nil {
			s.msgChan <- ErrorMsg{fmt.Errorf("failed to connect: %w", err)}
			return DisconnectedMsg{}
		}

		s.mu.Lock()
		s.client = client
		s.mu.Unlock()

		go s.runKeepAlive(ctx, client)

		s.msgChan <- StatusMsg{Message: "Opening chat session..."}
		session, err := client.NewSession()
		if err != nil {
			_ = client.Close()
			s.msgChan <- ErrorMsg{fmt.Errorf("failed to create session: %w", err)}
			return DisconnectedMsg{}
		}

		s.mu.Lock()
		s.chatSession = session
		s.mu.Unlock()

		if err := session.RequestSubsystem("chat"); err != nil {
			_ = session.Close()
			_ = client.Close()
			s.msgChan <- ErrorMsg{fmt.Errorf("failed to request chat subsystem: %w", err)}
			return DisconnectedMsg{}
		}

		stdout, err := session.StdoutPipe()
		if err != nil {
			s.msgChan <- ErrorMsg{fmt.Errorf("failed to get stdout pipe: %w", err)}
			return DisconnectedMsg{}
		}

		go func() {
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				line := scanner.Bytes()
				var msg serverMessage
				if err := json.Unmarshal(line, &msg); err != nil {
					s.msgChan <- SystemBroadcastMsg{Text: "received malformed message from server"}
					continue
				}

				switch msg.Type {
				case "welcome":
					var p welcomePayload
					if err := json.Unmarshal(msg.Payload, &p); err == nil {
						s.msgChan <- WelcomeMsg{Identity: p.Identity}
					}
				case "system_broadcast":
					var p broadcastPayload
					if err := json.Unmarshal(msg.Payload, &p); err == nil {
						s.msgChan <- SystemBroadcastMsg{Text: p.Text}
					}
				case "chat_broadcast":
					var p broadcastPayload
					if err := json.Unmarshal(msg.Payload, &p); err == nil {
						s.msgChan <- ChatBroadcastMsg{Nickname: p.Nickname, Text: p.Text}
					}
				case "search_results":
					var p searchResultsPayload
					if err := json.Unmarshal(msg.Payload, &p); err == nil {
						s.msgChan <- SearchResultsMsg{Results: p.Results}
					}
				}
			}
			// When scanner.Scan() returns false, the connection is broken.
			s.Close()
			s.msgChan <- DisconnectedMsg{}
		}()

		s.msgChan <- StatusMsg{Message: fmt.Sprintf("Connected as %s", s.profile.Nickname)}
		return nil
	}
}

// **NEW:** runKeepAlive sends a ping to the server on an interval to prevent timeouts.
func (s *Service) runKeepAlive(ctx context.Context, client *ssh.Client) {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.mu.Lock()
			if client == nil {
				s.mu.Unlock()
				return
			}
			_, _, err := client.SendRequest("keepalive@golang.org", true, nil)
			s.mu.Unlock()
			if err != nil {
				// The connection is likely dead. The main stdout reader will detect
				// this and trigger the reconnect logic. We can just exit here.
				return
			}
		}
	}
}

// sendCommand is a generic helper to send JSON commands to the server.
func (s *Service) sendCommand(cmdType string, payload interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.chatSession == nil {
		return fmt.Errorf("chat session is not active")
	}
	stdin, err := s.chatSession.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	cmd := map[string]interface{}{
		"type":    cmdType,
		"payload": payload,
	}

	cmdBytes, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("failed to marshal command: %w", err)
	}

	_, err = fmt.Fprintln(stdin, string(cmdBytes))
	return err
}

func (s *Service) SendMessage(msg string) error {
	return s.sendCommand("chat_message", map[string]string{"text": msg})
}

func (s *Service) SearchFiles(query string) error {
	return s.sendCommand("search", map[string]string{"query": query})
}

func (s *Service) FetchTopFiles() error {
	return s.sendCommand("top_files", map[string]string{})
}

func (s *Service) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stopKeepAlive != nil {
		s.stopKeepAlive()
		s.stopKeepAlive = nil
	}
	if s.chatSession != nil {
		_ = s.chatSession.Close()
		s.chatSession = nil
	}
	if s.client != nil {
		_ = s.client.Close()
		s.client = nil
	}
}