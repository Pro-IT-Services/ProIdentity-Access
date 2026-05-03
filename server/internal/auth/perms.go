// Package auth — permissions catalog used to gate admin endpoints and UI elements.
package auth

import (
	"encoding/json"

	"github.com/jmoiron/sqlx"
)

// Perm is a permission key. Stored as a string so it survives JSON round-trips
// and is easy to read in the DB.
type Perm string

const (
	PermUsersManage     Perm = "users.manage"
	PermRolesManage     Perm = "roles.manage"
	PermResourcesManage Perm = "resources.manage"
	PermServersManage   Perm = "servers.manage"
	PermSessionsManage  Perm = "sessions.manage"
	PermAuditRead       Perm = "audit.read"
	PermDenialsRead     Perm = "denials.read"
	PermTrafficRead     Perm = "traffic.read"
	PermSystemSettings  Perm = "system.settings"
	PermTopologyRead    Perm = "topology.read"
	PermDiagnosticsRead Perm = "diagnostics.read"
)

// PermDef describes a permission for the UI catalog endpoint.
type PermDef struct {
	Key         Perm   `json:"key"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Category    string `json:"category"`
}

// PermCatalog returns all known permissions in display order.
func PermCatalog() []PermDef {
	return []PermDef{
		{PermUsersManage,     "Manage users",        "Create, edit, disable, delete users; manage their server access.", "Access"},
		{PermResourcesManage, "Manage resources",    "Create, edit, delete resources and bundles; attach bundles to servers.", "Access"},
		{PermServersManage,   "Manage servers",      "Create, edit, delete WireGuard servers; manage attachments.", "Access"},
		{PermSessionsManage,  "Manage sessions",     "View live sessions and terminate them.", "Access"},
		{PermTopologyRead,    "View topology",       "See the access graph (Topology page).", "Visibility"},
		{PermTrafficRead,     "View traffic",        "See per-user / per-resource traffic analytics.", "Visibility"},
		{PermDenialsRead,     "View denials",        "See blocked-attempt log.", "Visibility"},
		{PermAuditRead,       "View audit log",      "See the who-did-what audit trail.", "Visibility"},
		{PermDiagnosticsRead, "View diagnostics",    "See server health and drift warnings.", "Visibility"},
		{PermRolesManage,     "Manage roles",        "Create, edit, delete roles and assign permissions.", "Admin"},
		{PermSystemSettings,  "Manage settings",     "Edit server-wide settings.", "Admin"},
	}
}

// AllPerms returns every permission key (used for is_admin users).
func AllPerms() []Perm {
	c := PermCatalog()
	out := make([]Perm, len(c))
	for i, d := range c {
		out[i] = d.Key
	}
	return out
}

// LoadUserPermissions returns the union of permissions granted by every role
// the user is in. is_admin users always get every permission.
func LoadUserPermissions(db *sqlx.DB, userID string, isAdmin bool) ([]Perm, error) {
	if isAdmin {
		return AllPerms(), nil
	}
	var rows []struct {
		Permissions *string `db:"permissions"` // JSON array as string
	}
	err := db.Select(&rows, `
		SELECT g.permissions
		FROM `+"`groups`"+` g
		JOIN user_groups ug ON ug.group_id = g.id
		WHERE ug.user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	seen := map[Perm]struct{}{}
	for _, r := range rows {
		if r.Permissions == nil {
			continue
		}
		var perms []Perm
		if err := json.Unmarshal([]byte(*r.Permissions), &perms); err != nil {
			continue
		}
		for _, p := range perms {
			seen[p] = struct{}{}
		}
	}
	out := make([]Perm, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	return out, nil
}
