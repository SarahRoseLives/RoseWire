// SERVER/protocol.go
package main

import "encoding/json"

// InboundMessage is the structure for messages received from the client.
type InboundMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// OutboundMessage is the structure for messages sent to the client.
type OutboundMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

// --- Client to Server Payloads ---

type SharePayload struct {
	Files []SharedFile `json:"files"`
}

type SearchPayload struct {
	Query string `json:"query"`
}

type GetFilePayload struct {
	FileName string `json:"fileName"`
	Peer     string `json:"peer"`
}

type ChatMessagePayload struct {
	Text string `json:"text"`
}

type UploadDataPayload struct {
	TransferID string `json:"transferID"`
	Data       string `json:"data"` // base64 encoded
}

type UploadDonePayload struct {
	TransferID string `json:"transferID"`
}

type UploadErrorPayload struct {
	TransferID string `json:"transferID"`
	Message    string `json:"message"`
}

// --- Server to Client Payloads ---

type SearchResultsPayload struct {
	Results []SearchResult `json:"results"`
}

type NetworkStatsPayload struct {
	Users           []map[string]string `json:"users"`
	RelayServers    int                 `json:"relayServers"`
	TotalUsers      int                 `json:"totalUsers"`
	ActiveTransfers int                 `json:"activeTransfers"`
	TotalTransfers  int                 `json:"totalTransfers"`
}

type ChatBroadcastPayload struct {
	Timestamp string `json:"timestamp"`
	Nickname  string `json:"nickname"` // This will be the full federated name
	Text      string `json:"text"`
	IsSystem  bool   `json:"isSystem"`
}

type TransferStartPayload struct {
	TransferID string `json:"transferID"`
	FileName   string `json:"fileName"`
	Size       int64  `json:"size"`
	FromUser   string `json:"fromUser"`
	Hash       string `json:"Hash,omitempty"` // The SHA256 hash of the file
}

type UploadRequestPayload struct {
	TransferID string `json:"transferID"`
	FileName   string `json:"fileName"`
}

type TransferErrorPayload struct {
	TransferID string `json:"transferID"`
	Message    string `json:"message"`
}

// --- Server to Server (S2S) Payloads ---

type Activity struct {
	Type   string          `json:"type"`
	Actor  string          `json:"actor"`
	Object json.RawMessage `json:"object"`
}

type ChatActivityObject struct {
	Content string `json:"content"`
}

type ShareActivityObject struct {
	Files []SharedFile `json:"files"`
}

type S2STransferRequest struct {
	TransferID          string `json:"transferID"`
	FileName            string `json:"fileName"`
	FileOwner           string `json:"fileOwner"`           // The full federated name of the user who has the file
	RequesterPeer       string `json:"requesterPeer"`       // The full federated name of the user who wants the file
	RequesterPeerDomain string `json:"requesterPeerDomain"` // The domain of the server making the request (e.g., "instance-a.com")
}

// InstanceActor defines the JSON object served at the /actor endpoint.
type InstanceActor struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	PublicKey string `json:"publicKey"` // PEM-encoded public key
}