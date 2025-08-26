package main

import (
	"encoding/json"
	"fmt"
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
	adminConfigFile = "admin.json"
	sessionName     = "rosewire-admin-session"
)

// AdminConfig holds all administrative settings.
type AdminConfig struct {
	mu               sync.RWMutex
	PasswordHash     string   `json:"password_hash"`
	BlockedPeers     []string `json:"blocked_peers"`
	BannedUsers      []string `json:"banned_users"`
	BlockedFileTypes []string `json:"blocked_file_types"`
}

// AdminHandler manages all HTTP requests for the admin panel.
type AdminHandler struct {
	Store       *sessions.CookieStore
	AdminConfig *AdminConfig
	ChatHub     *ChatHub
}

// NewAdminHandler creates a new handler for the admin interface.
func NewAdminHandler(store *sessions.CookieStore, cfg *AdminConfig, hub *ChatHub) *AdminHandler {
	return &AdminHandler{
		Store:       store,
		AdminConfig: cfg,
		ChatHub:     hub,
	}
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
	fmt.Print("Please create a password for the admin web panel: ")
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

	log.Println("Admin password has been set successfully.")
	return nil
}

// AuthMiddleware checks if a user has a valid admin session.
func (h *AdminHandler) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, _ := h.Store.Get(r, sessionName)
		auth, ok := session.Values["authenticated"].(bool)
		if !ok || !auth {
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
	password := r.FormValue("password")
	if CheckPasswordHash(password, h.AdminConfig.PasswordHash) {
		session, _ := h.Store.Get(r, sessionName)
		session.Values["authenticated"] = true
		err := session.Save(r, w)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/admin", http.StatusFound)
	} else {
		// Redirect back to login with an error message
		http.Redirect(w, r, "/admin?error=invalid_password", http.StatusFound)
	}
}

// HandleLogout logs the admin out.
func (h *AdminHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	session, _ := h.Store.Get(r, sessionName)
	session.Values["authenticated"] = false
	session.Options.MaxAge = -1 // Expire the cookie
	session.Save(r, w)
	http.Redirect(w, r, "/admin", http.StatusFound)
}

