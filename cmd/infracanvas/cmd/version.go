package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	// Version information (will be set during build)
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Display version information",
	Long:  `Display version information including version number, git commit, and build date.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("infracanvas version %s\n", Version)
		fmt.Printf("Git commit: %s\n", GitCommit)
		fmt.Printf("Build date: %s\n", BuildDate)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
