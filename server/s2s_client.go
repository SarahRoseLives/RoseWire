package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// S2SClient is responsible for sending activities to peer servers.
type S2SClient struct {
	client       *http.Client
	sharedSecret string
}

// NewS2SClient creates a new client for S2S communication.
func NewS2SClient(secret string) *S2SClient {
	return &S2SClient{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		sharedSecret: secret,
	}
}

// PushActivity sends an activity to a peer's inbox.
func (c *S2SClient) PushActivity(peerDomain string, activity Activity) error {
	// For local testing, we use http. In production, this should be https.
	url := fmt.Sprintf("http://%s/api/s2s/inbox", peerDomain)

	body, err := json.Marshal(activity)
	if err != nil {
		return fmt.Errorf("failed to marshal activity: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.sharedSecret != "" {
		req.Header.Set("Authorization", "Bearer "+c.sharedSecret)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to %s: %w", peerDomain, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("S2S Push to %s failed with status: %s", peerDomain, resp.Status)
		return fmt.Errorf("peer server returned non-2xx status: %s", resp.Status)
	}

	log.Printf("Successfully pushed activity of type '%s' to peer %s", activity.Type, peerDomain)
	return nil
}