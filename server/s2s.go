package main

import (
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
}

func NewS2SHandler(cfg *Config, hub *ChatHub, dataManager *DataStreamManager) *S2SHandler {
	return &S2SHandler{
		Cfg:         cfg,
		Hub:         hub,
		DataManager: dataManager,
	}
}

func (h *S2SHandler) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// GET requests (like search) don't need auth
		if r.Method == "GET" {
			next.ServeHTTP(w, r)
			return
		}
		if h.Cfg.SharedSecret == "" {
			log.Printf("SECURITY WARNING: S2S endpoint accessed but no shared_secret is configured.")
			http.Error(w, "Endpoint not configured", http.StatusServiceUnavailable)
			return
		}
		authHeader := r.Header.Get("Authorization")
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token != h.Cfg.SharedSecret {
			log.Printf("S2S unauthorized access attempt from %s", r.RemoteAddr)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
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

	// Store the domain we need to forward the data back to.
	h.Hub.federatedTransfers.Store(req.TransferID, &federatedTransferState{
		targetDomain: req.RequesterPeerDomain,
	})

	// Tell our local client (the file owner) to start the upload process.
	ok := h.Hub.unicast("upload_request", UploadRequestPayload{
		TransferID: req.TransferID,
		FileName:   req.FileName,
	}, req.FileOwner)

	if !ok {
		log.Printf("S2S Transfer: Could not find or message local user %s to start upload.", req.FileOwner)
		h.Hub.federatedTransfers.Delete(req.TransferID) // Clean up
		http.Error(w, "File owner is not online on this server.", http.StatusNotFound)
		return
	}

	log.Printf("S2S Transfer: Sent 'upload_request' to local user %s.", req.FileOwner)
	w.WriteHeader(http.StatusAccepted)
}

// RelayData receives a proxied data stream from another server and pipes it to a local client.
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

	// --- START FIX ---
	// Wait for up to 10 seconds for the client's SSH channel to appear.
	for i := 0; i < 100; i++ { // 100 * 100ms = 10 seconds
		channel, ok = h.DataManager.GetAndRemovePending(streamKey)
		if ok {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	// --- END FIX ---

	if !ok {
		log.Printf("S2S Relay Error: Timed out waiting for pending client channel for stream %s", streamKey)
		http.Error(w, "No pending stream for this transfer ID", http.StatusNotFound)
		return
	}
	defer channel.Close()
	defer r.Body.Close()

	// Pipe the incoming HTTP request body directly into the SSH channel.
	bytesCopied, err := io.Copy(channel, r.Body)
	if err != nil {
		log.Printf("S2S Relay Error: Failed during copy for stream %s: %v", streamKey, err)
		// Can't send HTTP error as headers are likely already sent.
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

// Peers is an S2S endpoint that returns the server's current list of known peers.
func (h *S2SHandler) Peers(w http.ResponseWriter, r *http.Request) {
	h.Hub.mu.Lock()
	// Create a copy to avoid race conditions while encoding
	peers := make([]string, len(h.Hub.config.Peers))
	copy(peers, h.Hub.config.Peers)
	h.Hub.mu.Unlock()

	log.Printf("S2S Peers: Responding to peer request with %d known peers.", len(peers))
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(peers); err != nil {
		http.Error(w, "Failed to encode peers", http.StatusInternalServerError)
	}
}