// tui/library_panel.go
package tui

import (
	"fmt"
	"log"
	"path/filepath"
	"rosetui/library"
	"rosetui/ssh"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// --- Commands ---
type loadLibraryPathCmd struct{}
type scanLibraryCmd string

// --- Messages ---
type libraryScanResultMsg struct{ files []library.LocalFile }
type libraryPathLoadedMsg string
type libraryErrMsg struct{ err error }
type filesHashedMsg struct{ files []ssh.ShareableFile } // Internal message after hashing is done

func (e libraryErrMsg) Error() string { return e.err.Error() }

type libraryPanelModel struct {
	state       libraryState
	libraryPath string
	files       []library.LocalFile
	isLoading   bool
	spinner     spinner.Model
	table       table.Model
	textInput   textinput.Model
	width, height int
	styles      *LibraryPanelStyles
	err         error
}

type libraryState uint

const (
	stateLoadingPath libraryState = iota
	stateReady
	stateScanning
	stateSettingPath
	stateHashing
)

func NewLibraryPanel() *libraryPanelModel {
	styles := DefaultLibraryPanelStyles()
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = styles.Spinner

	ti := textinput.New()
	ti.Placeholder = "/path/to/your/share/folder"
	ti.CharLimit = 256
	ti.Width = 50

	tbl := table.New(
		table.WithColumns([]table.Column{
			{Title: "File Name", Width: 40},
			{Title: "Size", Width: 15},
		}),
		table.WithFocused(true),
		table.WithHeight(10),
	)
	s := table.DefaultStyles()
	s.Header = s.Header.Bold(true).Foreground(lipgloss.Color("205"))
	s.Selected = s.Selected.Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57")).Bold(false)
	tbl.SetStyles(s)

	return &libraryPanelModel{
		state:     stateLoadingPath,
		isLoading: true,
		spinner:   sp,
		table:     tbl,
		textInput: ti,
		styles:    styles,
	}
}

func (m *libraryPanelModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, func() tea.Msg {
		return loadLibraryPathCmd{}
	})
}

