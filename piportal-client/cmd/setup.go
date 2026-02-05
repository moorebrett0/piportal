package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Configure PiPortal (interactive)",
	Long: `Interactive setup wizard for PiPortal.

This will guide you through:
  1. Entering your PiPortal server URL
  2. Choosing your subdomain
  3. Setting your default local port

Your device will be registered automatically and configuration
saved to ~/.config/piportal/config.yaml`,
	RunE: runSetup,
}

func init() {
	rootCmd.AddCommand(setupCmd)
}

func runSetup(cmd *cobra.Command, args []string) error {
	fmt.Println()
	fmt.Println("  ╔═══════════════════════════════════════╗")
	fmt.Println("  ║        PiPortal Setup Wizard          ║")
	fmt.Println("  ╚═══════════════════════════════════════╝")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	// Step 1: Server URL
	fmt.Println("  Step 1: PiPortal server URL")
	fmt.Println("  ─────────────────────────────────────────")
	fmt.Println()
	fmt.Println("  Enter the URL of your PiPortal server")
	fmt.Println("  (e.g. https://piportal.example.com)")
	fmt.Println()
	fmt.Print("  Server URL: ")
	serverURL, _ := reader.ReadString('\n')
	serverURL = strings.TrimSpace(serverURL)

	// Validate and normalize server URL
	serverURL, err := normalizeServerURL(serverURL)
	if err != nil {
		return err
	}

	// Step 2: Choose subdomain
	fmt.Println()
	fmt.Println("  Step 2: Choose your subdomain")
	fmt.Println("  ─────────────────────────────────────────")
	fmt.Println()
	fmt.Print("  Subdomain: ")
	subdomain, _ := reader.ReadString('\n')
	subdomain = strings.TrimSpace(strings.ToLower(subdomain))

	if err := validateSubdomain(subdomain); err != nil {
		return err
	}

	// Step 3: Default local port
	fmt.Println()
	fmt.Println("  Step 3: Default local port")
	fmt.Println("  ─────────────────────────────────────────")
	fmt.Println()
	fmt.Println("  Which local port should we forward to?")
	fmt.Println("  (You can override this with --port when starting)")
	fmt.Println()
	fmt.Print("  Port [8080]: ")
	portStr, _ := reader.ReadString('\n')
	portStr = strings.TrimSpace(portStr)

	port := 8080
	if portStr != "" {
		fmt.Sscanf(portStr, "%d", &port)
	}

	if port <= 0 || port > 65535 {
		return fmt.Errorf("invalid port: %d", port)
	}

	// Step 4: Register with server
	fmt.Println()
	fmt.Println("  Step 4: Registering device...")
	fmt.Println("  ─────────────────────────────────────────")
	fmt.Println()

	token, err := registerDevice(serverURL, subdomain)
	if err != nil {
		fmt.Printf("  ✗ Registration failed: %v\n", err)
		fmt.Println()
		fmt.Println("  You can try a different subdomain, or register manually")
		fmt.Println("  in the dashboard and use 'piportal start --token <token>'")
		return err
	}

	fmt.Printf("  ✓ Registered as %s\n", subdomain)

	// Derive WebSocket URL from server URL
	wsURL := deriveWebSocketURL(serverURL)

	// Save config
	config := map[string]interface{}{
		"server":     wsURL,
		"server_url": serverURL,
		"token":      token,
		"subdomain":  subdomain,
		"local_port": port,
		"local_host": "127.0.0.1",
	}

	configPath, err := saveConfig(config)
	if err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Done!
	fmt.Println()
	fmt.Println("  ✓ Configuration saved!")
	fmt.Println()
	fmt.Printf("  Config file: %s\n", configPath)
	fmt.Printf("  Server:      %s\n", serverURL)
	fmt.Printf("  Subdomain:   %s\n", subdomain)
	fmt.Println()
	fmt.Println("  To start your tunnel, run:")
	fmt.Println()
	fmt.Printf("    piportal start --port %d\n", port)
	fmt.Println()
	fmt.Println("  Or install as a system service:")
	fmt.Println()
	fmt.Printf("    sudo piportal service install --port %d\n", port)
	fmt.Println()

	return nil
}

// normalizeServerURL validates and normalizes the server URL
func normalizeServerURL(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("server URL is required")
	}

	// Add https:// if no scheme provided
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		s = "https://" + s
	}

	// Parse and validate
	u, err := url.Parse(s)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	if u.Host == "" {
		return "", fmt.Errorf("invalid URL: no host")
	}

	// Remove trailing slash
	return strings.TrimSuffix(u.String(), "/"), nil
}

// deriveWebSocketURL converts https://example.com to wss://example.com/tunnel
func deriveWebSocketURL(serverURL string) string {
	wsURL := strings.Replace(serverURL, "https://", "wss://", 1)
	wsURL = strings.Replace(wsURL, "http://", "ws://", 1)
	return wsURL + "/tunnel"
}

// registerDevice calls the PiPortal API to register a new device
func registerDevice(serverURL, subdomain string) (string, error) {
	reqBody, _ := json.Marshal(map[string]string{
		"subdomain": subdomain,
	})

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(
		serverURL+"/api/register",
		"application/json",
		bytes.NewReader(reqBody),
	)
	if err != nil {
		return "", fmt.Errorf("could not reach server: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Success   bool   `json:"success"`
		Token     string `json:"token"`
		Subdomain string `json:"subdomain"`
		Error     string `json:"error"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("invalid response from server")
	}

	if !result.Success {
		return "", fmt.Errorf("%s", result.Error)
	}

	return result.Token, nil
}

func validateSubdomain(s string) error {
	if s == "" {
		return fmt.Errorf("subdomain is required")
	}
	if len(s) < 3 || len(s) > 30 {
		return fmt.Errorf("subdomain must be 3-30 characters")
	}
	if !regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`).MatchString(s) && len(s) > 2 {
		return fmt.Errorf("subdomain must be lowercase alphanumeric with hyphens, no leading/trailing hyphens")
	}
	reserved := []string{"www", "api", "app", "admin", "mail", "ftp", "ssh", "tunnel"}
	for _, r := range reserved {
		if s == r {
			return fmt.Errorf("'%s' is a reserved subdomain", s)
		}
	}
	return nil
}

func saveConfig(config map[string]interface{}) (string, error) {
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		configDir = filepath.Join(home, ".config")
	}
	configDir = filepath.Join(configDir, "piportal")

	if err := os.MkdirAll(configDir, 0700); err != nil {
		return "", err
	}

	configPath := filepath.Join(configDir, "config.yaml")
	data, err := yaml.Marshal(config)
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return "", err
	}

	return configPath, nil
}

func getConfigPath() string {
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		home, _ := os.UserHomeDir()
		configDir = filepath.Join(home, ".config")
	}
	return filepath.Join(configDir, "piportal", "config.yaml")
}
