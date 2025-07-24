package home

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const downloadsDir = "downloads"

// DownloadsLoadedMsg is sent when the downloads directory has been scanned.
type DownloadsLoadedMsg []download

// ScanDownloadsCmd creates a command that scans the downloads directory.
func ScanDownloadsCmd() tea.Cmd {
	return func() tea.Msg {
		files, err := scanDownloads()
		if err != nil {
			// Reuse logEntry for error reporting
			return logEntry{Time: "[ERR]", Message: "Scan downloads failed: " + err.Error()}
		}
		return DownloadsLoadedMsg(files)
	}
}

// scanDownloads reads the downloads directory. For now, it just lists completed files.
func scanDownloads() ([]download, error) {
	var downloads []download
	entries, err := os.ReadDir(downloadsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Not an error if the folder doesn't exist
		}
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue // Skip directories
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		downloads = append(downloads, download{
			FileName: info.Name(),
			Progress: formatBytes(info.Size()), // Use progress field to show file size
			Status:   "COMPLETED",
			Source:   "Unknown",
		})
	}
	return downloads, nil
}

// renderDownloadsPanel draws the UI for the Downloads tab.
func renderDownloadsPanel(m Model) string {
	var b strings.Builder
	b.WriteString(sectionTitle.Render("Downloads:\n"))
	line := lipgloss.NewStyle().Foreground(pink).Width(m.Width).Render(strings.Repeat("-", m.Width))
	b.WriteString(line + "\n")
	header := fmt.Sprintf("%-2s %-24s %-20s %-12s %-12s", "", "File", "Size", "Status", "Source Peer")
	b.WriteString(sectionTitle.Render(header) + "\n")
	b.WriteString(line + "\n")

	if len(m.Downloads) == 0 {
		b.WriteString("\n  No downloads found in the 'downloads' directory.\n")
	}

	for i, d := range m.Downloads {
		cursor := " "
		if i == m.Cursor {
			cursor = cursorStyle.Render(">")
		}
		row := fmt.Sprintf("%-2s %-24s %-20s %-12s %-12s", cursor, d.FileName, d.Progress, d.Status, d.Source)
		b.WriteString(row + "\n")
	}
	b.WriteString("\n" + cursorStyle.Render("[R] Refresh List") + "\n")
	return b.String()
}