package admin

import (
	"net/http"

	"github.com/jmoiron/sqlx"
)

// TopologyHandler exposes /admin/topology — the full access graph in one payload.
type TopologyHandler struct{ DB *sqlx.DB }

type topPerson struct {
	ID       string `db:"id"       json:"id"`
	Username string `db:"username" json:"username"`
	IsAdmin  bool   `db:"is_admin" json:"is_admin"`
}
type topBundle struct {
	ID   string `db:"id"   json:"id"`
	Name string `db:"name" json:"name"`
}
type topResource struct {
	ID        string `db:"id"         json:"id"`
	Name      string `db:"name"       json:"name"`
	IPAddress string `db:"ip_address" json:"ip_address"`
	Type      string `db:"type"       json:"type"`
	Mask      *int   `db:"mask"       json:"mask"`
}
type topServer struct {
	ID     string `db:"id"     json:"id"`
	Name   string `db:"name"   json:"name"`
	Subnet string `db:"subnet" json:"subnet"`
}
type edge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Get GET /admin/topology
//
// Returns the simplified access flow Person -> Server -> Bundle -> Resource.
// Person->Server and Server->Bundle are DERIVED from the underlying group/role
// tables so the UI doesn't have to expose roles. Specifically:
//
//   Person -> Server : user has direct access (user_server_access) OR is in a
//                      group that's attached to the server (user_groups +
//                      group_server_access).
//   Server -> Bundle : a bundle is granted by any group that's attached to that
//                      server (group_server_access JOIN group_access).
//   Bundle -> Resource: resource_group_members (unchanged).
//
// All edge slices are guaranteed non-nil (empty arrays serialize to []).
func (h *TopologyHandler) Get(w http.ResponseWriter, r *http.Request) {
	var people []topPerson
	var bundles []topBundle
	var resources []topResource
	var servers []topServer

	if err := h.DB.Select(&people, `SELECT id, username, is_admin FROM users WHERE is_active=1 ORDER BY username`); err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	if err := h.DB.Select(&bundles, `SELECT id, name FROM resource_groups ORDER BY name`); err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	if err := h.DB.Select(&resources, `SELECT id, name, ip_address, type, mask FROM resources ORDER BY name`); err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	if err := h.DB.Select(&servers, `SELECT id, name, subnet FROM wg_servers WHERE is_active=1 ORDER BY name`); err != nil {
		jsonError(w, 500, err.Error())
		return
	}

	type linkRow struct {
		A string `db:"a"`
		B string `db:"b"`
	}
	read := func(q string) []edge {
		var out []linkRow
		_ = h.DB.Select(&out, q)
		es := make([]edge, 0, len(out))
		for _, r := range out {
			es = append(es, edge{From: r.A, To: r.B})
		}
		return es
	}

	personServer := read(`
		SELECT usa.user_id AS a, usa.server_id AS b
		FROM user_server_access usa
		JOIN users u       ON u.id = usa.user_id       AND u.is_active = 1
		JOIN wg_servers s  ON s.id = usa.server_id     AND s.is_active = 1`)

	serverAllowedBundle := read(`
		SELECT server_id AS a, bundle_id AS b FROM server_bundle_access`)

	userBundle := read(`
		SELECT CONCAT(user_id, ':', server_id) AS a, bundle_id AS b FROM user_bundle_access`)

	bundleResource := read(`
		SELECT resource_group_id AS a, resource_id AS b FROM resource_group_members`)

	jsonOK(w, map[string]any{
		"people":                people,
		"bundles":               bundles,
		"resources":             resources,
		"servers":               servers,
		"person_server":         personServer,
		"server_allowed_bundle": serverAllowedBundle,
		"user_bundle":           userBundle,
		"bundle_resource":       bundleResource,
	})
}
