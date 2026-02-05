package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const defaultServer = "https://piportal.dev"

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Configure PiPortal (interactive)",
	Long: `Interactive setup wizard for PiPortal.

This will guide you through:
  1. Choosing your subdomain
  2. Setting your default local port

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

	// Step 1: Choose subdomain
	fmt.Println("  Step 1: Choose your subdomain")
	fmt.Println("  ─────────────────────────────────────────")
	fmt.Println()
	fmt.Println("  Your tunnel will be available at:")
	fmt.Println("  https://<subdomain>.piportal.dev")
	fmt.Println()
	fmt.Print("  Subdomain: ")
	subdomain, _ := reader.ReadString('\n')
	subdomain = strings.TrimSpace(strings.ToLower(subdomain))

	if err := validateSubdomain(subdomain); err != nil {
		return err
	}

	// Step 2: Default local port
	fmt.Println()
	fmt.Println("  Step 2: Default local port")
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

	// Step 3: Register with server
	fmt.Println()
	fmt.Println("  Step 3: Registering device...")
	fmt.Println("  ─────────────────────────────────────────")
	fmt.Println()

	token, err := registerDevice(subdomain)
	if err != nil {
		fmt.Printf("  ✗ Registration failed: %v\n", err)
		fmt.Println()
		fmt.Println("  You can try a different subdomain, or register manually at:")
		fmt.Println("  https://piportal.dev")
		return err
	}

	fmt.Printf("  ✓ Registered as %s.piportal.dev\n", subdomain)

	// Save config
	config := map[string]interface{}{
		"server":     "wss://piportal.dev/tunnel",
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
	fmt.Printf("  Your URL:    https://%s.piportal.dev\n", subdomain)
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

// registerDevice calls the PiPortal API to register a new device
func registerDevice(subdomain string) (string, error) {
	reqBody, _ := json.Marshal(map[string]string{
		"subdomain": subdomain,
	})

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(
		defaultServer+"/api/register",
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
