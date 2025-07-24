package home

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// searchResult represents a single item found in a search.
type searchResult struct {
	FileName string
	Peer     string
	Size     string
}

// SearchResultsMsg is sent when the server responds with search results.
type SearchResultsMsg []searchResult

// SearchCmd creates a command to send a search query to the server.
func SearchCmd(c *ChatClient, query string) tea.Cmd {
	return func() tea.Msg {
		if c == nil {
			return logEntry{Time: "[ERR]", Message: "Cannot search, not connected."}
		}
		command := "/search " + query
		c.Send(command)
		// We don't return a message here; the result will come from the server
		// and be handled by the chatLineListener.
		return nil
	}
}

// ParseSearchResults decodes the server's search result payload.
// Payload format: name1:raw_size1:peer1|name2:raw_size2:peer2
func ParseSearchResults(payload string) tea.Msg {
	var results []searchResult
	if strings.TrimSpace(payload) == "" {
		return SearchResultsMsg(results)
	}

	items := strings.Split(payload, "|")
	for _, item := range items {
		parts := strings.SplitN(item, ":", 3)
		if len(parts) != 3 {
			continue // Malformed part
		}
		size, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			continue // Malformed size
		}

		results = append(results, searchResult{
			FileName: parts[0],
			Peer:     parts[2],
			Size:     formatBytes(size), // formatBytes is in shared.go
		})
	}
	return SearchResultsMsg(results)
}

// renderSearchPanel draws the UI for the Search tab.
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
	header := fmt.Sprintf("%-2s %-24s %-20s %-12s %-12s", "", "File", "Peer", "Size", "Action")
	b.WriteString(sectionTitle.Render(header) + "\n")
	b.WriteString(line + "\n")

	if len(m.SearchResults) == 0 {
		b.WriteString("\n  No results. Type a query and press Enter to search.\n")
	}

	for i, r := range m.SearchResults {
		cursor := " "
		// Note: The cursor logic here is illustrative. A real implementation might need
		// to adjust how the cursor behaves when the input box is not active.
		if i == m.Cursor && !m.InputMode {
			cursor = cursorStyle.Render(">")
		}
		row := fmt.Sprintf("%-2s %-24s %-20s %-12s %s", cursor, r.FileName, r.Peer, r.Size, cursorStyle.Render("[Download]"))
		b.WriteString(row + "\n")
	}
	return b.String()
}