package main

import (
	"fmt"
	"os"
	"strings"

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
	Name     string
	IsDir    bool
	Size     string
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

type model struct {
	currentTab tab
	cursor     int
	width      int
	height     int
	input      string // For search box
	inputMode  bool   // True if editing search input

	// Mock data
	searchResults []searchResult
	sharedFiles   []sharedFile
	downloads     []download
	peers         []peer
	logs          []logEntry
}

func initialModel() model {
	return model{
		searchResults: []searchResult{
			{"ubuntu.iso", "alice@host2", "1.5 GB"},
			{"project.zip", "bob@host3", "200 MB"},
			{"movie.mkv", "eve@host5", "700 MB"},
		},
		sharedFiles: []sharedFile{
			{"holiday_photos/", true, ""},
			{"notes.txt", false, "4 KB"},
			{"music.mp3", false, "6 MB"},
		},
		downloads: []download{
			{"ubuntu.iso", "1.2 GB/1.5 GB (80%)", "DOWNLOADING", "alice@host2"},
			{"music.mp3", "COMPLETE", "COMPLETE", "bob@host3"},
			{"notes.txt", "FAILED", "FAILED", "eve@host5"},
		},
		peers: []peer{
			{"alice", "host2", true},
			{"bob", "host3", false},
			{"eve", "host5", true},
		},
		logs: []logEntry{
			{"[14:32]", "Connected to alice@host2"},
			{"[14:33]", "Downloaded ubuntu.iso from alice@host2"},
			{"[14:35]", `alice@host2: "Hey, check out my new files!"`},
		},
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case m.inputMode:
			switch msg.String() {
			case "enter", "esc":
				m.inputMode = false
			case "backspace":
				if len(m.input) > 0 {
					m.input = m.input[:len(m.input)-1]
				}
			default:
				if msg.Type == tea.KeyRunes {
					m.input += msg.String()
				}
			}
		default:
			switch msg.String() {
			case "ctrl+c", "q":
				return m, tea.Quit
			case "tab":
				m.currentTab = (m.currentTab + 1) % numTabs
				m.cursor = 0
			case "shift+tab":
				m.currentTab = (m.currentTab - 1 + numTabs) % numTabs
				m.cursor = 0
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
				}
			case "down", "j":
				m.cursor++
			case "enter":
				// Panel specific actions
				if m.currentTab == tabSearch && !m.inputMode && m.cursor == 0 {
					m.inputMode = true
					m.input = ""
				}
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

func (m model) View() string {
	var b strings.Builder

	// Header
	header := lipgloss.NewStyle().
		Background(lipgloss.Color("#0f0f0f")).
		Foreground(lipgloss.Color("#FFD700")).
		Padding(0, 1).
		Render(fmt.Sprintf(" RoseWire (SSH Edition) - [SarahRoseLives@localhost]"))
	b.WriteString(header + "\n")

	// Tabs
	var tabViews []string
	tabStyle := lipgloss.NewStyle().Padding(0, 2)
	activeTabStyle := tabStyle.Copy().Bold(true).Foreground(lipgloss.Color("229"))
	for i, label := range tabLabels {
		if tab(i) == m.currentTab {
			tabViews = append(tabViews, activeTabStyle.Render(label))
		} else {
			tabViews = append(tabViews, tabStyle.Render(label))
		}
	}
	tabsRow := lipgloss.JoinHorizontal(lipgloss.Top, tabViews...)
	b.WriteString(tabsRow + "\n")
	b.WriteString(strings.Repeat("═", m.width) + "\n")

	// Panel
	switch m.currentTab {
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

	// Footer
	footer := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Padding(0, 1).
		Render("[Tab] Switch Panel  [↑/↓] Move  [Enter] Select/Edit  [Q] Quit")
	b.WriteString("\n" + footer)
	return b.String()
}

func renderSearchPanel(m model) string {
	var b strings.Builder
	b.WriteString("Search for files: ")
	if m.inputMode {
		b.WriteString(fmt.Sprintf("[_ %s_]\n", m.input))
	} else {
		b.WriteString("[Press Enter to type your query]\n")
	}
	b.WriteString(strings.Repeat("-", 60) + "\n")
	// Table header
	b.WriteString(fmt.Sprintf("%-2s %-16s %-14s %-8s %s\n", "", "File Name", "Peer", "Size", "Action"))
	b.WriteString(strings.Repeat("-", 60) + "\n")
	// Results
	for i, r := range m.searchResults {
		cursor := " "
		if i == m.cursor && !m.inputMode {
			cursor = ">"
		}
		b.WriteString(fmt.Sprintf("%s %-16s %-14s %-8s [Download]\n", cursor, r.FileName, r.Peer, r.Size))
	}
	return b.String()
}

func renderSharedPanel(m model) string {
	var b strings.Builder
	b.WriteString("Shared Files (your library):\n")
	b.WriteString(strings.Repeat("-", 40) + "\n")
	b.WriteString(fmt.Sprintf("%-2s %-20s %-8s\n", "", "Name", "Size"))
	b.WriteString(strings.Repeat("-", 40) + "\n")
	for i, f := range m.sharedFiles {
		cursor := " "
		if i == m.cursor {
			cursor = ">"
		}
		name := f.Name
		if f.IsDir {
			name += " [Folder]"
		}
		b.WriteString(fmt.Sprintf("%s %-20s %-8s\n", cursor, name, f.Size))
	}
	b.WriteString("\n[A] Add file/folder   [D] Delete\n")
	return b.String()
}

func renderDownloadsPanel(m model) string {
	var b strings.Builder
	b.WriteString("Downloads:\n")
	b.WriteString(strings.Repeat("-", 55) + "\n")
	b.WriteString(fmt.Sprintf("%-2s %-16s %-18s %-10s %-12s\n", "", "File", "Progress/Status", "Status", "Source Peer"))
	b.WriteString(strings.Repeat("-", 55) + "\n")
	for i, d := range m.downloads {
		cursor := " "
		if i == m.cursor {
			cursor = ">"
		}
		b.WriteString(fmt.Sprintf("%s %-16s %-18s %-10s %-12s\n", cursor, d.FileName, d.Progress, d.Status, d.Source))
	}
	return b.String()
}

func renderPeersPanel(m model) string {
	var b strings.Builder
	b.WriteString("Peers:\n")
	b.WriteString(strings.Repeat("-", 40) + "\n")
	b.WriteString(fmt.Sprintf("%-2s %-10s %-14s %-9s\n", "", "Name", "Host", "Status"))
	b.WriteString(strings.Repeat("-", 40) + "\n")
	for i, p := range m.peers {
		cursor := " "
		if i == m.cursor {
			cursor = ">"
		}
		status := "OFFLINE"
		if p.Online {
			status = "ONLINE"
		}
		b.WriteString(fmt.Sprintf("%s %-10s %-14s %-9s [Remove]\n", cursor, p.Name, p.Host, status))
	}
	b.WriteString("\n[A] Add peer (by SSH endpoint)\n")
	return b.String()
}

func renderLogsPanel(m model) string {
	var b strings.Builder
	b.WriteString("Logs & Chat:\n")
	b.WriteString(strings.Repeat("-", 55) + "\n")
	for _, entry := range m.logs {
		b.WriteString(fmt.Sprintf("%-7s %s\n", entry.Time, entry.Message))
	}
	return b.String()
}

func main() {
	if err := tea.NewProgram(initialModel(), tea.WithAltScreen()).Start(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}
