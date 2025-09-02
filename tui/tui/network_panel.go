// tui/network_panel.go
package tui

import (
	"fmt"
	"rosetui/ssh"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// A command to request a network stats refresh.
type requestNetworkStatsCmd struct{}

type networkPanelModel struct {
	isLoading bool
	stats     ssh.NetworkStatsMsg
	spinner   spinner.Model
	viewport  viewport.Model
	styles    *NetworkPanelStyles
	width     int
	height    int
}

func NewNetworkPanel() *networkPanelModel {
	styles := DefaultNetworkPanelStyles()
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = styles.Spinner

	vp := viewport.New(80, 20) // Dynamic
	vp.Style = styles.UserList

	return &networkPanelModel{
		isLoading: true,
		spinner:   sp,
		viewport:  vp,
		styles:    styles,
	}
}

func (m *networkPanelModel) Init() tea.Cmd {
	m.isLoading = true
	return tea.Batch(m.spinner.Tick, func() tea.Msg {
		return requestNetworkStatsCmd{}
	})
}

func (m *networkPanelModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var vpCmd, spCmd tea.Cmd

	switch msg := msg.(type) {
	case ssh.NetworkStatsMsg:
		m.isLoading = false
		m.stats = msg
		m.updateViewportContent()

	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			m.isLoading = true
			cmds = append(cmds, m.spinner.Tick, func() tea.Msg {
				return requestNetworkStatsCmd{}
			})
		default:
			m.viewport, vpCmd = m.viewport.Update(msg)
			cmds = append(cmds, vpCmd)
		}
	}

	m.spinner, spCmd = m.spinner.Update(msg)
	cmds = append(cmds, spCmd)
	return m, tea.Batch(cmds...)
}

func (m *networkPanelModel) View() string {
	if m.isLoading {
		return fmt.Sprintf("\n %s Fetching network stats...", m.spinner.View())
	}

	statsHeader := m.renderStatsHeader()
	usersHeader := m.styles.Header.Render("Users on the Network")
	userList := m.viewport.View()

	return lipgloss.JoinVertical(lipgloss.Left,
		statsHeader,
		usersHeader,
		userList,
	)
}

func (m *networkPanelModel) renderStatsHeader() string {
	users := m.styles.StatBox.Render(
		fmt.Sprintf("%d\n%s", m.stats.TotalUsers, "Users Online"),
	)
	relays := m.styles.StatBox.Render(
		fmt.Sprintf("%d\n%s", m.stats.RelayServers, "Relay Servers"),
	)
	active := m.styles.StatBox.Render(
		fmt.Sprintf("%d\n%s", m.stats.ActiveTransfers, "Active Transfers"),
	)
	total := m.styles.StatBox.Render(
		fmt.Sprintf("%d\n%s", m.stats.TotalTransfers, "Total Transfers"),
	)

	return lipgloss.JoinHorizontal(lipgloss.Top, users, relays, active, total)
}

func (m *networkPanelModel) updateViewportContent() {
	var b strings.Builder
	for _, user := range m.stats.Users {
		statusStyle := m.styles.StatusOffline
		if user.Status == "Online" {
			statusStyle = m.styles.StatusOnline
		}
		status := statusStyle.Render("● " + user.Status)
		nick := m.styles.Nickname.Render(user.Nickname)
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, status, nick))
		b.WriteString("\n")
	}
	m.viewport.SetContent(b.String())
}

func (m *networkPanelModel) SetSize(w, h int) {
	m.width = w
	m.height = h

	// **FIX:** The stats boxes (6 lines) and the header (3 lines) take up 9 lines total.
	const fixedHeight = 9
	m.viewport.Width = w
	m.viewport.Height = h - fixedHeight
	m.styles.UserList.Width(w)
	m.styles.UserList.Height(h - fixedHeight)

	targetBoxWidth := w / 4
	horizontalFrame := m.styles.StatBox.GetHorizontalFrameSize()
	_, rightMargin, _, leftMargin := m.styles.StatBox.GetMargin()
	horizontalMargin := leftMargin + rightMargin
	innerContentWidth := targetBoxWidth - horizontalFrame - horizontalMargin

	m.styles.StatBox.Width(innerContentWidth)

	if len(m.stats.Users) > 0 {
		m.updateViewportContent()
	}
}

// --- Styling ---

type NetworkPanelStyles struct {
	Spinner         lipgloss.Style
	Header          lipgloss.Style
	StatBox         lipgloss.Style
	UserList        lipgloss.Style
	StatusOnline    lipgloss.Style
	StatusOffline   lipgloss.Style
	Nickname        lipgloss.Style
}

func DefaultNetworkPanelStyles() *NetworkPanelStyles {
	return &NetworkPanelStyles{
		Spinner: lipgloss.NewStyle().Foreground(lipgloss.Color("205")),
		Header: lipgloss.NewStyle().
			Bold(true).
			Padding(1, 0, 1, 2),
		StatBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(1, 2).
			Margin(0, 1).
			Align(lipgloss.Center),
		UserList: lipgloss.NewStyle().Padding(0, 2),
		StatusOnline: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#26C281")). // Green
			Width(12),
		StatusOffline: lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Width(12),
		Nickname: lipgloss.NewStyle().Bold(true),
	}
}