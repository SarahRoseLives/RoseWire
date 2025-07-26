package main

import (
	"encoding/json"
	"html/template"
	"net/http"
	"os"
	"time"
)

// ServerStatus contains health and network info for the web status page.
type ServerStatus struct {
	Hostname          string   `json:"hostname"`
	Addr              string   `json:"address"`
	StartTime         string   `json:"start_time"`
	UptimeSeconds     int64    `json:"uptime_seconds"`
	TotalUsers        int      `json:"total_users"`
	Users             []string `json:"users"`
	FilesShared       int      `json:"files_shared"`
	TransfersInFlight int      `json:"transfers_in_flight"`
	TotalTransfers    int      `json:"total_transfers"`
	RelayServers      int      `json:"relay_servers"`
}

// StatusService serves the status page.
type StatusService struct {
	Hub       *ChatHub
	StartedAt time.Time
	ListenOn  string
	tmpl      *template.Template
}

func NewStatusService(hub *ChatHub, listenOn string) *StatusService {
	tmpl := template.Must(template.New("status").Parse(statusPageHTML))
	return &StatusService{
		Hub:       hub,
		StartedAt: time.Now(),
		ListenOn:  listenOn,
		tmpl:      tmpl,
	}
}

func (s *StatusService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/api/status" {
		s.apiStatus(w, r)
		return
	}
	hostname, _ := os.Hostname()
	users := []string{}
	filesShared := 0
	s.Hub.mu.Lock()
	for nick := range s.Hub.clients {
		users = append(users, nick)
	}
	for _, files := range s.Hub.fileRegistry.files {
		filesShared += len(files)
	}
	transfers := len(s.Hub.transfers)
	totalTransfers := s.Hub.totalTransfers // Add this field to ChatHub struct
	s.Hub.mu.Unlock()

	status := ServerStatus{
		Hostname:          hostname,
		Addr:              s.ListenOn,
		StartTime:         s.StartedAt.Format(time.RFC3339),
		UptimeSeconds:     int64(time.Since(s.StartedAt).Seconds()),
		TotalUsers:        len(users),
		Users:             users,
		FilesShared:       filesShared,
		TransfersInFlight: transfers,
		TotalTransfers:    totalTransfers,
		RelayServers:      1, // if you add multi-server later you can make this dynamic
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = s.tmpl.Execute(w, status)
}

