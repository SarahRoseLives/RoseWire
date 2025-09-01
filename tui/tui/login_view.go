// tui/login_view.go
package tui

import (
	"fmt"
	"rosetui/config"
	"rosetui/profile"
	"rosetui/ssh"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// --- Message Types for async operations ---

type profilesLoadedMsg struct{ profiles []profile.Profile }
type profileCreatedMsg struct{ profile profile.Profile }
type loginSuccessMsg struct{ Profile profile.Profile }
type errMsg struct{ err error }

func (e errMsg) Error() string { return e.err.Error() }

// --- Model ---

type LoginModel struct {
	// State
	profiles      []profile.Profile
	focusIndex    int
	creatingNew   bool
	cursor        int
	statusMessage string
	isLoading     bool
	width, height int // For window size

	// UI Components
	inputs []textinput.Model

	// Styling
	styles *LoginStyles
}

// NewLoginModel initializes the login view's state and UI components.
func NewLoginModel() LoginModel {
	styles := DefaultStyles()
	m := LoginModel{
		styles: styles,
	}

	m.inputs = make([]textinput.Model, 2)
	var t textinput.Model

	// Input for Server Address
	t = textinput.New()
	t.Placeholder = "rosewire.rosevines.network"
	t.Focus()
	t.CharLimit = 64
	t.Width = 40
	t.Prompt = "┃ "
	t.PromptStyle = styles.FocusedStyle
	t.TextStyle = styles.FocusedStyle
	m.inputs[0] = t

	// Input for New Nickname
	t = textinput.New()
	t.Placeholder = "Your Nickname"
	t.CharLimit = 32
	t.Width = 30
	t.Prompt = "┃ "
	m.inputs[1] = t

	return m
}

// --- Bubble Tea Interface ---

func (m LoginModel) Init() tea.Cmd {
	return tea.Batch(
		loadProfilesCmd,
		loadConfigCmd,
		textinput.Blink,
	)
}

func (m LoginModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	// --- Window & Async Message Handling ---
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case config.AppConfig:
		m.inputs[0].SetValue(msg.ServerAddress)
		return m, nil

	case profilesLoadedMsg:
		m.profiles = msg.profiles
		m.isLoading = false
		return m, nil

	case profileCreatedMsg:
		m.isLoading = true
		m.statusMessage = "Profile created! Logging in..."
		return m, loginCmd(msg.profile, m.inputs[0].Value())

	case loginSuccessMsg:
		return m, func() tea.Msg { return msg }

	case errMsg:
		m.isLoading = false
		m.statusMessage = "Error: " + msg.err.Error()
		return m, nil

	// --- UI Interaction Handling ---
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			if m.creatingNew {
				m.creatingNew = false
				m.statusMessage = ""
				m.inputs[1].Blur() // Ensure new nick input is blurred
				m.inputs[0].Focus()
				m.focusIndex = 0
				return m, nil
			}
			return m, tea.Quit

		case tea.KeyTab, tea.KeyShiftTab, tea.KeyUp, tea.KeyDown:
			if m.isLoading {
				return m, nil
			}
			if m.creatingNew {
				return handleCreateNewNav(m, msg)
			}
			return handleLoginListNav(m, msg)

		case tea.KeyEnter:
			if m.isLoading {
				return m, nil
			}
			if m.creatingNew {
				return handleSubmitCreateNew(m)
			}
			return handleLogin(m)
		}
	}

	// Handle text input updates
	return updateInputs(m, msg)
}

// view renders the entire login screen, centering the dialog.
func (m LoginModel) View() string {
	if m.width == 0 {
		return "Initializing..."
	}

	var viewContent string
	if m.isLoading {
		// Simple loading view
		viewContent = fmt.Sprintf("\n %s %s\n", m.styles.SpinnerStyle.Render("●"), m.statusMessage)
	} else if m.creatingNew {
		viewContent = m.viewCreateNew()
	} else {
		viewContent = m.viewLoginList()
	}

	// Combine the main view with the help footer
	dialog := lipgloss.JoinVertical(lipgloss.Center, viewContent, m.viewHelp())

	// Place the dialog in the center of the screen
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
		m.styles.DialogBox.Render(dialog),
	)
}

// --- Commands for async operations ---

