package home

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func renderNetworkPanel(m Model) string {
	var b strings.Builder
	b.WriteString(sectionTitle.Render("Network Peers:\n"))
	line := lipgloss.NewStyle().Foreground(pink).Width(m.Width).Render(strings.Repeat("-", m.Width))
	b.WriteString(line + "\n")
	header := fmt.Sprintf("%-2s %-10s %-14s %-9s", "", "Name", "Host", "Status")
	b.WriteString(sectionTitle.Render(header) + "\n")
	b.WriteString(line + "\n")

	// Clamp cursor
	if m.Cursor >= len(m.Peers) {
		m.Cursor = len(m.Peers) - 1
	}
	if m.Cursor < 0 {
		m.Cursor = 0
	}

	for i, p := range m.Peers {
		cursor := "  "
		if i == m.Cursor {
			cursor = cursorStyle.Render("> ")
		}
		status := normalStyle.Render("OFFLINE")
		if p.Online {
			status = cursorStyle.Render("ONLINE")
		}
		row := fmt.Sprintf("%-2s%-10s %-14s %-9s %s", cursor, p.Name, p.Host, status, cursorStyle.Render("[Remove]"))
		b.WriteString(row + "\n")
	}
	b.WriteString("\n" + cursorStyle.Render("[A] Add peer (coming soon)") + "\n")
	return b.String()
}