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

// authMiddleware protects S2S endpoints using the shared secret.
func (h *S2SHandler) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Public GET endpoints like /search do not require auth.
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

// handleCreateActivity processes incoming "Create" activities, like chat messages.
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

// handleShareActivity processes incoming "Share" activities.
func (h *S2SHandler) handleShareActivity(activity Activity) {
	var shareObj ShareActivityObject
	if err := json.Unmarshal(activity.Object, &shareObj); err != nil {
		log.Printf("S2S Error: could not decode Share activity object: %v", err)
		return
	}
	h.Hub.fileRegistry.UpdateUserFiles(activity.Actor, shareObj.Files)
	log.Printf("S2S: Ingested %d shared files from %s into the local registry.", len(shareObj.Files), activity.Actor)
}

// NEW: Add a handler for S2S search requests.
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

	// Search the local registry for the query.
	results := h.Hub.fileRegistry.Search(query)
	log.Printf("S2S Search: Found %d results for query '%s' for a peer.", len(results), query)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(results); err != nil {
		http.Error(w, "Failed to encode results", http.StatusInternalServerError)
	}
}