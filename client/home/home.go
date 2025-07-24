package home

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tab int

const (
	tabSearch tab = iota
	tabShared
	tabDownloads
	tabPeers
	tabLogs
	numTabs
)

var tabLabels = []string{"Search", "Shared", "Downloads", "Peers", "Logs/Chat"}

// searchResult is now defined in search.go

type sharedFile struct {
	Name    string
	IsDir   bool
	Size    string
	rawSize int64 // For internal use
}

type download struct {
	FileName string
	Progress string
	Status   string
	Source   string
}

type peer struct {
	Name   string
	Host   string
	Online bool
}

type logEntry struct {
	Time    string
	Message string
}

type Model struct {
	Nickname string
	Key      string
	CurrentTab tab
	Cursor     int
	Width      int
	Height     int
	Input      string // For search box
	InputMode  bool   // True if editing search input

	// Chat integration
	chatClient *ChatClient
	chatInput     string
	chatInputMode bool

	// Data stores
	SearchResults []searchResult
	SharedFiles   []sharedFile
	Downloads     []download
	Peers         []peer
	Logs          []logEntry
}

var (
	pink           = lipgloss.Color("#ff81b3")
	pinkHeader     = lipgloss.NewStyle().Background(lipgloss.Color("#2b0036")).Foreground(pink).Padding(0, 1).Bold(true)
	tabStyle       = lipgloss.NewStyle().Padding(0, 2)
	activeTabStyle = tabStyle.Copy().Bold(true).Foreground(pink)
	cursorStyle    = lipgloss.NewStyle().Foreground(pink)
	footerStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Padding(0, 1)
	sectionTitle   = lipgloss.NewStyle().Foreground(pink).Bold(true)
	normalStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
)

// --- Chat message event for Bubble Tea
type chatLineMsg string

// chatLineListener now dispatches between search results and chat messages.
func chatLineListener(c *ChatClient) tea.Cmd {
	return func() tea.Msg {
		line, ok := <-c.Receive()
		if !ok {
			return nil
		}

		if strings.HasPrefix(line, "[SEARCH] ") {
			payload := strings.TrimPrefix(line, "[SEARCH] ")
			return ParseSearchResults(payload)
		}

		return chatLineMsg(line)
	}
}

func NewModel(nickname, key string, client *ChatClient) Model {
	return Model{
		Nickname: nickname,
		Key:      key,
		// Pass the already-connected client
		chatClient: client,
		// Start with empty search results
		SearchResults: []searchResult{},
		// SharedFiles and Downloads are now populated from the filesystem
		SharedFiles: []sharedFile{},
		Downloads:   []download{},
		Peers: []peer{
			{"alice", "host2", true},
			{"bob", "host3", false},
			{"eve", "host5", true},
		},
		Logs: []logEntry{
			{"[SYS]", "Welcome to RoseWire!"},
		},
	}
}