// ServeAdminPanel serves the login page or the admin dashboard.
func (h *AdminHandler) ServeAdminPanel(w http.ResponseWriter, r *http.Request) {
	session, _ := h.Store.Get(r, sessionName)
	auth, ok := session.Values["authenticated"].(bool)
	if !ok || !auth {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, adminLoginHTML)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, adminDashboardHTML)
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
		list = &h.AdminConfig.BannedUsers
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

	// For user bans, disconnect them if they are currently online.
	if req.Type == "user" && req.Action == "add" {
		federatedName := fmt.Sprintf("@%s@%s", req.Value, h.ChatHub.config.Domain)
		h.ChatHub.mu.Lock()
		if client, ok := h.ChatHub.clients[federatedName]; ok {
			log.Printf("Admin: Disconnecting banned user %s", federatedName)
			go client.Close()
		}
		h.ChatHub.mu.Unlock()
	}

	if err := h.AdminConfig.Save(adminConfigFile); err != nil {
		http.Error(w, "Failed to save config", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

const adminLoginHTML = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>RoseWire Admin Login</title>
    <style>
        body { font-family: sans-serif; background: #2c2f33; color: #fff; display: flex; justify-content: center; align-items: center; height: 100vh; margin: 0; }
        .login-box { background: #23272a; padding: 40px; border-radius: 8px; box-shadow: 0 4px 15px rgba(0,0,0,0.5); width: 300px; }
        h1 { text-align: center; color: #ea4c89; margin-bottom: 24px; }
        label { display: block; margin-bottom: 8px; color: #99aab5; }
        input[type="password"] { width: 100%; padding: 10px; border-radius: 4px; border: 1px solid #7289da; background: #40444b; color: #fff; box-sizing: border-box; }
        button { width: 100%; padding: 12px; background: #7289da; color: #fff; border: none; border-radius: 4px; cursor: pointer; font-size: 16px; margin-top: 20px; }
        button:hover { background: #677bc4; }
        .error { color: #f04747; text-align: center; margin-top: 15px; }
    </style>
</head>
<body>
    <div class="login-box">
        <h1>RoseWire Admin</h1>
        <form action="/admin/login" method="post">
            <label for="password">Password:</label>
            <input type="password" id="password" name="password" required>
            <button type="submit">Login</button>
        </form>
        <script>
            const urlParams = new URLSearchParams(window.location.search);
            if (urlParams.get('error') === 'invalid_password') {
                const errorDiv = document.createElement('div');
                errorDiv.className = 'error';
                errorDiv.textContent = 'Invalid password.';
                document.querySelector('form').appendChild(errorDiv);
            }
        </script>
    </div>
</body>
</html>
`

const adminDashboardHTML = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8"><title>RoseWire Admin Dashboard</title>
    <style>
        body { font-family: sans-serif; background: #2c2f33; color: #fff; margin: 0; padding: 20px; }
        .container { max-width: 1200px; margin: auto; }
        header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 30px; }
        header h1 { color: #ea4c89; margin: 0; }
        header a { color: #7289da; text-decoration: none; }
        .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(350px, 1fr)); gap: 20px; }
        .panel { background: #23272a; padding: 20px; border-radius: 8px; }
        .panel h2 { margin-top: 0; border-bottom: 2px solid #ea4c89; padding-bottom: 10px; }
        ul { list-style: none; padding: 0; }
        li { background: #40444b; padding: 10px; border-radius: 4px; margin-bottom: 8px; display: flex; justify-content: space-between; align-items: center; }
        .remove-btn { background: #f04747; color: white; border: none; cursor: pointer; padding: 5px 10px; border-radius: 4px; }
        .form-group { display: flex; gap: 10px; margin-top: 15px; }
        input[type="text"] { flex-grow: 1; padding: 8px; border-radius: 4px; border: 1px solid #7289da; background: #40444b; color: #fff; }
        .add-btn { background: #43b581; color: white; border: none; cursor: pointer; padding: 8px 15px; border-radius: 4px; }
    </style>
</head>
<body>
<div class="container">
    <header>
        <h1>Admin Dashboard</h1>
        <a href="/admin/logout">Logout</a>
    </header>
    <div class="grid">
        <div class="panel" id="peers-panel">
            <h2>Blocked Peers</h2>
            <ul id="blocked-peers-list"></ul>
            <div class="form-group">
                <input type="text" id="peer-input" placeholder="example.com">
                <button class="add-btn" onclick="addItem('peer', 'peer-input')">Block</button>
            </div>
        </div>
        <div class="panel" id="users-panel">
            <h2>Banned Users</h2>
            <ul id="banned-users-list"></ul>
            <div class="form-group">
                <input type="text" id="user-input" placeholder="nickname">
                <button class="add-btn" onclick="addItem('user', 'user-input')">Ban</button>
            </div>
        </div>
        <div class="panel" id="filetypes-panel">
            <h2>Blocked Filetypes</h2>
            <ul id="blocked-filetypes-list"></ul>
            <div class="form-group">
                <input type="text" id="filetype-input" placeholder=".exe">
                <button class="add-btn" onclick="addItem('filetype', 'filetype-input')">Block</button>
            </div>
        </div>
    </div>
</div>
<script>
    async function fetchConfig() {
        const response = await fetch('/api/admin/config');
        const config = await response.json();
        renderList('blocked-peers-list', config.blocked_peers || [], 'peer');
        renderList('banned-users-list', config.banned_users || [], 'user');
        renderList('blocked-filetypes-list', config.blocked_file_types || [], 'filetype');
    }

    function renderList(listId, items, type) {
        const listEl = document.getElementById(listId);
        listEl.innerHTML = '';
        if (!items || items.length === 0) {
            listEl.innerHTML = '<li>None</li>';
            return;
        }
        items.forEach(item => {
            const li = document.createElement('li');
            li.textContent = item;
            const removeBtn = document.createElement('button');
            removeBtn.className = 'remove-btn';
            removeBtn.textContent = 'Remove';
            removeBtn.onclick = () => updateItem('remove', type, item);
            li.appendChild(removeBtn);
            listEl.appendChild(li);
        });
    }

    async function updateItem(action, type, value) {
        if (!value) return;
        await fetch('/api/admin/update', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ action, type, value })
        });
        fetchConfig();
    }

    function addItem(type, inputId) {
        const input = document.getElementById(inputId);
        const value = input.value.trim();
        if (value) {
            updateItem('add', type, value);
            input.value = '';
        }
    }

    document.addEventListener('DOMContentLoaded', fetchConfig);
</script>
</body>
</html>
`