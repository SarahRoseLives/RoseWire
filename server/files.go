package main

import (
	"log"
	"strconv"
	"strings"
	"sync"
)

// SharedFile represents a file a user is sharing.
type SharedFile struct {
	Name  string
	Size  int64
	IsDir bool
}

// SearchResult includes the peer's nickname along with file info.
type SearchResult struct {
	FileName string
	Size     int64
	Peer     string
}

// FileRegistry tracks all files shared by all connected users.
type FileRegistry struct {
	mu    sync.Mutex
	files map[string][]SharedFile // nickname -> list of files
}

// NewFileRegistry creates a new, empty file registry.
func NewFileRegistry() *FileRegistry {
	return &FileRegistry{
		files: make(map[string][]SharedFile),
	}
}

// UpdateUserFiles replaces the list of shared files for a given user.
func (r *FileRegistry) UpdateUserFiles(nickname string, fileList []SharedFile) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(fileList) > 0 {
		r.files[nickname] = fileList
		log.Printf("Updated file list for %s with %d items.", nickname, len(fileList))
	} else {
		delete(r.files, nickname)
		log.Printf("Cleared file list for %s.", nickname)
	}
}

// RemoveUser clears all file information for a user (e.g., on disconnect).
func (r *FileRegistry) RemoveUser(nickname string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.files, nickname)
	log.Printf("Removed user %s from file registry.", nickname)
}

// Search finds files matching the query across all online users.
func (r *FileRegistry) Search(query string) []SearchResult {
	r.mu.Lock()
	defer r.mu.Unlock()

	var results []SearchResult
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return results
	}

	for nickname, files := range r.files {
		for _, file := range files {
			if !file.IsDir && strings.Contains(strings.ToLower(file.Name), query) {
				results = append(results, SearchResult{
					FileName: file.Name,
					Size:     file.Size,
					Peer:     nickname,
				})
			}
		}
	}
	log.Printf("Search for '%s' returned %d results.", query, len(results))
	return results
}

// ParseShareCommand decodes a command string like "/share file:size:isDir|file2:size2:isDir2"
// into a slice of SharedFile objects.
func ParseShareCommand(payload string) ([]SharedFile, error) {
	var files []SharedFile
	payload = strings.TrimSpace(payload)
	if payload == "" {
		return files, nil // An empty list is a valid update (means they're sharing nothing).
	}

	parts := strings.Split(payload, "|")
	for _, part := range parts {
		if part == "" {
			continue
		}
		fileInfo := strings.SplitN(part, ":", 3)
		if len(fileInfo) != 3 {
			log.Printf("Warning: malformed file info part: %s", part)
			continue
		}

		name := fileInfo[0]
		size, err := strconv.ParseInt(fileInfo[1], 10, 64)
		if err != nil {
			log.Printf("Warning: malformed size in file info: %s", part)
			continue
		}
		isDir, err := strconv.ParseBool(fileInfo[2])
		if err != nil {
			log.Printf("Warning: malformed isDir flag in file info: %s", part)
			continue
		}

		files = append(files, SharedFile{
			Name:  name,
			Size:  size,
			IsDir: isDir,
		})
	}
	return files, nil
}