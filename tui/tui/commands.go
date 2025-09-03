// tui/commands.go
package tui

import "rosetui/ssh"

// This file contains command definitions used to communicate between
// different tea.Model components within the TUI. Panels create these
// commands, and the main AppModel receives and processes them, usually
// by calling a method on the ssh.Service.

// --- Chat Panel Commands ---
type sendChatMsgCmd string

// --- Search Panel Commands ---
type searchFilesCmd string
type fetchTopFilesCmd struct{}

// --- Network Panel Commands ---
type requestNetworkStatsCmd struct{}

// --- Library Panel Commands ---
type shareFilesCmd struct{ files []ssh.ShareableFile }

// setLibraryPathCmd is sent to app_view to update the ssh.Service with the correct library path.
type setLibraryPathCmd string

// --- Transfers/Download Commands ---
type downloadFileCmd struct{ file ssh.SearchResult }
type retryDownloadCmd struct{ transfer ssh.Transfer }