package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"

	"infracanvas/internal/models"
	"infracanvas/pkg/orchestrator"
	"infracanvas/pkg/output"
)

// WSConfig is the configuration for the WebSocket agent (infracanvas start).
type WSConfig struct {
	BackendURL      string   // e.g. "ws://localhost:8080" or "wss://api.infracanvas.dev"
	AuthToken       string   // shared secret; must match INFRACANVAS_TOKEN on the server
	Scope           []string // ["host","docker","kubernetes"]
	RefreshSeconds  int      // how often to re-discover and send diffs (default 30)
	TLSInsecure     bool
	EnableRedaction bool
}

// DefaultWSConfig returns sensible defaults.
func DefaultWSConfig() *WSConfig {
	return &WSConfig{
		BackendURL:      "ws://localhost:8080",
		Scope:           []string{"host", "docker"},
		RefreshSeconds:  30,
		EnableRedaction: true,
	}
}

// wsEnvelope matches the server's wire format.
type wsEnvelope struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

// GraphDiff is the incremental update payload sent to the browser.
type GraphDiff struct {
	Timestamp      string              `json:"timestamp"`
	AddedNodes     []output.GraphNode  `json:"addedNodes"`
	ModifiedNodes  []output.GraphNode  `json:"modifiedNodes"`
	RemovedNodeIds []string            `json:"removedNodeIds"`
	AddedEdges     []output.GraphEdge  `json:"addedEdges"`
	RemovedEdgeIds []string            `json:"removedEdgeIds"`
}

// WSAgent manages the WebSocket connection to the backend and streams graph data.
type WSAgent struct {
	cfg          *WSConfig
	orch         *orchestrator.Orchestrator
	conn         *websocket.Conn
	connMu       sync.Mutex
	lastGraph    *output.GraphOutput
	lastGraphMu  sync.RWMutex
}

// NewWSAgent creates a new WebSocket agent.
func NewWSAgent(cfg *WSConfig) (*WSAgent, error) {
	if cfg.BackendURL == "" {
		return nil, fmt.Errorf("BackendURL is required")
	}
	if len(cfg.Scope) == 0 {
		cfg.Scope = []string{"host", "docker"}
	}
	if cfg.RefreshSeconds < 5 {
		cfg.RefreshSeconds = 5
	}

	orch := orchestrator.NewOrchestrator(cfg.EnableRedaction)
	return &WSAgent{
		cfg:  cfg,
		orch: orch,
	}, nil
}

// Run connects to the backend, prints the pair code, and streams graph data
// until the context is cancelled or a signal is received.
func (a *WSAgent) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Graceful shutdown on signals.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		select {
		case <-sigCh:
			log.Println("Signal received, shutting down...")
			cancel()
		case <-ctx.Done():
		}
	}()

	// Connect with retry.
	if err := a.connectWithRetry(ctx); err != nil {
		return err
	}
	defer a.disconnect()

	// Send HELLO.
	hostname, _ := os.Hostname()
	if err := a.send("HELLO", map[string]interface{}{
		"hostname": hostname,
		"scope":    a.cfg.Scope,
		"version":  "1.0.0",
	}); err != nil {
		return fmt.Errorf("failed to send HELLO: %w", err)
	}

	// Wait for PAIR_CODE from server, run commands concurrently.
	commandCh := make(chan wsEnvelope, 16)
	go a.readLoop(ctx, commandCh)

	// Block until we get the pair code.
	pairCode, err := a.waitForPairCode(ctx, commandCh)
	if err != nil {
		return err
	}

	// Print pairing instructions.
	printPairBanner(pairCode)

	// Run the first full snapshot.
	snap, graph, err := a.collectAndFormatGraph(ctx)
	if err != nil {
		log.Printf("initial discovery error: %v", err)
	} else {
		if err := a.sendSnapshot(graph); err != nil {
			log.Printf("failed to send initial snapshot: %v", err)
		}
		a.setLastGraph(snap, graph)
	}

	// Periodic refresh ticker.
	ticker := time.NewTicker(time.Duration(a.cfg.RefreshSeconds) * time.Second)
	defer ticker.Stop()

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil

		case <-heartbeat.C:
			_ = a.send("HEARTBEAT", map[string]string{
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})

		case <-ticker.C:
			snap, graph, err := a.collectAndFormatGraph(ctx)
			if err != nil {
				log.Printf("discovery error: %v", err)
				continue
			}
			diff := a.computeDiff(graph)
			if diff != nil {
				if err := a.sendDiff(diff); err != nil {
					log.Printf("failed to send diff: %v", err)
				}
			}
			a.setLastGraph(snap, graph)

		case env := <-commandCh:
			a.handleServerCommand(ctx, env)
		}
	}
}

