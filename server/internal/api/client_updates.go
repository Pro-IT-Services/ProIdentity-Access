package api

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/go-chi/chi/v5"
)

//go:embed client_updates/windows/*
var clientUpdateFS embed.FS

func (s *Server) handleClientUpdateManifest(w http.ResponseWriter, r *http.Request) {
	data, err := fs.ReadFile(clientUpdateFS, "client_updates/windows/latest.json")
	if err != nil {
		jsonError(w, http.StatusNotFound, "no client update published")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func (s *Server) handleClientUpdateDownload(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "file")
	if name == "" || name != path.Base(name) || !strings.HasSuffix(strings.ToLower(name), ".msi") {
		http.NotFound(w, r)
		return
	}

	data, err := fs.ReadFile(clientUpdateFS, "client_updates/windows/"+name)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}
