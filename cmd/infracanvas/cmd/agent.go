package cmd

import (
	"context"
	"fmt"
	"log"

	"github.com/spf13/cobra"
	"infracanvas/pkg/agent"
)

var (
	configFile string
)

// agentCmd represents the agent command
var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Run the infrastructure discovery agent",
	Long: `Run the infrastructure discovery agent in continuous mode.

The agent performs continuous background collection with real-time updates 
to a backend platform. It supports periodic collection, event-based watching, 
and incremental updates.

Example:
  rix agent --config /etc/rix/agent.yaml
  rix agent  # Uses default config and environment variables`,
	RunE: runAgent,
}

func init() {
	rootCmd.AddCommand(agentCmd)

	agentCmd.Flags().StringVarP(&configFile, "config", "c", "", "Path to agent configuration file")
}

func runAgent(cmd *cobra.Command, args []string) error {
	// Load configuration
	config, err := agent.LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Print configuration summary
	if !quiet {
		log.Printf("Agent Configuration:")
		log.Printf("  Backend URL: %s", config.BackendURL)
		log.Printf("  Agent Name: %s", config.AgentName)
		log.Printf("  Scope: %v", config.Scope)
		log.Printf("  Host Interval: %ds", config.HostInterval)
		log.Printf("  Docker Interval: %ds", config.DockerInterval)
		log.Printf("  Kubernetes Interval: %ds", config.KubernetesInterval)
		log.Printf("  Heartbeat Interval: %ds", config.HeartbeatInterval)
		log.Printf("  Redaction Enabled: %v", config.EnableRedaction)
		log.Printf("  Watchers Enabled: %v", config.EnableWatchers)
	}

	// Create agent
	ag, err := agent.NewAgent(config)
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	// Run agent
	ctx := context.Background()
	if err := ag.Run(ctx); err != nil {
		return fmt.Errorf("agent error: %w", err)
	}

	return nil
}
