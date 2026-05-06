package model

import "time"

type User struct {
	ID                 string     `db:"id"            json:"id"`
	Username           string     `db:"username"      json:"username"`
	Email              string     `db:"email"         json:"email"`
	FirstName          string     `db:"first_name"    json:"first_name"`
	LastName           string     `db:"last_name"     json:"last_name"`
	PasswordHash       string     `db:"password_hash" json:"-"`
	TOTPSecret         *string    `db:"totp_secret"   json:"-"`
	TOTPEnabled        bool       `db:"totp_enabled"  json:"totp_enabled"`
	IsAdmin            bool       `db:"is_admin"      json:"is_admin"`
	IsActive           bool       `db:"is_active"     json:"is_active"`
	ConfigKey          []byte     `db:"config_key"    json:"-"`
	PushAuthSyncedAt   *time.Time `db:"push_auth_synced_at"   json:"push_auth_synced_at,omitempty"`
	PushAuthSyncStatus *string    `db:"push_auth_sync_status" json:"push_auth_sync_status,omitempty"`
	PushAuthSyncError  *string    `db:"push_auth_sync_error"  json:"push_auth_sync_error,omitempty"`
	CreatedAt          time.Time  `db:"created_at"    json:"created_at"`
	UpdatedAt          time.Time  `db:"updated_at"    json:"updated_at"`
}

type Passkey struct {
	ID           string    `db:"id"            json:"id"`
	UserID       string    `db:"user_id"       json:"user_id"`
	Name         string    `db:"name"          json:"name"`
	CredentialID []byte    `db:"credential_id" json:"-"`
	PublicKey    []byte    `db:"public_key"    json:"-"`
	SignCount    uint32    `db:"sign_count"    json:"-"`
	AAGUID       string    `db:"aaguid"        json:"-"`
	CreatedAt    time.Time `db:"created_at"    json:"created_at"`
}

type Group struct {
	ID          string    `db:"id"          json:"id"`
	Name        string    `db:"name"        json:"name"`
	Description *string   `db:"description" json:"description"`
	CreatedAt   time.Time `db:"created_at"  json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"  json:"updated_at"`
}

type Resource struct {
	ID          string    `db:"id"          json:"id"`
	Name        string    `db:"name"        json:"name"`
	IPAddress   string    `db:"ip_address"  json:"ip_address"`
	Type        string    `db:"type"        json:"type"`  // "host" or "network"
	Mask        *int      `db:"mask"        json:"mask"`  // CIDR prefix, only for "network"
	Ports       *string   `db:"ports"       json:"ports"` // e.g. "80,443,8080-8090"
	Description *string   `db:"description" json:"description"`
	CreatedAt   time.Time `db:"created_at"  json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"  json:"updated_at"`
}

type Installation struct {
	ID               string     `db:"id"                json:"id"`
	DeviceName       string     `db:"device_name"       json:"device_name"`
	ClientPublicKey  string     `db:"client_public_key" json:"-"`
	ServerPrivateKey string     `db:"server_private_key" json:"-"`
	ServerPublicKey  string     `db:"server_public_key"  json:"-"`
	UserID           *string    `db:"user_id"           json:"user_id"`
	IsActive         bool       `db:"is_active"         json:"is_active"`
	LastSeen         *time.Time `db:"last_seen"         json:"last_seen"`
	CreatedAt        time.Time  `db:"created_at"        json:"created_at"`
}

type ResourceGroup struct {
	ID          string    `db:"id"          json:"id"`
	Name        string    `db:"name"        json:"name"`
	Description *string   `db:"description" json:"description"`
	CreatedAt   time.Time `db:"created_at"  json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"  json:"updated_at"`
}

type Session struct {
	ID              string    `db:"id"                json:"id"`
	UserID          string    `db:"user_id"           json:"user_id"`
	ServerID        *string   `db:"server_id"         json:"server_id"`
	ClientPublicKey string    `db:"client_public_key" json:"client_public_key"`
	AssignedIP      string    `db:"assigned_ip"       json:"assigned_ip"`
	SourceIP        *string   `db:"source_ip"         json:"source_ip,omitempty"`
	DeviceID        *string   `db:"device_id"         json:"device_id,omitempty"`
	DeviceName      *string   `db:"device_name"       json:"device_name,omitempty"`
	UserAgent       *string   `db:"user_agent"        json:"user_agent,omitempty"`
	CreatedAt       time.Time `db:"created_at"        json:"created_at"`
	LastKeepalive   time.Time `db:"last_keepalive"    json:"last_keepalive"`
}

type IPPool struct {
	ServerID  string  `db:"server_id"`
	IP        string  `db:"ip"`
	InUse     bool    `db:"in_use"`
	SessionID *string `db:"session_id"`
}

// Setting is a key-value configuration entry stored in the DB.
type Setting struct {
	Key   string `db:"key"   json:"key"`
	Value string `db:"value" json:"value"`
}

type WGServer struct {
	ID            string    `db:"id"             json:"id"`
	Name          string    `db:"name"           json:"name"`
	Endpoint      string    `db:"endpoint"       json:"endpoint"`
	Port          int       `db:"port"           json:"port"`
	InterfaceName string    `db:"interface_name" json:"interface_name"`
	PrivateKey    string    `db:"private_key"    json:"-"`
	PublicKey     string    `db:"public_key"     json:"public_key"`
	Subnet        string    `db:"subnet"         json:"subnet"`
	DNS           *string   `db:"dns"            json:"dns"`
	External      bool      `db:"external"       json:"external"`
	IsActive      bool      `db:"is_active"      json:"is_active"`
	CreatedAt     time.Time `db:"created_at"     json:"created_at"`
}

type WGServerEndpoint struct {
	ID             string     `db:"id"               json:"id"`
	ServerID       string     `db:"server_id"        json:"server_id"`
	Name           string     `db:"name"             json:"name"`
	Host           string     `db:"host"             json:"host"`
	Port           int        `db:"port"             json:"port"`
	Priority       int        `db:"priority"         json:"priority"`
	Enabled        bool       `db:"enabled"          json:"enabled"`
	LastResolvedIP *string    `db:"last_resolved_ip" json:"last_resolved_ip,omitempty"`
	LastResolvedAt *time.Time `db:"last_resolved_at" json:"last_resolved_at,omitempty"`
	CreatedAt      time.Time  `db:"created_at"       json:"created_at"`
	UpdatedAt      *time.Time `db:"updated_at"       json:"updated_at,omitempty"`
}
