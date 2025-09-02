// library/manager.go
package library

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// LocalFile represents a file on the local filesystem.
type LocalFile struct {
	Name string
	Size int64
}

// Config stores the path to the user's library.
type Config struct {
	LibraryPath string `json:"library_path"`
}

// getConfigPath returns the path to the library configuration file.
func getConfigPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	appDir := filepath.Join(configDir, "rosetui")
	// The app dir should already exist from the main config, but we ensure it here.
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(appDir, "library.json"), nil
}

// LoadConfig reads the library path from disk.
func LoadConfig() (string, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return "", err
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return "", nil // No config file yet, return empty path
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", err
	}

	var config Config
	err = json.Unmarshal(data, &config)
	return config.LibraryPath, err
}

// SaveConfig writes the library path to disk.
func SaveConfig(path string) error {
	configPath, err := getConfigPath()
	if err != nil {
		return err
	}

	config := Config{LibraryPath: path}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

// ScanDirectory lists all files in the given path.
func ScanDirectory(path string) ([]LocalFile, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("could not read directory: %w", err)
	}

	var files []LocalFile
	for _, entry := range entries {
		if !entry.IsDir() {
			info, err := entry.Info()
			if err != nil {
				continue // Skip files we can't get info for
			}
			files = append(files, LocalFile{
				Name: info.Name(),
				Size: info.Size(),
			})
		}
	}
	return files, nil
}

// GetFileHash calculates the SHA256 hash of a file.
func GetFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}