package cmd

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	startPort   int
	startHost   string
	startServer string
	startToken  string
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the tunnel",
	Long: `Start forwarding traffic from your public URL to a local port.

Examples:
  # Use config file defaults
  piportal start

  # Forward to a specific port
  piportal start --port 3000

  # Forward to a different host
  piportal start --port 3000 --host 192.168.1.100`,
	RunE: runStart,
}

func init() {
	rootCmd.AddCommand(startCmd)

	startCmd.Flags().IntVarP(&startPort, "port", "p", 0, "Local port to forward to")
	startCmd.Flags().StringVar(&startHost, "host", "", "Local host to forward to (default: 127.0.0.1)")
	startCmd.Flags().StringVar(&startServer, "server", "", "Server URL (overrides config)")
	startCmd.Flags().StringVar(&startToken, "token", "", "Device token (overrides config)")
}

// Config matches the config file structure
type Config struct {
	Server    string `yaml:"server"`
	Token     string `yaml:"token"`
	Subdomain string `yaml:"subdomain"`
	LocalPort int    `yaml:"local_port"`
	LocalHost string `yaml:"local_host"`
}

func loadConfig() (*Config, error) {
	cfg := &Config{
		Server:    "wss://tunnel.piportal.dev",
		LocalHost: "127.0.0.1",
		LocalPort: 8080,
	}

	// Try to load config file
	configPath := getConfigPath()
	data, err := os.ReadFile(configPath)
	if err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("invalid config file: %w", err)
		}
	}

	// Also check /etc/piportal for system-wide config (for service mode)
	if cfg.Token == "" {
		sysConfig := "/etc/piportal/config.yaml"
		data, err := os.ReadFile(sysConfig)
		if err == nil {
			yaml.Unmarshal(data, cfg)
		}
	}

	return cfg, nil
}

func runStart(cmd *cobra.Command, args []string) error {
	// Load config
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	// CLI flags override config
	if startPort != 0 {
		cfg.LocalPort = startPort
	}
	if startHost != "" {
		cfg.LocalHost = startHost
	}
	if startServer != "" {
		cfg.Server = startServer
	}
	if startToken != "" {
		cfg.Token = startToken
	}

	// Validate
	if cfg.Token == "" {
		fmt.Println("No token configured. Run 'piportal setup' first, or use --token.")
		fmt.Println()
		fmt.Println("  piportal setup          # Interactive setup")
		fmt.Println("  piportal start --token <token> --port 8080")
		fmt.Println()
		return fmt.Errorf("token required")
	}

	if cfg.LocalPort <= 0 || cfg.LocalPort > 65535 {
		return fmt.Errorf("invalid port: %d", cfg.LocalPort)
	}

	// Set up logging
	log.SetFlags(log.Ltime)

	// Print startup banner
	fmt.Println()
	fmt.Printf("  PiPortal %s\n", Version)
	fmt.Println("  ─────────────────────────────────────────")
	fmt.Printf("  Forwarding:  %s:%d\n", cfg.LocalHost, cfg.LocalPort)
	if cfg.Subdomain != "" {
		fmt.Printf("  Public URL:  https://%s.piportal.dev\n", cfg.Subdomain)
	}
	fmt.Println()

	// Check for updates in background
	go checkForUpdates()

	// Create and start tunnel
	tunnel := NewTunnel(cfg)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\n  Shutting down...")
		tunnel.Stop()
	}()

	// Run the tunnel (blocks until stopped)
	return tunnel.Run()
}

// getConfigDir returns the config directory path
func getConfigDir() string {
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		home, _ := os.UserHomeDir()
		configDir = filepath.Join(home, ".config")
	}
	return filepath.Join(configDir, "piportal")
}

// checkForUpdates checks if a newer version is available (non-blocking)
func checkForUpdates() {
	latest, err := getLatestVersion()
	if err != nil {
		return // Silently fail - don't disrupt the user
	}

	if latest.Version != Version {
		fmt.Printf("  📦 Update available: %s → %s\n", Version, latest.Version)
		fmt.Println("     Run 'piportal upgrade' to update")
		fmt.Println()
	}
}