func loadProfilesCmd() tea.Msg {
	profiles, err := profile.LoadProfiles()
	if err != nil {
		return errMsg{err}
	}
	return profilesLoadedMsg{profiles}
}

func loadConfigCmd() tea.Msg {
	cfg, err := config.LoadConfig()
	if err != nil {
		return errMsg{err}
	}
	return cfg
}

func createProfileCmd(nickname string) tea.Cmd {
	return func() tea.Msg {
		p, err := profile.CreateProfile(nickname)
		if err != nil {
			return errMsg{err}
		}
		return profileCreatedMsg{p}
	}
}

func loginCmd(p profile.Profile, serverAddress string) tea.Cmd {
	return func() tea.Msg {
		// Save the server address for next time
		err := config.SaveConfig(config.AppConfig{ServerAddress: serverAddress})
		if err != nil {
			return errMsg{err}
		}
		// Test the SSH connection
		err = ssh.TestConnection(p, serverAddress)
		if err != nil {
			return errMsg{err}
		}
		return loginSuccessMsg{p}
	}
}

// --- View Rendering Logic ---

// ASCII art logo for "RoseWire"
const logo = `
  _____             __          ___
 |  __ \            \ \        / (_)
 | |__) |___  ___  __\ \  /\  / / _ _ __ ___
 |  _  // _ \/ __|/ _ \ \/  \/ / | | '__/ _ \
 | | \ \ (_) \__ \  __/\  /\  /  | | | |  __/
 |_|  \_\___/|___/\___| \/  \/   |_|_|  \___|
`

// viewLoginList renders the main view with the list of profiles.
func (m LoginModel) viewLoginList() string {
	var b strings.Builder

	b.WriteString(m.styles.LogoStyle.Render(logo))
	b.WriteString("\nSelect a profile or create a new one.\n\n")

	for i, p := range m.profiles {
		b.WriteString(m.renderChoice(i, p.Nickname))
	}
	b.WriteString(m.renderChoice(len(m.profiles), "Create New Profile..."))

	b.WriteString("\n\nInstance Address:\n")
	b.WriteString(m.inputs[0].View() + "\n")
	b.WriteString(m.renderButton("Login", m.focusIndex == 1))

	// Render status message if it exists
	if m.statusMessage != "" {
		b.WriteString("\n\n" + m.styles.ErrorStyle.Render(m.statusMessage))
	}

	return b.String()
}

// viewCreateNew renders the form for creating a new profile.
func (m LoginModel) viewCreateNew() string {
	var b strings.Builder
	b.WriteString(m.styles.LogoStyle.Render(logo))
	b.WriteString("\nCreate a New Profile\n\n")
	b.WriteString("Enter a nickname for your new identity:\n")
	b.WriteString(m.inputs[1].View() + "\n\n")
	b.WriteString(m.renderButton("Generate & Login", m.focusIndex == 1))

	if m.statusMessage != "" {
		b.WriteString("\n\n" + m.styles.ErrorStyle.Render(m.statusMessage))
	}

	return b.String()
}

// viewHelp renders the footer with key instructions.
func (m LoginModel) viewHelp() string {
	return m.styles.HelpStyle.Render("\n↑/↓: navigate • tab: switch focus • enter: select • esc: back • ctrl+c: quit")
}

// renderChoice styles and renders a single line in the profile list.
func (m LoginModel) renderChoice(idx int, choice string) string {
	// Is this choice the currently selected one?
	isFocused := m.cursor == idx && m.focusIndex == 0

	if isFocused {
		return m.styles.FocusedItem.Render("> " + choice)
	}
	return m.styles.BlurredItem.Render("  " + choice)
}

// renderButton renders a styled button.
func (m LoginModel) renderButton(text string, isFocused bool) string {
	if isFocused {
		return m.styles.FocusedButton.Render(text)
	}
	return m.styles.BlurredButton.Render(text)
}

// --- Update Logic Helpers ---

