// ssh/client.go
package ssh

import (
	"fmt"
	"net"
	"os"
	"rosetui/profile"
	"time"

	"golang.org/x/crypto/ssh"
)

// TestConnection attempts to authenticate with the server to verify credentials.
func TestConnection(p profile.Profile, serverAddress string) error {
	key, err := os.ReadFile(p.KeyPath)
	if err != nil {
		return fmt.Errorf("unable to read private key: %w", err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return fmt.Errorf("unable to parse private key: %w", err)
	}

	config := &ssh.ClientConfig{
		User: p.Nickname,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // Note: In a real-world app, you'd verify the host key.
		Timeout:         10 * time.Second,
	}

	// Connect to the SSH server on the custom port
	conn, err := ssh.Dial("tcp", net.JoinHostPort(serverAddress, "2222"), config)
	if err != nil {
		return fmt.Errorf("failed to dial: %w", err)
	}
	defer conn.Close()

	// If we reached here, authentication was successful.
	return nil
}