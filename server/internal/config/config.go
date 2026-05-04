package config

import (
	"fmt"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
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
