package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
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

// NEW: Add a method to search a peer for files.
func (c *S2SClient) SearchPeer(peerDomain string, query string) ([]SearchResult, error) {
	// URL-encode the query to handle special characters safely.
	encodedQuery := url.QueryEscape(query)
	url := fmt.Sprintf("http://%s/api/s2s/search?query=%s", peerDomain, encodedQuery)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create search request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send search request to %s: %w", peerDomain, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Printf("S2S Search to %s failed with status %s: %s", peerDomain, resp.Status, string(bodyBytes))
		return nil, fmt.Errorf("peer server returned non-200 status: %s", resp.Status)
	}

	var results []SearchResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("failed to decode search results from %s: %w", peerDomain, err)
	}

	log.Printf("S2S Search: Received %d results from peer %s for query '%s'", len(results), peerDomain, query)
	return results, nil
}