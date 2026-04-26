package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	// Global flags
	outputFormat string
	scope        []string
	verbose      bool
	quiet        bool
)

// rootCmd represents the base command. With no subcommand, it runs `serve`
// so a bare `infracanvas` brings up the dashboard immediately.
var rootCmd = &cobra.Command{
	Use:   "infracanvas",
	Short: "Run a local infrastructure dashboard for this machine",
	Long: `InfraCanvas shows you what's running on this machine — host, Docker, Kubernetes —
in a live visual canvas. Run with no arguments to start the dashboard.

  infracanvas              # start the dashboard (same as 'serve')
  infracanvas serve        # explicit form, with flags
  infracanvas discover     # one-shot CLI discovery
  infracanvas start        # agent-only mode (connects to a remote relay)`,
	RunE: runServe,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "table", "Output format (json, yaml, table)")
	rootCmd.PersistentFlags().StringSliceVarP(&scope, "scope", "s", []string{"host", "docker", "kubernetes"}, "Discovery scope (host, docker, kubernetes)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Suppress non-essential output")
}