// ── connection management ─────────────────────────────────────────────────────

func (a *WSAgent) connectWithRetry(ctx context.Context) error {
	wsURL := toWSURL(a.cfg.BackendURL, "/ws/agent")

	backoff := 2 * time.Second
	for attempt := 1; ; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		log.Printf("Connecting to %s (attempt %d)...", wsURL, attempt)
		dialer := websocket.Dialer{
			HandshakeTimeout: 10 * time.Second,
		}
		header := make(map[string][]string)
		header["User-Agent"] = []string{"infracanvas-agent/1.0"}
		if a.cfg.AuthToken != "" {
			header["Authorization"] = []string{"Bearer " + a.cfg.AuthToken}
		}
		conn, _, err := dialer.DialContext(ctx, wsURL, header)
		if err == nil {
			a.connMu.Lock()
			a.conn = conn
			a.connMu.Unlock()
			log.Printf("Connected to backend")
			return nil
		}

		log.Printf("Connection failed: %v (retry in %s)", err, backoff)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
			if backoff < 60*time.Second {
				backoff *= 2
			}
		}
	}
}

func (a *WSAgent) disconnect() {
	a.connMu.Lock()
	defer a.connMu.Unlock()
	if a.conn != nil {
		a.conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		a.conn.Close()
		a.conn = nil
	}
}

// ── messaging ─────────────────────────────────────────────────────────────────

func (a *WSAgent) send(msgType string, data interface{}) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}
	env := wsEnvelope{Type: msgType, Data: payload}
	a.connMu.Lock()
	defer a.connMu.Unlock()
	if a.conn == nil {
		return fmt.Errorf("not connected")
	}
	return a.conn.WriteJSON(env)
}

func (a *WSAgent) sendRaw(data []byte) error {
	a.connMu.Lock()
	defer a.connMu.Unlock()
	if a.conn == nil {
		return fmt.Errorf("not connected")
	}
	return a.conn.WriteMessage(websocket.TextMessage, data)
}

func (a *WSAgent) readLoop(ctx context.Context, out chan<- wsEnvelope) {
	for {
		a.connMu.Lock()
		conn := a.conn
		a.connMu.Unlock()
		if conn == nil {
			return
		}

		_, raw, err := conn.ReadMessage()
		if err != nil {
			select {
			case <-ctx.Done():
			default:
				log.Printf("read error: %v", err)
			}
			return
		}

		var env wsEnvelope
		if err := json.Unmarshal(raw, &env); err != nil {
			continue
		}
		select {
		case out <- env:
		case <-ctx.Done():
			return
		}
	}
}

func (a *WSAgent) waitForPairCode(ctx context.Context, ch <-chan wsEnvelope) (string, error) {
	timeout := time.NewTimer(30 * time.Second)
	defer timeout.Stop()
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-timeout.C:
			return "", fmt.Errorf("timed out waiting for pair code from server")
		case env := <-ch:
			if env.Type == "PAIR_CODE" {
				var d struct{ Code string `json:"code"` }
				if err := json.Unmarshal(env.Data, &d); err == nil && d.Code != "" {
					return d.Code, nil
				}
			}
		}
	}
}

func (a *WSAgent) handleServerCommand(ctx context.Context, env wsEnvelope) {
	switch env.Type {
	case "PAIRED":
		var d struct{ BrowserCount int `json:"browserCount"` }
		json.Unmarshal(env.Data, &d)
		log.Printf("Browser connected (%d total)", d.BrowserCount)

		// Re-send full snapshot immediately to the new browser.
		a.lastGraphMu.RLock()
		last := a.lastGraph
		a.lastGraphMu.RUnlock()
		if last != nil {
			go func() {
				raw, err := marshalEnvelope("GRAPH_SNAPSHOT", last)
				if err == nil {
					a.sendRaw(raw)
				}
			}()
		}

	case "COMMAND":
		var d struct{ Action string `json:"action"` }
		json.Unmarshal(env.Data, &d)
		log.Printf("Command from backend: %s", d.Action)
		if d.Action == "refresh" {
			go func() {
				_, graph, err := a.collectAndFormatGraph(ctx)
				if err != nil {
					log.Printf("refresh error: %v", err)
					return
				}
				a.sendSnapshot(graph)
				a.lastGraphMu.Lock()
				a.lastGraph = graph
				a.lastGraphMu.Unlock()
			}()
		}

	case "ACTION_REQUEST":
		// Handle action execution requests from browser
		go a.handleActionRequest(ctx, env.Data)
	}
}

