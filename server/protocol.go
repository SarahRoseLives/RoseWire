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

// FIX: This struct was accidentally removed and is now restored.
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

// Activity represents a federated event, inspired by ActivityPub.
type Activity struct {
	Type   string          `json:"type"`   // e.g., "Create", "Share"
	Actor  string          `json:"actor"`  // The federated user address, e.g., "@rose@instance.com"
	Object json.RawMessage `json:"object"` // The actual content (e.g., a chat message)
}

// ChatActivityObject is the content of a chat message activity.
type ChatActivityObject struct {
	Content string `json:"content"`
}