# RoseWire

**Modern Peer-to-Peer Music/Media Sharing and Chat, Powered by SSH**

RoseWire is a modern reimagining of classic P2P music sharing applications, designed for the future but inspired by the past. It enables users to connect, chat, and share music or media files across a decentralized network, all secured and authenticated via SSH. The project includes a cross-platform Flutter desktop client and a Go-based SSH relay server.

---

## Features

- **Decentralized file sharing:** Users share their local music/media library with the network.
- **Global chat:** Real-time network-wide chat among authenticated users.
- **Search across the network:** Quickly find files available from all connected peers.
- **Fast, secure transfers:** All communication and file transfers use the SSH protocol.
- **Modern UI:** Beautiful Material3 desktop interface with themed panels for chat, search, transfers, and more.
- **User authentication:** Each user is identified by an SSH public key and a unique nickname.
- **Network health dashboard:** Built-in web server for real-time status and statistics.

---

## Architecture

### 1. **Client (Flutter Desktop App)**
- Written in Dart/Flutter.
- Manages SSH connections and user authentication (via locally generated SSH key pairs).
- Allows users to:
  - Log in with or create new SSH profiles.
  - Select a local folder to share as their library.
  - Broadcast their shared file list to the network.
  - Search and download files from other users.
  - Participate in global chat.
  - Monitor transfers, network stats, and more.

### 2. **Server (Go SSH Relay)**
- Written in Go using `golang.org/x/crypto/ssh`.
- Handles multiple concurrent SSH clients.
- Maintains file registry, user registry, and active transfer states.
- Relays chat messages, search requests, and file transfer commands.
- Offers a status web interface for network and usage statistics.

---

## Quick Start

### Prerequisites

- Go 1.18+ (for the server)
- Flutter 3.10+ (for the desktop client)
- (Optional) OpenSSH for generating `server_ed25519` host key

### 1. **Run the SSH Relay Server**

```sh
# Generate SSH server key (if not present)
ssh-keygen -t ed25519 -f server_ed25519

# Build & run (from project root)
go run main.go chat.go files.go protocol.go status.go
```

The server will listen on port `2222` for SSH connections and on `127.0.0.1:8080` for the status dashboard.

### 2. **Run the Flutter Desktop Client**

```sh
# (In a separate terminal, from project root)
cd <flutter_project_dir>
flutter pub get
flutter run -d windows|macos|linux
```

- On first run, you'll be prompted to create a nickname and generate an SSH key.
- Select a local folder to share your music/media library.

---

## Screenshots

> *(Add screenshots of the chat, search, file transfer, and network panels here)*

---

## How It Works

### Authentication
- Users log in with a nickname and an SSH keypair (generated and stored locally).
- The server keeps a registry of nicknames and their associated public keys.

### File Sharing
- Users select a folder to share; the client broadcasts the file list to the server.
- Other users can search and request files, triggering peer-to-peer transfers via SSH channels.

### Chat & Search
- All chat messages and search requests are relayed through the SSH subsystem.
- The server maintains global state and relays messages to all connected clients.

### Transfers
- File downloads are chunked and base64-encoded over SSH, with real-time progress and error handling.
- The server tracks current and historical transfer counts.

### Network Status
- Visit `http://127.0.0.1:8080/` on the server to see live stats: users online, active transfers, total transfers, and more.

---

## Folder Structure

### Client (Flutter)
```
main.dart
/models/search_result.dart
/services/ssh_chat_service.dart
/ui/desktop/...
```

### Server (Go)
```
main.go
chat.go
files.go
protocol.go
status.go
```

---

## Security Notes

- All network traffic is encrypted via SSH.
- Nicknames are globally unique and tied to SSH public keys.
- Server does **not** store user files, only metadata needed for transfers.

---

## Roadmap

- [ ] Multi-relay/server support
- [ ] User avatars and presence indicators
- [ ] Mobile client (Flutter)
- [ ] Improved search (fuzzy, genre, etc.)
- [ ] User-configurable sharing permissions

---

## License

MIT License

---

## Credits

- Inspired by the spirit of WinMX, Soulseek, and other classic P2P platforms.
- Built using Flutter, Go, and SSH.

---

*RoseWire — Inspired by the classics, built for the future.*