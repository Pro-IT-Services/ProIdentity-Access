package session

// This file is intentionally minimal.
// IP pool logic has moved to internal/ippool package.
// Functions are kept here as thin wrappers for any remaining internal callers.

import (
	"github.com/jmoiron/sqlx"
	"proidentity/internal/ippool"
)

// InitPool is kept for backward compatibility. Use ippool.InitServer for new code.
// serverID is required in the new schema; this wrapper is a no-op if called without one.
func InitPool(db *sqlx.DB, cidr string) error {
	// Cannot init without a server_id in the new schema; callers should use ippool.InitServer.
	_ = db
	_ = cidr
	return nil
}

// AllocateIP is kept for internal session manager use.
func AllocateIP(db *sqlx.DB, serverID, sessionID string) (string, error) {
	return ippool.Allocate(db, serverID, sessionID)
}

// ReleaseIP is kept for internal session manager use.
func ReleaseIP(db *sqlx.DB, serverID, ip string) error {
	return ippool.Release(db, serverID, ip)
}
