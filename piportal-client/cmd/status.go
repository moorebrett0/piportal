package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current configuration and status",
	Long:  `Display the current PiPortal configuration and connection status.`,
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	fmt.Println()
	fmt.Println("  PiPortal Status")
	fmt.Println("  ─────────────────────────────────────────")
	fmt.Println()

	// Check for config file
	configPath := getConfigPath()
	data, err := os.ReadFile(configPath)
	if err != nil {
		fmt.Println("  Status:      Not configured")
		fmt.Println()
		fmt.Println("  Run 'piportal setup' to get started.")
		fmt.Println()
		return nil
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("invalid config file: %w", err)
	}

	fmt.Printf("  Config file: %s\n", configPath)
	fmt.Printf("  Server:      %s\n", cfg.Server)
	if cfg.Subdomain != "" {
		fmt.Printf("  Public URL:  https://%s.piportal.dev\n", cfg.Subdomain)
	}
	fmt.Printf("  Local addr:  %s:%d\n", cfg.LocalHost, cfg.LocalPort)
	fmt.Printf("  Token:       %s...\n", maskToken(cfg.Token))
	fmt.Println()

	// Fetch bandwidth usage
	usage, err := fetchUsage(cfg.Token)
	if err == nil {
		fmt.Println("  Bandwidth Usage")
		fmt.Println("  ─────────────────────────────────────────")
		fmt.Printf("  Tier:        %s\n", strings.Title(usage.Tier))
		fmt.Printf("  Month:       %s\n", usage.Month)
		fmt.Printf("  Used:        %s / %s (%.2f%%)\n", usage.UsedHuman, usage.LimitHuman, usage.PercentUsed)
		fmt.Println()
	}

	// TODO: Actually check if tunnel is running (via PID file or similar)
	fmt.Println("  Connection:  Not running")
	fmt.Println()
	fmt.Println("  Run 'piportal start' to connect.")
	fmt.Println()

	return nil
}

func maskToken(token string) string {
	if len(token) <= 8 {
		return "****"
	}
	return token[:8] + "****"
}

type UsageResponse struct {
	Subdomain   string  `json:"subdomain"`
	Tier        string  `json:"tier"`
	Month       string  `json:"month"`
	BytesIn     int64   `json:"bytes_in"`
	BytesOut    int64   `json:"bytes_out"`
	BytesTotal  int64   `json:"bytes_total"`
	Limit       int64   `json:"limit"`
	LimitHuman  string  `json:"limit_human"`
	UsedHuman   string  `json:"used_human"`
	PercentUsed float64 `json:"percent_used"`
}

func fetchUsage(token string) (*UsageResponse, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("GET", defaultServer+"/api/usage", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %d", resp.StatusCode)
	}

	var usage UsageResponse
	if err := json.NewDecoder(resp.Body).Decode(&usage); err != nil {
		return nil, err
	}

	return &usage, nil
}
