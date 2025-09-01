// config/manager.go
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// AppConfig holds the application's configuration, like the last used server.
type AppConfig struct {
	ServerAddress string `json:"server_address"`
}

// GetConfigPath returns the path to the configuration file.
func GetConfigPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	appDir := filepath.Join(configDir, "rosetui")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(appDir, "config.json"), nil
}

// LoadConfig reads the configuration from disk.
func LoadConfig() (AppConfig, error) {
	configPath, err := GetConfigPath()
	if err != nil {
		return AppConfig{}, err
	}

	var config AppConfig
	// Set default if file doesn't exist
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		config.ServerAddress = "rosewire.rosevines.network"
		return config, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return AppConfig{}, err
	}

	err = json.Unmarshal(data, &config)
	return config, err
}

// SaveConfig writes the current configuration to disk.
func SaveConfig(config AppConfig) error {
	configPath, err := GetConfigPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}