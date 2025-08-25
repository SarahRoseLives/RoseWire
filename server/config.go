package main

import (
	"encoding/json"
	"os"
)

// Config holds the server's public-facing configuration.
type Config struct {
	Domain       string `json:"domain"`
	SharedSecret string `json:"shared_secret"` // A secret shared between trusted peers for S2S auth
}

// LoadConfig reads the configuration from a JSON file.
func LoadConfig(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	cfg := &Config{}
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}