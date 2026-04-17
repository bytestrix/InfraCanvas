package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"infracanvas/pkg/orchestrator"
)

var (
	exportOutput string
	exportFormat string
)

// exportCmd represents the export command
var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export infrastructure data",
	Long: `Export infrastructure discovery data to a file in various formats.
Supports JSON, YAML, and graph formats for external analysis.`,
	RunE: runExport,
}

func init() {
	rootCmd.AddCommand(exportCmd)

	exportCmd.Flags().StringVarP(&exportOutput, "output", "o", "", "Output file path (default: stdout)")
	exportCmd.Flags().StringVarP(&exportFormat, "format", "f", "json", "Export format (json, yaml, graph)")
}

func runExport(cmd *cobra.Command, args []string) error {
	if !quiet {
		fmt.Fprintln(os.Stderr, "Exporting infrastructure data...")
	}

	// Execute discovery
	orch := orchestrator.NewOrchestrator(true)
	ctx := context.Background()
	snapshot, err := orch.Discover(ctx, scope)
	if err != nil {
		return fmt.Errorf("discovery failed: %w", err)
	}

	// Prepare output writer
	var writer *os.File
	if exportOutput == "" {
		writer = os.Stdout
	} else {
		writer, err = os.Create(exportOutput)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer writer.Close()
	}

	// Export based on format
	switch exportFormat {
	case "json":
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(snapshot); err != nil {
			return fmt.Errorf("failed to encode JSON: %w", err)
		}

	case "yaml":
		encoder := yaml.NewEncoder(writer)
		encoder.SetIndent(2)
		if err := encoder.Encode(snapshot); err != nil {
			return fmt.Errorf("failed to encode YAML: %w", err)
		}

	case "graph":
		// Export as graph format (nodes and edges)
		graph := map[string]interface{}{
			"nodes": snapshot.Entities,
			"edges": snapshot.Relations,
		}
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(graph); err != nil {
			return fmt.Errorf("failed to encode graph: %w", err)
		}

	default:
		return fmt.Errorf("unsupported export format: %s", exportFormat)
	}

	if !quiet && exportOutput != "" {
		fmt.Fprintf(os.Stderr, "Data exported to %s\n", exportOutput)
	}

	return nil
}