func handleLoginListNav(m LoginModel, msg tea.KeyMsg) (LoginModel, tea.Cmd) {
	switch msg.String() {
	case "up":
		if m.focusIndex == 0 { // Navigate list
			m.cursor--
			if m.cursor < 0 {
				m.cursor = len(m.profiles)
			}
		}
	case "down":
		if m.focusIndex == 0 { // Navigate list
			m.cursor++
			if m.cursor > len(m.profiles) {
				m.cursor = 0
			}
		}
	case "tab", "shift+tab":
		m.focusIndex = (m.focusIndex + 1) % 2 // 0=list/input, 1=button
		if m.focusIndex == 0 {
			m.inputs[0].Focus()
		} else {
			m.inputs[0].Blur()
		}
	}
	return m, nil
}

func handleCreateNewNav(m LoginModel, msg tea.KeyMsg) (LoginModel, tea.Cmd) {
	if msg.String() == "tab" || msg.String() == "shift+tab" {
		m.focusIndex = (m.focusIndex + 1) % 2 // 0=input, 1=button
		if m.focusIndex == 0 {
			m.inputs[1].Focus()
		} else {
			m.inputs[1].Blur()
		}
	}
	return m, nil
}

func handleLogin(m LoginModel) (LoginModel, tea.Cmd) {
	// Pressed enter on the button or the list
	if m.focusIndex == 1 || m.focusIndex == 0 {
		serverAddress := m.inputs[0].Value()
		if serverAddress == "" {
			m.statusMessage = "Server address cannot be empty."
			return m, nil
		}

		if m.cursor == len(m.profiles) {
			// "Create New Profile" is selected
			m.creatingNew = true
			m.statusMessage = ""
			m.focusIndex = 0
			m.inputs[0].Blur()
			m.inputs[1].Focus()
		} else if len(m.profiles) > 0 && m.cursor < len(m.profiles) {
			// A profile is selected, attempt login
			m.isLoading = true
			m.statusMessage = "Connecting..."
			selectedProfile := m.profiles[m.cursor]
			return m, loginCmd(selectedProfile, serverAddress)
		}
	}
	return m, nil
}

func handleSubmitCreateNew(m LoginModel) (LoginModel, tea.Cmd) {
	// Only submit if the button is focused or enter is pressed in the text field
	if m.focusIndex == 1 || m.focusIndex == 0 {
		nickname := strings.TrimSpace(m.inputs[1].Value())
		if nickname == "" {
			m.statusMessage = "Nickname cannot be empty."
			return m, nil
		}
		m.isLoading = true
		m.statusMessage = "Generating new keys..."
		return m, createProfileCmd(nickname)
	}
	return m, nil
}

func updateInputs(m LoginModel, msg tea.Msg) (LoginModel, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	// Update the appropriate input based on the current view
	if m.creatingNew {
		m.inputs[1], cmd = m.inputs[1].Update(msg)
		cmds = append(cmds, cmd)
	} else {
		m.inputs[0], cmd = m.inputs[0].Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// --- Styling ---

type LoginStyles struct {
	DialogBox     lipgloss.Style
	LogoStyle     lipgloss.Style
	HelpStyle     lipgloss.Style
	FocusedStyle  lipgloss.Style
	BlurredStyle  lipgloss.Style
	FocusedItem   lipgloss.Style
	BlurredItem   lipgloss.Style
	FocusedButton lipgloss.Style
	BlurredButton lipgloss.Style
	SpinnerStyle  lipgloss.Style
	ErrorStyle    lipgloss.Style
}

func DefaultStyles() *LoginStyles {
	var s LoginStyles

	s.DialogBox = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#EC4899")).
		Padding(1, 2).
		BorderTop(true).BorderLeft(true).BorderRight(true).BorderBottom(true)

	s.LogoStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#EC4899")).
		Bold(true)

	s.HelpStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))

	s.FocusedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	s.BlurredStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	s.FocusedItem = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#EC4899")).
		PaddingLeft(1)

	s.BlurredItem = lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		PaddingLeft(1)

	s.FocusedButton = lipgloss.NewStyle().
		Background(lipgloss.Color("#EC4899")).
		Foreground(lipgloss.Color("230")).
		Bold(true).
		Padding(0, 3).
		MarginTop(1)

	s.BlurredButton = lipgloss.NewStyle().
		Foreground(lipgloss.Color("250")).
		Bold(true).
		Padding(0, 3).
		MarginTop(1)

	s.SpinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	s.ErrorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#E74C3C")). // Red
		Bold(true)

	return &s
}