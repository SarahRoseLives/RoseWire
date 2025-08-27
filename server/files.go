// SERVER/files.go
package main

import (
	"log"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
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

// UserFileEntry holds a user's file list and the last time it was updated.
type UserFileEntry struct {
	Files       []SharedFile
	LastUpdated time.Time
}

// FileRegistry tracks all files shared by all connected users.
type FileRegistry struct {
	mu    sync.Mutex
	files map[string]*UserFileEntry // nickname -> entry with files and timestamp
}

// NewFileRegistry creates a new, empty file registry.
func NewFileRegistry() *FileRegistry {
	return &FileRegistry{
		files: make(map[string]*UserFileEntry),
	}
}

// UpdateUserFiles replaces the list of shared files for a given user
// and updates their activity timestamp.
func (r *FileRegistry) UpdateUserFiles(nickname string, fileList []SharedFile) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// An empty file list from a federated peer is a signal they have no files to share,
	// but it still counts as an activity ping. A local user disconnecting will call
	// RemoveUser directly.
	if _, ok := r.files[nickname]; ok || len(fileList) > 0 {
		r.files[nickname] = &UserFileEntry{
			Files:       fileList,
			LastUpdated: time.Now(),
		}
		log.Printf("Updated file list for %s with %d items.", nickname, len(fileList))
	}
}

// RemoveUser clears all file information for a user (e.g., on local disconnect).
func (r *FileRegistry) RemoveUser(nickname string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.files, nickname)
	log.Printf("Removed user %s from file registry.", nickname)
}

// CleanupStaleEntries removes file listings for users who haven't been active
// within the timeout period. This is crucial for federated users.
func (r *FileRegistry) CleanupStaleEntries(timeout time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	cleanedCount := 0
	for nickname, entry := range r.files {
		if now.Sub(entry.LastUpdated) > timeout {
			delete(r.files, nickname)
			cleanedCount++
		}
	}

	if cleanedCount > 0 {
		log.Printf("Cleaned up %d stale user entries from file registry.", cleanedCount)
	}
}

// VerifyFileOwner checks if a specific user is sharing a file with a specific name.
func (r *FileRegistry) VerifyFileOwner(filename, owner string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, ok := r.files[owner]
	if !ok {
		return false
	}

	for _, file := range entry.Files {
		if file.Name == filename {
			return true
		}
	}

	return false
}

// Search finds files matching the query across all online users.
// It excludes files owned by the requester.
// If the query is a federated username, it returns all files for that user.
func (r *FileRegistry) Search(query string, requester string) []SearchResult {
	r.mu.Lock()
	defer r.mu.Unlock()

	var results []SearchResult
	trimmedQuery := strings.TrimSpace(query)

	if strings.HasPrefix(trimmedQuery, "@") && strings.Count(trimmedQuery, "@") == 2 {
		targetUser := trimmedQuery
		if targetUser == requester {
			return results // Don't return results if a user searches for themselves.
		}

		entry, ok := r.files[targetUser]
		if ok {
			for _, file := range entry.Files {
				if !file.IsDir {
					results = append(results, SearchResult{
						FileName: file.Name,
						Size:     file.Size,
						Peer:     targetUser,
					})
				}
			}
		}
		log.Printf("Direct user search for '%s' returned %d local results.", query, len(results))
		return results
	}

	lowerQuery := strings.ToLower(trimmedQuery)
	if lowerQuery == "" {
		return results
	}

	for nickname, entry := range r.files {
		if nickname == requester {
			continue
		}

		for _, file := range entry.Files {
			if !file.IsDir && strings.Contains(strings.ToLower(file.Name), lowerQuery) {
				results = append(results, SearchResult{
					FileName: file.Name,
					Size:     file.Size,
					Peer:     nickname,
				})
			}
		}
	}
	log.Printf("Keyword search for '%s' by '%s' returned %d results.", query, requester, len(results))
	return results
}

// TopFiles returns up to N largest files shared across all users.
func (r *FileRegistry) TopFiles(limit int) []SearchResult {
	r.mu.Lock()
	defer r.mu.Unlock()

	var allFiles []SearchResult
	for nickname, entry := range r.files {
		for _, file := range entry.Files {
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

	entry, ok := r.files[owner]
	if !ok {
		return SharedFile{}, false
	}

	for _, file := range entry.Files {
		if file.Name == filename {
			return file, true
		}
	}

	return SharedFile{}, false
}