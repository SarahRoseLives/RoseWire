// ssh/service.go
package ssh

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"rosetui/profile"
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
// **NEW:** A message containing parsed search results.
type SearchResultsMsg struct{ Results []SearchResult }

// A message indicating a change in the connection status.
type StatusMsg struct{ Message string }

// A message indicating a fatal connection error.
type ErrorMsg struct{ Err error }

func (e ErrorMsg) Error() string { return e.Err.Error() }

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

// **NEW:** Structs for parsing search results from the server.
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
	client      *ssh.Client
	chatSession *ssh.Session
	profile     profile.Profile
	serverAddr  string
	msgChan     chan tea.Msg // Channel to send messages back to the TUI
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
		s.msgChan <- StatusMsg{Message: "Authenticating..."}
		key, err := os.ReadFile(s.profile.KeyPath)
		if err != nil {
			return ErrorMsg{fmt.Errorf("unable to read private key: %w", err)}
		}

		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return ErrorMsg{fmt.Errorf("unable to parse private key: %w", err)}
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
			return ErrorMsg{fmt.Errorf("failed to connect: %w", err)}
		}
		s.client = client

		s.msgChan <- StatusMsg{Message: "Opening chat session..."}
		session, err := client.NewSession()
		if err != nil {
			_ = client.Close()
			return ErrorMsg{fmt.Errorf("failed to create session: %w", err)}
		}
		s.chatSession = session

		if err := session.RequestSubsystem("chat"); err != nil {
			_ = session.Close()
			_ = client.Close()
			return ErrorMsg{fmt.Errorf("failed to request chat subsystem: %w", err)}
		}

		stdout, err := session.StdoutPipe()
		if err != nil {
			return ErrorMsg{fmt.Errorf("failed to get stdout pipe: %w", err)}
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
				// **NEW:** Handle search results.
				case "search_results":
					var p searchResultsPayload
					if err := json.Unmarshal(msg.Payload, &p); err == nil {
						s.msgChan <- SearchResultsMsg{Results: p.Results}
					}
				}
			}
			s.msgChan <- StatusMsg{Message: "Connection lost."}
			_ = session.Close()
			_ = client.Close()
		}()

		s.msgChan <- StatusMsg{Message: fmt.Sprintf("Connected as %s", s.profile.Nickname)}
		return nil
	}
}

// sendCommand is a generic helper to send JSON commands to the server.
func (s *Service) sendCommand(cmdType string, payload interface{}) error {
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

// **NEW:** Methods for search functionality.
func (s *Service) SearchFiles(query string) error {
	return s.sendCommand("search", map[string]string{"query": query})
}

func (s *Service) FetchTopFiles() error {
	return s.sendCommand("top_files", map[string]string{})
}

func (s *Service) Close() {
	if s.chatSession != nil {
		_ = s.chatSession.Close()
	}
	if s.client != nil {
		_ = s.client.Close()
	}
}