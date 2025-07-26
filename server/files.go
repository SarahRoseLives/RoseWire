// files.go
package main

import (
	"log"
	"sort"
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
	FileName string `json:"fileName"`
	Size     int64  `json:"size"`
	Peer     string `json:"peer"`
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

// VerifyFileOwner checks if a specific user is sharing a file with a specific name.
func (r *FileRegistry) VerifyFileOwner(filename, owner string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	userFiles, ok := r.files[owner]
	if !ok {
		return false
	}

	for _, file := range userFiles {
		if file.Name == filename {
			return true
		}
	}

	return false
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

// TopFiles returns up to N largest files shared across all users.
func (r *FileRegistry) TopFiles(limit int) []SearchResult {
	r.mu.Lock()
	defer r.mu.Unlock()

	var allFiles []SearchResult
	for nickname, files := range r.files {
		for _, file := range files {
			if !file.IsDir {
				allFiles = append(allFiles, SearchResult{
					FileName: file.Name,
					Size:     file.Size,
					Peer:     nickname,
				})
			}
		}
	}

	sort.Slice(allFiles, func(i, j int) bool {
		return allFiles[i].Size > allFiles[j].Size
	})

	if len(allFiles) > limit {
		allFiles = allFiles[:limit]
	}

	return allFiles
}

// ParseShareCommand decodes a command string.
func ParseShareCommand(payload string) ([]SharedFile, error) {
	var files []SharedFile
	payload = strings.TrimSpace(payload)
	if payload == "" {
		return files, nil
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

// FindFile finds a specific file by a specific owner and returns its info.
func (r *FileRegistry) FindFile(filename, owner string) (SharedFile, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	userFiles, ok := r.files[owner]
	if !ok {
		return SharedFile{}, false
	}

	for _, file := range userFiles {
		if file.Name == filename {
			return file, true
		}
	}

	return SharedFile{}, false
}