package agent

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"infracanvas/pkg/orchestrator"
)

// Agent represents the infrastructure discovery agent
type Agent struct {
	config          *Config
	orchestrator    *orchestrator.Orchestrator
	backendClient   *BackendClient
	strategyManager *StrategyManager
	watcherManager  *WatcherManager
	startTime       time.Time
	collectionErrors int
}

// NewAgent creates a new agent instance
func NewAgent(config *Config) (*Agent, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	orch := orchestrator.NewOrchestrator(config.EnableRedaction)
	backendClient := NewBackendClient(config)
	strategyManager := NewStrategyManager(config, orch)

	// Get Kubernetes discovery for cache invalidation
	k8sDiscovery := orch.GetKubernetesDiscovery()
	
	watcherManager, err := NewWatcherManager(config, backendClient, k8sDiscovery)
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher manager: %w", err)
	}

	return &Agent{
		config:          config,
		orchestrator:    orch,
		backendClient:   backendClient,
		strategyManager: strategyManager,
		watcherManager:  watcherManager,
		startTime:       time.Now(),
	}, nil
}

// Run starts the agent main loop
func (a *Agent) Run(ctx context.Context) error {
	log.Println("Starting infrastructure discovery agent...")

	// Register with backend
	if err := a.register(); err != nil {
		return fmt.Errorf("failed to register with backend: %w", err)
	}

	// Perform initial full discovery
	log.Println("Performing initial discovery...")
	snapshot, err := a.strategyManager.CollectInitial(ctx)
	if err != nil {
		return fmt.Errorf("initial discovery failed: %w", err)
	}

	// Send initial snapshot to backend
	log.Println("Sending initial snapshot to backend...")
	if err := a.backendClient.SendSnapshot(snapshot); err != nil {
		log.Printf("Failed to send initial snapshot: %v", err)
		a.collectionErrors++
	}

	// Start event watchers if enabled
	if a.config.EnableWatchers {
		log.Println("Starting event watchers...")
		if err := a.watcherManager.StartAll(ctx); err != nil {
			log.Printf("Failed to start watchers: %v", err)
		}
	}

	// Start command receiver
	commandChan, err := a.backendClient.ReceiveCommands(ctx)
	if err != nil {
		log.Printf("Failed to start command receiver: %v", err)
	}

	// Create tickers for periodic collection
	hostTicker := time.NewTicker(a.config.GetHostIntervalDuration())
	defer hostTicker.Stop()

	dockerTicker := time.NewTicker(a.config.GetDockerIntervalDuration())
	defer dockerTicker.Stop()

	kubernetesTicker := time.NewTicker(a.config.GetKubernetesIntervalDuration())
	defer kubernetesTicker.Stop()

	heartbeatTicker := time.NewTicker(a.config.GetHeartbeatIntervalDuration())
	defer heartbeatTicker.Stop()

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	log.Println("Agent is running. Press Ctrl+C to stop.")

	// Main event loop
	for {
		select {
		case <-ctx.Done():
			log.Println("Context cancelled, shutting down...")
			return a.shutdown()

		case sig := <-sigChan:
			log.Printf("Received signal %v, shutting down...", sig)
			return a.shutdown()

		case <-hostTicker.C:
			if containsScope(a.config.Scope, "host") {
				a.collectAndSendDelta(ctx, "host")
			}

		case <-dockerTicker.C:
			if containsScope(a.config.Scope, "docker") {
				a.collectAndSendDelta(ctx, "docker")
			}

		case <-kubernetesTicker.C:
			if containsScope(a.config.Scope, "kubernetes") {
				a.collectAndSendDelta(ctx, "kubernetes")
			}

		case <-heartbeatTicker.C:
			a.sendHeartbeat()

		case cmd := <-commandChan:
			a.handleCommand(ctx, cmd)
		}
	}
}

