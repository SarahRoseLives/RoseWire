package home

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const sharedDir = "uploads"
const downloadsDir = "downloads"

// renderLibraryPanel draws the UI for the combined Library tab.
// This version uses lipgloss.JoinHorizontal for robust column alignment.
func renderLibraryPanel(m Model) string {
	var b strings.Builder

	// Define column styles
	cursorColStyle := lipgloss.NewStyle().Width(2)
	nameColStyle := lipgloss.NewStyle().Width(31) // Width includes a space separator
	sizeColStyle := lipgloss.NewStyle().Width(10)

	// --- Shared Files Section ---
	b.WriteString(sectionTitle.Render(fmt.Sprintf("My Shared Files (from '%s' folder):\n", sharedDir)))
	line := lipgloss.NewStyle().Foreground(pink).Width(m.Width).Render(strings.Repeat("-", m.Width))
	b.WriteString(line + "\n")

	// Create and render the header row
	header := lipgloss.JoinHorizontal(lipgloss.Left,
		cursorColStyle.Render(""),
		nameColStyle.Render("Name"),
		sizeColStyle.Render("Size"),
	)
	b.WriteString(sectionTitle.Render(header) + "\n")
	b.WriteString(line + "\n")

	sharedFiles := filterLibrary(m.LibraryFiles, "Shared")
	if len(sharedFiles) == 0 {
		b.WriteString("\n  No files found in the 'uploads' directory.\n")
	} else {
		for _, f := range sharedFiles {
			name := f.Name
			if f.IsDir {
				name = filepath.Join(name, "/")
			}
			// Create and render a data row
			row := lipgloss.JoinHorizontal(lipgloss.Left,
				cursorColStyle.Render(""), // Empty cursor for now
				nameColStyle.Render(name),
				sizeColStyle.Render(f.Size),
			)
			b.WriteString(row + "\n")
		}
	}

	// --- Downloads Section ---
	b.WriteString("\n" + sectionTitle.Render("My Downloads:\n"))
	b.WriteString(line + "\n")
	b.WriteString(sectionTitle.Render(header) + "\n") // Reuse the same header
	b.WriteString(line + "\n")

	downloadedFiles := filterLibrary(m.LibraryFiles, "Downloaded")
	if len(downloadedFiles) == 0 {
		b.WriteString("\n  No files found in the 'downloads' directory.\n")
	} else {
		for _, f := range downloadedFiles {
			// Create and render a data row
			row := lipgloss.JoinHorizontal(lipgloss.Left,
				cursorColStyle.Render(""), // Empty cursor for now
				nameColStyle.Render(f.Name),
				sizeColStyle.Render(f.Size),
			)
			b.WriteString(row + "\n")
		}
	}

	b.WriteString("\n" + cursorStyle.Render("[R] Refresh Lists") + "\n")
	return b.String()
}


//
// --- The rest of the file remains the same ---
//

// ScanSharedCmd creates a command that scans the uploads directory.
func ScanSharedCmd() tea.Cmd {
	return func() tea.Msg {
		files, err := scanDir(sharedDir, "Shared")
		if err != nil {
			return logEntry{Time: "[ERR]", Message: "Scan uploads failed: " + err.Error()}
		}
		return sharedFilesLoadedMsg(files)
	}
}

// ScanDownloadsCmd creates a command that scans the downloads directory.
func ScanDownloadsCmd() tea.Cmd {
	return func() tea.Msg {
		files, err := scanDir(downloadsDir, "Downloaded")
		if err != nil {
			return logEntry{Time: "[ERR]", Message: "Scan downloads failed: " + err.Error()}
		}
		return downloadsLoadedMsg(files)
	}
}

// scanDir is a generic function to read a directory.
func scanDir(dirName, fileType string) ([]libraryFile, error) {
	var files []libraryFile
	entries, err := os.ReadDir(dirName)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Not an error if the folder doesn't exist yet
		}
		return nil, err
	}

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, libraryFile{
			Name:    info.Name(),
			IsDir:   info.IsDir(),
			Size:    formatBytes(info.Size()),
			rawSize: info.Size(),
			Type:    fileType,
		})
	}
	return files, nil
}

// NotifyServerOfSharedFilesCmd creates a command to send the file list to the server.
func NotifyServerOfSharedFilesCmd(c *ChatClient, files []libraryFile) tea.Cmd {
	return func() tea.Msg {
		if c == nil || c.sshClient == nil {
			return logEntry{Time: "[ERR]", Message: "Cannot notify server, not connected."}
		}

		var parts []string
		for _, f := range files {
			part := fmt.Sprintf("%s:%d:%t", f.Name, f.rawSize, f.IsDir)
			parts = append(parts, part)
		}
		payload := strings.Join(parts, "|")
		command := "/share " + payload
		c.Send(command)

		return logEntry{Time: "[SYS]", Message: "Shared file list sent to server."}
	}
}

// formatBytes converts bytes to a human-readable string.
func formatBytes(b int64) string {
	if b == 0 {
		return ""
	}
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

// EnsureUserDirs creates user-specific directories like 'uploads' and 'downloads'.
func EnsureUserDirs() error {
	if err := os.MkdirAll(sharedDir, 0755); err != nil {
		return fmt.Errorf("create uploads dir: %w", err)
	}
	if err := os.MkdirAll(downloadsDir, 0755); err != nil {
		return fmt.Errorf("create downloads dir: %w", err)
	}
	// Create a placeholder file in uploads for user guidance
	placeholderPath := filepath.Join(sharedDir, "README.txt")
	if _, err := os.Stat(placeholderPath); os.IsNotExist(err) {
		content := []byte("Place files and folders you want to share in this directory.")
		_ = os.WriteFile(placeholderPath, content, 0644)
	}
	return nil
}