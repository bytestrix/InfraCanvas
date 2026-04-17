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

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "infracanvas",
	Short: "Infrastructure Discovery CLI",
	Long: `infracanvas is a comprehensive infrastructure discovery tool that provides complete 
visibility into system infrastructure across bare metal, virtual machines, 
containers, and Kubernetes environments.`,
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
