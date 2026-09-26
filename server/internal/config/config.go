package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

// Default session lifetimes. TokenTTL doubles as the inactivity window: a
// client that is closed for longer than this must sign in again. SessionMaxAge
// caps how long a continuously-refreshed session may live before a full
// re-authentication is required.
const (
	defaultTokenTTL      = 720 * time.Hour  // 30 days
	defaultSessionMaxAge = 2160 * time.Hour // 90 days
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Auth     AuthConfig     `yaml:"auth"`
}

type ServerConfig struct {
	Host        string   `yaml:"host"`
	Port        int      `yaml:"port"`
	CORSOrigins []string `yaml:"cors_origins"`
}

type DatabaseConfig struct {
	DSN string `yaml:"dsn"`
}

type AuthConfig struct {
	JWTSecret string `yaml:"jwt_secret"`
	// TokenTTL is how long an issued login token stays valid. It is also the
	// inactivity window — a client that has been closed longer than this is
	// signed out. Accepts a Go duration string (e.g. "720h"). Default 30 days.
	TokenTTL string `yaml:"token_ttl"`
	// SessionMaxAge caps the total lifetime of a session that keeps refreshing
	// while in use, after which a full re-authentication is required. Accepts a
	// Go duration string (e.g. "2160h"). Default 90 days.
	SessionMaxAge string `yaml:"session_max_age"`

	// Parsed forms, populated by Load. Not read from YAML directly.
	TokenTTLDur      time.Duration `yaml:"-"`
	SessionMaxAgeDur time.Duration `yaml:"-"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if v := os.Getenv("PROIDENTITY_SERVER_HOST"); v != "" {
		cfg.Server.Host = v
	}
	if v := os.Getenv("PROIDENTITY_SERVER_PORT"); v != "" {
		port, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("PROIDENTITY_SERVER_PORT must be an integer")
		}
		cfg.Server.Port = port
	}
	if v := os.Getenv("PROIDENTITY_DATABASE_DSN"); v != "" {
		cfg.Database.DSN = v
	}
	if v := os.Getenv("PROIDENTITY_JWT_SECRET"); v != "" {
		cfg.Auth.JWTSecret = v
	}
	if v := os.Getenv("PROIDENTITY_TOKEN_TTL"); v != "" {
		cfg.Auth.TokenTTL = v
	}
	if v := os.Getenv("PROIDENTITY_SESSION_MAX_AGE"); v != "" {
		cfg.Auth.SessionMaxAge = v
	}
	if cfg.Auth.TokenTTLDur, err = parseDurationDefault(cfg.Auth.TokenTTL, defaultTokenTTL); err != nil {
		return nil, fmt.Errorf("auth.token_ttl: %w", err)
	}
	if cfg.Auth.SessionMaxAgeDur, err = parseDurationDefault(cfg.Auth.SessionMaxAge, defaultSessionMaxAge); err != nil {
		return nil, fmt.Errorf("auth.session_max_age: %w", err)
	}
	// A session can never live shorter than a single token.
	if cfg.Auth.SessionMaxAgeDur < cfg.Auth.TokenTTLDur {
		cfg.Auth.SessionMaxAgeDur = cfg.Auth.TokenTTLDur
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Server.Host == "" {
		cfg.Server.Host = "0.0.0.0"
	}
	if cfg.Auth.JWTSecret == "" {
		return nil, fmt.Errorf("auth.jwt_secret must be set")
	}
	if os.Getenv("PROIDENTITY_ALLOW_INSECURE_DEFAULTS") != "1" {
		if cfg.Auth.JWTSecret == "change-this-to-a-random-secret-in-production" || len(cfg.Auth.JWTSecret) < 32 {
			return nil, fmt.Errorf("auth.jwt_secret must be at least 32 characters and not use the default placeholder")
		}
	}
	return &cfg, nil
}

// parseDurationDefault parses a Go duration string, falling back to def when
// empty. A non-empty but invalid or non-positive value is an error.
func parseDurationDefault(s string, def time.Duration) (time.Duration, error) {
	if s == "" {
		return def, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, err
	}
	if d <= 0 {
		return 0, fmt.Errorf("must be positive")
	}
	return d, nil
}
