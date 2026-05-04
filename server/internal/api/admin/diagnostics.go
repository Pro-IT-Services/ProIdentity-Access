package admin

import (
	"net/http"

	"github.com/jmoiron/sqlx"
	"proidentity/internal/wireguard"
)

// DiagnosticsHandler exposes /admin/diagnostics.
type DiagnosticsHandler struct {
	DB       *sqlx.DB
	Registry *wireguard.Registry
}

// Diagnostic represents one issue surfaced to the admin UI.
type Diagnostic struct {
	ID       string `json:"id"`        // stable string for UI dedupe
	Severity string `json:"severity"`  // "warn" | "error" | "info"
	Title    string `json:"title"`
	Detail   string `json:"detail,omitempty"`
	Subject  string `json:"subject,omitempty"` // e.g. server name, session id
}

// List GET /admin/diagnostics — returns all current health/drift issues.
func (h *DiagnosticsHandler) List(w http.ResponseWriter, r *http.Request) {
	out := []Diagnostic{}

	// Index of running servers by ID (in-memory registry).
	running := map[string]*wireguard.ServerInstance{}
	for _, inst := range h.Registry.All() {
		running[inst.Server.ID] = inst
	}

	// Pull every DB-known server.
	dbServers, err := h.Registry.AllFromDB()
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}

	for _, srv := range dbServers {
		// (1) Server in DB but not running (boot-time setup failure or marked inactive).
		if _, ok := running[srv.ID]; !ok {
			sev := "warn"
			detail := "The server's WireGuard interface is not running. Likely the boot sequence failed (check daemon logs) or the server was marked inactive."
			if !srv.IsActive {
				sev = "info"
				detail = "Server is marked inactive; no sessions can be created on it."
			}
			out = append(out, Diagnostic{
				ID: "server-not-running:" + srv.ID, Severity: sev,
				Title: "Server " + srv.Name + " is not running", Detail: detail, Subject: srv.Name,
			})
			continue // can't deep-check kernel state if not running
		}

		inst := running[srv.ID]

		// (2) Sessions in DB whose peer is missing from the WG interface.
		var sessions []struct {
			ID              string `db:"id"`
			ClientPublicKey string `db:"client_public_key"`
		}
		_ = h.DB.Select(&sessions,
			"SELECT id, client_public_key FROM sessions WHERE server_id=?", srv.ID)

		dbPeers := map[string]string{} // pubkey -> session_id
		for _, s := range sessions {
			dbPeers[s.ClientPublicKey] = s.ID
			_, err := inst.Manager.PeerHandshakeAge(s.ClientPublicKey)
			if err != nil {
				out = append(out, Diagnostic{
					ID: "session-orphan:" + s.ID, Severity: "warn",
					Title:   "Session has no kernel peer",
					Detail:  "DB session " + s.ID + " on " + srv.Name + " refers to a WireGuard peer that does not exist on the interface. The client will not be able to connect.",
					Subject: srv.Name,
				})
			}
		}

		// (3) WG peers on the interface not present in the sessions table.
		// (Best-effort — we use the existing manager but it doesn't expose peer list directly,
		// so for now we skip this check. Could be added with a Manager.ListPeers() method.)
	}

	jsonOK(w, out)
}
