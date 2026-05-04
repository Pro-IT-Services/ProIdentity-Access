package ipc

import "encoding/json"

// --- JSON-RPC 2.0 transport types ---

// Request is a JSON-RPC request sent from the GUI to the daemon.
type Request struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
	ID     int             `json:"id"`
}

// Response is a JSON-RPC response sent from the daemon to the GUI.
type Response struct {
	Result json.RawMessage `json:"result,omitempty"`
	Error  *RPCError       `json:"error,omitempty"`
	ID     int             `json:"id"`
}

// RPCError represents an error in a JSON-RPC response.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *RPCError) Error() string { return e.Message }

// Event is a server-sent event pushed to connected GUI clients.
type Event struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// --- Domain types shared between daemon and GUI ---

// TunnelStatus represents the state of a WireGuard tunnel.
type TunnelStatus string

const (
	StatusDisconnected TunnelStatus = "disconnected"
	StatusConnecting   TunnelStatus = "connecting"
	StatusConnected    TunnelStatus = "connected"
	StatusError        TunnelStatus = "error"
)

// TunnelInfo is the tunnel representation sent to UI clients.
type TunnelInfo struct {
	ID         string       `json:"id"`
	Name       string       `json:"name"`
	Status     TunnelStatus `json:"status"`
	Addresses  []string     `json:"addresses"`
	DNS        []string     `json:"dns"`
	MTU        int          `json:"mtu"`
	ListenPort int          `json:"listen_port"`
	PrivateKey string       `json:"private_key,omitempty"` // always empty for UI responses
	Peers      []PeerInfo   `json:"peers"`
	Error      string       `json:"error,omitempty"`
	IsManaged  bool         `json:"is_managed,omitempty"` // true for managed VPN session tunnels
}

// PeerInfo is the peer representation sent to UI clients.
type PeerInfo struct {
	PublicKey           string   `json:"public_key,omitempty"` // always empty for UI responses
	Endpoint            string   `json:"endpoint"`
	AllowedIPs          []string `json:"allowed_ips"`
	PersistentKeepalive int      `json:"persistent_keepalive"`
}

// StatsInfo holds runtime statistics for a connected tunnel.
type StatsInfo struct {
	TunnelID      string `json:"tunnel_id"`
	RxBytes       int64  `json:"rx_bytes"`
	TxBytes       int64  `json:"tx_bytes"`
	LastHandshake int64  `json:"last_handshake"` // Unix timestamp seconds
}

// --- Method parameter / result types ---

type ImportParams struct {
	Name          string `json:"name"`
	ConfigContent string `json:"config_content"`
}

type SetEncryptionKeyParams struct {
	Key []byte `json:"key"` // 32-byte AES-256 key, JSON-encoded as base64
}

type TunnelIDParam struct {
	ID string `json:"id"`
}

type StatusResult struct {
	DaemonVersion string `json:"daemon_version"`
	Running       bool   `json:"running"`
}

// Event type constants
const (
	EventTunnelChanged = "tunnel.changed" // payload: TunnelInfo
	EventStatsUpdate   = "stats.update"   // payload: StatsInfo
)

// RPC method names
const (
	MethodListTunnels      = "tunnel.list"
	MethodImportTunnel     = "tunnel.import"
	MethodImportEphemeral  = "tunnel.import_ephemeral" // import without persisting to disk
	MethodDeleteTunnel     = "tunnel.delete"
	MethodConnectTunnel    = "tunnel.connect"
	MethodDisconnect       = "tunnel.disconnect"
	MethodGetStats         = "tunnel.stats"
	MethodDaemonStatus     = "daemon.status"
	MethodSetEncryptionKey = "daemon.set_encryption_key"
)

// Error codes
const (
	ErrCodeInternal    = -32603
	ErrCodeNotFound    = -32001
	ErrCodeBadParams   = -32602
	ErrCodeTunnelError = -32002
	ErrCodeForbidden   = -32003
)
