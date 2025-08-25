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
func (c *S2SClient) PushActivity(peerAddress string, activity Activity) error {
	url := fmt.Sprintf("http://%s/api/s2s/inbox", peerAddress)

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
		return fmt.Errorf("failed to send request to %s: %w", peerAddress, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("S2S Push to %s failed with status: %s", peerAddress, resp.Status)
		return fmt.Errorf("peer server returned non-2xx status: %s", resp.Status)
	}

	log.Printf("Successfully pushed activity of type '%s' to peer %s", activity.Type, peerAddress)
	return nil
}

// SearchPeer searches a peer for files.
func (c *S2SClient) SearchPeer(peerAddress string, query string) ([]SearchResult, error) {
	encodedQuery := url.QueryEscape(query)
	url := fmt.Sprintf("http://%s/api/s2s/search?query=%s", peerAddress, encodedQuery)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create search request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send search request to %s: %w", peerAddress, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Printf("S2S Search to %s failed with status %s: %s", peerAddress, resp.Status, string(bodyBytes))
		return nil, fmt.Errorf("peer server returned non-200 status: %s", resp.Status)
	}

	var results []SearchResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("failed to decode search results from %s: %w", err)
	}

	log.Printf("S2S Search: Received %d results from peer %s for query '%s'", len(results), peerAddress, query)
	return results, nil
}

// RequestTransfer sends a request to a peer to initiate a file transfer.
func (c *S2SClient) RequestTransfer(peerAddress string, transferReq S2STransferRequest) error {
	url := fmt.Sprintf("http://%s/api/s2s/transfers", peerAddress)

	body, err := json.Marshal(transferReq)
	if err != nil {
		return fmt.Errorf("failed to marshal transfer request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create transfer request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.sharedSecret != "" {
		req.Header.Set("Authorization", "Bearer "+c.sharedSecret)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send transfer request to %s: %w", peerAddress, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("peer %s did not accept transfer request, status: %s", peerAddress, resp.Status)
	}

	log.Printf("S2S Transfer: Peer %s accepted transfer request %s.", peerAddress, transferReq.TransferID)
	return nil
}

// RelayStream POSTs a stream of data to a peer's relay endpoint. It now accepts the full peer address.
func (c *S2SClient) RelayStream(targetPeerAddress, transferID, streamIndex string, data io.Reader) error {
	url := fmt.Sprintf("http://%s/api/s2s/data/%s/%s", targetPeerAddress, transferID, streamIndex)
	streamClient := &http.Client{} // Use a client without a timeout for streaming

	req, err := http.NewRequest("POST", url, data)
	if err != nil {
		return fmt.Errorf("failed to create relay request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	if c.sharedSecret != "" {
		req.Header.Set("Authorization", "Bearer "+c.sharedSecret)
	}

	log.Printf("S2S Relay: Sending data for stream %s:%s to %s", transferID, streamIndex, targetPeerAddress)
	resp, err := streamClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send relay stream to %s: %w", targetPeerAddress, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("peer %s returned non-OK status for relay stream: %s", targetPeerAddress, resp.Status)
	}

	log.Printf("S2S Relay: Successfully sent stream %s:%s", transferID, streamIndex)
	return nil
}