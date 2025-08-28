// SERVER/admin.go
package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"syscall"

	"github.com/gorilla/sessions"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"
)

const (
	adminConfigFile    = "admin.json"
	sessionName        = "rosewire-admin-session"
	adminLoginHTMLFile = "admin_login.html"
	adminDashHTMLFile  = "admin_dashboard.html"
)

// AdminConfig holds all administrative settings.
type AdminConfig struct {
	mu                    sync.RWMutex
	PasswordHash          string   `json:"password_hash"`
	BlockedPeers          []string `json:"blocked_peers"`
	BannedUsers           []string `json:"banned_users"`           // Local users who cannot log in
	BlockedFederatedUsers []string `json:"blocked_federated_users"` // Federated users who are muted
	BlockedFileTypes      []string `json:"blocked_file_types"`
}

// AdminHandler manages all HTTP requests for the admin panel.
type AdminHandler struct {
	Store       *sessions.CookieStore
	AdminConfig *AdminConfig
	ChatHub     *ChatHub
	LoginHTML   string
	DashHTML    string
}

// NewAdminHandler creates a new handler for the admin interface.
func NewAdminHandler(store *sessions.CookieStore, cfg *AdminConfig, hub *ChatHub) *AdminHandler {
	loginHTML := mustLoadHTML(adminLoginHTMLFile)
	dashHTML := mustLoadHTML(adminDashHTMLFile)
	return &AdminHandler{
		Store:       store,
		AdminConfig: cfg,
		ChatHub:     hub,
		LoginHTML:   loginHTML,
		DashHTML:    dashHTML,
	}
}

// mustLoadHTML loads HTML from a file or panics if it fails
func mustLoadHTML(filename string) string {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Fatalf("Failed to load HTML file %s: %v", filename, err)
	}
	return string(data)
}

// LoadAdminConfig loads the admin configuration from disk.
func LoadAdminConfig(path string) (*AdminConfig, error) {
	cfg := &AdminConfig{}
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil // Return empty config if it doesn't exist
		}
		return nil, err
	}
	defer file.Close()
	if err := json.NewDecoder(file).Decode(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Save writes the admin configuration to disk.
func (c *AdminConfig) Save(path string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(c)
}

// HashPassword generates a bcrypt hash of the password.
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

// CheckPasswordHash compares a raw password with a hash.
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// SetupAdminPassword prompts the admin to set an initial password via the console.
func SetupAdminPassword(config *AdminConfig) error {
	fmt.Println("--- Admin Password Setup ---")
	fmt.Print("Please create a password for the admin web panel (user: SYSTEM): ")
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return err
	}
	password := strings.TrimSpace(string(bytePassword))
	fmt.Println()

	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters long")
	}

	fmt.Print("Confirm password: ")
	bytePassword, err = term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return err
	}
	confirmPassword := strings.TrimSpace(string(bytePassword))
	fmt.Println()

	if password != confirmPassword {
		return fmt.Errorf("passwords do not match")
	}

	hash, err := HashPassword(password)
	if err != nil {
		return fmt.Errorf("could not hash password: %v", err)
	}

	config.PasswordHash = hash
	if err := config.Save(adminConfigFile); err != nil {
		return fmt.Errorf("failed to save admin configuration: %v", err)
	}

	log.Println("Admin password has been set successfully for the SYSTEM user.")
	return nil
}

// AuthMiddleware checks if a user has a valid admin session.
func (h *AdminHandler) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, _ := h.Store.Get(r, sessionName)
		auth, ok := session.Values["authenticated"].(bool)
		user, userOk := session.Values["username"].(string)

		if !ok || !auth || !userOk || user != "SYSTEM" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// HandleLogin processes the admin login form submission.
func (h *AdminHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	username := r.FormValue("username")
	password := r.FormValue("password")

	if username == "SYSTEM" && CheckPasswordHash(password, h.AdminConfig.PasswordHash) {
		session, _ := h.Store.Get(r, sessionName)
		session.Values["authenticated"] = true
		session.Values["username"] = "SYSTEM"
		err := session.Save(r, w)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/admin", http.StatusFound)
	} else {
		// Redirect back to login with an error message
		http.Redirect(w, r, "/admin?error=invalid_credentials", http.StatusFound)
	}
}

