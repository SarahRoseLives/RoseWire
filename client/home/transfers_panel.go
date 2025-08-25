package home

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss" // <-- Add this line
)

func renderTransfersPanel(m Model) string {
	var b strings.Builder
	b.WriteString(sectionTitle.Render("File Transfers:\n"))
	line := lipgloss.NewStyle().Foreground(pink).Width(m.Width).Render(strings.Repeat("-", m.Width))
	b.WriteString(line + "\n")

	b.WriteString("\n  File transfer monitoring is coming soon!\n\n")

	// Example of what it might look like
	b.WriteString(fmt.Sprintf("  %-24s %-10s %s\n", "MyMovie.mkv", "25%", "[Downloading from alice]"))
	b.WriteString(fmt.Sprintf("  %-24s %-10s %s\n", "AnotherSong.mp3", "78%", "[Downloading from eve]"))

	return b.String()
}