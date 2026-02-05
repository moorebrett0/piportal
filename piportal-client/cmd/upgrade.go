package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/spf13/cobra"
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade PiPortal to the latest version",
	Long: `Check for and install the latest version of PiPortal.

This will download the latest binary and replace the current one.`,
	RunE: runUpgrade,
}

var checkOnly bool

func init() {
	upgradeCmd.Flags().BoolVar(&checkOnly, "check", false, "Only check for updates, don't install")
	rootCmd.AddCommand(upgradeCmd)
}

type VersionInfo struct {
	Version     string `json:"version"`
	ReleaseDate string `json:"release_date"`
	Changelog   string `json:"changelog"`
}

func runUpgrade(cmd *cobra.Command, args []string) error {
	fmt.Println()
	fmt.Println("  PiPortal Upgrade")
	fmt.Println("  ─────────────────────────────────────────")
	fmt.Println()

	// Get current version
	currentVersion := Version
	fmt.Printf("  Current version: %s\n", currentVersion)

	// Check latest version from server
	fmt.Println("  Checking for updates...")
	latest, err := getLatestVersion()
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	fmt.Printf("  Latest version:  %s\n", latest.Version)
	fmt.Println()

	if currentVersion == latest.Version {
		fmt.Println("  ✓ You're already on the latest version!")
		fmt.Println()
		return nil
	}

	if latest.Changelog != "" {
		fmt.Println("  What's new:")
		fmt.Printf("    %s\n", latest.Changelog)
		fmt.Println()
	}

	if checkOnly {
		fmt.Println("  Run 'piportal upgrade' to install the update.")
		fmt.Println()
		return nil
	}

	// Determine architecture
	arch := getArch()
	if arch == "" {
		return fmt.Errorf("unsupported architecture: %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	fmt.Printf("  Downloading piportal-linux-%s...\n", arch)

	// Download new binary
	downloadURL := fmt.Sprintf("%s/downloads/piportal-linux-%s", defaultServer, arch)
	newBinary, err := downloadBinary(downloadURL)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	// Get current executable path
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not determine executable path: %w", err)
	}

	// Replace current binary
	fmt.Println("  Installing...")
	if err := replaceBinary(execPath, newBinary); err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}

	fmt.Println()
	fmt.Printf("  ✓ Upgraded to version %s!\n", latest.Version)
	fmt.Println()
	fmt.Println("  If running as a service, restart it:")
	fmt.Println("    sudo systemctl restart piportal")
	fmt.Println()

	return nil
}

func getLatestVersion() (*VersionInfo, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(defaultServer + "/api/version")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %d", resp.StatusCode)
	}

	var info VersionInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}

	return &info, nil
}

func getArch() string {
	if runtime.GOOS != "linux" {
		return ""
	}

	switch runtime.GOARCH {
	case "arm64":
		return "arm64"
	case "arm":
		return "arm"
	case "amd64":
		return "amd64"
	default:
		return ""
	}
}

func downloadBinary(url string) ([]byte, error) {
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download returned %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func replaceBinary(path string, newBinary []byte) error {
	// Write to temp file first
	tmpPath := path + ".new"
	if err := os.WriteFile(tmpPath, newBinary, 0755); err != nil {
		return err
	}

	// Atomic rename (works on Unix)
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return err
	}

	return nil
}
