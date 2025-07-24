package home

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tab int

const (
	tabSearch tab = iota
	tabShared
	tabDownloads
	tabPeers
	tabLogs
	numTabs
)

var tabLabels = []string{"Search", "Shared", "Downloads", "Peers", "Logs/Chat"}

type searchResult struct {
	FileName string
	Peer     string
	Size     string
}

type sharedFile struct {
	Name   string
	IsDir  bool
	Size   string
}

type download struct {
	FileName string
	Progress string
	Status   string
	Source   string
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

type Model struct {
	Nickname   string
	Key        string
	CurrentTab tab
	Cursor     int
	Width      int
	Height     int
	Input      string // For search box
	InputMode  bool   // True if editing search input

	// Chat integration
	chatClient    *ChatClient
	chatConnected bool
	chatInput     string
	chatInputMode bool

	// Mock data (logs now includes chat)
	SearchResults []searchResult
	SharedFiles   []sharedFile
	Downloads     []download
	Peers         []peer
	Logs          []logEntry
}

var (
	pink        = lipgloss.Color("#ff81b3")
	pinkHeader  = lipgloss.NewStyle().
			Background(lipgloss.Color("#2b0036")).
			Foreground(pink).
			Padding(0, 1).
			Bold(true)
	tabStyle       = lipgloss.NewStyle().Padding(0, 2)
	activeTabStyle = tabStyle.Copy().Bold(true).Foreground(pink)
	cursorStyle    = lipgloss.NewStyle().Foreground(pink)
	footerStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Padding(0, 1)
	sectionTitle   = lipgloss.NewStyle().Foreground(pink).Bold(true)
	normalStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
)

func NewModel(nickname, key string) Model {
	return Model{
		Nickname: nickname,
		Key:      key,
		SearchResults: []searchResult{
			{"ubuntu.iso", "alice@host2", "1.5 GB"},
			{"project.zip", "bob@host3", "200 MB"},
			{"movie.mkv", "eve@host5", "700 MB"},
		},
		SharedFiles: []sharedFile{
			{"holiday_photos/", true, ""},
			{"notes.txt", false, "4 KB"},
			{"music.mp3", false, "6 MB"},
		},
		Downloads: []download{
			{"ubuntu.iso", "1.2 GB/1.5 GB (80%)", "DOWNLOADING", "alice@host2"},
			{"music.mp3", "COMPLETE", "COMPLETE", "bob@host3"},
			{"notes.txt", "FAILED", "FAILED", "eve@host5"},
		},
		Peers: []peer{
			{"alice", "host2", true},
			{"bob", "host3", false},
			{"eve", "host5", true},
		},
		Logs: []logEntry{
			{"[14:32]", "Connected to alice@host2"},
			{"[14:33]", "Downloaded ubuntu.iso from alice@host2"},
			{"[14:35]", `alice@host2: "Hey, check out my new files!"`},
		},
	}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	// Chat: handle incoming messages, connect/disconnect as tab changes
	if m.CurrentTab == tabLogs && m.chatClient == nil {
		client := NewChatClient(m.Nickname, m.Key, "127.0.0.1:2222")
		go func() {
			_ = client.Connect() // error handling can be improved
		}()
		m.chatClient = client
	}
	if m.chatClient != nil && m.CurrentTab == tabLogs {
		select {
		case line := <-m.chatClient.Receive():
			entry := ParseChatLine(line)
			m.Logs = append(m.Logs, logEntry{
				Time:    entry.Time,
				Message: fmt.Sprintf("%s: %s", entry.Sender, entry.Message),
			})
		default:
		}
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Chat input mode
		if m.CurrentTab == tabLogs && m.chatInputMode {
			switch msg.String() {
			case "enter":
				if strings.TrimSpace(m.chatInput) != "" && m.chatClient != nil {
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
		switch {
		case m.InputMode:
			switch msg.String() {
			case "enter", "esc":
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
		default:
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
				m.Cursor++
			case "enter":
				if m.CurrentTab == tabSearch && !m.InputMode && m.Cursor == 0 {
					m.InputMode = true
					m.Input = ""
				} else if m.CurrentTab == tabLogs && !m.chatInputMode {
					m.chatInputMode = true
				}
			}
		}
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
	}
	return m, nil
}

func (m Model) View() string {
	var b strings.Builder

	// Pink header, stretch to width
	header := pinkHeader.Width(m.Width).Render(fmt.Sprintf("🌹 RoseWire - [%s | %s]", m.Nickname, filepath.Base(m.Key)))
	b.WriteString(header + "\n")

	// Tabs - use pink for active, stretch to width
	var tabViews []string
	for i, label := range tabLabels {
		if tab(i) == m.CurrentTab {
			tabViews = append(tabViews, activeTabStyle.Render(label))
		} else {
			tabViews = append(tabViews, tabStyle.Render(label))
		}
	}
	tabsRow := lipgloss.JoinHorizontal(lipgloss.Top, tabViews...)
	tabsLine := lipgloss.NewStyle().Width(m.Width).Render(tabsRow)
	b.WriteString(tabsLine + "\n")

	// Horizontal line (fill width)
	b.WriteString(lipgloss.NewStyle().Foreground(pink).Width(m.Width).Render(strings.Repeat("─", m.Width)) + "\n")

	// Panel
	switch m.CurrentTab {
	case tabSearch:
		b.WriteString(renderSearchPanel(m))
	case tabShared:
		b.WriteString(renderSharedPanel(m))
	case tabDownloads:
		b.WriteString(renderDownloadsPanel(m))
	case tabPeers:
		b.WriteString(renderPeersPanel(m))
	case tabLogs:
		b.WriteString(renderLogsPanel(m))
	}

	// Footer - fill width
	footer := footerStyle.Width(m.Width).Render("[Tab] Switch Panel  [↑/↓] Move  [Enter] Select/Edit/Chat  [Q] Quit")
	b.WriteString("\n" + footer)
	return b.String()
}

func renderSearchPanel(m Model) string {
	var b strings.Builder
	b.WriteString(sectionTitle.Render("Search for files: "))
	if m.InputMode {
		b.WriteString(cursorStyle.Render(fmt.Sprintf("[_ %s_]\n", m.Input)))
	} else {
		b.WriteString("[Press Enter to type your query]\n")
	}
	line := lipgloss.NewStyle().Foreground(pink).Width(m.Width).Render(strings.Repeat("-", m.Width))
	b.WriteString(line + "\n")
	header := fmt.Sprintf("%-2s %-16s %-14s %-8s %s", "", "File Name", "Peer", "Size", "Action")
	b.WriteString(sectionTitle.Render(header) + "\n")
	b.WriteString(line + "\n")
	for i, r := range m.SearchResults {
		cursor := " "
		if i == m.Cursor && !m.InputMode {
			cursor = cursorStyle.Render(">")
		}
		row := fmt.Sprintf("%s %-16s %-14s %-8s %s", cursor, r.FileName, r.Peer, r.Size, cursorStyle.Render("[Download]"))
		b.WriteString(row + "\n")
	}
	return b.String()
}

func renderSharedPanel(m Model) string {
	var b strings.Builder
	b.WriteString(sectionTitle.Render("Shared Files (your library):\n"))
	line := lipgloss.NewStyle().Foreground(pink).Width(m.Width).Render(strings.Repeat("-", m.Width))
	b.WriteString(line + "\n")
	header := fmt.Sprintf("%-2s %-20s %-8s", "", "Name", "Size")
	b.WriteString(sectionTitle.Render(header) + "\n")
	b.WriteString(line + "\n")
	for i, f := range m.SharedFiles {
		cursor := " "
		if i == m.Cursor {
			cursor = cursorStyle.Render(">")
		}
		name := f.Name
		if f.IsDir {
			name += " [Folder]"
		}
		row := fmt.Sprintf("%s %-20s %-8s", cursor, name, f.Size)
		b.WriteString(row + "\n")
	}
	b.WriteString("\n" + cursorStyle.Render("[A] Add file/folder   [D] Delete") + "\n")
	return b.String()
}

func renderDownloadsPanel(m Model) string {
	var b strings.Builder
	b.WriteString(sectionTitle.Render("Downloads:\n"))
	line := lipgloss.NewStyle().Foreground(pink).Width(m.Width).Render(strings.Repeat("-", m.Width))
	b.WriteString(line + "\n")
	header := fmt.Sprintf("%-2s %-16s %-18s %-10s %-12s", "", "File", "Progress/Status", "Status", "Source Peer")
	b.WriteString(sectionTitle.Render(header) + "\n")
	b.WriteString(line + "\n")
	for i, d := range m.Downloads {
		cursor := " "
		if i == m.Cursor {
			cursor = cursorStyle.Render(">")
		}
		row := fmt.Sprintf("%s %-16s %-18s %-10s %-12s", cursor, d.FileName, d.Progress, d.Status, d.Source)
		b.WriteString(row + "\n")
	}
	return b.String()
}

func renderPeersPanel(m Model) string {
	var b strings.Builder
	b.WriteString(sectionTitle.Render("Peers:\n"))
	line := lipgloss.NewStyle().Foreground(pink).Width(m.Width).Render(strings.Repeat("-", m.Width))
	b.WriteString(line + "\n")
	header := fmt.Sprintf("%-2s %-10s %-14s %-9s", "", "Name", "Host", "Status")
	b.WriteString(sectionTitle.Render(header) + "\n")
	b.WriteString(line + "\n")
	for i, p := range m.Peers {
		cursor := " "
		if i == m.Cursor {
			cursor = cursorStyle.Render(">")
		}
		status := normalStyle.Render("OFFLINE")
		if p.Online {
			status = cursorStyle.Render("ONLINE")
		}
		row := fmt.Sprintf("%s %-10s %-14s %-9s %s", cursor, p.Name, p.Host, status, cursorStyle.Render("[Remove]"))
		b.WriteString(row + "\n")
	}
	b.WriteString("\n" + cursorStyle.Render("[A] Add peer (by SSH endpoint)") + "\n")
	return b.String()
}

func renderLogsPanel(m Model) string {
	var b strings.Builder
	b.WriteString(sectionTitle.Render("Logs & Chat:\n"))
	line := lipgloss.NewStyle().Foreground(pink).Width(m.Width).Render(strings.Repeat("-", m.Width))
	b.WriteString(line + "\n")
	for _, entry := range m.Logs {
		b.WriteString(fmt.Sprintf("%-7s %s\n", entry.Time, entry.Message))
	}
	// Chat input bar
	if m.chatInputMode {
		b.WriteString("\n> " + m.chatInput + "_\n")
	} else {
		b.WriteString("\n[Enter] Type a chat message\n")
	}
	return b.String()
}