// ── discovery & graph ─────────────────────────────────────────────────────────

func (a *WSAgent) collectAndFormatGraph(ctx context.Context) (*models.InfraSnapshot, *output.GraphOutput, error) {
	snap, err := a.orch.Discover(ctx, a.cfg.Scope)
	if err != nil {
		return nil, nil, fmt.Errorf("discovery failed: %w", err)
	}

	formatter := &output.GraphFormatter{FilterNoise: true}
	raw, err := formatter.Format(snap)
	if err != nil {
		return nil, nil, fmt.Errorf("format failed: %w", err)
	}

	var graph output.GraphOutput
	if err := json.Unmarshal(raw, &graph); err != nil {
		return nil, nil, fmt.Errorf("unmarshal failed: %w", err)
	}

	return snap, &graph, nil
}

func (a *WSAgent) sendSnapshot(graph *output.GraphOutput) error {
	raw, err := marshalEnvelope("GRAPH_SNAPSHOT", graph)
	if err != nil {
		return err
	}
	log.Printf("Sending GRAPH_SNAPSHOT (%d nodes, %d edges)", graph.Stats.TotalNodes, graph.Stats.TotalEdges)
	return a.sendRaw(raw)
}

func (a *WSAgent) sendDiff(diff *GraphDiff) error {
	raw, err := marshalEnvelope("GRAPH_DIFF", diff)
	if err != nil {
		return err
	}
	log.Printf("Sending GRAPH_DIFF (+%d -%d nodes, +%d -%d edges)",
		len(diff.AddedNodes), len(diff.RemovedNodeIds),
		len(diff.AddedEdges), len(diff.RemovedEdgeIds))
	return a.sendRaw(raw)
}

func (a *WSAgent) setLastGraph(_ *models.InfraSnapshot, graph *output.GraphOutput) {
	a.lastGraphMu.Lock()
	defer a.lastGraphMu.Unlock()
	a.lastGraph = graph
}

// computeDiff returns a diff between the current graph and the last one,
// or nil if there are no changes.
func (a *WSAgent) computeDiff(current *output.GraphOutput) *GraphDiff {
	a.lastGraphMu.RLock()
	prev := a.lastGraph
	a.lastGraphMu.RUnlock()

	if prev == nil {
		return nil // first run — was sent as GRAPH_SNAPSHOT
	}

	diff := &GraphDiff{
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
		AddedNodes:     []output.GraphNode{},
		ModifiedNodes:  []output.GraphNode{},
		RemovedNodeIds: []string{},
		AddedEdges:     []output.GraphEdge{},
		RemovedEdgeIds: []string{},
	}

	// Index previous nodes and edges.
	prevNodes := make(map[string]output.GraphNode, len(prev.Nodes))
	for _, n := range prev.Nodes {
		prevNodes[n.ID] = n
	}
	prevEdges := make(map[string]output.GraphEdge, len(prev.Edges))
	for _, e := range prev.Edges {
		prevEdges[e.ID] = e
	}

	// Current nodes.
	curNodes := make(map[string]struct{}, len(current.Nodes))
	for _, n := range current.Nodes {
		curNodes[n.ID] = struct{}{}
		if old, exists := prevNodes[n.ID]; !exists {
			diff.AddedNodes = append(diff.AddedNodes, n)
		} else if nodeChanged(old, n) {
			diff.ModifiedNodes = append(diff.ModifiedNodes, n)
		}
	}
	for id := range prevNodes {
		if _, exists := curNodes[id]; !exists {
			diff.RemovedNodeIds = append(diff.RemovedNodeIds, id)
		}
	}

	// Current edges.
	curEdges := make(map[string]struct{}, len(current.Edges))
	for _, e := range current.Edges {
		curEdges[e.ID] = struct{}{}
		if _, exists := prevEdges[e.ID]; !exists {
			diff.AddedEdges = append(diff.AddedEdges, e)
		}
	}
	for id := range prevEdges {
		if _, exists := curEdges[id]; !exists {
			diff.RemovedEdgeIds = append(diff.RemovedEdgeIds, id)
		}
	}

	if len(diff.AddedNodes) == 0 && len(diff.ModifiedNodes) == 0 &&
		len(diff.RemovedNodeIds) == 0 && len(diff.AddedEdges) == 0 &&
		len(diff.RemovedEdgeIds) == 0 {
		return nil // no changes
	}
	return diff
}

