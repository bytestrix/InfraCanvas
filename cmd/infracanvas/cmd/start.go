package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"infracanvas/pkg/agent"
)

var (
	backendURL     string
	agentToken     string
	refreshSeconds int
	tlsInsecure    bool
	noRedact       bool
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the agent and stream infrastructure to the canvas",
	Long: `Start connects to the InfraCanvas backend, receives a pair code, and begins
streaming live infrastructure graph data to the canvas.

Run this on the machine you want to observe:

  infracanvas start
  infracanvas start --backend ws://localhost:8080
  infracanvas start --scope host,docker --refresh 15

Then open the canvas UI and enter the displayed pair code.

Environment variables:
  INFRACANVAS_BACKEND   Override --backend
  INFRACANVAS_SCOPE     Override --scope (comma-separated)
  INFRACANVAS_TOKEN     Override --token (shared secret)`,
	RunE: runStart,
}

func init() {
	rootCmd.AddCommand(startCmd)

	startCmd.Flags().StringVar(&backendURL, "backend", "", "Backend WebSocket URL (default: ws://localhost:8080)")
	startCmd.Flags().StringVar(&agentToken, "token", "", "Auth token matching INFRACANVAS_TOKEN on the server")
	startCmd.Flags().IntVar(&refreshSeconds, "refresh", 30, "Seconds between refresh cycles")
	startCmd.Flags().BoolVar(&tlsInsecure, "tls-insecure", false, "Skip TLS certificate verification")
	startCmd.Flags().BoolVar(&noRedact, "no-redact", false, "Disable sensitive data redaction")
}

func runStart(cmd *cobra.Command, args []string) error {
	cfg := agent.DefaultWSConfig()

	// Resolve backend URL: flag > env > default.
	if backendURL != "" {
		cfg.BackendURL = backendURL
	} else if envURL := os.Getenv("INFRACANVAS_BACKEND"); envURL != "" {
		cfg.BackendURL = envURL
	}

	// Resolve auth token: flag > env.
	if agentToken != "" {
		cfg.AuthToken = agentToken
	} else if envToken := os.Getenv("INFRACANVAS_TOKEN"); envToken != "" {
		cfg.AuthToken = envToken
	}

	// Scope: use the global --scope flag.
	cfg.Scope = scope

	// Override scope from env if set.
	if envScope := os.Getenv("INFRACANVAS_SCOPE"); envScope != "" {
		cfg.Scope = splitComma(envScope)
	}

	cfg.RefreshSeconds = refreshSeconds
	cfg.TLSInsecure = tlsInsecure
	cfg.EnableRedaction = !noRedact

	if !quiet {
		fmt.Fprintf(os.Stderr, "Connecting to %s (scope: %v, refresh: %ds)...\n",
			cfg.BackendURL, cfg.Scope, cfg.RefreshSeconds)
	}

	ag, err := agent.NewWSAgent(cfg)
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	return ag.Run(context.Background())
}

func splitComma(s string) []string {
	var parts []string
	for _, p := range splitOn(s, ',') {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

func splitOn(s string, sep rune) []string {
	var parts []string
	start := 0
	for i, r := range s {
		if r == sep {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}
