package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
)

// S2SHandler holds dependencies for handling S2S requests.
type S2SHandler struct {
	Cfg *Config
	Hub *ChatHub
}

func NewS2SHandler(cfg *Config, hub *ChatHub) *S2SHandler {
	return &S2SHandler{
		Cfg: cfg,
		Hub: hub,
	}
}

func (h *S2SHandler) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

// NEW: Add a handler to initiate a federated transfer.
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

	log.Printf("S2S Transfer: Received request for %s from %s", req.FileName, req.RequesterPeer)

	// This server now tells its local client (the file owner) to start the upload process.
	// The unicast will succeed if the user is currently online.
	ok := h.Hub.unicast("upload_request", UploadRequestPayload{
		TransferID: req.TransferID,
		FileName:   req.FileName,
	}, req.FileOwner)

	if !ok {
		log.Printf("S2S Transfer: Could not find or message local user %s to start upload.", req.FileOwner)
		http.Error(w, "File owner is not online on this server.", http.StatusNotFound)
		return
	}

	log.Printf("S2S Transfer: Sent 'upload_request' to local user %s.", req.FileOwner)
	w.WriteHeader(http.StatusAccepted)
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