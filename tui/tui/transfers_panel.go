// tui/transfers_panel.go
package tui

import (
	"fmt"
	"rosetui/ssh"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type transfersPanelModel struct {
	transfers    []ssh.Transfer
	table        table.Model
	progressBars map[string]progress.Model // Map transfer ID to its progress bar
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
		// We update all bars with this frame message.
		for id, bar := range m.progressBars {
			newModel, cmd := bar.Update(msg)
			if newBar, ok := newModel.(progress.Model); ok {
				m.progressBars[id] = newBar
			}
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
	if m.height == 0 {
		return ""
	}
	if len(m.transfers) == 0 {
		return "\n  No active or recent transfers."
	}

	tableView := m.table.View()
	tableHeight := lipgloss.Height(tableView)

	// The total available panel height is m.height.
	// We subtract the table's current height to find the space for the filler.
	fillerHeight := m.height - tableHeight
	if fillerHeight < 0 {
		fillerHeight = 0
	}
	filler := strings.Repeat("\n", fillerHeight)

	return lipgloss.JoinVertical(lipgloss.Left, tableView, filler)
}

func (m *transfersPanelModel) SetSize(w, h int) {
	m.width, m.height = w, h
	// Total panel height is h. The table can use all of it.
	// The View() function will manage placing the help text at the bottom.
	m.table.SetHeight(h)
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