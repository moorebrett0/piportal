package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime)

	// Load configuration
	config := LoadConfig()
	if err := config.Validate(); err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	// Initialize database
	store, err := NewStore(config.DatabasePath)
	if err != nil {
		log.Fatalf("Database error: %v", err)
	}
	defer store.Close()

	// Create tunnel manager
	tunnels := NewTunnelManager(store)

	// Create handler
	handler := NewHandler(config, store, tunnels)

	// Start server
	if config.DevMode {
		log.Printf("Starting PiPortal server in DEVELOPMENT mode")
		log.Printf("Listening on %s", config.HTTPAddr)
		log.Printf("Base domain: %s", config.BaseDomain)
		log.Printf("Database: %s", config.DatabasePath)
		log.Println()
		log.Println("Dev mode endpoints:")
		log.Printf("  Main site:    http://localhost%s/", config.HTTPAddr)
		log.Printf("  Register:     POST http://localhost%s/api/register", config.HTTPAddr)
		log.Printf("  Status:       GET http://localhost%s/api/status", config.HTTPAddr)
		log.Printf("  Tunnel WS:    ws://localhost%s/tunnel", config.HTTPAddr)
		log.Printf("  Proxy test:   http://localhost%s/?subdomain=<name>", config.HTTPAddr)
		log.Println()

		go func() {
			if err := http.ListenAndServe(config.HTTPAddr, handler); err != nil {
				log.Fatalf("HTTP server error: %v", err)
			}
		}()
	} else {
		log.Printf("Starting PiPortal server")
		log.Printf("Domain: %s", config.BaseDomain)

		// TODO: Add TLS support (Let's Encrypt or manual certs)
		go func() {
			if err := http.ListenAndServe(config.HTTPAddr, handler); err != nil {
				log.Fatalf("HTTP server error: %v", err)
			}
		}()
	}

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
}
