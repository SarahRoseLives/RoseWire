package main

import "encoding/json"

// InboundMessage is the structure for messages received from the client.
// It uses json.RawMessage to delay payload parsing until the type is known.
type InboundMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// OutboundMessage is the structure for messages sent to the client.
// It uses interface{} so that Go structs can be marshalled directly.
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
	Nickname  string `json:"nickname"`
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