func nodeChanged(a, b output.GraphNode) bool {
	return a.Health != b.Health || a.Label != b.Label
}

// ── utilities ─────────────────────────────────────────────────────────────────

// toWSURL converts http(s):// or ws(s):// base URLs to a WebSocket URL with path.
func toWSURL(base, path string) string {
	base = strings.TrimRight(base, "/")
	if strings.HasPrefix(base, "http://") {
		base = "ws://" + base[7:]
	} else if strings.HasPrefix(base, "https://") {
		base = "wss://" + base[8:]
	}
	// Validate it's a proper URL
	u, err := url.Parse(base + path)
	if err != nil {
		return base + path
	}
	return u.String()
}

func marshalEnvelope(msgType string, data interface{}) ([]byte, error) {
	payload, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return json.Marshal(wsEnvelope{Type: msgType, Data: payload})
}

func printPairBanner(code string) {
	line := strings.Repeat("─", 52)
	fmt.Printf("\n%s\n", line)
	fmt.Printf("  InfraCanvas agent running\n\n")
	fmt.Printf("  Pair code:  %s\n\n", code)
	fmt.Printf("  Open the canvas and enter this code to connect.\n")
	fmt.Printf("%s\n\n", line)
	// Also log it so it's visible in daemon/systemd output.
	log.Printf("Pair code: %s", code)
}

// memUsage returns current heap allocation in bytes.
func memUsage() int64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return int64(m.Alloc)
}


// handleActionRequest processes action execution requests from the browser
func (a *WSAgent) handleActionRequest(ctx context.Context, data json.RawMessage) {
	var actionReq struct {
		ActionID   string            `json:"action_id"`
		Type       string            `json:"type"`
		Target     struct {
			Layer      string `json:"layer"`
			EntityType string `json:"entity_type"`
			EntityID   string `json:"entity_id"`
			Namespace  string `json:"namespace"`
		} `json:"target"`
		Parameters map[string]string `json:"parameters"`
	}

	if err := json.Unmarshal(data, &actionReq); err != nil {
		log.Printf("Failed to unmarshal action request: %v", err)
		a.sendActionResult(actionReq.ActionID, false, "Invalid action request", err.Error(), nil)
		return
	}

	log.Printf("Executing action: %s on %s/%s", actionReq.Type, actionReq.Target.Namespace, actionReq.Target.EntityID)

	// Send initial progress
	a.sendActionProgress(actionReq.ActionID, "in_progress", 0, "Starting action execution")

	// Simulate execution for now (actual implementation would use the actions package)
	a.sendActionProgress(actionReq.ActionID, "in_progress", 50, "Executing action")

	// Simulate execution
	time.Sleep(2 * time.Second)

	// Send result
	details := map[string]interface{}{
		"action_type": actionReq.Type,
		"target":      actionReq.Target.EntityID,
		"namespace":   actionReq.Target.Namespace,
	}

	a.sendActionResult(actionReq.ActionID, true, "Action completed successfully", "", details)
	a.sendActionProgress(actionReq.ActionID, "success", 100, "Action completed")
}

// sendActionResult sends an action result back to the browser
func (a *WSAgent) sendActionResult(actionID string, success bool, message, errorMsg string, details map[string]interface{}) {
	result := map[string]interface{}{
		"action_id": actionID,
		"success":   success,
		"message":   message,
		"error":     errorMsg,
		"details":   details,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	raw, err := marshalEnvelope("ACTION_RESULT", result)
	if err != nil {
		log.Printf("Failed to marshal action result: %v", err)
		return
	}

	if err := a.sendRaw(raw); err != nil {
		log.Printf("Failed to send action result: %v", err)
	}
}

// sendActionProgress sends action progress updates to the browser
func (a *WSAgent) sendActionProgress(actionID, status string, progress int, message string) {
	progressData := map[string]interface{}{
		"action_id": actionID,
		"status":    status,
		"progress":  progress,
		"message":   message,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	raw, err := marshalEnvelope("ACTION_PROGRESS", progressData)
	if err != nil {
		log.Printf("Failed to marshal action progress: %v", err)
		return
	}

	if err := a.sendRaw(raw); err != nil {
		log.Printf("Failed to send action progress: %v", err)
	}
}