func (m Model) Init() tea.Cmd {
	// Listen for chat messages and scan local file directories at startup
	return tea.Batch(
		chatLineListener(m.chatClient),
		ScanUploadsCmd(),
		ScanDownloadsCmd(),
	)
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	// Handle the list of files from the local 'uploads' scan
	case SharedFilesLoadedMsg:
		m.SharedFiles = msg
		// After loading our files, create a command to notify the server
		return m, NotifyServerOfSharedFilesCmd(m.chatClient, m.SharedFiles)

	// Handle the list of files from the local 'downloads' scan
	case DownloadsLoadedMsg:
		m.Downloads = msg
		return m, nil

	// Handle incoming search results
	case SearchResultsMsg:
		m.SearchResults = msg
		return m, nil

	case chatLineMsg:
		// Handle a new chat message
		entry := ParseChatLine(string(msg))
		m.Logs = append(m.Logs, logEntry{
			Time:    entry.Time,
			Message: fmt.Sprintf("%s: %s", entry.Sender, entry.Message),
		})
		// Listen for the next chat message
		return m, chatLineListener(m.chatClient)

	// A log entry can now be a message
	case logEntry:
		m.Logs = append(m.Logs, msg)
		return m, nil

	case tea.KeyMsg:
		// Chat input mode
		if m.CurrentTab == tabLogs && m.chatInputMode {
			switch msg.String() {
			case "enter":
				if strings.TrimSpace(m.chatInput) != "" && m.chatClient != nil {
					m.chatClient.Send(m.chatInput)
					m.Logs = append(m.Logs, logEntry{
						Time:    time.Now().Format("[15:04]"),
						Message: fmt.Sprintf("%s: %s", m.Nickname, m.chatInput),
					})
				}
				m.chatInput = ""
				m.chatInputMode = false
			case "esc":
				m.chatInput = ""
				m.chatInputMode = false
			case "backspace":
				if len(m.chatInput) > 0 {
					m.chatInput = m.chatInput[:len(m.chatInput)-1]
				}
			default:
				if msg.Type == tea.KeyRunes {
					m.chatInput += msg.String()
				}
			}
			return m, nil
		}
		switch {
		case m.InputMode:
			switch msg.String() {
			case "enter":
				m.InputMode = false
				if m.CurrentTab == tabSearch && strings.TrimSpace(m.Input) != "" {
					return m, SearchCmd(m.chatClient, m.Input)
				}
			case "esc":
				m.InputMode = false
			case "backspace":
				if len(m.Input) > 0 {
					m.Input = m.Input[:len(m.Input)-1]
				}
			default:
				if msg.Type == tea.KeyRunes {
					m.Input += msg.String()
				}
			}
		default:
			switch msg.String() {
			case "ctrl+c", "q":
				if m.chatClient != nil {
					m.chatClient.Close()
				}
				return m, tea.Quit
			case "tab":
				m.CurrentTab = (m.CurrentTab + 1) % numTabs
				m.Cursor = 0
			case "shift+tab":
				m.CurrentTab = (m.CurrentTab - 1 + numTabs) % numTabs
				m.Cursor = 0
			case "up", "k":
				if m.Cursor > 0 {
					m.Cursor--
				}
			case "down", "j":
				m.Cursor++
			case "enter":
				if m.CurrentTab == tabSearch && !m.InputMode {
					m.InputMode = true
					m.Input = ""
				} else if m.CurrentTab == tabLogs && !m.chatInputMode {
					m.chatInputMode = true
				}
			case "r": // Refresh list
				if m.CurrentTab == tabShared {
					return m, ScanUploadsCmd()
				}
				if m.CurrentTab == tabDownloads {
					return m, ScanDownloadsCmd()
				}
			}
		}
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
	}
	return m, nil
}

func (m Model) View() string {
	var b strings.Builder

	// Pink header, stretch to width
	header := pinkHeader.Width(m.Width).Render(fmt.Sprintf("🌹 RoseWire - [%s | %s]", m.Nickname, filepath.Base(m.Key)))
	b.WriteString(header + "\n")

	// Tabs - use pink for active, stretch to width
	var tabViews []string
	for i, label := range tabLabels {
		if tab(i) == m.CurrentTab {
			tabViews = append(tabViews, activeTabStyle.Render(label))
		} else {
			tabViews = append(tabViews, tabStyle.Render(label))
		}
	}
	tabsRow := lipgloss.JoinHorizontal(lipgloss.Top, tabViews...)
	tabsLine := lipgloss.NewStyle().Width(m.Width).Render(tabsRow)
	b.WriteString(tabsLine + "\n")

	// Horizontal line (fill width)
	b.WriteString(lipgloss.NewStyle().Foreground(pink).Width(m.Width).Render(strings.Repeat("─", m.Width)) + "\n")

	// Panel
	switch m.CurrentTab {
	case tabSearch:
		b.WriteString(renderSearchPanel(m))
	case tabShared:
		b.WriteString(renderSharedPanel(m))
	case tabDownloads:
		b.WriteString(renderDownloadsPanel(m))
	case tabPeers:
		b.WriteString(renderPeersPanel(m))
	case tabLogs:
		b.WriteString(renderLogsPanel(m))
	}

	// Footer - fill width
	footer := footerStyle.Width(m.Width).Render("[Tab] Switch Panel  [↑/↓] Move  [Enter] Select/Edit/Chat  [R] Refresh [Q] Quit")
	b.WriteString("\n" + footer)
	return b.String()
}

