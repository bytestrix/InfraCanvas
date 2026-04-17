package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"infracanvas/internal/models"
	"infracanvas/pkg/orchestrator"
	"infracanvas/pkg/output"
)

var (
	namespace   string
	labels      []string
	noRedaction bool
)

// discoverCmd represents the discover command
var discoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Perform full infrastructure discovery",
	Long: `Discover performs a complete infrastructure discovery across the specified scope.
It collects information about hosts, containers, Kubernetes resources, and their relationships.`,
	RunE: runDiscover,
}

func init() {
	rootCmd.AddCommand(discoverCmd)

	discoverCmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Filter by Kubernetes namespace")
	discoverCmd.Flags().StringSliceVarP(&labels, "labels", "l", []string{}, "Filter by labels (key=value)")
	discoverCmd.Flags().BoolVar(&noRedaction, "no-redaction", false, "Disable sensitive data redaction")
}

func runDiscover(cmd *cobra.Command, args []string) error {
	if !quiet {
		fmt.Fprintln(os.Stderr, "Starting infrastructure discovery...")
	}

	// Create orchestrator
	orch := orchestrator.NewOrchestrator(!noRedaction)

	// Execute discovery
	ctx := context.Background()
	snapshot, err := orch.Discover(ctx, scope)
	if err != nil {
		return fmt.Errorf("discovery failed: %w", err)
	}

	// Display warnings if any errors occurred
	if len(snapshot.Metadata.Errors) > 0 && !quiet {
		fmt.Fprintln(os.Stderr, "\nWarnings:")
		for _, collErr := range snapshot.Metadata.Errors {
			fmt.Fprintf(os.Stderr, "  [%s] %s\n", collErr.Layer, collErr.Message)
		}
		fmt.Fprintln(os.Stderr)
	}

	// Format and output results
	if err := formatOutput(snapshot); err != nil {
		return fmt.Errorf("failed to format output: %w", err)
	}

	if !quiet {
		fmt.Fprintf(os.Stderr, "\nDiscovery completed in %v\n", snapshot.Metadata.CollectionDuration)
		fmt.Fprintf(os.Stderr, "Discovered %d entities and %d relationships\n", len(snapshot.Entities), len(snapshot.Relations))
	}

	return nil
}

func formatOutput(data interface{}) error {
	switch outputFormat {
	case "json":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(data)
	case "yaml":
		encoder := yaml.NewEncoder(os.Stdout)
		encoder.SetIndent(2)
		return encoder.Encode(data)
	case "table":
		snapshot, ok := data.(*models.InfraSnapshot)
		if !ok {
			return fmt.Errorf("invalid data type for table format")
		}
		formatter := output.NewFormatter(output.FormatTable)
		result, err := formatter.Format(snapshot)
		if err != nil {
			return err
		}
		fmt.Print(string(result))
		return nil
	case "graph":
		snapshot, ok := data.(*models.InfraSnapshot)
		if !ok {
			return fmt.Errorf("invalid data type for graph format")
		}
		formatter := output.NewFormatter(output.FormatGraph)
		result, err := formatter.Format(snapshot)
		if err != nil {
			return err
		}
		fmt.Print(string(result))
		return nil
	default:
		return fmt.Errorf("unsupported output format: %s", outputFormat)
	}
}
