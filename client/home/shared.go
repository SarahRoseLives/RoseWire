package home

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

const uploadsDir = "uploads"

// SharedFilesLoadedMsg is sent when the uploads directory has been scanned.
type SharedFilesLoadedMsg []sharedFile

// ScanUploadsCmd creates a command that scans the uploads directory.
func ScanUploadsCmd() tea.Cmd {
	return func() tea.Msg {
		files, err := scanUploads()
		if err != nil {
			return logEntry{Time: "[ERR]", Message: "Scan uploads failed: " + err.Error()}
		}
		return SharedFilesLoadedMsg(files)
	}
}

// scanUploads reads the uploads directory and returns a list of sharedFile structs.
func scanUploads() ([]sharedFile, error) {
	var shared []sharedFile
	entries, err := os.ReadDir(uploadsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Not an error if the folder doesn't exist yet
		}
		return nil, err
	}

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue // Skip files we can't get info for
		}
		shared = append(shared, sharedFile{
			Name:    info.Name(),
			IsDir:   info.IsDir(),
			Size:    formatBytes(info.Size()),
			rawSize: info.Size(),
		})
	}
	return shared, nil
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

// NotifyServerOfSharedFilesCmd creates a command to send the file list to the server.
func NotifyServerOfSharedFilesCmd(c *ChatClient, files []sharedFile) tea.Cmd {
	return func() tea.Msg {
		if c == nil || c.sshClient == nil {
			return logEntry{Time: "[ERR]", Message: "Cannot notify server, not connected."}
		}

		var parts []string
		for _, f := range files {
			// Format: name:raw_size_bytes:isDir
			part := fmt.Sprintf("%s:%d:%t", f.Name, f.rawSize, f.IsDir)
			parts = append(parts, part)
		}
		payload := strings.Join(parts, "|")
		command := "/share " + payload
		c.Send(command)

		return logEntry{Time: "[SYS]", Message: "Shared file list sent to server."}
	}
}

// EnsureUserDirs creates user-specific directories like 'uploads' and 'downloads'.
func EnsureUserDirs() error {
	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		return fmt.Errorf("create uploads dir: %w", err)
	}
	if err := os.MkdirAll("downloads", 0755); err != nil {
		return fmt.Errorf("create downloads dir: %w", err)
	}
	// Create a placeholder file in uploads for user guidance
	placeholderPath := filepath.Join(uploadsDir, "README.txt")
	if _, err := os.Stat(placeholderPath); os.IsNotExist(err) {
		content := []byte("Place files and folders you want to share in this directory.")
		_ = os.WriteFile(placeholderPath, content, 0644)
	}
	return nil
}