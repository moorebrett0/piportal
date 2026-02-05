package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Version is set at build time
var Version = "0.1.4"

var rootCmd = &cobra.Command{
	Use:   "piportal",
	Short: "Expose local services to the internet",
	Long: `PiPortal - Secure tunnels for your Raspberry Pi and IoT devices.

Expose your local web server to the internet with a simple command.
Connect to your self-hosted PiPortal server and get a public URL
that forwards to your local service.`,
	// If no subcommand is given, show help
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.Version = Version
	rootCmd.SetVersionTemplate("piportal version {{.Version}}\n")
}