// register registers the agent with the backend
func (a *Agent) register() error {
	hostname, _ := os.Hostname()

	req := &RegistrationRequest{
		AgentID:   a.config.AgentID,
		AgentName: a.config.AgentName,
		Hostname:  hostname,
		OS:        runtime.GOOS,
		Scope:     a.config.Scope,
		Version:   "1.0.0", // TODO: Get from build info
		Metadata: map[string]string{
			"arch": runtime.GOARCH,
		},
	}

	resp, err := a.backendClient.Register(req)
	if err != nil {
		return err
	}

	// Update agent ID if provided by backend
	if resp.AgentID != "" {
		a.config.AgentID = resp.AgentID
	}

	log.Printf("Agent registered successfully: %s", resp.Message)
	return nil
}

// collectAndSendDelta performs periodic collection and sends delta to backend
func (a *Agent) collectAndSendDelta(ctx context.Context, layer string) {
	log.Printf("Collecting %s layer...", layer)

	// Collect current state
	currentSnapshot, err := a.strategyManager.CollectPeriodic(ctx, layer)
	if err != nil {
		log.Printf("Failed to collect %s layer: %v", layer, err)
		a.collectionErrors++
		return
	}

	// Get last snapshot
	lastSnapshot := a.strategyManager.GetLastSnapshot()

	// Merge the partial snapshot into the full snapshot
	mergedSnapshot := a.strategyManager.MergeSnapshots(lastSnapshot, currentSnapshot)

	// Calculate delta
	delta := a.strategyManager.CalculateDelta(lastSnapshot, mergedSnapshot)

	// Only send if there are changes
	if len(delta.Added) > 0 || len(delta.Modified) > 0 || len(delta.Removed) > 0 {
		log.Printf("Sending delta: %d added, %d modified, %d removed",
			len(delta.Added), len(delta.Modified), len(delta.Removed))

		if err := a.backendClient.SendDelta(delta); err != nil {
			log.Printf("Failed to send delta: %v", err)
			a.collectionErrors++
		}
	}

	// Update last snapshot
	a.strategyManager.UpdateLastSnapshot(mergedSnapshot)
}

// sendHeartbeat sends a heartbeat to the backend
func (a *Agent) sendHeartbeat() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	health := &AgentHealth{
		Status:           "running",
		Uptime:           int64(time.Since(a.startTime).Seconds()),
		LastCollection:   time.Now(),
		CollectionErrors: a.collectionErrors,
		MemoryUsage:      int64(memStats.Alloc),
	}

	if err := a.backendClient.SendHeartbeat(health); err != nil {
		log.Printf("Failed to send heartbeat: %v", err)
	}
}

// handleCommand processes a command from the backend
func (a *Agent) handleCommand(ctx context.Context, cmd Command) {
	log.Printf("Received command: %s (ID: %s)", cmd.Type, cmd.ID)

	switch cmd.Type {
	case "refresh":
		// Perform full discovery and send snapshot
		snapshot, err := a.strategyManager.CollectInitial(ctx)
		if err != nil {
			log.Printf("Failed to refresh: %v", err)
			return
		}
		if err := a.backendClient.SendSnapshot(snapshot); err != nil {
			log.Printf("Failed to send snapshot: %v", err)
		}

	case "diagnostics":
		// Send diagnostic information
		log.Println("Diagnostics command not yet implemented")

	case "action":
		// Execute an action
		log.Println("Action command not yet implemented")

	default:
		log.Printf("Unknown command type: %s", cmd.Type)
	}
}

// shutdown performs graceful shutdown
func (a *Agent) shutdown() error {
	log.Println("Shutting down agent...")

	// Stop all watchers
	a.watcherManager.StopAll()

	// Send final heartbeat
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	health := &AgentHealth{
		Status:           "stopped",
		Uptime:           int64(time.Since(a.startTime).Seconds()),
		LastCollection:   time.Now(),
		CollectionErrors: a.collectionErrors,
		MemoryUsage:      int64(memStats.Alloc),
	}

	if err := a.backendClient.SendHeartbeat(health); err != nil {
		log.Printf("Failed to send final heartbeat: %v", err)
	}

	log.Println("Agent stopped")
	return nil
}

// containsScope checks if a scope is in the list
func containsScope(scopes []string, scope string) bool {
	for _, s := range scopes {
		if s == scope {
			return true
		}
	}
	return false
}
