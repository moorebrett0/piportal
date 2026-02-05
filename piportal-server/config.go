package main

import (
	"flag"
	"fmt"
	"os"
)

// Config holds server configuration
type Config struct {
	// HTTP server settings
	HTTPAddr  string // Address for HTTP server (e.g., ":80")
	HTTPSAddr string // Address for HTTPS server (e.g., ":443")

	// TLS settings
	TLSCert string // Path to TLS certificate
	TLSKey  string // Path to TLS private key
	AutoTLS bool   // Use automatic TLS with Let's Encrypt

	// Domain settings
	BaseDomain string // Base domain (e.g., "piportal.dev")

	// Database
	DatabasePath string // Path to SQLite database

	// JWT secret for dashboard auth
	JWTSecret string

	// Development mode
	DevMode bool // Skip TLS, allow localhost

	// Reverse proxy mode (TLS handled by Caddy/nginx)
	BehindProxy bool
}

// LoadConfig loads configuration from flags and environment
func LoadConfig() *Config {
	cfg := &Config{}

	flag.StringVar(&cfg.HTTPAddr, "http", ":80", "HTTP listen address")
	flag.StringVar(&cfg.HTTPSAddr, "https", ":443", "HTTPS listen address")
	flag.StringVar(&cfg.TLSCert, "tls-cert", "", "Path to TLS certificate")
	flag.StringVar(&cfg.TLSKey, "tls-key", "", "Path to TLS private key")
	flag.BoolVar(&cfg.AutoTLS, "auto-tls", false, "Use Let's Encrypt for TLS")
	flag.StringVar(&cfg.BaseDomain, "domain", "piportal.dev", "Base domain for tunnels")
	flag.StringVar(&cfg.DatabasePath, "db", "piportal.db", "Path to SQLite database")
	flag.BoolVar(&cfg.DevMode, "dev", false, "Development mode (no TLS, allows localhost)")
	flag.BoolVar(&cfg.BehindProxy, "behind-proxy", false, "Running behind reverse proxy (TLS handled externally)")

	flag.Parse()

	// Environment overrides
	if v := os.Getenv("PIPORTAL_DOMAIN"); v != "" {
		cfg.BaseDomain = v
	}
	if v := os.Getenv("PIPORTAL_DB"); v != "" {
		cfg.DatabasePath = v
	}
	if os.Getenv("PIPORTAL_DEV") == "1" {
		cfg.DevMode = true
	}
	if v := os.Getenv("PIPORTAL_JWT_SECRET"); v != "" {
		cfg.JWTSecret = v
	} else if cfg.DevMode {
		cfg.JWTSecret = "piportal-dev-secret-do-not-use-in-prod"
	}

	return cfg
}

// Validate checks the configuration
func (c *Config) Validate() error {
	if c.BaseDomain == "" {
		return fmt.Errorf("base domain is required")
	}
	// TLS not required if behind proxy or in dev mode
	if !c.DevMode && !c.BehindProxy && !c.AutoTLS && (c.TLSCert == "" || c.TLSKey == "") {
		return fmt.Errorf("TLS certificate and key required (or use -auto-tls, -behind-proxy, or -dev)")
	}
	if c.JWTSecret == "" {
		return fmt.Errorf("PIPORTAL_JWT_SECRET is required (or use -dev mode)")
	}
	return nil
}
