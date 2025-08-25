package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"golang.org/x/crypto/ssh"
)

// S2SHandler holds dependencies for handling S2S requests.
type S2SHandler struct {
	Cfg         *Config
	Hub         *ChatHub
	DataManager *DataStreamManager
	S2SClient   *S2SClient // Added for fetching peer keys
}

func NewS2SHandler(cfg *Config, hub *ChatHub, dataManager *DataStreamManager, s2sClient *S2SClient) *S2SHandler {
	return &S2SHandler{
		Cfg:         cfg,
		Hub:         hub,
		DataManager: dataManager,
		S2SClient:   s2sClient,
	}
}

func (h *S2SHandler) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// GET requests (like search and peers) don't need auth
		if r.Method == "GET" {
			next.ServeHTTP(w, r)
			return
		}

		// For streaming data, we only check the identity header for now.
		// A more advanced implementation would use chunked signing.
		if strings.HasPrefix(r.URL.Path, "/api/s2s/data/") {
			if r.Header.Get("X-RoseWire-Identity") == "" {
				http.Error(w, "Unauthorized: Missing identity header", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		identity := r.Header.Get("X-RoseWire-Identity")
		signatureB64 := r.Header.Get("X-RoseWire-Signature")

		if identity == "" || signatureB64 == "" {
			log.Printf("S2S unauthorized: Missing identity or signature from %s", r.RemoteAddr)
			http.Error(w, "Unauthorized: Missing identity or signature header", http.StatusUnauthorized)
			return
		}

		signature, err := base64.StdEncoding.DecodeString(signatureB64)
		if err != nil {
			log.Printf("S2S unauthorized: Invalid signature encoding from %s", r.RemoteAddr)
			http.Error(w, "Unauthorized: Invalid signature format", http.StatusBadRequest)
			return
		}

		// Read the body, which is required for signature verification.
		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("S2S error: Could not read body for verification from %s: %v", r.RemoteAddr, err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		// VERY IMPORTANT: Put the body back so the actual handler can read it.
		r.Body = io.NopCloser(bytes.NewBuffer(body))

		// Find the full peer address (host:port) to fetch the key
		var peerAddress string
		for _, p := range h.Cfg.Peers {
			if strings.HasPrefix(p, identity) {
				peerAddress = p
				break
			}
		}
		if peerAddress == "" {
			log.Printf("S2S unauthorized: Could not find peer address for identity '%s'", identity)
			http.Error(w, "Unauthorized: Unknown peer", http.StatusUnauthorized)
			return
		}

		publicKey, err := h.S2SClient.getPeerPublicKey(peerAddress)
		if err != nil {
			log.Printf("S2S unauthorized: Could not get public key for '%s': %v", identity, err)
			http.Error(w, "Unauthorized: Could not retrieve peer key", http.StatusUnauthorized)
			return
		}

		if !ed25519.Verify(publicKey, body, signature) {
			log.Printf("S2S unauthorized: Invalid signature for identity '%s'", identity)
			http.Error(w, "Unauthorized: Invalid signature", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Inbox is the endpoint for receiving activities from other servers.
func (h *S2SHandler) Inbox(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var activity Activity
	if err := json.NewDecoder(r.Body).Decode(&activity); err != nil {
		http.Error(w, "Bad request: could not decode activity", http.StatusBadRequest)
		return
	}
	log.Printf("S2S Inbox: Received activity of type '%s' from actor '%s'", activity.Type, activity.Actor)
	switch activity.Type {
	case "Create":
		h.handleCreateActivity(activity)
	case "Share":
		h.handleShareActivity(activity)
	default:
		log.Printf("S2S Inbox: Received unhandled activity type '%s'", activity.Type)
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *S2SHandler) InitiateTransfer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req S2STransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request: could not decode transfer request", http.StatusBadRequest)
		return
	}

	log.Printf("S2S Transfer: Received request for %s from %s. Will forward to %s", req.FileName, req.RequesterPeer, req.RequesterPeerDomain)

	h.Hub.federatedTransfers.Store(req.TransferID, &federatedTransferState{
		targetDomain: req.RequesterPeerDomain,
	})

	ok := h.Hub.unicast("upload_request", UploadRequestPayload{
		TransferID: req.TransferID,
		FileName:   req.FileName,
	}, req.FileOwner)

	if !ok {
		log.Printf("S2S Transfer: Could not find or message local user %s to start upload.", req.FileOwner)
		h.Hub.federatedTransfers.Delete(req.TransferID)
		http.Error(w, "File owner is not online on this server.", http.StatusNotFound)
		return
	}

	log.Printf("S2S Transfer: Sent 'upload_request' to local user %s.", req.FileOwner)
	w.WriteHeader(http.StatusAccepted)
}

func (h *S2SHandler) RelayData(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	vars := mux.Vars(r)
	transferID := vars["tid"]
	streamIndex := vars["idx"]
	streamKey := fmt.Sprintf("%s:%s", transferID, streamIndex)

	log.Printf("S2S Relay: Receiving data for stream %s", streamKey)

	var channel ssh.Channel
	var ok bool

	for i := 0; i < 100; i++ {
		channel, ok = h.DataManager.GetAndRemovePending(streamKey)
		if ok {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !ok {
		log.Printf("S2S Relay Error: Timed out waiting for pending client channel for stream %s", streamKey)
		http.Error(w, "No pending stream for this transfer ID", http.StatusNotFound)
		return
	}
	defer channel.Close()
	defer r.Body.Close()

	bytesCopied, err := io.Copy(channel, r.Body)
	if err != nil {
		log.Printf("S2S Relay Error: Failed during copy for stream %s: %v", streamKey, err)
		return
	}
	log.Printf("S2S Relay: Finished for stream %s, copied %d bytes.", streamKey, bytesCopied)
	w.WriteHeader(http.StatusOK)
}

func (h *S2SHandler) Search(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	query := r.URL.Query().Get("query")
	if query == "" {
		http.Error(w, "Missing 'query' parameter", http.StatusBadRequest)
		return
	}
	results := h.Hub.fileRegistry.Search(query)
	log.Printf("S2S Search: Found %d results for query '%s' for a peer.", len(results), query)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(results); err != nil {
		http.Error(w, "Failed to encode results", http.StatusInternalServerError)
	}
}

func (h *S2SHandler) handleCreateActivity(activity Activity) {
	var chatObj ChatActivityObject
	if err := json.Unmarshal(activity.Object, &chatObj); err != nil {
		log.Printf("S2S Error: could not decode Create activity object: %v", err)
		return
	}
	payload := ChatBroadcastPayload{
		Timestamp: time.Now().Format("15:04"),
		Nickname:  activity.Actor,
		Text:      chatObj.Content,
		IsSystem:  false,
	}
	h.Hub.broadcast("chat_broadcast", payload, "")
	log.Printf("S2S: Relayed federated chat message from %s to local clients.", activity.Actor)
}

func (h *S2SHandler) handleShareActivity(activity Activity) {
	var shareObj ShareActivityObject
	if err := json.Unmarshal(activity.Object, &shareObj); err != nil {
		log.Printf("S2S Error: could not decode Share activity object: %v", err)
		return
	}
	h.Hub.fileRegistry.UpdateUserFiles(activity.Actor, shareObj.Files)
	log.Printf("S2S: Ingested %d shared files from %s into the local registry.", len(shareObj.Files), activity.Actor)
}

func (h *S2SHandler) Peers(w http.ResponseWriter, r *http.Request) {
	h.Hub.mu.Lock()
	peers := make([]string, len(h.Hub.config.Peers))
	copy(peers, h.Hub.config.Peers)
	h.Hub.mu.Unlock()

	log.Printf("S2S Peers: Responding to peer request with %d known peers.", len(peers))
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(peers); err != nil {
		http.Error(w, "Failed to encode peers", http.StatusInternalServerError)
	}
}