func (m *libraryPanelModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case loadLibraryPathCmd:
		m.state = stateLoadingPath
		m.isLoading = true
		return m, func() tea.Msg {
			path, err := library.LoadConfig()
			if err != nil {
				return libraryErrMsg{err}
			}
			return libraryPathLoadedMsg(path)
		}

	case libraryPathLoadedMsg:
		m.libraryPath = string(msg)
		if m.libraryPath == "" {
			m.state = stateReady
			m.isLoading = false
			m.err = nil
			return m, nil
		}
		// If path exists, scan it and tell the service about it.
		return m, tea.Batch(
			func() tea.Msg { return scanLibraryCmd(m.libraryPath) },
			func() tea.Msg { return setLibraryPathCmd(m.libraryPath) },
		)

	case scanLibraryCmd:
		m.state = stateScanning
		m.isLoading = true
		return m, func() tea.Msg {
			files, err := library.ScanDirectory(string(msg))
			if err != nil {
				return libraryErrMsg{err}
			}
			return libraryScanResultMsg{files}
		}

	case libraryScanResultMsg:
		m.files = msg.files
		var rows []table.Row
		for _, file := range m.files {
			rows = append(rows, table.Row{file.Name, formatBytes(file.Size)})
		}
		m.table.SetRows(rows)
		m.state = stateHashing
		m.isLoading = true
		// After scanning, hash the files to share them
		return m, m.hashAndShareFilesCmd()

	case filesHashedMsg:
		m.state = stateReady
		m.isLoading = false
		m.err = nil
		// Now that internal state is updated, send the command to the parent app model
		return m, func() tea.Msg {
			return shareFilesCmd{files: msg.files}
		}

	case libraryErrMsg:
		m.err = msg.err
		m.isLoading = false
		m.state = stateReady
		return m, nil

	case tea.KeyMsg:
		if m.state == stateSettingPath {
			return m.updateSettingPath(msg)
		}

		switch msg.String() {
		case "s":
			m.state = stateSettingPath
			m.textInput.SetValue(m.libraryPath)
			m.textInput.Focus()
			return m, textinput.Blink
		case "r":
			if m.libraryPath != "" {
				return m, func() tea.Msg { return scanLibraryCmd(m.libraryPath) }
			}
		}
	}

	var tblCmd tea.Cmd
	m.table, tblCmd = m.table.Update(msg)
	cmds = append(cmds, tblCmd)

	if m.isLoading {
		var spCmd tea.Cmd
		m.spinner, spCmd = m.spinner.Update(msg)
		cmds = append(cmds, spCmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *libraryPanelModel) View() string {
	if m.height == 0 {
		return ""
	}
	if m.isLoading {
		status := "Loading..."
		switch m.state {
		case stateScanning:
			status = "Scanning directory..."
		case stateHashing:
			status = "Hashing files for sharing..."
		}
		return fmt.Sprintf("\n %s %s", m.spinner.View(), status)
	}

	var headerBuilder strings.Builder
	headerBuilder.WriteString(m.styles.Header.Render("My Shared Library"))
	pathStr := "No folder selected. Press 's' to set."
	if m.libraryPath != "" {
		pathStr = m.libraryPath
	}
	headerBuilder.WriteString(m.styles.Path.Render(pathStr))

	if m.err != nil {
		headerBuilder.WriteString(m.styles.Error.Render("\nError: " + m.err.Error()))
	}

	headerView := headerBuilder.String()

	if m.state == stateSettingPath {
		// For the settings view, we don't need a filler.
		return lipgloss.JoinVertical(lipgloss.Left,
			headerView,
			"\n\nEnter new library path and press Enter. (Press Esc to cancel)\n",
			m.textInput.View(),
		)
	}

	// For the main table view, calculate filler to push help text down.
	mainContent := lipgloss.JoinVertical(lipgloss.Left, headerView, m.table.View())
	contentHeight := lipgloss.Height(mainContent)

	fillerHeight := m.height - contentHeight
	if fillerHeight < 0 {
		fillerHeight = 0
	}
	filler := strings.Repeat("\n", fillerHeight)

	return mainContent + filler
}

func (m *libraryPanelModel) updateSettingPath(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg.Type {
	case tea.KeyEnter:
		newPath := m.textInput.Value()
		m.textInput.Blur()
		m.libraryPath = newPath
		return m, tea.Batch(
			func() tea.Msg {
				err := library.SaveConfig(newPath)
				if err != nil {
					return libraryErrMsg{err}
				}
				return scanLibraryCmd(newPath)
			},
			func() tea.Msg { return setLibraryPathCmd(newPath) },
		)
	case tea.KeyEsc:
		m.state = stateReady
		m.textInput.Blur()
	}
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m *libraryPanelModel) hashAndShareFilesCmd() tea.Cmd {
	return func() tea.Msg {
		var shareableFiles []ssh.ShareableFile
		for _, file := range m.files {
			fullPath := filepath.Join(m.libraryPath, file.Name)
			hash, err := library.GetFileHash(fullPath)
			if err != nil {
				log.Printf("Could not hash file %s: %v", file.Name, err)
				continue
			}
			shareableFiles = append(shareableFiles, ssh.ShareableFile{
				Name: file.Name,
				Size: file.Size,
				Hash: hash,
			})
		}
		return filesHashedMsg{files: shareableFiles}
	}
}

func (m *libraryPanelModel) SetSize(w, h int) {
	m.width, m.height = w, h
	m.styles.Header.Width(w)
	m.styles.Path.Width(w)
	m.textInput.Width = w - 4

	// Height of non-table elements: Header(2) + Path(2) + potential Error(1) + Table Borders/Header(3) = ~8
	tableHeight := h - 8
	if tableHeight < 1 {
		tableHeight = 1
	}
	m.table.SetHeight(tableHeight)
	m.table.SetWidth(w - 4)

	// Re-calculate column widths
	nameWidth := (w - 4) - 15 - 3 // total - size col - borders
	m.table.SetColumns([]table.Column{
		{Title: "File Name", Width: nameWidth},
		{Title: "Size", Width: 15},
	})
}

// --- Styling ---
type LibraryPanelStyles struct {
	Spinner lipgloss.Style
	Header  lipgloss.Style
	Path    lipgloss.Style
	Error   lipgloss.Style
}

func DefaultLibraryPanelStyles() *LibraryPanelStyles {
	return &LibraryPanelStyles{
		Spinner: lipgloss.NewStyle().Foreground(lipgloss.Color("205")),
		Header:  lipgloss.NewStyle().Bold(true).Padding(1, 0, 0, 2),
		Path:    lipgloss.NewStyle().Faint(true).Padding(0, 0, 1, 2),
		Error:   lipgloss.NewStyle().Foreground(lipgloss.Color("#E74C3C")),
	}
}