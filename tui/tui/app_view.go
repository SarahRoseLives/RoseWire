// tui/app_view.go
package tui

import (
	"fmt"
	"log"
	"math"
	"rosetui/profile"
	"rosetui/ssh"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type activePanel uint

const (
	panelChat activePanel = iota
	panelSearch
	panelLibrary
	panelTransfers
	panelNetwork
)

var panelNames = map[activePanel]string{
	panelChat:      "Chat",
	panelSearch:    "Search",
	panelLibrary:   "Library",
	panelTransfers: "Transfers",
	panelNetwork:   "Network",
}

type attemptReconnectMsg struct{}

type AppModel struct {
	profile             profile.Profile
	serverAddr          string
	width, height       int
	sshService          *ssh.Service
	msgChan             chan tea.Msg
	activePanel         activePanel
	panels              map[activePanel]tea.Model
	statusMessage       string
	currentUserIdentity string
	styles              *AppStyles
	isReconnecting      bool
	reconnectAttempts   int
}

func NewAppModel(p profile.Profile, server string) AppModel {
	msgChan := make(chan tea.Msg)
	chat := NewChatPanel()
	search := NewSearchPanel()
	network := NewNetworkPanel()
	library := NewLibraryPanel()
	transfers := NewTransfersPanel()

	return AppModel{
		profile:      p,
		serverAddr:   server,
		msgChan:      msgChan,
		sshService:   ssh.NewService(p, server, msgChan),
		activePanel:  panelChat,
		// **FIX:** Ensure the new panels are correctly assigned, replacing the placeholders.
		panels: map[activePanel]tea.Model{
			panelChat:      chat,
			panelSearch:    search,
			panelLibrary:   library,
			panelTransfers: transfers,
			panelNetwork:   network,
		},
		statusMessage: "Initializing...",
		styles:        DefaultAppStyles(),
	}
}

func (m *AppModel) listenForMessages() tea.Cmd {
	return func() tea.Msg {
		return <-m.msgChan
	}
}

func (m AppModel) Init() tea.Cmd {
	return tea.Batch(
		m.sshService.Connect(),
		m.listenForMessages(),
		m.panels[m.activePanel].Init(),
	)
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Route messages to the correct panel first
	switch msg.(type) {
	case ssh.ChatBroadcastMsg, ssh.SystemBroadcastMsg, ssh.WelcomeMsg:
		if m.panels[panelChat] != nil {
			updatedPanel, cmd := m.panels[panelChat].Update(msg)
			m.panels[panelChat] = updatedPanel
			cmds = append(cmds, cmd)
		}
	case ssh.SearchResultsMsg:
		if m.panels[panelSearch] != nil {
			updatedPanel, cmd := m.panels[panelSearch].Update(msg)
			m.panels[panelSearch] = updatedPanel
			cmds = append(cmds, cmd)
		}
	case ssh.NetworkStatsMsg:
		if m.panels[panelNetwork] != nil {
			updatedPanel, cmd := m.panels[panelNetwork].Update(msg)
			m.panels[panelNetwork] = updatedPanel
			cmds = append(cmds, cmd)
		}
	case ssh.TransfersUpdateMsg:
		if m.panels[panelTransfers] != nil {
			updatedPanel, cmd := m.panels[panelTransfers].Update(msg)
			m.panels[panelTransfers] = updatedPanel
			cmds = append(cmds, cmd)
		}
	}

	// Handle global messages and commands
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		navStyle := m.styles.Nav
		_, rightMargin, _, leftMargin := navStyle.GetMargin()
		navWidth := navStyle.GetWidth() + leftMargin + rightMargin
		contentWidth := m.width - navWidth
		contentHeight := m.height - 3

		m.panels[panelChat].(*chatPanelModel).SetSize(contentWidth, contentHeight)
		m.panels[panelSearch].(*searchPanelModel).SetSize(contentWidth, contentHeight)
		m.panels[panelNetwork].(*networkPanelModel).SetSize(contentWidth, contentHeight)
		m.panels[panelLibrary].(*libraryPanelModel).SetSize(contentWidth, contentHeight)
		m.panels[panelTransfers].(*transfersPanelModel).SetSize(contentWidth, contentHeight)

	case tea.KeyMsg:
		// Let active panel handle its own keys first
		updatedPanel, cmd := m.panels[m.activePanel].Update(msg)
		m.panels[m.activePanel] = updatedPanel
		cmds = append(cmds, cmd)
		// Then handle global keys
		switch msg.String() {
		case "ctrl+c":
			m.sshService.Close()
			return m, tea.Quit
		case "tab":
			m.activePanel = (m.activePanel + 1) % 5
			cmds = append(cmds, m.panels[m.activePanel].Init())
		case "shift+tab":
			if m.activePanel == 0 {
				m.activePanel = panelNetwork
			} else {
				m.activePanel--
			}
			cmds = append(cmds, m.panels[m.activePanel].Init())
		}

	case ssh.StatusMsg:
		m.statusMessage = msg.Message
		cmds = append(cmds, m.listenForMessages())
	case ssh.WelcomeMsg:
		m.currentUserIdentity = msg.Identity
		if m.isReconnecting {
			m.isReconnecting = false
			m.reconnectAttempts = 0
			m.statusMessage = "Reconnected successfully!"
		}
		cmds = append(cmds, m.listenForMessages())

	// Generic listener append for messages handled by panels
	case ssh.ChatBroadcastMsg, ssh.SystemBroadcastMsg, ssh.SearchResultsMsg, ssh.NetworkStatsMsg, ssh.TransfersUpdateMsg:
		cmds = append(cmds, m.listenForMessages())

	case ssh.ErrorMsg:
		m.statusMessage = "ERROR: " + msg.Error()
		if m.isReconnecting {
			maxDelay := 30.0
			delaySeconds := math.Min(float64(5*m.reconnectAttempts), maxDelay)
			backoff := time.Duration(delaySeconds) * time.Second
			m.statusMessage += fmt.Sprintf(" Retrying in %.0f seconds...", backoff.Seconds())
			cmd := tea.Tick(backoff, func(t time.Time) tea.Msg { return attemptReconnectMsg{} })
			cmds = append(cmds, cmd)
		}
		cmds = append(cmds, m.listenForMessages())
	case ssh.DisconnectedMsg:
		if !m.isReconnecting {
			m.isReconnecting = true
			m.reconnectAttempts = 0
			delay := 5 * time.Second
			m.statusMessage = fmt.Sprintf("Connection lost. Retrying in %.0f seconds...", delay.Seconds())
			cmd := tea.Tick(delay, func(t time.Time) tea.Msg { return attemptReconnectMsg{} })
			cmds = append(cmds, cmd)
		}
		cmds = append(cmds, m.listenForMessages())
	case attemptReconnectMsg:
		if m.isReconnecting {
			m.reconnectAttempts++
			m.statusMessage = fmt.Sprintf("Reconnecting... (attempt %d)", m.reconnectAttempts)
			cmds = append(cmds, m.sshService.Connect(), m.listenForMessages())
		}

	// --- Panel-specific Commands ---
	case fetchTopFilesCmd:
		m.statusMessage = "Fetching top files..."
		if err := m.sshService.FetchTopFiles(); err != nil {
			log.Printf("Error fetching top files: %v", err)
		}
	case searchFilesCmd:
		query := string(msg)
		m.statusMessage = fmt.Sprintf("Searching for '%s'...", query)
		if err := m.sshService.SearchFiles(query); err != nil {
			log.Printf("Error searching: %v", err)
		}
	case sendChatMsgCmd:
		if err := m.sshService.SendMessage(string(msg)); err != nil {
			log.Printf("Error sending message: %v", err)
		}
	case requestNetworkStatsCmd:
		m.statusMessage = "Refreshing network stats..."
		if err := m.sshService.RequestNetworkStats(); err != nil {
			log.Printf("Error requesting stats: %v", err)
		}
	case shareFilesCmd:
		m.statusMessage = fmt.Sprintf("Sharing %d files...", len(msg.files))
		if err := m.sshService.ShareFiles(msg.files); err != nil {
			log.Printf("Error sharing files: %v", err)
		}
	case downloadFileCmd:
		m.statusMessage = fmt.Sprintf("Requesting download: %s", msg.file.FileName)
		if err := m.sshService.DownloadFile(msg.file); err != nil {
			log.Printf("Error requesting download: %v", err)
		}
		// Also switch to transfers panel
		m.activePanel = panelTransfers
	case retryDownloadCmd:
		m.statusMessage = fmt.Sprintf("Retrying download: %s", msg.transfer.FileName)
		searchResult := ssh.SearchResult{FileName: msg.transfer.FileName, Peer: msg.transfer.FromUser, Size: msg.transfer.Size}
		if err := m.sshService.DownloadFile(searchResult); err != nil {
			log.Printf("Error retrying download: %v", err)
		}

	// Route any other message to the active panel.
	// This is important for spinner ticks, etc.
	default:
		// Make sure panel isn't nil before updating.
		if m.panels[m.activePanel] != nil {
			updatedPanel, cmd := m.panels[m.activePanel].Update(msg)
			m.panels[m.activePanel] = updatedPanel
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m AppModel) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	nav := m.renderNav()
	content := ""
	if m.panels[m.activePanel] != nil {
		content = m.panels[m.activePanel].View()
	}

	main := lipgloss.JoinHorizontal(lipgloss.Top, nav, content)

	return lipgloss.JoinVertical(lipgloss.Left,
		m.renderHeader(),
		main,
		m.renderFooter(),
		m.renderHelp(),
	)
}

func (m AppModel) renderNav() string {
	var navItems []string
	for i := panelChat; i <= panelNetwork; i++ {
		if i == m.activePanel {
			navItems = append(navItems, m.styles.NavActive.Render("▶ "+panelNames[i]))
		} else {
			navItems = append(navItems, m.styles.NavInactive.Render("  "+panelNames[i]))
		}
	}
	return m.styles.Nav.Render(strings.Join(navItems, "\n"))
}

func (m AppModel) renderHeader() string {
	headerText := fmt.Sprintf(" RoseWire TUI | %s ", m.profile.Nickname)
	return m.styles.Header.Width(m.width).Render(headerText)
}

func (m AppModel) renderFooter() string {
	return m.styles.Footer.Width(m.width).Render(m.statusMessage)
}

func (m AppModel) renderHelp() string {
	baseHelp := "Tab/Shift+Tab: Switch Panels • Ctrl+C: Quit"
	var panelHelp string

	switch m.activePanel {
	case panelSearch:
		panelHelp = "↑/↓: Navigate • Enter: Download"
	case panelNetwork:
		panelHelp = "↑/↓: Scroll List • r: Refresh Stats"
	case panelLibrary:
		panelHelp = "↑/↓: Scroll List • s: Set Path • r: Refresh"
	case panelTransfers:
		panelHelp = "↑/↓: Select • r: Retry Failed"
	}

	if panelHelp != "" {
		return m.styles.Help.Width(m.width).Render(baseHelp + " • " + panelHelp)
	}
	return m.styles.Help.Width(m.width).Render(baseHelp)
}

// --- Styling ---
type AppStyles struct {
	Header      lipgloss.Style
	Footer      lipgloss.Style
	Help        lipgloss.Style
	Nav         lipgloss.Style
	NavActive   lipgloss.Style
	NavInactive lipgloss.Style
}

func DefaultAppStyles() *AppStyles {
	return &AppStyles{
		Header: lipgloss.NewStyle().
			Background(lipgloss.Color("#EC4899")).
			Foreground(lipgloss.Color("230")).
			Bold(true),
		Footer: lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("250")),
		Help: lipgloss.NewStyle().
			Background(lipgloss.Color("234")).
			Foreground(lipgloss.Color("244")),
		Nav: lipgloss.NewStyle().
			Width(15).
			Margin(0, 2),
		NavActive:   lipgloss.NewStyle().Foreground(lipgloss.Color("#EC4899")).Bold(true),
		NavInactive: lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
	}
}