func (s *StatusService) apiStatus(w http.ResponseWriter, r *http.Request) {
	hostname, _ := os.Hostname()
	users := []string{}
	filesShared := 0
	s.Hub.mu.Lock()
	for nick := range s.Hub.clients {
		users = append(users, nick)
	}
	for _, files := range s.Hub.fileRegistry.files {
		filesShared += len(files)
	}
	transfers := len(s.Hub.transfers)
	totalTransfers := s.Hub.totalTransfers
	s.Hub.mu.Unlock()

	status := ServerStatus{
		Hostname:          hostname,
		Addr:              s.ListenOn,
		StartTime:         s.StartedAt.Format(time.RFC3339),
		UptimeSeconds:     int64(time.Since(s.StartedAt).Seconds()),
		TotalUsers:        len(users),
		Users:             users,
		FilesShared:       filesShared,
		TransfersInFlight: transfers,
		TotalTransfers:    totalTransfers,
		RelayServers:      1,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

const statusPageHTML = `
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>RoseWire Network Status</title>
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <style>
  html, body {
    height: 100%;
    margin: 0;
    font-family: 'Montserrat', Arial, sans-serif;
    background: #29213c;
    color: #fff;
  }
  body {
    min-height: 100vh;
    padding: 0;
    margin: 0;
    background: linear-gradient(135deg, #2e2d4d 0%, #1a1633 100%);
  }
  .top-bar {
    background: #18142b;
    padding: 24px 32px 8px 32px;
    border-radius: 24px 24px 0 0;
    box-shadow: 0 2px 12px #0008;
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  .logo {
    font-size: 2.3rem;
    font-weight: 700;
    color: #ff6ec4;
    letter-spacing: 1px;
    display: flex;
    align-items: center;
    gap: 14px;
  }
  .logo-badge {
    font-size: 1rem;
    margin-left: 14px;
    padding: 2px 12px;
    border-radius: 18px;
    background: #2d2644;
    color: #fff;
    border: 1px solid #7c5bcf;
    font-weight: 600;
    letter-spacing: 1px;
  }
  .userbox {
    font-size: 1.1rem;
    font-weight: 600;
    color: #fff;
    background: #23203b;
    padding: 8px 24px;
    border-radius: 18px;
    border: 1px solid #4d3d7c;
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .main-content {
    max-width: 680px;
    margin: 40px auto 0 auto;
    padding: 24px;
    background: rgba(32, 20, 52, 0.95);
    border-radius: 22px;
    box-shadow: 0 4px 24px #0004;
  }
  .section-title {
    font-size: 1.6rem;
    font-weight: 700;
    margin-bottom: 8px;
    color: #f1cfff;
    letter-spacing: 1px;
  }
  .stats-box {
    display: flex;
    gap: 18px;
    margin: 18px 0 32px 0;
  }
  .stat {
    flex: 1;
    background: #1e1831;
    border-radius: 15px;
    padding: 24px 0 18px 0;
    text-align: center;
    border: 1.5px solid #443269;
    box-shadow: 0 1px 6px #0005;
    display: flex;
    flex-direction: column;
    align-items: center;
  }
  .stat .icon {
    font-size: 2.7rem;
    margin-bottom: 7px;
    color: #ff6ec4;
  }
  .stat .count {
    font-size: 1.9rem;
    font-weight: 700;
    color: #fff;
    margin-bottom: 2px;
    letter-spacing: 1px;
  }
  .stat .desc {
    font-size: 1.08rem;
    color: #caa9ec;
    font-weight: 500;
    letter-spacing: 1px;
  }
  .users-section {
    margin-top: 28px;
  }
  .userlist {
    background: #23203b;
    border-radius: 12px;
    padding: 0;
    margin: 0;
    list-style: none;
    border: 1px solid #3d2f53;
    overflow: hidden;
  }
  .useritem {
    display: flex;
    align-items: center;
    padding: 14px 18px;
    border-bottom: 1px solid #2e2340;
    font-size: 1.08rem;
    color: #ffe7ff;
    gap: 14px;
  }
  .useritem:last-child {
    border-bottom: none;
  }
  .useravatar {
    width: 32px;
    height: 32px;
    background: linear-gradient(135deg, #a144e8 30%, #ff6ec4 100%);
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    font-weight: 700;
    font-size: 1.2rem;
    color: #fff;
    box-shadow: 0 1px 6px #0004;
  }
  .userstatus {
    margin-left: auto;
    padding: 2px 14px;
    background: #1bbd6a;
    color: #fff;
    border-radius: 12px;
    font-size: 0.96rem;
    font-weight: 600;
    letter-spacing: 1px;
  }
  .footer {
    text-align: right;
    color: #9787b8;
    font-size: 1rem;
    margin-top: 40px;
    opacity: 0.95;
    padding-bottom: 16px;
    letter-spacing: 1px;
  }
  .connected-bar {
    margin: 0;
    padding: 0;
    background: #11161e;
    color: #10d47f;
    font-size: 1.02rem;
    font-weight: 600;
    padding: 10px 28px 10px 32px;
    border-radius: 0 0 24px 24px;
    border-top: 1px solid #281e3c;
    display: flex;
    align-items: center;
    gap: 8px;
  }
  @media (max-width: 850px) {
    .main-content { margin: 18px 2vw 0 2vw; }
    .top-bar { padding: 18px 10px 6px 10px;}
    .connected-bar { padding: 8px 10px 8px 10px; }
  }
  @media (max-width: 600px) {
    .main-content { padding: 10px;}
    .stats-box { flex-direction: column; gap: 12px;}
    .top-bar {padding: 12px 4px 4px 4px;}
    .footer {font-size: 0.9rem;}
  }
  </style>
  <!-- Optionally add Google Fonts for Montserrat -->
  <link href="https://fonts.googleapis.com/css?family=Montserrat:400,600,700&display=swap" rel="stylesheet">
  <!-- Optionally add Material Icons for user/transfer icons -->
  <link href="https://fonts.googleapis.com/icon?family=Material+Icons+Outlined" rel="stylesheet">
</head>
<body>
  <div class="top-bar">
    <span class="logo">RoseWire <span class="logo-badge">powered by SSH</span></span>
    <span class="userbox">SYSTEM</span>
  </div>
  <div class="main-content">
    <div class="section-title">Network Stats</div>
    <div class="stats-box">
      <div class="stat">
        <span class="icon material-icons-outlined">groups</span>
        <span class="count">{{.TotalUsers}}</span>
        <span class="desc">Users Online</span>
      </div>
      <div class="stat">
        <span class="icon material-icons-outlined">dns</span>
        <span class="count">{{.RelayServers}}</span>
        <span class="desc">Relay Servers</span>
      </div>
      <div class="stat">
        <span class="icon material-icons-outlined">compare_arrows</span>
        <span class="count">{{.TransfersInFlight}}</span>
        <span class="desc">Active Transfers</span>
      </div>
      <div class="stat">
        <span class="icon material-icons-outlined">library_books</span>
        <span class="count">{{.TotalTransfers}}</span>
        <span class="desc">Total Transfers</span>
      </div>
    </div>
    <div class="section-title users-section">Users on the Network</div>
    <ul class="userlist">
      {{- range .Users }}
      <li class="useritem">
        <span class="useravatar">{{ index . 0 }}</span>
        <span>{{ . }}</span>
        <span class="userstatus">Online</span>
      </li>
      {{- end }}
    </ul>
  </div>
  <div class="footer">RoseWire 2.0 - Modern Edition</div>
  <div class="connected-bar">
    <span class="material-icons-outlined" style="font-size:1.2em;">cloud_done</span>
    Connected via SSH as SYSTEM
  </div>
</body>
</html>
`