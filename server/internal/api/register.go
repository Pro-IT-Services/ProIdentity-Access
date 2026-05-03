package api

import (
	"encoding/base64"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"proidentity/internal/devcrypto"
	"proidentity/internal/model"
)

type registerRequest struct {
	DeviceName      string `json:"device_name"`
	ClientPublicKey string `json:"client_public_key"` // base64 X25519 public key
}

type registerResponse struct {
	DeviceID        string `json:"device_id"`
	ServerPublicKey string `json:"server_public_key"` // base64 X25519 public key
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := decode(r, &req); err != nil || req.DeviceName == "" || req.ClientPublicKey == "" {
		jsonError(w, 400, "device_name and client_public_key required")
		return
	}
	req.DeviceName = strings.TrimSpace(req.DeviceName)
	if req.DeviceName == "" || len(req.DeviceName) > 128 {
		jsonError(w, http.StatusBadRequest, "device_name must be 1-128 characters")
		return
	}
	pub, err := base64.StdEncoding.DecodeString(req.ClientPublicKey)
	if err != nil || len(pub) != 32 {
		jsonError(w, http.StatusBadRequest, "client_public_key must be a base64 X25519 public key")
		return
	}

	// Generate a per-device server key pair
	serverPriv, serverPub, err := devcrypto.GenerateKeyPair()
	if err != nil {
		jsonError(w, 500, "key generation failed")
		return
	}

	id := uuid.New().String()
	now := time.Now()
	_, err = s.db.Exec(`
		INSERT INTO installations (id, device_name, client_public_key, server_private_key, server_public_key, last_seen, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, req.DeviceName, req.ClientPublicKey, serverPriv, serverPub, now, now,
	)
	if err != nil {
		jsonError(w, 500, "registration failed")
		return
	}

	jsonOK(w, registerResponse{DeviceID: id, ServerPublicKey: serverPub})
}

// handleListMyInstallations returns installations belonging to the authenticated user.
func (s *Server) handleListMyInstallations(w http.ResponseWriter, r *http.Request) {
	claims := claimsFrom(r)
	var insts []model.Installation
	s.db.Select(&insts, `SELECT id, device_name, user_id, is_active, last_seen, created_at FROM installations WHERE user_id=? ORDER BY created_at DESC`, claims.UserID)
	jsonOK(w, insts)
}
