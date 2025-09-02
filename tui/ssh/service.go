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
type WelcomeMsg struct{ Identity string }
type SystemBroadcastMsg struct{ Text string }
type ChatBroadcastMsg struct {
	Nickname string
	Text     string
}
type SearchResultsMsg struct{ Results []SearchResult }
type NetworkStatsMsg struct {
	Users           []NetworkUser `json:"users"`
	RelayServers    int           `json:"relayServers"`
	TotalUsers      int           `json:"totalUsers"`
	TotalTransfers  int           `json:"totalTransfers"`
	ActiveTransfers int           `json:"activeTransfers"`
}
type TransfersUpdateMsg struct{ Transfers []Transfer }
type StatusMsg struct{ Message string }
type ErrorMsg struct{ Err error }

func (e ErrorMsg) Error() string { return e.Err.Error() }

type DisconnectedMsg struct{}

// --- Data Models ---
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
type NetworkUser struct {
	Nickname string `json:"nickname"`
	Status   string `json:"status"`
}
type ShareableFile struct {
	Name string `json:"Name"`
	Size int64  `json:"Size"`
	Hash string `json:"Hash"`
}
type Transfer struct {
	ID       string
	FileName string
	FromUser string
	Size     int64
	Status   string
	Error    string
	Speed    string
	Progress float64
}
type transferStartPayload struct {
	TransferID string `json:"transferID"`
	FileName   string `json:"fileName"`
	Size       int64  `json:"size"`
	FromUser   string `json:"fromUser"`
}
type transferErrorPayload struct {
	TransferID string `json:"transferID"`
	Message    string `json:"message"`
}

// --- Service ---
type Service struct {
	client        *ssh.Client
	chatSession   *ssh.Session
	profile       profile.Profile
	serverAddr    string
	msgChan       chan tea.Msg
	stopKeepAlive context.CancelFunc
	mu            sync.Mutex
	transfers     map[string]*Transfer
}

func NewService(p profile.Profile, serverAddr string, msgChan chan tea.Msg) *Service {
	return &Service{
		profile:   p,
		serverAddr: serverAddr,
		msgChan:    msgChan,
		transfers: make(map[string]*Transfer),
	}
}

func (s *Service) Connect() tea.Cmd {
	return func() tea.Msg {
		s.mu.Lock()
		if s.stopKeepAlive != nil {
			s.stopKeepAlive()
		}
		ctx, cancel := context.WithCancel(context.Background())
		s.stopKeepAlive = cancel
		s.mu.Unlock()

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
					var payload struct {
						Results []SearchResult `json:"results"`
					}
					if err := json.Unmarshal(msg.Payload, &payload); err == nil {
						s.msgChan <- SearchResultsMsg{Results: payload.Results}
					}
				case "network_stats":
					var p NetworkStatsMsg
					if err := json.Unmarshal(msg.Payload, &p); err == nil {
						s.msgChan <- p
					}
				case "transfer_start":
					var p transferStartPayload
					if err := json.Unmarshal(msg.Payload, &p); err == nil {
						s.handleTransferStart(p)
					}
				case "transfer_error":
					var p transferErrorPayload
					if err := json.Unmarshal(msg.Payload, &p); err == nil {
						s.handleTransferError(p)
					}
				}
			}
			s.Close()
			s.msgChan <- DisconnectedMsg{}
		}()

		s.msgChan <- StatusMsg{Message: fmt.Sprintf("Connected via SSH as %s", s.profile.Nickname)}
		return nil
	}
}
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
				return
			}
		}
	}
}
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
func (s *Service) RequestNetworkStats() error {
	return s.sendCommand("get_stats", map[string]string{})
}
func (s *Service) ShareFiles(files []ShareableFile) error {
	return s.sendCommand("share", map[string][]ShareableFile{"files": files})
}
func (s *Service) DownloadFile(file SearchResult) error {
	return s.sendCommand("get_file", map[string]string{"fileName": file.FileName, "peer": file.Peer})
}
func (s *Service) handleTransferStart(p transferStartPayload) {
	s.mu.Lock()
	defer s.mu.Unlock()

	transfer := &Transfer{
		ID:       p.TransferID,
		FileName: p.FileName,
		FromUser: p.FromUser,
		Size:     p.Size,
		Status:   "Active",
	}
	s.transfers[p.TransferID] = transfer
	s.publishTransfers()

	go s.simulateDownload(p.TransferID)
}
func (s *Service) handleTransferError(p transferErrorPayload) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if t, ok := s.transfers[p.TransferID]; ok {
		t.Status = "Failed"
		t.Error = p.Message
		t.Progress = 0
		s.publishTransfers()
	}
}
func (s *Service) simulateDownload(transferID string) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	lastProgressTime := time.Now()
	var lastProgressBytes int64

	for range ticker.C {
		s.mu.Lock()
		t, ok := s.transfers[transferID]
		if !ok || t.Status != "Active" {
			s.mu.Unlock()
			return
		}

		t.Progress += 0.02
		if t.Progress >= 1.0 {
			t.Progress = 1.0
			t.Status = "Complete"
			t.Speed = ""
			s.publishTransfers()
			s.mu.Unlock()
			return
		}

		now := time.Now()
		elapsed := now.Sub(lastProgressTime).Seconds()
		if elapsed > 0.5 {
			currentBytes := int64(t.Progress * float64(t.Size))
			bytesSinceLast := currentBytes - lastProgressBytes
			speedBytesPerSec := float64(bytesSinceLast) / elapsed
			t.Speed = fmt.Sprintf("%.2f MB/s", speedBytesPerSec/(1024*1024))

			lastProgressTime = now
			lastProgressBytes = currentBytes
		}

		s.publishTransfers()
		s.mu.Unlock()
	}
}
func (s *Service) publishTransfers() {
	var transferList []Transfer
	for _, t := range s.transfers {
		transferList = append(transferList, *t)
	}
	s.msgChan <- TransfersUpdateMsg{Transfers: transferList}
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