// renderSearchPanel moved to search.go

func renderSharedPanel(m Model) string {
	var b strings.Builder
	b.WriteString(sectionTitle.Render(fmt.Sprintf("Shared Files (from your '%s' folder):\n", uploadsDir)))
	line := lipgloss.NewStyle().Foreground(pink).Width(m.Width).Render(strings.Repeat("-", m.Width))
	b.WriteString(line + "\n")
	header := fmt.Sprintf("%-2s %-30s %-10s", "", "Name", "Size")
	b.WriteString(sectionTitle.Render(header) + "\n")
	b.WriteString(line + "\n")

	if len(m.SharedFiles) == 0 {
		b.WriteString("\n  No files found in the 'uploads' directory.\n")
	}

	for i, f := range m.SharedFiles {
		cursor := " "
		if i == m.Cursor {
			cursor = cursorStyle.Render(">")
		}
		name := f.Name
		if f.IsDir {
			name = filepath.Join(name, "/")
		}
		row := fmt.Sprintf("%s %-30s %-10s", cursor, name, f.Size)
		b.WriteString(row + "\n")
	}
	b.WriteString("\n" + cursorStyle.Render("[R] Refresh List") + "\n")
	return b.String()
}

func renderPeersPanel(m Model) string {
	var b strings.Builder
	b.WriteString(sectionTitle.Render("Peers:\n"))
	line := lipgloss.NewStyle().Foreground(pink).Width(m.Width).Render(strings.Repeat("-", m.Width))
	b.WriteString(line + "\n")
	header := fmt.Sprintf("%-2s %-10s %-14s %-9s", "", "Name", "Host", "Status")
	b.WriteString(sectionTitle.Render(header) + "\n")
	b.WriteString(line + "\n")
	for i, p := range m.Peers {
		cursor := " "
		if i == m.Cursor {
			cursor = cursorStyle.Render(">")
		}
		status := normalStyle.Render("OFFLINE")
		if p.Online {
			status = cursorStyle.Render("ONLINE")
		}
		row := fmt.Sprintf("%s %-10s %-14s %-9s %s", cursor, p.Name, p.Host, status, cursorStyle.Render("[Remove]"))
		b.WriteString(row + "\n")
	}
	b.WriteString("\n" + cursorStyle.Render("[A] Add peer (by SSH endpoint)") + "\n")
	return b.String()
}

func renderLogsPanel(m Model) string {
	var b strings.Builder
	b.WriteString(sectionTitle.Render("Logs & Chat:\n"))
	line := lipgloss.NewStyle().Foreground(pink).Width(m.Width).Render(strings.Repeat("-", m.Width))
	b.WriteString(line + "\n")
	// Render logs from the bottom up to keep recent messages visible
	maxLogs := m.Height - 12 // Heuristic for available space
	if maxLogs < 1 {
		maxLogs = 1
	}
	start := len(m.Logs) - maxLogs
	if start < 0 {
		start = 0
	}
	for _, entry := range m.Logs[start:] {
		b.WriteString(fmt.Sprintf("%-7s %s\n", entry.Time, entry.Message))
	}
	// Chat input bar
	if m.chatInputMode {
		b.WriteString("\n> " + m.chatInput + "_\n")
	} else {
		b.WriteString("\n[Enter] Type a chat message\n")
	}
	return b.String()
}