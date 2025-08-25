package home

import (
	"strings"

	"github.com/charmbracelet/lipgloss" // <-- FIX: Added missing import
)

func renderSettingsPanel(m Model) string {
	var b strings.Builder
	b.WriteString(sectionTitle.Render("Settings:\n"))
	line := lipgloss.NewStyle().Foreground(pink).Width(m.Width).Render(strings.Repeat("-", m.Width))
	b.WriteString(line + "\n")

	b.WriteString("\n  A settings panel is coming soon!\n\n")
	b.WriteString("  [ ] Enable dark mode\n")
	b.WriteString("  [x] Auto-connect on startup\n")

	return b.String()
}