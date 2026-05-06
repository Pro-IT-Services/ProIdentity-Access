package api

import (
	"embed"
	"io/fs"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"proidentity/internal/api/admin"
	"proidentity/internal/audit"
	"proidentity/internal/auth"
	"proidentity/internal/config"
	"proidentity/internal/requestip"
	"proidentity/internal/session"
	"proidentity/internal/wireguard"
)

//go:embed ui/dist
var uiFS embed.FS

// Server is the HTTP API server.
type Server struct {
	cfg      *config.Config
	db       *sqlx.DB
	sessions *session.Manager
	registry *wireguard.Registry
	wa       *webauthn.WebAuthn
	audit    *audit.Recorder
	router   *chi.Mux
}

func NewServer(cfg *config.Config, db *sqlx.DB, sessions *session.Manager, registry *wireguard.Registry, wa *webauthn.WebAuthn) *Server {
	s := &Server{cfg: cfg, db: db, sessions: sessions, registry: registry, wa: wa, audit: audit.New(db)}
	s.router = s.buildRouter()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Server) buildRouter() *chi.Mux {
	r := chi.NewRouter()
	r.Use(realIPMiddleware)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(securityHeaders)
	r.Use(s.corsMiddleware)

	// API v1
	r.Route("/api/v1", func(r chi.Router) {
		// Registration is always plaintext (key exchange hasn't happened yet)
		r.With(rateLimit(10, time.Minute)).Post("/register", s.handleRegister)
		r.Get("/client-updates/windows/latest", s.handleClientUpdateManifest)
		r.Get("/client-updates/windows/{file}", s.handleClientUpdateDownload)

		// Public routes — decrypt if device-encrypted, but no auth required
		r.Group(func(r chi.Router) {
			r.Use(s.deviceMiddleware)
			r.Get("/info", s.handleInfo)
			r.With(rateLimit(20, time.Minute)).Post("/auth/login", s.handleLogin)
			r.Get("/auth/push-status/{id}", s.handlePushAuthPollPublic)
			r.With(rateLimit(20, time.Minute)).Post("/auth/passkey/login/begin", s.handlePasskeyLoginBegin)
			r.With(rateLimit(20, time.Minute)).Post("/auth/passkey/login/finish", s.handlePasskeyLoginFinish)
		})

		// Authenticated
		r.Group(func(r chi.Router) {
			r.Use(s.Authenticate)
			r.Use(s.deviceMiddleware)
			r.Use(s.auditMiddleware)

			r.Get("/auth/me", s.handleMe)
			r.Get("/auth/permissions", s.handlePermCatalog)
			r.Post("/auth/change-password", s.handleChangePassword)
			r.Post("/auth/totp/setup", s.handleTOTPSetup)
			r.Post("/auth/totp/confirm", s.handleTOTPConfirm)
			r.Post("/auth/totp/disable", s.handleTOTPDisable)
			r.Post("/auth/passkey/register/begin", s.handlePasskeyRegisterBegin)
			r.Post("/auth/passkey/register/finish", s.handlePasskeyRegisterFinish)
			r.Get("/auth/passkeys", s.handleListPasskeys)
			r.Delete("/auth/passkeys/{id}", s.handleDeletePasskey)

			// Servers available to the current user
			r.Get("/servers", s.handleListUserServers)

			// User config storage (encrypted WireGuard configs)
			r.Get("/user/config-key", s.handleGetConfigKey)
			r.Get("/user/configs", s.handleListUserConfigs)
			r.Post("/user/configs", s.handleUploadUserConfig)
			r.Get("/user/configs/{id}", s.handleDownloadUserConfig)
			r.Delete("/user/configs/{id}", s.handleDeleteUserConfig)

			// Push auth (ProIdentity Cloud)
			r.Post("/auth/push", s.handlePushAuthCreate)
			r.Get("/auth/push/{id}", s.handlePushAuthPoll)
			r.Post("/auth/push/verify-totp", s.handlePushVerifyTOTP)

			// Sessions
			r.Post("/sessions", s.handleCreateSession)
			r.Get("/sessions/mine", s.handleMySessions)
			r.Post("/sessions/{id}/keepalive", s.handleKeepalive)
			r.Delete("/sessions/{id}", s.handleDeleteSession)

			// Installations (user)
			r.Get("/installations/mine", s.handleListMyInstallations)

			// Admin endpoints — gated per-resource by named permissions.
			// is_admin users implicitly hold every permission (loaded by Authenticate),
			// so existing admin behavior is preserved.

			// Users + their reach + direct server access
			uh := &admin.UserHandler{DB: s.db}
			r.With(RequirePerm(auth.PermUsersManage)).Group(func(r chi.Router) {
				r.Get("/admin/users", uh.List)
				r.Post("/admin/users", uh.Create)
				r.Get("/admin/users/{id}", uh.Get)
				r.Put("/admin/users/{id}", uh.Update)
				r.Delete("/admin/users/{id}", uh.Delete)
				r.Get("/admin/users/{id}/groups", uh.ListGroups)
				r.Post("/admin/users/{id}/groups", uh.AddGroup)
				r.Delete("/admin/users/{id}/groups/{gid}", uh.RemoveGroup)
				r.Get("/admin/users/{id}/reach", uh.Reach)
				r.Get("/admin/users/{id}/servers", uh.ListServers)
				r.Post("/admin/users/{id}/servers", uh.AddServer)
				r.Delete("/admin/users/{id}/servers/{sid}", uh.RemoveServer)
				r.Get("/admin/users/{id}/servers/{sid}/bundles", uh.ListBundles)
				r.Post("/admin/users/{id}/servers/{sid}/bundles", uh.AddBundle)
				r.Delete("/admin/users/{id}/servers/{sid}/bundles/{bid}", uh.RemoveBundle)
				r.Get("/admin/installations", (&admin.InstallationHandler{DB: s.db}).List)
				r.Delete("/admin/installations/{id}", (&admin.InstallationHandler{DB: s.db}).Delete)
				r.Get("/admin/user-configs", s.handleAdminListUserConfigs)
				r.Delete("/admin/user-configs/{id}", s.handleAdminDeleteUserConfig)
			})

			// Roles (admin permissions)
			gh := &admin.GroupHandler{DB: s.db}
			r.With(RequirePerm(auth.PermRolesManage)).Group(func(r chi.Router) {
				r.Get("/admin/groups", gh.List)
				r.Post("/admin/groups", gh.Create)
				r.Get("/admin/groups/{id}", gh.Get)
				r.Put("/admin/groups/{id}", gh.Update)
				r.Delete("/admin/groups/{id}", gh.Delete)
				r.Put("/admin/groups/{id}/permissions", gh.UpdatePermissions)
				// Legacy role-as-access endpoints kept for backward compat — also gated here.
				r.Get("/admin/groups/{id}/access", gh.ListAccess)
				r.Post("/admin/groups/{id}/access", gh.AddAccess)
				r.Delete("/admin/groups/{id}/access/{rgid}", gh.RemoveAccess)
			})

			// Resources + bundles
			rh := &admin.ResourceHandler{DB: s.db}
			rgh := &admin.ResourceGroupHandler{DB: s.db}
			r.With(RequirePerm(auth.PermResourcesManage)).Group(func(r chi.Router) {
				r.Get("/admin/resources", rh.List)
				r.Post("/admin/resources", rh.Create)
				r.Get("/admin/resources/{id}", rh.Get)
				r.Put("/admin/resources/{id}", rh.Update)
				r.Delete("/admin/resources/{id}", rh.Delete)

				r.Get("/admin/resource-groups", rgh.List)
				r.Post("/admin/resource-groups", rgh.Create)
				r.Get("/admin/resource-groups/{id}", rgh.Get)
				r.Put("/admin/resource-groups/{id}", rgh.Update)
				r.Delete("/admin/resource-groups/{id}", rgh.Delete)
				r.Post("/admin/resource-groups/{id}/resources", rgh.AddMember)
				r.Delete("/admin/resource-groups/{id}/resources/{rid}", rgh.RemoveMember)
				r.Get("/admin/resource-groups/{id}/servers", rgh.ListServers)
				r.Post("/admin/resource-groups/{id}/servers", rgh.AttachServer)
				r.Delete("/admin/resource-groups/{id}/servers/{sid}", rgh.DetachServer)
			})

			// WireGuard servers
			svh := &admin.ServerHandler{Registry: s.registry, DB: s.db}
			r.With(RequirePerm(auth.PermServersManage)).Group(func(r chi.Router) {
				r.Get("/admin/servers", svh.List)
				r.Post("/admin/servers", svh.Create)
				r.Get("/admin/servers/{id}", svh.Get)
				r.Put("/admin/servers/{id}", svh.Update)
				r.Delete("/admin/servers/{id}", svh.Delete)
				r.Get("/admin/servers/{id}/groups", svh.ListGroups)
				r.Post("/admin/servers/{id}/groups", svh.AddGroup)
				r.Delete("/admin/servers/{id}/groups/{gid}", svh.RemoveGroup)
				r.Get("/admin/servers/{id}/users", svh.ListUsers)
				r.Post("/admin/servers/{id}/users", svh.AddUser)
				r.Delete("/admin/servers/{id}/users/{uid}", svh.RemoveUser)
				r.Get("/admin/servers/{id}/bundles", svh.ListBundles)
				r.Post("/admin/servers/{id}/bundles", svh.AddBundle)
				r.Delete("/admin/servers/{id}/bundles/{bid}", svh.RemoveBundle)
			})

			// Sessions (live + terminate)
			r.With(RequirePerm(auth.PermSessionsManage)).Group(func(r chi.Router) {
				sh := &admin.SessionHandler{DB: s.db}
				r.Get("/admin/sessions", sh.List)
				r.Get("/admin/vpn-events", (&admin.VPNEventHandler{DB: s.db}).List)
				r.Delete("/admin/sessions/{id}", func(w http.ResponseWriter, r *http.Request) {
					id := chi.URLParam(r, "id")
					if err := s.sessions.Terminate(id); err != nil {
						jsonError(w, 500, err.Error())
						return
					}
					jsonOK(w, map[string]bool{"ok": true})
				})
			})

			// Visibility-only endpoints
			r.With(RequirePerm(auth.PermDiagnosticsRead)).Get("/admin/diagnostics",
				(&admin.DiagnosticsHandler{DB: s.db, Registry: s.registry}).List)
			r.With(RequirePerm(auth.PermAuditRead)).Get("/admin/audit",
				(&admin.AuditHandler{DB: s.db}).List)
			r.With(RequirePerm(auth.PermDenialsRead)).Get("/admin/denials",
				(&admin.DenialHandler{DB: s.db}).List)
			r.With(RequirePerm(auth.PermTrafficRead)).Group(func(r chi.Router) {
				th := &admin.TrafficHandler{DB: s.db}
				r.Get("/admin/traffic/top", th.Top)
				r.Get("/admin/traffic/summary", th.Summary)
			})
			r.With(RequirePerm(auth.PermTopologyRead)).Get("/admin/topology",
				(&admin.TopologyHandler{DB: s.db}).Get)

			// System settings
			r.With(RequirePerm(auth.PermSystemSettings)).Group(func(r chi.Router) {
				sth := &admin.SettingsHandler{DB: s.db}
				r.Get("/admin/settings", sth.List)
				r.Put("/admin/settings", sth.Update)
			})
		})
	})

	// Serve the React Web UI (embedded) with SPA fallback
	uiSub, err := fs.Sub(uiFS, "ui/dist")
	if err == nil {
		fileServer := http.FileServer(http.FS(uiSub))
		r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
			// Try to open the file; if it doesn't exist serve index.html (SPA)
			path := strings.TrimPrefix(r.URL.Path, "/")
			f, err := uiSub.Open(path)
			if err == nil {
				f.Close()
				fileServer.ServeHTTP(w, r)
				return
			}
			// Unknown path — serve index.html directly so React Router handles it
			data, err := fs.ReadFile(uiSub, "index.html")
			if err != nil {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			w.Write(data)
		})
	}

	return r
}

func realIPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ip := requestip.ClientIP(r); ip != "" {
			r.RemoteAddr = ip
		}
		next.ServeHTTP(w, r)
	})
}

// handleInfo returns public server metadata (no auth required).
func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	vpnName := s.setting("vpn_name")
	if vpnName == "" {
		vpnName = "Managed VPN"
	}
	jsonOK(w, map[string]any{"vpn_name": vpnName, "push_auth_enabled": s.pushAuthEnabled()})
}

// setting reads a value from the settings table.
func (s *Server) setting(key string) string {
	var val string
	s.db.Get(&val, "SELECT `value` FROM settings WHERE `key`=?", key)
	return val
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && s.isAllowedOrigin(origin, r.Host) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Device-ID")
		}
		if r.Method == http.MethodOptions {
			if origin != "" && !s.isAllowedOrigin(origin, r.Host) {
				w.WriteHeader(http.StatusForbidden)
			} else {
				w.WriteHeader(http.StatusNoContent)
			}
			return
		}
		next.ServeHTTP(w, r)
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) isAllowedOrigin(origin, host string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	if strings.EqualFold(u.Host, host) {
		return true
	}
	for _, allowed := range s.cfg.Server.CORSOrigins {
		if strings.EqualFold(strings.TrimRight(origin, "/"), strings.TrimRight(allowed, "/")) {
			return true
		}
	}
	return false
}

func newUUID() string {
	return uuid.New().String()
}
