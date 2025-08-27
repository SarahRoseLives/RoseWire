package main

import (
	"encoding/json"
	"os"
)

// Config holds the server's public-facing configuration.
type Config struct {
	Domain         string   `json:"domain"`
	SshListenAddr  string   `json:"ssh_listen_addr"`  // Address for the SSH server, e.g., "0.0.0.0:2222"
	HttpListenAddr string   `json:"http_listen_addr"` // Address for the HTTP server, e.g., "0.0.0.0:8080"
	Peers          []string `json:"peers"`            // A list of peer instance domains to push activities to
	CertFile       string   `json:"cert_file"`        // Path to the TLS certificate file (e.g., fullchain.pem)
	KeyFile        string   `json:"key_file"`         // Path to the TLS private key file (e.g., privkey.pem)
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