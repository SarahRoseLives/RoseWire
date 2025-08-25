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
		// Do not require auth for GET requests to allow for simple health checks in the future
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
		// FIX: Use the correct integer status code constant.
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
	default:
		log.Printf("S2S Inbox: Received unhandled activity type '%s'", activity.Type)
	}

	// Respond immediately to the sending server.
	w.WriteHeader(http.StatusAccepted)
}

// handleCreateActivity processes incoming "Create" activities, like chat messages.
func (h *S2SHandler) handleCreateActivity(activity Activity) {
	// For now, we assume the object of a "Create" activity is a chat message.
	var chatObj ChatActivityObject
	if err := json.Unmarshal(activity.Object, &chatObj); err != nil {
		log.Printf("S2S Error: could not decode Create activity object: %v", err)
		return
	}

	// Construct a payload to broadcast to our local clients.
	payload := ChatBroadcastPayload{
		Timestamp: time.Now().Format("15:04"),
		Nickname:  activity.Actor, // The Nickname is the full federated name of the original author.
		Text:      chatObj.Content,
		IsSystem:  false,
	}

	// Broadcast the federated message to all clients connected to this instance.
	h.Hub.broadcast("chat_broadcast", payload, "")
	log.Printf("S2S: Relayed federated chat message from %s to local clients.", activity.Actor)
}