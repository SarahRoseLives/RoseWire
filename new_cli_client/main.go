package main

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

type Panel int

const (
	PanelSearch Panel = iota
	PanelTransfers
	PanelLibrary
	PanelChat
	PanelNetwork
	PanelSettings
	panelCount
)

var (
	colorBg            = lipgloss.Color("#191622")
	colorAccent        = lipgloss.Color("#ea76cb")
	colorInactive      = lipgloss.Color("#6e6a86")
	colorPanelText     = lipgloss.Color("#e0def4")
	colorStatusBg      = lipgloss.Color("#232136")
	colorStatusText    = lipgloss.Color("#ea76cb")
	colorTabBarBg      = lipgloss.Color("#232136")
	colorTabActiveBg   = lipgloss.Color("#2a273f")
	colorTabActiveFg   = lipgloss.Color("#ea76cb")
	colorTabInactiveFg = lipgloss.Color("#e0def4")
)

var tabBarLabels = []string{
	"RoseWire",
	"Search",
	"Transfers",
	"Library",
	"Chat",
	"Network",
	"Settings",
}

type chatMsg struct {
	time   string
	sender string
	text   string
	system bool
}

type model struct {
	activePanel   Panel
	width, height int

	chatInput   textinput.Model
	chatHistory []chatMsg
}

func initialModel() model {
	chatInput := textinput.New()
	chatInput.Placeholder = "Type your message and press Enter"
	chatInput.Prompt = ">> "
	chatInput.CharLimit = 140
	chatInput.PromptStyle = lipgloss.NewStyle().Foreground(colorInactive)
	chatInput.TextStyle = lipgloss.NewStyle().Foreground(colorPanelText)

	chatHistory := []chatMsg{
		{"[23:12]", "SYSTEM", "Welcome to RoseVines Chat!", true},
		{"[23:12]", "SYSTEM", "rose joined the chat.", true},
		{"[23:13]", "SYSTEM", "sarahrose joined the chat.", true},
	}

	return model{
		activePanel: PanelSearch,
		chatInput:   chatInput,
		chatHistory: chatHistory,
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			m.activePanel = (m.activePanel + 1) % panelCount
			return m, nil
		case "shift+tab":
			m.activePanel = (m.activePanel - 1 + panelCount) % panelCount
			return m, nil
		}
	}

	var cmd tea.Cmd
	switch m.activePanel {
	case PanelChat:
		m.chatInput, cmd = m.chatInput.Update(msg)
		if k, ok := msg.(tea.KeyMsg); ok && k.String() == "enter" {
			val := strings.TrimSpace(m.chatInput.Value())
			if val != "" {
				m.chatHistory = append(m.chatHistory, chatMsg{"[23:16]", "rose", val, false})
			}
			m.chatInput.SetValue("")
		}
	}
	return m, cmd
}

func (m model) View() string {
	return lipgloss.JoinVertical(lipgloss.Left,
		m.renderTabBar(),
		m.renderMain(),
		m.renderStatusbar(),
	)
}

func (m model) renderTabBar() string {
	var tabs []string
	for i, label := range tabBarLabels {
		if i == 0 {
			tabs = append(tabs, lipgloss.NewStyle().
				Foreground(colorAccent).
				Bold(true).
				Padding(0, 2).
				Render(label))
			continue
		}
		active := int(m.activePanel)+1 == i
		tabStyle := lipgloss.NewStyle().
			Padding(0, 2).
			Bold(active)
		if active {
			tabStyle = tabStyle.Foreground(colorTabActiveFg).Background(colorTabActiveBg)
		} else {
			tabStyle = tabStyle.Foreground(colorTabInactiveFg)
		}
		tabs = append(tabs, tabStyle.Render(label))
	}
	return lipgloss.NewStyle().
		Background(colorTabBarBg).
		Width(m.width).
		Padding(0, 0).
		Render(strings.Join(tabs, " | "))
}

func (m model) renderMain() string {
	switch m.activePanel {
	case PanelSearch:
		return m.renderPlaceholderPanel("Search")
	case PanelTransfers:
		return m.renderPlaceholderPanel("Transfers")
	case PanelLibrary:
		return m.renderPlaceholderPanel("Library")
	case PanelChat:
		return m.renderChatPanel()
	case PanelNetwork:
		return m.renderPlaceholderPanel("Network")
	case PanelSettings:
		return m.renderPlaceholderPanel("Settings")
	default:
		return ""
	}
}

func (m model) renderPlaceholderPanel(label string) string {
	return lipgloss.NewStyle().
		Foreground(colorInactive).
		Padding(3, 4).
		Width(m.width).
		Height(m.height - 4).
		Render(fmt.Sprintf("[%s panel placeholder]", label))
}

func (m model) renderChatPanel() string {
	header := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Foreground(colorAccent).Bold(true).Render("RoseVines Chat"),
		lipgloss.NewStyle().Foreground(colorInactive).PaddingLeft(2).Render("Username: rose"),
		lipgloss.NewStyle().Foreground(colorInactive).PaddingLeft(2).Render("Mode: SSH"),
		lipgloss.NewStyle().Foreground(colorInactive).PaddingLeft(2).Render("Connected to: Garden"),
	)
	var history strings.Builder
	for _, msg := range m.chatHistory {
		var line string
		if msg.system {
			line = lipgloss.NewStyle().Foreground(colorStatusText).Render(fmt.Sprintf("%s SYSTEM: %s", msg.time, msg.text))
		} else {
			line = lipgloss.NewStyle().Foreground(colorAccent).Render(fmt.Sprintf("%s %s: ", msg.time, msg.sender)) +
				lipgloss.NewStyle().Foreground(colorPanelText).Render(msg.text)
		}
		history.WriteString(line + "\n")
	}
	inputBox := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(colorAccent).
		Padding(0, 1).
		Width(m.width - 10).
		Render(m.chatInput.View())
	return lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Padding(1, 2).Render(header),
		lipgloss.NewStyle().Margin(1, 2).Render(history.String()),
		inputBox,
		lipgloss.NewStyle().Foreground(colorInactive).Padding(1, 2).Render("/quit to exit"),
	)
}

func (m model) renderStatusbar() string {
	status := ""
	switch m.activePanel {
	case PanelChat:
		status = "Connected via SSH as Rose"
	default:
		status = "RoseWire 2.0 - Modern Edition"
	}
	return lipgloss.NewStyle().
		Background(colorStatusBg).
		Foreground(colorStatusText).
		Padding(0, 2).
		Width(m.width).
		Render(status)
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if err := p.Start(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}