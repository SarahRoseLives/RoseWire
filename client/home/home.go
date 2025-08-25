package home

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss" // <-- FIX IS HERE
)

type tab int

// The new tab layout
const (
	tabSearch tab = iota
	tabTransfers
	tabLibrary
	tabChat
	tabNetwork
	tabSettings
	numTabs
)

// Updated labels for the tabs
var tabLabels = []string{"Search", "Transfers", "Library", "Chat", "Network", "Settings"}

// Data structures for different panels
type searchResult struct {
	FileName string
	Peer     string
	Size     string
	rawSize  int64
}

type libraryFile struct {
	Name    string
	IsDir   bool
	Size    string
	rawSize int64
	Type    string // "Shared" or "Downloaded"
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

// Main Model
type Model struct {
	Nickname string
	Key      string
	Width    int
	Height   int
	Cursor   int

	// State
	CurrentTab tab
	InputMode  bool // For search box
	Input      string

	// Chat
	chatClient    *ChatClient
	chatInput     string
	chatInputMode bool

	// Data stores for panels
	SearchResults []searchResult
	LibraryFiles  []libraryFile // A single list for the library panel
	Peers         []peer
	Logs          []logEntry
}

// --- Bubble Tea Messages ---

type searchResultsMsg []searchResult
type sharedFilesLoadedMsg []libraryFile
type downloadsLoadedMsg []libraryFile
type chatLineMsg string

// --- Styles ---

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

func NewModel(nickname, key string, client *ChatClient) Model {
	return Model{
		Nickname:      nickname,
		Key:           key,
		chatClient:    client,
		SearchResults: []searchResult{},
		LibraryFiles:  []libraryFile{},
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
		listenForChat(m.chatClient),
		ScanSharedCmd(),
		ScanDownloadsCmd(),
	)
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	// Handle newly scanned shared files
	case sharedFilesLoadedMsg:
		// Remove old shared files and add new ones
		m.LibraryFiles = filterLibrary(m.LibraryFiles, "Downloaded")
		m.LibraryFiles = append(m.LibraryFiles, msg...)
		// Notify server of our shared files
		return m, NotifyServerOfSharedFilesCmd(m.chatClient, msg)

	// Handle newly scanned downloaded files
	case downloadsLoadedMsg:
		// Remove old downloaded files and add new ones
		m.LibraryFiles = filterLibrary(m.LibraryFiles, "Shared")
		m.LibraryFiles = append(m.LibraryFiles, msg...)
		return m, nil

	case searchResultsMsg:
		m.SearchResults = msg
		return m, nil

	case chatLineMsg:
		entry := ParseChatLine(string(msg))
		m.Logs = append(m.Logs, logEntry{
			Time:    entry.Time,
			Message: fmt.Sprintf("%s: %s", entry.Sender, entry.Message),
		})
		return m, listenForChat(m.chatClient)

	case logEntry:
		m.Logs = append(m.Logs, msg)
		return m, nil

	case tea.KeyMsg:
		// Handle chat input mode
		if m.CurrentTab == tabChat && m.chatInputMode {
			return updateChatInput(m, msg)
		}
		// Handle search input mode
		if m.CurrentTab == tabSearch && m.InputMode {
			var cmd tea.Cmd
			m, cmd = updateSearchInput(m, msg)
			return m, cmd
		}
		// Handle global key presses
		return updateGlobalKeys(m, msg)

	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
	}
	return m, nil
}

func (m Model) View() string {
	var b strings.Builder

	// Header
	header := pinkHeader.Width(m.Width).Render(fmt.Sprintf("🌹 RoseWire - [%s | %s]", m.Nickname, filepath.Base(m.Key)))
	b.WriteString(header + "\n")

	// Tabs
	var tabViews []string
	for i, label := range tabLabels {
		style := tabStyle
		if tab(i) == m.CurrentTab {
			style = activeTabStyle
		}
		tabViews = append(tabViews, style.Render(label))
	}
	tabsRow := lipgloss.JoinHorizontal(lipgloss.Top, tabViews...)
	b.WriteString(lipgloss.NewStyle().Width(m.Width).Render(tabsRow) + "\n")
	b.WriteString(lipgloss.NewStyle().Foreground(pink).Width(m.Width).Render(strings.Repeat("─", m.Width)) + "\n")

	// Panel Content
	switch m.CurrentTab {
	case tabSearch:
		b.WriteString(renderSearchPanel(m))
	case tabTransfers:
		b.WriteString(renderTransfersPanel(m))
	case tabLibrary:
		b.WriteString(renderLibraryPanel(m))
	case tabChat:
		b.WriteString(renderChatPanel(m))
	case tabNetwork:
		b.WriteString(renderNetworkPanel(m))
	case tabSettings:
		b.WriteString(renderSettingsPanel(m))
	}

	// Footer
	footer := footerStyle.Width(m.Width).Render("[Tab] Switch Panel  [↑/↓] Move  [Enter] Select/Edit/Chat  [R] Refresh [Q] Quit")
	b.WriteString("\n" + footer)
	return b.String()
}

// --- Update Helpers ---

func updateGlobalKeys(m Model, msg tea.KeyMsg) (Model, tea.Cmd) {
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
		m.Cursor++ // We'll clamp this in each panel's view logic
	case "enter":
		if m.CurrentTab == tabSearch && !m.InputMode {
			m.InputMode = true
			m.Input = ""
		} else if m.CurrentTab == tabChat && !m.chatInputMode {
			m.chatInputMode = true
		}
	case "r": // Refresh list
		if m.CurrentTab == tabLibrary {
			return m, tea.Batch(ScanSharedCmd(), ScanDownloadsCmd())
		}
	}
	return m, nil
}

func updateSearchInput(m Model, msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.InputMode = false
		if strings.TrimSpace(m.Input) != "" {
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
	return m, nil
}

func updateChatInput(m Model, msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if strings.TrimSpace(m.chatInput) != "" {
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

// --- Chat Listener ---

// listenForChat dispatches between search results and chat messages.
func listenForChat(c *ChatClient) tea.Cmd {
	return func() tea.Msg {
		line, ok := <-c.Receive()
		if !ok {
			return nil // Channel closed
		}
		// Handle search results command from server
		if strings.HasPrefix(line, "[SEARCH] ") {
			payload := strings.TrimPrefix(line, "[SEARCH] ")
			return ParseSearchResults(payload)
		}
		// Default to a regular chat line
		return chatLineMsg(line)
	}
}

// utility to filter the library slice
func filterLibrary(files []libraryFile, keepType string) []libraryFile {
	var filtered []libraryFile
	for _, f := range files {
		if f.Type == keepType {
			filtered = append(filtered, f)
		}
	}
	return filtered
}