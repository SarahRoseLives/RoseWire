package home

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// renderChatPanel draws the UI for the Logs & Chat tab.
func renderChatPanel(m Model) string {
	var b strings.Builder
	b.WriteString(sectionTitle.Render("Logs & Chat:\n"))
	line := lipgloss.NewStyle().Foreground(pink).Width(m.Width).Render(strings.Repeat("-", m.Width))
	b.WriteString(line + "\n")

	// Heuristic for available space for logs
	maxLogs := m.Height - 12
	if maxLogs < 1 {
		maxLogs = 1
	}
	start := len(m.Logs) - maxLogs
	if start < 0 {
		start = 0
	}
	for _, entry := range m.Logs[start:] {
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

// ChatLogEntry represents a single chat message for the UI.
type ChatLogEntry struct {
	Time    string
	Sender  string
	Message string
}

// ParseChatLine parses "[14:35] alice: hello" or just "alice: hi"
func ParseChatLine(line string) ChatLogEntry {
	ts := time.Now().Format("[15:04]")
	sender := "???"
	msg := line
	if i := strings.Index(line, "] "); i > 0 {
		ts = line[:i+1]
		rest := line[i+2:]
		if j := strings.Index(rest, ": "); j > 0 {
			sender = rest[:j]
			msg = rest[j+2:]
		} else {
			msg = rest
		}
	} else if j := strings.Index(line, ": "); j > 0 {
		sender = line[:j]
		msg = line[j+2:]
	}
	return ChatLogEntry{Time: ts, Sender: sender, Message: msg}
}