// profile/manager.go
package profile

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Profile represents a user identity with a nickname and a path to their private key.
type Profile struct {
	Nickname string
	KeyPath  string
}

// getKeysDir ensures the directory for storing keys exists and returns its path.
func getKeysDir() (string, error) {
	dataDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	keysPath := filepath.Join(dataDir, "rosetui", "keys")
	if err := os.MkdirAll(keysPath, 0755); err != nil {
		return "", err
	}
	return keysPath, nil
}

// LoadProfiles scans the keys directory and loads all found profiles.
func LoadProfiles() ([]Profile, error) {
	keysDir, err := getKeysDir()
	if err != nil {
		return nil, err
	}

	files, err := os.ReadDir(keysDir)
	if err != nil {
		return nil, err
	}

	var profiles []Profile
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".pem") {
			keyPath := filepath.Join(keysDir, file.Name())
			nickPath := strings.Replace(keyPath, ".pem", ".nick", 1)

			if _, err := os.Stat(nickPath); err == nil {
				nickBytes, err := os.ReadFile(nickPath)
				if err != nil {
					continue // Skip if we can't read the nick file
				}
				nickname := strings.TrimSpace(string(nickBytes))
				profiles = append(profiles, Profile{
					Nickname: nickname,
					KeyPath:  keyPath,
				})
			}
		}
	}

	return profiles, nil
}

// CreateProfile generates a new RSA key pair and saves it with a nickname.
func CreateProfile(nickname string) (Profile, error) {
	keysDir, err := getKeysDir()
	if err != nil {
		return Profile{}, err
	}

	// Generate a new private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return Profile{}, err
	}

	// Encode private key to PEM format
	privateKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}
	pemData := pem.EncodeToMemory(privateKeyPEM)

	// Create a unique filename
	timestamp := time.Now().UnixMilli()
	safeNick := strings.ReplaceAll(nickname, " ", "_")
	baseName := fmt.Sprintf("%d_%s", timestamp, safeNick)

	keyPath := filepath.Join(keysDir, baseName+".pem")
	nickPath := filepath.Join(keysDir, baseName+".nick")

	// Write files to disk
	if err := os.WriteFile(keyPath, pemData, 0600); err != nil {
		return Profile{}, err
	}
	if err := os.WriteFile(nickPath, []byte(nickname), 0644); err != nil {
		return Profile{}, err
	}

	return Profile{
		Nickname: nickname,
		KeyPath:  keyPath,
	}, nil
}