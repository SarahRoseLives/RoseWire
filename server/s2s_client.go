// SERVER/s2s_client.go
package main

import (
	"bytes"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// S2SClient is responsible for sending activities to peer servers.
type S2SClient struct {
	client         *http.Client
	instanceSigner crypto.Signer
	instanceDomain string
	peerKeyCache   sync.Map // domain -> ed25519.PublicKey
}

// NewS2SClient creates a new client for S2S communication.
func NewS2SClient(signer crypto.Signer, domain string) *S2SClient {
	return &S2SClient{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		instanceSigner: signer,
		instanceDomain: domain,
	}
}

// getPeerPublicKey fetches and caches a peer's public key.
func (c *S2SClient) getPeerPublicKey(peerAddress string) (ed25519.PublicKey, error) {
	// Check cache first
	if key, ok := c.peerKeyCache.Load(peerAddress); ok {
		return key.(ed25519.PublicKey), nil
	}

	// Fetch from the peer's /actor endpoint
	url := fmt.Sprintf("http://%s/actor", peerAddress)
	resp, err := c.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch actor file from %s: %w", peerAddress, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("peer %s returned non-200 status for actor file: %s", peerAddress, resp.Status)
	}

	var actor InstanceActor
	if err := json.NewDecoder(resp.Body).Decode(&actor); err != nil {
		return nil, fmt.Errorf("failed to decode actor file from %s: %w", peerAddress, err)
	}

	// Decode the PEM-encoded public key
	block, _ := pem.Decode([]byte(actor.PublicKey))
	if block == nil {
		return nil, fmt.Errorf("failed to parse PEM block containing the public key")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	ed25519Pub, ok := pub.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key is not an ed25519 key")
	}

	// Store in cache
	c.peerKeyCache.Store(peerAddress, ed25519Pub)
	return ed25519Pub, nil
}

// signAndSend performs the common logic of signing a request body and sending it.
func (c *S2SClient) signAndSend(method, url string, body []byte) (*http.Response, error) {
	signature, err := c.instanceSigner.Sign(rand.Reader, body, crypto.Hash(0))
	if err != nil {
		return nil, fmt.Errorf("failed to sign request body: %w", err)
	}
	sig64 := base64.StdEncoding.EncodeToString(signature)

	var reader io.Reader
	if body != nil {
		reader = bytes.NewBuffer(body)
	}

	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-RoseWire-Identity", c.instanceDomain)
	req.Header.Set("X-RoseWire-Signature", sig64)

	return c.client.Do(req)
}

// PushActivity sends an activity to a peer's inbox.
func (c *S2SClient) PushActivity(peerAddress string, activity Activity) error {
	url := fmt.Sprintf("http://%s/api/s2s/inbox", peerAddress)
	body, err := json.Marshal(activity)
	if err != nil {
		return fmt.Errorf("failed to marshal activity: %w", err)
	}

	resp, err := c.signAndSend("POST", url, body)
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

// SearchPeer searches a peer for files (no signing needed for GET).
// --- START MODIFICATION: Add requester parameter ---
func (c *S2SClient) SearchPeer(peerAddress string, query string, requester string) ([]SearchResult, error) {
	// --- END MODIFICATION ---
	encodedQuery := url.QueryEscape(query)
	// --- START MODIFICATION: Add requester to URL ---
	encodedRequester := url.QueryEscape(requester)
	url := fmt.Sprintf("http://%s/api/s2s/search?query=%s&requester=%s", peerAddress, encodedQuery, encodedRequester)
	// --- END MODIFICATION ---

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

	resp, err := c.signAndSend("POST", url, body)
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

// RelayStream POSTs a stream of data to a peer's relay endpoint.
func (c *S2SClient) RelayStream(targetPeerAddress, transferID, streamIndex string, data io.Reader) error {
	// Note: Streaming bodies cannot be signed in one go. For now, we trust the initial transfer request.
	// A more advanced implementation could use chunked signing (e.g., AWS Signature V4).
	url := fmt.Sprintf("http://%s/api/s2s/data/%s/%s", targetPeerAddress, transferID, streamIndex)
	streamClient := &http.Client{}

	req, err := http.NewRequest("POST", url, data)
	if err != nil {
		return fmt.Errorf("failed to create relay request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-RoseWire-Identity", c.instanceDomain) // Still identify ourselves

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

// FetchPeers retrieves the list of known peers from another server.
func (c *S2SClient) FetchPeers(peerAddress string) ([]string, error) {
	url := fmt.Sprintf("http://%s/api/s2s/peers", peerAddress)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create peers request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send peers request to %s: %w", peerAddress, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("peer server returned non-200 status for peers: %s", resp.Status)
	}

	var peers []string
	if err := json.NewDecoder(resp.Body).Decode(&peers); err != nil {
		return nil, fmt.Errorf("failed to decode peers response: %w", err)
	}

	return peers, nil
}