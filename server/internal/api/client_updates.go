package api

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/go-chi/chi/v5"
)

// Desktop client installers and their signed manifests, published by the
// client build (tools/updatesign) into client_updates/<platform>/.
// Clients verify the manifest signature themselves; the server only hosts.
//
//go:embed all:client_updates
var clientUpdateFS embed.FS

// updatePlatforms maps the URL platform segment to its folder and package type.
// "windows" is the path used by clients up to 0.7.0.
var updatePlatforms = map[string]struct{ dir, ext string }{
	"windows":       {"windows-amd64", ".msi"},
	"windows-amd64": {"windows-amd64", ".msi"},
	"darwin-arm64":  {"darwin-arm64", ".pkg"},
	"darwin-amd64":  {"darwin-amd64", ".pkg"},
}

func (s *Server) handleClientUpdateManifest(w http.ResponseWriter, r *http.Request) {
	p, ok := updatePlatforms[chi.URLParam(r, "platform")]
	if !ok {
		jsonError(w, http.StatusNotFound, "unknown platform")
		return
	}
	data, err := fs.ReadFile(clientUpdateFS, path.Join("client_updates", p.dir, "latest.json"))
	if err != nil {
		jsonError(w, http.StatusNotFound, "no client update published")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func (s *Server) handleClientUpdateDownload(w http.ResponseWriter, r *http.Request) {
	p, ok := updatePlatforms[chi.URLParam(r, "platform")]
	name := chi.URLParam(r, "file")
	if !ok || name == "" || name != path.Base(name) || strings.HasPrefix(name, ".") ||
		!strings.HasSuffix(strings.ToLower(name), p.ext) {
		http.NotFound(w, r)
		return
	}
	file := path.Join("client_updates", p.dir, name)
	if _, err := fs.Stat(clientUpdateFS, file); err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	http.ServeFileFS(w, r, clientUpdateFS, file)
}
