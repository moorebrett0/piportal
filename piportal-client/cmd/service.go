package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var servicePort int

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Manage PiPortal as a system service",
	Long:  `Install or uninstall PiPortal as a systemd service for automatic startup.`,
}

var serviceInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install PiPortal as a system service",
	Long: `Install PiPortal as a systemd service.

This will:
  1. Copy your config to /etc/piportal/
  2. Create a piportal system user
  3. Install the systemd service
  4. Enable and start the service

Requires root privileges (use sudo).`,
	RunE: runServiceInstall,
}

var serviceUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove PiPortal system service",
	Long: `Remove PiPortal systemd service.

This will:
  1. Stop and disable the service
  2. Remove the systemd unit file
  3. Optionally remove /etc/piportal/

Requires root privileges (use sudo).`,
	RunE: runServiceUninstall,
}

var serviceStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show system service status",
	RunE:  runServiceStatus,
}

func init() {
	rootCmd.AddCommand(serviceCmd)
	serviceCmd.AddCommand(serviceInstallCmd)
	serviceCmd.AddCommand(serviceUninstallCmd)
	serviceCmd.AddCommand(serviceStatusCmd)

	serviceInstallCmd.Flags().IntVarP(&servicePort, "port", "p", 0, "Local port to forward to")
}

func runServiceInstall(cmd *cobra.Command, args []string) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("service install is only supported on Linux")
	}

	if os.Geteuid() != 0 {
		return fmt.Errorf("service install requires root privileges (use sudo)")
	}

	fmt.Println()
	fmt.Println("  Installing PiPortal Service")
	fmt.Println("  ─────────────────────────────────────────")
	fmt.Println()

	// Load user config
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	if cfg.Token == "" {
		return fmt.Errorf("no token configured - run 'piportal setup' first")
	}

	if servicePort != 0 {
		cfg.LocalPort = servicePort
	}

	// Create piportal user if needed
	fmt.Print("  Creating piportal user... ")
	if _, err := user.Lookup("piportal"); err != nil {
		cmd := exec.Command("useradd", "--system", "--no-create-home", "--shell", "/usr/sbin/nologin", "piportal")
		if err := cmd.Run(); err != nil {
			fmt.Println("✗")
			return fmt.Errorf("failed to create user: %w", err)
		}
		fmt.Println("✓")
	} else {
		fmt.Println("exists")
	}

	// Create /etc/piportal directory
	fmt.Print("  Creating /etc/piportal... ")
	if err := os.MkdirAll("/etc/piportal", 0700); err != nil {
		fmt.Println("✗")
		return err
	}
	fmt.Println("✓")

	// Write system config
	fmt.Print("  Writing config file... ")
	sysConfig := map[string]interface{}{
		"server":     cfg.Server,
		"token":      cfg.Token,
		"subdomain":  cfg.Subdomain,
		"local_port": cfg.LocalPort,
		"local_host": cfg.LocalHost,
	}
	data, _ := yaml.Marshal(sysConfig)
	if err := os.WriteFile("/etc/piportal/config.yaml", data, 0600); err != nil {
		fmt.Println("✗")
		return err
	}
	// Set ownership
	exec.Command("chown", "-R", "piportal:piportal", "/etc/piportal").Run()
	fmt.Println("✓")

	// Find the piportal binary
	binaryPath, err := exec.LookPath("piportal")
	if err != nil {
		// Try current directory
		binaryPath, _ = filepath.Abs("piportal")
		if _, err := os.Stat(binaryPath); err != nil {
			binaryPath = "/usr/local/bin/piportal"
		}
	}

	// Copy binary to /usr/local/bin if not there
	if binaryPath != "/usr/local/bin/piportal" {
		fmt.Print("  Installing binary... ")
		currentBinary, _ := os.Executable()
		input, err := os.ReadFile(currentBinary)
		if err != nil {
			fmt.Println("✗")
			return err
		}
		if err := os.WriteFile("/usr/local/bin/piportal", input, 0755); err != nil {
			fmt.Println("✗")
			return err
		}
		fmt.Println("✓")
	}

	// Write systemd unit file
	fmt.Print("  Installing systemd service... ")
	unitFile := `[Unit]
Description=PiPortal - Secure tunnel for your Pi
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=piportal
Group=piportal
ExecStart=/usr/local/bin/piportal start
Restart=on-failure
RestartSec=5
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
ReadOnlyPaths=/etc/piportal
StandardOutput=journal
StandardError=journal
SyslogIdentifier=piportal

[Install]
WantedBy=multi-user.target
`
	if err := os.WriteFile("/etc/systemd/system/piportal.service", []byte(unitFile), 0644); err != nil {
		fmt.Println("✗")
		return err
	}
	fmt.Println("✓")

	// Reload systemd
	fmt.Print("  Reloading systemd... ")
	if err := exec.Command("systemctl", "daemon-reload").Run(); err != nil {
		fmt.Println("✗")
		return err
	}
	fmt.Println("✓")

	// Enable and start service
	fmt.Print("  Enabling service... ")
	if err := exec.Command("systemctl", "enable", "piportal").Run(); err != nil {
		fmt.Println("✗")
		return err
	}
	fmt.Println("✓")

	fmt.Print("  Starting service... ")
	if err := exec.Command("systemctl", "start", "piportal").Run(); err != nil {
		fmt.Println("✗")
		return err
	}
	fmt.Println("✓")

	fmt.Println()
	fmt.Println("  ✓ PiPortal service installed and running!")
	fmt.Println()
	if cfg.Subdomain != "" {
		fmt.Printf("  Your tunnel: https://%s.piportal.dev\n", cfg.Subdomain)
	}
	fmt.Println()
	fmt.Println("  Useful commands:")
	fmt.Println("    sudo systemctl status piportal   # Check status")
	fmt.Println("    sudo journalctl -u piportal -f   # View logs")
	fmt.Println("    sudo systemctl restart piportal  # Restart")
	fmt.Println("    sudo piportal service uninstall  # Remove")
	fmt.Println()

	return nil
}

func runServiceUninstall(cmd *cobra.Command, args []string) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("service uninstall is only supported on Linux")
	}

	if os.Geteuid() != 0 {
		return fmt.Errorf("service uninstall requires root privileges (use sudo)")
	}

	fmt.Println()
	fmt.Println("  Uninstalling PiPortal Service")
	fmt.Println("  ─────────────────────────────────────────")
	fmt.Println()

	// Stop service
	fmt.Print("  Stopping service... ")
	exec.Command("systemctl", "stop", "piportal").Run()
	fmt.Println("✓")

	// Disable service
	fmt.Print("  Disabling service... ")
	exec.Command("systemctl", "disable", "piportal").Run()
	fmt.Println("✓")

	// Remove unit file
	fmt.Print("  Removing systemd service... ")
	os.Remove("/etc/systemd/system/piportal.service")
	exec.Command("systemctl", "daemon-reload").Run()
	fmt.Println("✓")

	fmt.Println()
	fmt.Println("  ✓ Service removed!")
	fmt.Println()
	fmt.Println("  Note: /etc/piportal/ and /usr/local/bin/piportal were kept.")
	fmt.Println("  Remove manually if no longer needed.")
	fmt.Println()

	return nil
}

func runServiceStatus(cmd *cobra.Command, args []string) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("service status is only supported on Linux")
	}

	output, err := exec.Command("systemctl", "status", "piportal").CombinedOutput()
	if err != nil {
		// systemctl status returns non-zero if service is not running
		fmt.Println(string(output))
		return nil
	}
	fmt.Println(string(output))
	return nil
}
