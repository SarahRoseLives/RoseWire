// tui/chat_panel.go
package tui

import (
	"fmt"
	"rosetui/ssh"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type chatPanelModel struct {
	viewport            viewport.Model
	textInput           textinput.Model
	messages            []string
	styles              *ChatPanelStyles
	currentUserIdentity string // The full @user@host identity
}

func NewChatPanel() *chatPanelModel {
	styles := DefaultChatPanelStyles()
	ti := textinput.New()
	ti.Placeholder = "Type a message and press Enter..."
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 80 // Will be dynamically set
	ti.Prompt = "┃ "
	ti.PromptStyle = styles.InputPrompt
	ti.TextStyle = styles.InputText

	vp := viewport.New(80, 20) // Will be dynamically set
	vp.Style = styles.Viewport

	return &chatPanelModel{ // Return as a pointer
		textInput: ti,
		viewport:  vp,
		styles:    styles,
	}
}

func (m *chatPanelModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *chatPanelModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
	)

	m.textInput, tiCmd = m.textInput.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			input := strings.TrimSpace(m.textInput.Value())
			if input != "" {
				// Also display the user's own message immediately.
				myMessage := m.styles.SelfMessage.Render(fmt.Sprintf("<%s> %s", m.currentUserIdentity, input))
				m.messages = append(m.messages, myMessage)
				m.updateViewport()

				m.textInput.Reset()
				return m, func() tea.Msg {
					return sendChatMsgCmd(input)
				}
			}
		}

	case ssh.WelcomeMsg:
		m.currentUserIdentity = msg.Identity
		welcomeText := m.styles.SystemMessage.Render(fmt.Sprintf("[System] Welcome! You are %s", msg.Identity))
		m.messages = append(m.messages, welcomeText)
		m.updateViewport()

	case ssh.SystemBroadcastMsg:
		systemText := m.styles.SystemMessage.Render(fmt.Sprintf("[System] %s", msg.Text))
		m.messages = append(m.messages, systemText)
		m.updateViewport()

	case ssh.ChatBroadcastMsg:
		// Don't re-print our own messages that the server echoes back.
		if msg.Nickname == m.currentUserIdentity {
			break
		}
		chatText := m.styles.OtherMessage.Render(fmt.Sprintf("<%s> %s", msg.Nickname, msg.Text))
		m.messages = append(m.messages, chatText)
		m.updateViewport()
	}

	return m, tea.Batch(tiCmd, vpCmd)
}

func (m *chatPanelModel) updateViewport() {
	m.viewport.SetContent(strings.Join(m.messages, "\n"))
	m.viewport.GotoBottom()
}

func (m *chatPanelModel) View() string {
	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.viewport.View(),
		m.styles.InputBox.Render(m.textInput.View()),
	)
}

func (m *chatPanelModel) SetSize(w, h int) {
	m.styles.InputBox.Width(w)

	inputBoxHorizontalFrameSize := m.styles.InputBox.GetHorizontalFrameSize()
	m.textInput.Width = w - inputBoxHorizontalFrameSize - len(m.textInput.Prompt)

	m.viewport.Width = w
	// The input box takes up 3 lines of height (1 for top border, 1 for content, 1 for bottom border).
	m.viewport.Height = h - 3
	m.styles.Viewport.Width(w)
	m.styles.Viewport.Height(h - 3)
}

// --- Styling ---

type ChatPanelStyles struct {
	Viewport      lipgloss.Style
	InputBox      lipgloss.Style
	InputPrompt   lipgloss.Style
	InputText     lipgloss.Style
	SystemMessage lipgloss.Style
	SelfMessage   lipgloss.Style
	OtherMessage  lipgloss.Style
}

func DefaultChatPanelStyles() *ChatPanelStyles {
	return &ChatPanelStyles{
		Viewport: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(0, 1),
		InputBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("205")).
			Padding(0, 1),
		InputPrompt:   lipgloss.NewStyle().Foreground(lipgloss.Color("205")),
		InputText:     lipgloss.NewStyle().Foreground(lipgloss.Color("250")),
		SystemMessage: lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Italic(true),
		SelfMessage:   lipgloss.NewStyle().Foreground(lipgloss.Color("#3498DB")).Bold(true),   // Blue
		OtherMessage:  lipgloss.NewStyle().Foreground(lipgloss.Color("#1ABC9C")).Bold(false), // Teal
	}
}