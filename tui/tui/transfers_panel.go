package tui

import (
	"fmt"
	"rosetui/ssh"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// --- Commands ---
type downloadFileCmd struct{ file ssh.SearchResult }
type retryDownloadCmd struct{ transfer ssh.Transfer }

type transfersPanelModel struct {
	transfers     []ssh.Transfer
	table         table.Model
	progressBars  map[string]progress.Model // Map transfer ID to its progress bar
	width, height int
}

func NewTransfersPanel() *transfersPanelModel {
	tbl := table.New(
		table.WithColumns([]table.Column{
			{Title: "File Name", Width: 40},
			{Title: "From", Width: 20},
			{Title: "Status", Width: 15},
			{Title: "Progress", Width: 25},
		}),
		table.WithFocused(true),
		table.WithHeight(10),
	)
	s := table.DefaultStyles()
	s.Header = s.Header.Bold(true).Foreground(lipgloss.Color("205"))
	s.Selected = s.Selected.Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57")).Bold(false)
	tbl.SetStyles(s)

	return &transfersPanelModel{
		table:        tbl,
		progressBars: make(map[string]progress.Model),
	}
}

func (m *transfersPanelModel) Init() tea.Cmd { return nil }

func (m *transfersPanelModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case ssh.TransfersUpdateMsg:
		m.transfers = msg.Transfers
		m.updateTableRows()
		// Update progress bars with new percentages
		for _, t := range m.transfers {
			if bar, ok := m.progressBars[t.ID]; ok {
				cmds = append(cmds, bar.SetPercent(t.Progress))
			}
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			if len(m.transfers) > 0 && m.table.Cursor() < len(m.transfers) {
				selected := m.transfers[m.table.Cursor()]
				if selected.Status == "Failed" {
					return m, func() tea.Msg { return retryDownloadCmd{transfer: selected} }
				}
			}
		}

	// Handle progress bar animation frames
	case progress.FrameMsg:
		// There is no ProgressID or ID method;
		// Instead, we update all bars with this frame message.
		for id, bar := range m.progressBars {
			newModel, cmd := bar.Update(msg)
			m.progressBars[id] = newModel.(progress.Model)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		m.updateTableRows() // Re-render table to show new frame

	}

	var tblCmd tea.Cmd
	m.table, tblCmd = m.table.Update(msg)
	cmds = append(cmds, tblCmd)

	return m, tea.Batch(cmds...)
}

func (m *transfersPanelModel) updateTableRows() {
	var rows []table.Row
	for _, t := range m.transfers {
		var progStr string
		if t.Status == "Active" {
			if _, ok := m.progressBars[t.ID]; !ok {
				m.progressBars[t.ID] = progress.New(
					progress.WithSolidFill("#EC4899"),
					progress.WithWidth(20),
				)
			}
			progStr = m.progressBars[t.ID].View() + " " + t.Speed
		} else if t.Status == "Failed" {
			progStr = lipgloss.NewStyle().Foreground(lipgloss.Color("#E74C3C")).Render(t.Error)
		} else {
			progStr = fmt.Sprintf("%.0f%%", t.Progress*100)
		}
		rows = append(rows, table.Row{t.FileName, t.FromUser, t.Status, progStr})
	}
	m.table.SetRows(rows)
}

func (m *transfersPanelModel) View() string {
	if len(m.transfers) == 0 {
		return "\n  No active or recent transfers."
	}
	help := "\n'r': retry failed transfer"
	return m.table.View() + help
}

func (m *transfersPanelModel) SetSize(w, h int) {
	m.width, m.height = w, h
	// Table Header(1), Table Border(2), Help(2) = 5
	m.table.SetHeight(h - 5)
	m.table.SetWidth(w - 4)
	// Re-calculate column widths
	nameWidth := w - 4 - 20 - 15 - 25 - 4 // total - from - status - progress - borders
	m.table.SetColumns([]table.Column{
		{Title: "File Name", Width: nameWidth},
		{Title: "From", Width: 20},
		{Title: "Status", Width: 15},
		{Title: "Progress", Width: 25},
	})
}