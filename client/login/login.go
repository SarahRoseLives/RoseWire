package login

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/user"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/crypto/ssh"
)

type step int

const (
	stepChooseAutoOrNew step = iota
	stepChooseAction
	stepListKeys
	stepCreateKey
	stepEnterNickname
	stepConnecting
	stepDone
)

type focus int

const (
	focusAutoLogin focus = iota
	focusExisting
	focusCreate
	focusKeyList
	focusNickname
	focusDone
)

var (
	pink         = lipgloss.Color("#ff81b3")
	cardMinWidth = 36 // minimum width for card
	cardMaxWidth = 60 // maximum width for card
	focusedStyle = lipgloss.NewStyle().Foreground(pink).Bold(true)
	normalStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	cardStyle    = lipgloss.NewStyle().
			Padding(1, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(pink).
			Align(lipgloss.Center).
			Background(lipgloss.Color("#1b1b1b"))
)

const (
	configPathDefault = ".rosewire_client"
	relayAddrDefault  = "127.0.0.1:2222"
)

type Model struct {
	Step      step
	Focus     focus
	Width     int
	Height    int
	Status    string
	Done      bool

	Keys        []string // List of public key file paths
	KeyCursor   int
	SelectedKey string

	CreatingKey  bool
	CreateKeyMsg string

	Nickname      string
	NicknameInput bool

	// Auto-login state
	autoLoginTried bool
	// Remembered username/key
	RememberedNickname string
	RememberedKeyPath  string
}

// Constructor
func NewModel() Model {
	return Model{
		Step:  stepChooseAutoOrNew,
		Focus: focusAutoLogin,
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

// Loads stored nickname/key path from ~/.rosewire_client (if present and valid)
func tryAutoLogin() (nickname, keypath string, err error) {
	usr, err := user.Current()
	if err != nil {
		return "", "", err
	}
	cfg := filepath.Join(usr.HomeDir, configPathDefault)
	data, err := os.ReadFile(cfg)
	if err != nil {
		return "", "", err
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) < 2 {
		return "", "", errors.New("incomplete rosewire config")
	}
	nick := strings.TrimSpace(lines[0])
	keypath = strings.TrimSpace(lines[1])
	if nick == "" || keypath == "" {
		return "", "", errors.New("rosewire config missing nickname/key")
	}
	if _, err := os.Stat(keypath); err != nil {
		return "", "", errors.New("key file missing: " + keypath)
	}
	return nick, keypath, nil
}

func saveLogin(nickname, keypath string) error {
	usr, err := user.Current()
	if err != nil {
		return err
	}
	cfg := filepath.Join(usr.HomeDir, configPathDefault)
	content := fmt.Sprintf("%s\n%s\n", nickname, keypath)
	return os.WriteFile(cfg, []byte(content), 0600)
}

func (m Model) Init() tea.Cmd {
	return func() tea.Msg {
		nick, keypath, err := tryAutoLogin()
		if err == nil {
			return autoLoginCandidateMsg{Nickname: nick, KeyPath: keypath}
		}
		return sshKeysMsg(findSSHKeys())
	}
}

type sshKeysMsg []string
type createKeyMsg string

type tryLoginMsg struct{ Nickname, KeyPath string }
type loginResultMsg struct{ Success bool; Err string }
type autoLoginCandidateMsg struct{ Nickname, KeyPath string }

func createSSHKeyCmd() tea.Cmd {
	return func() tea.Msg {
		usr, _ := user.Current()
		keyPath := filepath.Join(usr.HomeDir, ".ssh", "id_ed25519")
		pubPath := keyPath + ".pub"
		if _, err := os.Stat(pubPath); err == nil {
			return createKeyMsg(pubPath)
		}
		cmd := exec.Command("ssh-keygen", "-t", "ed25519", "-N", "", "-f", keyPath)
		cmd.Stdin = nil
		cmd.Stdout = nil
		cmd.Stderr = nil
		_ = cmd.Run()
		return createKeyMsg(pubPath)
	}
}

func tryLoginCmd(nickname, pubkeypath string) tea.Cmd {
	return func() tea.Msg {
		// Guess private key path for pubkey (strip .pub)
		priv := strings.TrimSuffix(pubkeypath, ".pub")
		key, err := os.ReadFile(priv)
		if err != nil {
			return loginResultMsg{false, "Failed to read private key: " + err.Error()}
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return loginResultMsg{false, "Invalid private key: " + err.Error()}
		}
		config := &ssh.ClientConfig{
			User: nickname,
			Auth: []ssh.AuthMethod{
				ssh.PublicKeys(signer),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			Timeout:         4 * time.Second,
		}
		client, err := ssh.Dial("tcp", relayAddrDefault, config)
		if err != nil {
			return loginResultMsg{false, "SSH login failed: " + err.Error()}
		}
		defer client.Close()
		session, err := client.NewSession()
		if err != nil {
			return loginResultMsg{false, "Session error: " + err.Error()}
		}
		defer session.Close()
		var buf bytes.Buffer
		session.Stdout = &buf
		session.Stderr = &buf
		_ = session.Shell()
		time.Sleep(200 * time.Millisecond)
		session.Close()
		msg := strings.TrimSpace(buf.String())
		if !strings.Contains(msg, "RoseWire relay") {
			return loginResultMsg{false, "Unexpected server response"}
		}
		// Save combo
		saveLogin(nickname, pubkeypath)
		return loginResultMsg{true, ""}
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case sshKeysMsg:
		m.Keys = msg
		if len(m.Keys) == 0 {
			m.Status = "No SSH keys found! Please create a key."
			m.Focus = focusCreate
		}
	case createKeyMsg:
		m.CreatingKey = false
		m.CreateKeyMsg = fmt.Sprintf("Created new SSH key at %s", string(msg))
		m.Keys = append(m.Keys, string(msg))
		m.KeyCursor = len(m.Keys) - 1
		m.Step = stepListKeys
		m.Focus = focusKeyList
	case autoLoginCandidateMsg:
		m.RememberedNickname = msg.Nickname
		m.RememberedKeyPath = msg.KeyPath
		m.Step = stepChooseAutoOrNew
		m.Focus = focusAutoLogin
	case tryLoginMsg:
		m.Step = stepConnecting
		m.Nickname = msg.Nickname
		m.SelectedKey = msg.KeyPath
		return m, tryLoginCmd(msg.Nickname, msg.KeyPath)
	case loginResultMsg:
		if msg.Success {
			m.Step = stepDone
			m.Done = true
			m.Status = ""
		} else {
			// Go back to menu, clear Remembered if that combo failed
			if m.Step == stepConnecting && m.Focus == focusAutoLogin {
				m.RememberedNickname = ""
				m.RememberedKeyPath = ""
			}
			m.Step = stepChooseAutoOrNew
			m.Status = "Login failed: " + msg.Err
			m.autoLoginTried = true // Don't auto-login again
			return m, m.Init()
		}
	case tea.WindowSizeMsg:
		m.Width, m.Height = msg.Width, msg.Height

	case tea.KeyMsg:
		if m.Done {
			return m, tea.Quit
		}
		switch m.Step {
		case stepChooseAutoOrNew:
			switch msg.String() {
			case "up", "down", "tab":
				if m.RememberedNickname != "" {
					if m.Focus == focusAutoLogin {
						m.Focus = focusExisting
					} else {
						m.Focus = focusAutoLogin
					}
				}
			case "enter":
				if m.Focus == focusAutoLogin && m.RememberedNickname != "" {
					// Try auto-login with remembered combo
					return m, tryLoginCmd(m.RememberedNickname, m.RememberedKeyPath)
				} else {
					m.Step = stepChooseAction
					m.Focus = focusExisting
					return m, func() tea.Msg { return sshKeysMsg(findSSHKeys()) }
				}
			}
		case stepChooseAction:
			switch msg.String() {
			case "tab", "right", "down":
				if m.Focus == focusExisting {
					m.Focus = focusCreate
				} else {
					m.Focus = focusExisting
				}
			case "shift+tab", "left", "up":
				if m.Focus == focusCreate {
					m.Focus = focusExisting
				} else {
					m.Focus = focusCreate
				}
			case "enter":
				if m.Focus == focusExisting {
					m.Step = stepListKeys
					m.Focus = focusKeyList
				} else {
					m.Step = stepCreateKey
					m.CreatingKey = true
					return m, createSSHKeyCmd()
				}
			case "esc":
				m.Step = stepChooseAutoOrNew
				m.Focus = focusAutoLogin
			}
		case stepListKeys:
			switch msg.String() {
			case "up", "k":
				if m.KeyCursor > 0 {
					m.KeyCursor--
				}
			case "down", "j":
				if m.KeyCursor < len(m.Keys)-1 {
					m.KeyCursor++
				}
			case "enter":
				if len(m.Keys) > 0 {
					m.SelectedKey = m.Keys[m.KeyCursor]
					m.Step = stepEnterNickname
					m.Focus = focusNickname
					m.NicknameInput = true
				}
			case "esc":
				m.Step = stepChooseAction
				m.Focus = focusExisting
			}
		case stepCreateKey:
			if !m.CreatingKey {
				m.Step = stepListKeys
				m.Focus = focusKeyList
			}
		case stepEnterNickname:
			if m.NicknameInput {
				switch msg.String() {
				case "enter":
					if strings.TrimSpace(m.Nickname) == "" {
						m.Status = "Nickname required"
					} else {
						m.Step = stepConnecting
						return m, tryLoginCmd(m.Nickname, m.SelectedKey)
					}
				case "backspace":
					if len(m.Nickname) > 0 {
						m.Nickname = m.Nickname[:len(m.Nickname)-1]
					}
				default:
					if msg.Type == tea.KeyRunes {
						m.Nickname += msg.String()
					}
				}
			}
		case stepConnecting:
			// Ignore keys
		case stepDone:
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) View() string {
	if m.Done && m.Step == stepDone {
		return lipgloss.NewStyle().Foreground(pink).Align(lipgloss.Center).Width(m.Width).Render(
			fmt.Sprintf("\n\n🌹 Welcome, %s!\n\nYour key: %s\nLoading RoseWire...",
				m.Nickname,
				filepath.Base(m.SelectedKey),
			),
		)
	}

	card := ""
	title := lipgloss.NewStyle().
		Foreground(pink).
		Background(lipgloss.Color("#2b0036")).
		Padding(0, 1).
		Bold(true).
		Render("🌹 RoseWire Login")

	switch m.Step {
	case stepChooseAutoOrNew:
		card = title + "\n\n"
		if m.RememberedNickname != "" {
			card += option(fmt.Sprintf("Log in as %s (%s)", m.RememberedNickname, filepath.Base(m.RememberedKeyPath)), m.Focus == focusAutoLogin) + "\n"
		}
		card += option("Use existing or new SSH key / nickname", m.Focus == focusExisting) + "\n"
		if m.Status != "" {
			card += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("210")).Render(m.Status)
		}
	case stepChooseAction:
		card = fmt.Sprintf(
			"%s\n\n%s\n%s\n\n%s",
			title,
			option("Use existing SSH key", m.Focus == focusExisting),
			option("Create new SSH key", m.Focus == focusCreate),
			lipgloss.NewStyle().Foreground(lipgloss.Color("210")).Render(m.Status),
		)
	case stepListKeys:
		card = title + "\n\nChoose your SSH key:\n\n"
		if len(m.Keys) == 0 {
			card += "No SSH keys found! Press Esc to go back.\n"
		}
		for i, k := range m.Keys {
			display := filepath.Base(k)
			if i == m.KeyCursor {
				card += focusedStyle.Render("> " + display + "\n")
			} else {
				card += normalStyle.Render("  " + display + "\n")
			}
		}
		card += "\n[Enter] Select  [Esc] Back"
	case stepCreateKey:
		card = title + "\n\n"
		if m.CreatingKey {
			card += focusedStyle.Render("Creating new SSH key...") + "\n"
		} else {
			card += fmt.Sprintf("SSH key created: %s\n\n[Press any key to continue]", m.CreateKeyMsg)
		}
	case stepEnterNickname:
		card = title + "\n\nEnter your nickname:\n\n"
		entry := m.Nickname
		if m.NicknameInput {
			entry += "_"
		}
		card += focusedStyle.Render(entry) + "\n"
		card += "\n[Enter] Continue"
		if m.Status != "" {
			card += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("210")).Render(m.Status)
		}
	case stepConnecting:
		card = title + "\n\n" + focusedStyle.Render(fmt.Sprintf("Logging in as %s...", m.Nickname))
	}

	cardWidth := m.Width / 3
	if cardWidth < cardMinWidth {
		cardWidth = cardMinWidth
	}
	if cardWidth > cardMaxWidth {
		cardWidth = cardMaxWidth
	}
	cardRendered := cardStyle.Width(cardWidth).Render(card)

	lines := strings.Split(cardRendered, "\n")
	padTop := (m.Height - len(lines)) / 2
	if padTop < 0 {
		padTop = 0
	}
	padLeft := (m.Width - lipgloss.Width(lines[0])) / 2
	if padLeft < 0 {
		padLeft = 0
	}
	pad := strings.Repeat("\n", padTop) + lipgloss.NewStyle().MarginLeft(padLeft).Render(cardRendered)
	return pad
}

func option(text string, focused bool) string {
	if focused {
		return focusedStyle.Render("> " + text)
	}
	return normalStyle.Render("  " + text)
}