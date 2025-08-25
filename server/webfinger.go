package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// WebFingerResponse is the structure for a JRD (JSON Resource Descriptor) response.
type WebFingerResponse struct {
	Subject string          `json:"subject"`
	Links   []WebFingerLink `json:"links"`
}

// WebFingerLink describes a link related to the resource.
type WebFingerLink struct {
	Rel      string `json:"rel"`
	Type     string `json:"type,omitempty"`
	Href     string `json:"href,omitempty"`
	Template string `json:"template,omitempty"`
}

// WebFingerHandler serves WebFinger requests.
type WebFingerHandler struct {
	Cfg    *Config
	NickDB *NickDB
}

func (h *WebFingerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	resource := r.URL.Query().Get("resource")
	if resource == "" {
		http.Error(w, "Missing 'resource' query parameter", http.StatusBadRequest)
		return
	}

	// The resource should be in the format "acct:nickname@domain"
	if !strings.HasPrefix(resource, "acct:") {
		http.Error(w, "Invalid resource format", http.StatusBadRequest)
		return
	}

	userAndDomain := strings.TrimPrefix(resource, "acct:")
	parts := strings.Split(userAndDomain, "@")
	if len(parts) != 2 {
		http.Error(w, "Invalid acct format", http.StatusBadRequest)
		return
	}

	nickname := parts[0]
	domain := parts[1]

	// This server only serves requests for its own domain.
	if domain != h.Cfg.Domain {
		http.Error(w, fmt.Sprintf("Resource not hosted on this server. Expected domain %s, got %s", h.Cfg.Domain, domain), http.StatusNotFound)
		return
	}

	h.NickDB.Lock()
	pubKey, ok := h.NickDB.NickToKey[nickname]
	h.NickDB.Unlock()

	if !ok {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	response := WebFingerResponse{
		Subject: resource,
		Links: []WebFingerLink{
			{
				// This custom link type tells other RoseWire servers where to find the public key.
				Rel:  "roswire:ssh_public_key",
				Type: "text/plain",
				Href: pubKey,
			},
		},
	}

	w.Header().Set("Content-Type", "application/jrd+json")
	json.NewEncoder(w).Encode(response)
}
