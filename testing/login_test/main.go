package main

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type loginStep int

const (
	stepChooseAction loginStep = iota
	stepListKeys
	stepCreateKey
	stepEnterNickname
	stepDone
)

type focusState int

const (
	focusActionExisting focusState = iota
	focusActionCreate
	focusKeyList
	focusNickname
	focusDone
)

var (
	focusedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("219")).Bold(true)
	normalStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	cardStyle    = lipgloss.NewStyle().
			Width(44).
			Padding(1, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#ff81b3")).
			Align(lipgloss.Center).
			Background(lipgloss.Color("#1b1b1b"))
)

type loginModel struct {
	step      loginStep
	focus     focusState
	width     int
	height    int
	status    string
	loggedIn  bool

	keys        []string // List of public key file paths
	keyCursor   int
	selectedKey string

	creatingKey  bool
	createKeyMsg string

	nickname      string
	nicknameInput bool
}

func initialLoginModel() loginModel {
	return loginModel{
		step:  stepChooseAction,
		focus: focusActionExisting,
	}
}

func findSSHKeys() []string {
	usr, err := user.Current()
	if err != nil {
		return nil
	}
	var keys []string
	keyDir := filepath.Join(usr.HomeDir, ".ssh")
	patterns := []string{"id_ed25519.pub", "id_rsa.pub", "id_ecdsa.pub", "id_dsa.pub"}
	for _, p := range patterns {
		full := filepath.Join(keyDir, p)
		if _, err := os.Stat(full); err == nil {
			keys = append(keys, full)
		}
	}
	return keys
}

func (m loginModel) Init() tea.Cmd {
	return func() tea.Msg {
		return sshKeysMsg(findSSHKeys())
	}
}

type sshKeysMsg []string
type createKeyMsg string

func createSSHKeyCmd() tea.Cmd {
	return func() tea.Msg {
		usr, _ := user.Current()
		path := filepath.Join(usr.HomeDir, ".ssh", "id_ed25519.pub")
		// Simulate key creation
		return createKeyMsg(path)
	}
}

func (m loginModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case sshKeysMsg:
		m.keys = msg
		if len(m.keys) == 0 {
			m.status = "No SSH keys found! Please create a key."
			m.focus = focusActionCreate
		}
	case createKeyMsg:
		m.creatingKey = false
		m.createKeyMsg = fmt.Sprintf("Created new SSH key at %s", string(msg))
		m.keys = append(m.keys, string(msg))
		m.keyCursor = len(m.keys) - 1
		m.step = stepListKeys
		m.focus = focusKeyList
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case tea.KeyMsg:
		if m.loggedIn {
			if msg.String() == "q" || msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			return m, nil
		}
		switch m.step {
		case stepChooseAction:
			switch msg.String() {
			case "tab", "right":
				if m.focus == focusActionExisting {
					m.focus = focusActionCreate
				} else {
					m.focus = focusActionExisting
				}
			case "shift+tab", "left":
				if m.focus == focusActionCreate {
					m.focus = focusActionExisting
				} else {
					m.focus = focusActionCreate
				}
			case "enter":
				if m.focus == focusActionExisting {
					m.step = stepListKeys
					m.focus = focusKeyList
				} else {
					m.step = stepCreateKey
					m.creatingKey = true
					return m, createSSHKeyCmd()
				}
			}
		case stepListKeys:
			switch msg.String() {
			case "up", "k":
				if m.keyCursor > 0 {
					m.keyCursor--
				}
			case "down", "j":
				if m.keyCursor < len(m.keys)-1 {
					m.keyCursor++
				}
			case "enter":
				m.selectedKey = m.keys[m.keyCursor]
				m.step = stepEnterNickname
				m.focus = focusNickname
				m.nicknameInput = true
			case "esc":
				m.step = stepChooseAction
				m.focus = focusActionExisting
			}
		case stepCreateKey:
			if !m.creatingKey {
				m.step = stepListKeys
				m.focus = focusKeyList
			}
		case stepEnterNickname:
			if m.nicknameInput {
				switch msg.String() {
				case "enter":
					if strings.TrimSpace(m.nickname) == "" {
						m.status = "Nickname required"
					} else {
						m.status = ""
						m.step = stepDone
						m.focus = focusDone
						m.loggedIn = true
					}
				case "backspace":
					if len(m.nickname) > 0 {
						m.nickname = m.nickname[:len(m.nickname)-1]
					}
				default:
					if msg.Type == tea.KeyRunes {
						m.nickname += msg.String()
					}
				}
			}
		case stepDone:
			if msg.String() == "q" || msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m loginModel) View() string {
	if m.loggedIn && m.step == stepDone {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#ff81b3")).Align(lipgloss.Center).Render(
			fmt.Sprintf("\n\n🌹 Welcome, %s!\n\nYour key: %s\nPress Q to quit or continue into RoseWire.",
				m.nickname,
				filepath.Base(m.selectedKey),
			),
		)
	}

	card := ""
	title := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ff81b3")).
		Background(lipgloss.Color("#2b0036")).
		Padding(0, 1).
		Bold(true).
		Render("🌹 RoseWire Login")

	switch m.step {
	case stepChooseAction:
		card = fmt.Sprintf(
			"%s\n\n%s\n%s\n\n%s",
			title,
			option("Use existing SSH key", m.focus == focusActionExisting),
			option("Create new SSH key", m.focus == focusActionCreate),
			lipgloss.NewStyle().Foreground(lipgloss.Color("210")).Render(m.status),
		)
	case stepListKeys:
		card = title + "\n\nChoose your SSH key:\n\n"
		if len(m.keys) == 0 {
			card += "No SSH keys found! Press Esc to go back.\n"
		}
		for i, k := range m.keys {
			display := filepath.Base(k)
			if i == m.keyCursor {
				card += focusedStyle.Render("> " + display + "\n")
			} else {
				card += normalStyle.Render("  " + display + "\n")
			}
		}
		card += "\n[Enter] Select  [Esc] Back"
	case stepCreateKey:
		card = title + "\n\n"
		if m.creatingKey {
			card += focusedStyle.Render("Creating new SSH key...") + "\n"
		} else {
			card += fmt.Sprintf("SSH key created: %s\n\n[Press any key to continue]", m.createKeyMsg)
		}
	case stepEnterNickname:
		card = title + "\n\nEnter your nickname:\n\n"
		entry := m.nickname
		if m.nicknameInput {
			entry += "_"
		}
		card += focusedStyle.Render(entry) + "\n"
		card += "\n[Enter] Continue"
		if m.status != "" {
			card += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("210")).Render(m.status)
		}
	}

	rendered := cardStyle.Render(card)
	lines := strings.Split(rendered, "\n")
	padTop := (m.height - len(lines)) / 2
	if padTop < 0 {
		padTop = 0
	}
	padLeft := (m.width - lipgloss.Width(lines[0])) / 2
	if padLeft < 0 {
		padLeft = 0
	}
	pad := strings.Repeat("\n", padTop) + lipgloss.NewStyle().MarginLeft(padLeft).Render(rendered)
	return pad
}

func option(text string, focused bool) string {
	if focused {
		return focusedStyle.Render("> " + text)
	}
	return normalStyle.Render("  " + text)
}

func main() {
	if err := tea.NewProgram(initialLoginModel(), tea.WithAltScreen()).Start(); err != nil {
		fmt.Println("Error running login TUI:", err)
		os.Exit(1)
	}
}