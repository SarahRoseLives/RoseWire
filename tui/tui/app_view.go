// tui/app_view.go
package tui

import (
	"fmt"
	"log"
	"rosetui/profile"
	"rosetui/ssh"
	"strings"

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
}

func NewAppModel(p profile.Profile, server string) AppModel {
	msgChan := make(chan tea.Msg)
	chat := NewChatPanel()

	// Create placeholder models for other panels for now
	placeholder := newPlaceholderPanel("Coming Soon!")

	return AppModel{
		profile:     p,
		serverAddr:  server,
		msgChan:     msgChan,
		sshService:  ssh.NewService(p, server, msgChan),
		activePanel: panelChat,
		panels: map[activePanel]tea.Model{
			// **FIX:** No longer need to take the address with `&`
			panelChat:      chat,
			panelSearch:    &placeholder,
			panelLibrary:   &placeholder,
			panelTransfers: &placeholder,
			panelNetwork:   &placeholder,
		},
		statusMessage: "Initializing...",
		styles:        DefaultAppStyles(),
	}
}

// listenForMessages is a command that waits for messages from the sshService channel.
func (m *AppModel) listenForMessages() tea.Cmd {
	return func() tea.Msg {
		return <-m.msgChan // Block until a message is received
	}
}

func (m AppModel) Init() tea.Cmd {
	return tea.Batch(m.sshService.Connect(), m.listenForMessages())
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		navWidth := m.styles.Nav.GetWidth()
		contentWidth := m.width - navWidth
		contentHeight := m.height - 2
		m.panels[panelChat].(*chatPanelModel).SetSize(contentWidth, contentHeight)

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.sshService.Close()
			return m, tea.Quit
		case "up":
			if m.activePanel > 0 {
				m.activePanel--
			}
		case "down":
			if m.activePanel < panelNetwork {
				m.activePanel++
			}
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
		if m.panels[panelChat] != nil {
			updatedPanel, cmd := m.panels[panelChat].Update(msg)
			m.panels[panelChat] = updatedPanel
			cmds = append(cmds, cmd)
		}
		cmds = append(cmds, m.listenForMessages())

	case ssh.SystemBroadcastMsg, ssh.ChatBroadcastMsg:
		if m.panels[panelChat] != nil {
			updatedPanel, cmd := m.panels[panelChat].Update(msg)
			m.panels[panelChat] = updatedPanel
			cmds = append(cmds, cmd)
		}
		cmds = append(cmds, m.listenForMessages())

	case ssh.ErrorMsg:
		m.statusMessage = "ERROR: " + msg.Error()

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

// --- Placeholder Panel ---
type placeholderPanel struct{ text string }

func newPlaceholderPanel(text string) placeholderPanel { return placeholderPanel{text} }
func (p placeholderPanel) Init() tea.Cmd               { return nil }
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
		Nav: lipgloss.NewStyle().
			Width(15).
			Margin(0, 2),
		NavActive:   lipgloss.NewStyle().Foreground(lipgloss.Color("#EC4899")).Bold(true),
		NavInactive: lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
	}
}