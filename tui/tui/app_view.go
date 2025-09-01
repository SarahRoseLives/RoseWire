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

// A command to trigger a reconnect attempt.
type attemptReconnectMsg struct{}

type AppModel struct {
	profile             profile.Profile
	serverAddr          string
	width, height       int
	sshService          *ssh.Service
	msgChan             chan tea.Msg // Channel for SSH service to send messages
	activePanel         activePanel
	panels              map[activePanel]tea.Model
	statusMessage       string
	currentUserIdentity string // Store the user's full @user@host identity
	styles              *AppStyles

	// State for handling automatic reconnection.
	isReconnecting    bool
	reconnectAttempts int
}

func NewAppModel(p profile.Profile, server string) AppModel {
	msgChan := make(chan tea.Msg)
	chat := NewChatPanel()
	search := NewSearchPanel()

	placeholder := newPlaceholderPanel("Coming Soon!")

	return AppModel{
		profile:      p,
		serverAddr:   server,
		msgChan:      msgChan,
		sshService:   ssh.NewService(p, server, msgChan),
		activePanel:  panelChat,
		panels: map[activePanel]tea.Model{
			panelChat:      chat,
			panelSearch:    search,
			panelLibrary:   &placeholder,
			panelTransfers: &placeholder,
			panelNetwork:   &placeholder,
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

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// **FIX:** Correctly calculate the navigation panel's total width.
		// The `GetMargin()` method returns all four margin values.
		navStyle := m.styles.Nav
		_, rightMargin, _, leftMargin := navStyle.GetMargin()
		navWidth := navStyle.GetWidth() + leftMargin + rightMargin

		contentWidth := m.width - navWidth

		// The header (1), footer (1), and help (1) bars take up 3 lines of height.
		contentHeight := m.height - 3

		m.panels[panelChat].(*chatPanelModel).SetSize(contentWidth, contentHeight)
		m.panels[panelSearch].(*searchPanelModel).SetSize(contentWidth, contentHeight)

	case tea.KeyMsg:
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
		default:
			if m.panels[m.activePanel] != nil {
				updatedPanel, cmd := m.panels[m.activePanel].Update(msg)
				m.panels[m.activePanel] = updatedPanel
				cmds = append(cmds, cmd)
			}
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
		if m.panels[panelChat] != nil {
			updatedPanel, cmd := m.panels[panelChat].Update(msg)
			m.panels[panelChat] = updatedPanel
			cmds = append(cmds, cmd)
		}
		cmds = append(cmds, m.listenForMessages())

	case ssh.ChatBroadcastMsg, ssh.SystemBroadcastMsg:
		if m.panels[panelChat] != nil {
			updatedPanel, cmd := m.panels[panelChat].Update(msg)
			m.panels[panelChat] = updatedPanel
			cmds = append(cmds, cmd)
		}
		cmds = append(cmds, m.listenForMessages())

	case ssh.SearchResultsMsg:
		if m.panels[panelSearch] != nil {
			updatedPanel, cmd := m.panels[panelSearch].Update(msg)
			m.panels[panelSearch] = updatedPanel
			cmds = append(cmds, cmd)
		}
		cmds = append(cmds, m.listenForMessages())

	case ssh.ErrorMsg:
		m.statusMessage = "ERROR: " + msg.Error()
		if m.isReconnecting {
			// Calculate exponential backoff: 5s, 10s, 20s, 30s, 30s...
			maxDelay := 30.0
			delaySeconds := math.Min(float64(5*m.reconnectAttempts), maxDelay)
			backoff := time.Duration(delaySeconds) * time.Second
			m.statusMessage += fmt.Sprintf(" Retrying in %.0f seconds...", backoff.Seconds())
			cmd := tea.Tick(backoff, func(t time.Time) tea.Msg { return attemptReconnectMsg{} })
			cmds = append(cmds, cmd)
		}
		cmds = append(cmds, m.listenForMessages())

	// Handle disconnection and initiate reconnection process.
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

	// Handle the scheduled reconnection attempt.
	case attemptReconnectMsg:
		if m.isReconnecting {
			m.reconnectAttempts++
			m.statusMessage = fmt.Sprintf("Reconnecting... (attempt %d)", m.reconnectAttempts)
			cmds = append(cmds, m.sshService.Connect(), m.listenForMessages())
		}

	case fetchTopFilesCmd:
		m.statusMessage = "Fetching top files..."
		err := m.sshService.FetchTopFiles()
		if err != nil {
			log.Printf("Error fetching top files: %v", err)
		}

	case searchFilesCmd:
		query := string(msg)
		if query == "" {
			m.statusMessage = "Fetching top files..."
			err := m.sshService.FetchTopFiles()
			if err != nil {
				log.Printf("Error fetching top files: %v", err)
			}
		} else {
			m.statusMessage = fmt.Sprintf("Searching for '%s'...", query)
			err := m.sshService.SearchFiles(query)
			if err != nil {
				log.Printf("Error searching: %v", err)
			}
		}

	case sendChatMsgCmd:
		err := m.sshService.SendMessage(string(msg))
		if err != nil {
			log.Printf("Error sending message: %v", err)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m AppModel) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	nav := m.renderNav()
	content := m.panels[m.activePanel].View()

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
	return m.styles.Help.Width(m.width).Render("Tab/Shift+Tab: Switch Panels • ↑/↓: Navigate List • Enter: Select • Ctrl+C: Quit")
}

// --- Placeholder Panel ---
type placeholderPanel struct{ text string }

func newPlaceholderPanel(text string) placeholderPanel { return placeholderPanel{text} }
func (p placeholderPanel) Init() tea.Cmd                  { return nil }
func (p placeholderPanel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return p, nil
}
func (p placeholderPanel) View() string {
	return lipgloss.NewStyle().Padding(2, 2).Render(p.text)
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