// HandleLogout logs the admin out.
func (h *AdminHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	session, _ := h.Store.Get(r, sessionName)
	session.Values = make(map[interface{}]interface{}) // Clear all session data
	session.Options.MaxAge = -1                      // Expire the cookie
	session.Save(r, w)
	http.Redirect(w, r, "/admin", http.StatusFound)
}

// ServeAdminPanel serves the login page or the admin dashboard.
func (h *AdminHandler) ServeAdminPanel(w http.ResponseWriter, r *http.Request) {
	session, _ := h.Store.Get(r, sessionName)
	auth, ok := session.Values["authenticated"].(bool)
	user, userOk := session.Values["username"].(string)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if !ok || !auth || !userOk || user != "SYSTEM" {
		fmt.Fprint(w, h.LoginHTML)
		return
	}
	fmt.Fprint(w, h.DashHTML)
}

// HandleGetConfig returns the current admin configuration as JSON.
func (h *AdminHandler) HandleGetConfig(w http.ResponseWriter, r *http.Request) {
	h.AdminConfig.mu.RLock()
	defer h.AdminConfig.mu.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(h.AdminConfig)
}

// HandleUpdateConfig adds or removes items from the admin config lists.
func (h *AdminHandler) HandleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Action string `json:"action"` // "add" or "remove"
		Type   string `json:"type"`   // "peer", "user", "filetype"
		Value  string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	req.Value = strings.ToLower(strings.TrimSpace(req.Value))
	if req.Value == "" {
		http.Error(w, "Value cannot be empty", http.StatusBadRequest)
		return
	}

	h.AdminConfig.mu.Lock()
	defer h.AdminConfig.mu.Unlock()

	var list *[]string
	switch req.Type {
	case "peer":
		list = &h.AdminConfig.BlockedPeers
	case "user":
		if strings.Contains(req.Value, "@") {
			// This is a federated user, add to the block list.
			list = &h.AdminConfig.BlockedFederatedUsers
		} else {
			// This is a local user, add to the ban list.
			// The SYSTEM user cannot be banned.
			if req.Value == "system" {
				http.Error(w, "Cannot ban the SYSTEM user", http.StatusBadRequest)
				return
			}
			list = &h.AdminConfig.BannedUsers
		}
	case "filetype":
		// Ensure filetypes start with a dot
		if !strings.HasPrefix(req.Value, ".") {
			req.Value = "." + req.Value
		}
		list = &h.AdminConfig.BlockedFileTypes
	default:
		http.Error(w, "Invalid type", http.StatusBadRequest)
		return
	}

	// Update the list
	if req.Action == "add" {
		// Avoid duplicates
		found := false
		for _, item := range *list {
			if item == req.Value {
				found = true
				break
			}
		}
		if !found {
			*list = append(*list, req.Value)
		}
	} else if req.Action == "remove" {
		newList := []string{}
		for _, item := range *list {
			if item != req.Value {
				newList = append(newList, item)
			}
		}
		*list = newList
	}

	// For user actions, disconnect local users or purge remote user files.
	if req.Type == "user" && req.Action == "add" {
		if strings.Contains(req.Value, "@") {
			// This is a federated user; remove their files from the local registry.
			log.Printf("Admin: Blocking federated user %s and removing their files.", req.Value)
			h.ChatHub.fileRegistry.RemoveUser(req.Value)
		} else {
			// This is a local user; disconnect them if they are online.
			federatedName := fmt.Sprintf("@%s@%s", req.Value, h.ChatHub.config.Domain)
			h.ChatHub.mu.Lock()
			if client, ok := h.ChatHub.clients[federatedName]; ok {
				log.Printf("Admin: Disconnecting banned user %s", federatedName)
				go client.Close()
			}
			h.ChatHub.mu.Unlock()
		}
	}

	if err := h.AdminConfig.Save(adminConfigFile); err != nil {
		http.Error(w, "Failed to save config", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}