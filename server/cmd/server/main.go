package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"proidentity/internal/api"
	"proidentity/internal/auth"
	"proidentity/internal/config"
	"proidentity/internal/db"
	"proidentity/internal/denial"
	"proidentity/internal/firewall"
	"proidentity/internal/flowmeter"
	"proidentity/internal/session"
	"proidentity/internal/wireguard"
)

func main() {
	cfgPath := "config.yaml"
	if len(os.Args) > 1 {
		cfgPath = os.Args[1]
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// Run all migrations with a migration-only connection. The runtime
	// connection below keeps multiStatements disabled.
	migrationDB, err := db.OpenForMigration(cfg.Database.DSN)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	if err := db.MigrateDir(migrationDB, migrationsDir()); err != nil {
		migrationDB.Close()
		log.Fatalf("migrate: %v", err)
	}
	migrationDB.Close()
	log.Println("migrations applied")

	// Database
	database, err := db.Open(cfg.Database.DSN)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer database.Close()

	// settings helper
	settings := func(key string) string {
		var val string
		database.Get(&val, "SELECT `value` FROM settings WHERE `key`=?", key)
		return val
	}

	// Firewall (no subnet at init; subnets added per-server via EnsureSubnet)
	fwMgr, err := firewall.NewManager()
	if err != nil {
		log.Fatalf("firewall: %v", err)
	}

	// WireGuard registry — starts all active servers from DB
	registry := wireguard.NewRegistry(database, fwMgr)
	if err := registry.Start(); err != nil {
		log.Fatalf("wireguard registry: %v", err)
	}

	// Auto-import existing wg0 from settings if no servers are in the DB yet
	if err := maybeImportLegacyServer(database, registry, settings); err != nil {
		log.Printf("warn: legacy server import: %v", err)
	}

	// Session manager + watchdog
	sessMgr := session.NewManager(database, registry, fwMgr, settings)
	sessMgr.StartWatchdog()

	// WebAuthn
	wa, err := auth.NewWebAuthn(
		settings("webauthn_rp_id"),
		settings("webauthn_rp_name"),
		settings("webauthn_origin"),
	)
	if err != nil {
		log.Fatalf("webauthn: %v", err)
	}

	// Ensure at least one admin user exists
	if err := ensureAdmin(database); err != nil {
		log.Printf("warn: ensure admin: %v", err)
	}

	// Denial collector — subscribe to NFLOG group 100 for wg deny events.
	denialCtx, denialCancel := context.WithCancel(context.Background())
	defer denialCancel()
	dc := denial.New(database, 0) // default flush interval
	if err := dc.Start(denialCtx); err != nil {
		log.Printf("warn: denial collector not started: %v (denials won't be recorded)", err)
	} else {
		log.Printf("denial collector listening on NFLOG group %d", denial.NflogGroup)
	}

	// Flow meter — sample conntrack to attribute per-resource traffic.
	if err := flowmeter.EnableConntrackAccounting(); err != nil {
		log.Printf("warn: enable nf_conntrack_acct: %v (per-flow byte counters won't work)", err)
	}
	fmCtx, fmCancel := context.WithCancel(context.Background())
	defer fmCancel()
	fm := flowmeter.New(database, 0)
	if err := fm.Start(fmCtx); err != nil {
		log.Printf("warn: flow meter not started: %v (per-resource analytics disabled)", err)
	} else {
		log.Printf("flow meter sampling conntrack every 30s")
	}

	// HTTP server
	srv := api.NewServer(cfg, database, sessMgr, registry, wa)
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("listening on http://%s", addr)

	go func() {
		if err := http.ListenAndServe(addr, srv); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http: %v", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("shutting down, terminating all sessions...")
	sessMgr.TerminateAll()
}

func migrationsDir() string {
	if dir := os.Getenv("PROIDENTITY_MIGRATIONS_DIR"); dir != "" {
		return dir
	}
	if _, err := os.Stat("migrations"); err == nil {
		return "migrations"
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Join(filepath.Dir(exe), "..", "migrations")
		if _, statErr := os.Stat(dir); statErr == nil {
			return dir
		}
	}
	return "migrations"
}

// maybeImportLegacyServer imports the existing wg0 config from settings as an external
// server if no wg_servers rows exist yet (i.e., first run after migration).
func maybeImportLegacyServer(database *sqlx.DB, registry *wireguard.Registry, settings func(string) string) error {
	var count int
	if err := database.Get(&count, "SELECT COUNT(*) FROM wg_servers"); err != nil {
		return err
	}
	if count > 0 {
		return nil // servers already registered
	}

	iface := settings("wg_interface")
	if iface == "" {
		iface = "wg0"
	}
	endpoint := settings("wg_endpoint")
	if endpoint == "" {
		return nil // no endpoint configured, skip
	}
	subnet := settings("vpn_subnet")
	if subnet == "" {
		subnet = "10.8.0.0/24"
	}
	dns := settings("vpn_dns")

	// Try to get the public key from the running interface
	mgr, err := wireguard.NewManager(iface)
	if err != nil {
		log.Printf("legacy import: cannot open %s: %v (skipping)", iface, err)
		return nil
	}
	pubKey, err := mgr.ServerPublicKey()
	mgr.Close()
	if err != nil || pubKey == "" {
		log.Printf("legacy import: cannot read pubkey from %s: %v (skipping)", iface, err)
		return nil
	}

	_, err = registry.CreateExternal("Default (wg0)", endpoint, 51820, iface, pubKey, subnet, dns)
	if err != nil {
		return fmt.Errorf("import legacy server: %w", err)
	}
	log.Printf("imported legacy WireGuard server %s as external server", iface)
	return nil
}

// ensureAdmin creates an admin user from env vars WG_ADMIN_USER / WG_ADMIN_PASS
// if no admin user exists yet.
func ensureAdmin(database *sqlx.DB) error {
	var count int
	if err := database.Get(&count, "SELECT COUNT(*) FROM users WHERE is_admin=1"); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	username := os.Getenv("WG_ADMIN_USER")
	password := os.Getenv("WG_ADMIN_PASS")
	email := os.Getenv("WG_ADMIN_EMAIL")
	if username == "" {
		username = "admin"
	}
	if password == "" {
		if os.Getenv("PROIDENTITY_ALLOW_INSECURE_DEFAULTS") != "1" {
			return fmt.Errorf("WG_ADMIN_PASS must be set before bootstrapping the first admin user")
		}
		password = "changeme"
	}
	if email == "" {
		email = "admin@localhost"
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}

	id := uuid.New().String()
	_, err = database.Exec(
		`INSERT INTO users (id, username, email, password_hash, is_admin, is_active) VALUES (?, ?, ?, ?, 1, 1)`,
		id, username, email, hash,
	)
	if err != nil {
		return err
	}
	log.Printf("created admin user %q (set WG_ADMIN_USER / WG_ADMIN_PASS env to customize)", username)
	return nil
}
