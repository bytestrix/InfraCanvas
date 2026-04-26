package agent

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"

	"infracanvas/internal/models"
	"infracanvas/pkg/actions"
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
	// QuietPairBanner suppresses the "Pair code: …" stdout banner. Used when
	// the agent runs in-process under `infracanvas serve`, where pair codes
	// are irrelevant (local auto-pair).
	QuietPairBanner bool
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

// execSession holds state for an active interactive exec session.
// Exactly one of dockerSess or ptmx will be non-nil.
type execSession struct {
	dockerSess *actions.ExecSession // Docker exec (container terminal)
	ptmx       *os.File             // host PTY (VM terminal)
	cancel     context.CancelFunc
}

// WSAgent manages the WebSocket connection to the backend and streams graph data.
type WSAgent struct {
	cfg            *WSConfig
	orch           *orchestrator.Orchestrator
	actionExecutor *actions.ActionExecutor
	conn           *websocket.Conn
	connMu         sync.Mutex
	lastGraph      *output.GraphOutput
	lastGraphMu    sync.RWMutex
	execSessions   sync.Map // sessionID → *execSession
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

	executor, err := actions.NewActionExecutor()
	if err != nil {
		log.Printf("[agent] Warning: action executor init failed: %v", err)
	}

	return &WSAgent{
		cfg:            cfg,
		orch:           orch,
		actionExecutor: executor,
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

	// Print pairing instructions (skip in serve-mode where it's noise).
	if !a.cfg.QuietPairBanner {
		printPairBanner(pairCode)
	}

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
		_ = a.conn.WriteMessage(websocket.CloseMessage,
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
		_ = json.Unmarshal(env.Data, &d)
		log.Printf("Browser connected (%d total)", d.BrowserCount)

		// Re-send full snapshot immediately to the new browser.
		a.lastGraphMu.RLock()
		last := a.lastGraph
		a.lastGraphMu.RUnlock()
		if last != nil {
			go func() {
				raw, err := marshalEnvelope("GRAPH_SNAPSHOT", last)
				if err == nil {
					_ = a.sendRaw(raw)
				}
			}()
		}

	case "COMMAND":
		var d struct{ Action string `json:"action"` }
		_ = json.Unmarshal(env.Data, &d)
		log.Printf("Command from backend: %s", d.Action)
		if d.Action == "refresh" {
			go func() {
				_, graph, err := a.collectAndFormatGraph(ctx)
				if err != nil {
					log.Printf("refresh error: %v", err)
					return
				}
				_ = a.sendSnapshot(graph)
				a.lastGraphMu.Lock()
				a.lastGraph = graph
				a.lastGraphMu.Unlock()
			}()
		}

	case "ACTION_REQUEST":
		go a.handleActionRequest(ctx, env.Data)

	case "EXEC_START":
		go a.handleExecStart(ctx, env.Data)

	case "EXEC_INPUT":
		go a.handleExecInput(env.Data)

	case "EXEC_RESIZE":
		go a.handleExecResize(env.Data)

	case "EXEC_END":
		go a.handleExecEnd(env.Data)
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



// handleActionRequest processes action execution requests from the browser.
func (a *WSAgent) handleActionRequest(ctx context.Context, data json.RawMessage) {
	var req struct {
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

	if err := json.Unmarshal(data, &req); err != nil {
		log.Printf("[action] unmarshal error: %v", err)
		a.sendActionResult("", false, "Invalid action request", err.Error(), nil)
		return
	}

	log.Printf("[action] %s on %s/%s (layer=%s)", req.Type, req.Target.Namespace, req.Target.EntityID, req.Target.Layer)
	a.sendActionProgress(req.ActionID, "in_progress", 10, "Starting…")

	// Special case: docker logs streaming
	if req.Type == "docker_logs" {
		a.handleDockerLogs(ctx, req.ActionID, req.Target.EntityID, req.Parameters)
		return
	}

	if a.actionExecutor == nil {
		a.sendActionResult(req.ActionID, false, "Action executor not available", "executor init failed", nil)
		return
	}

	// Map frontend action type → backend ActionType
	actionType, layer := mapFrontendActionType(req.Type)
	if layer != "" {
		req.Target.Layer = layer
	}

	// Normalize entity ID (strip type prefix like "container/", "pod/")
	entityID := normalizeEntityID(req.Target.EntityID)

	action := &actions.Action{
		ID:   req.ActionID,
		Type: actionType,
		Target: actions.ActionTarget{
			Layer:      req.Target.Layer,
			EntityType: req.Target.EntityType,
			EntityID:   entityID,
			Namespace:  req.Target.Namespace,
		},
		Parameters:  req.Parameters,
		RequestedAt: time.Now(),
	}

	a.sendActionProgress(req.ActionID, "in_progress", 50, "Executing…")

	result, err := a.actionExecutor.ExecuteAction(ctx, action)
	if err != nil && result == nil {
		a.sendActionResult(req.ActionID, false, "Execution error", err.Error(), nil)
		a.sendActionProgress(req.ActionID, "failed", 100, "Failed")
		return
	}

	details := map[string]interface{}{
		"action_type": req.Type,
		"target":      req.Target.EntityID,
		"output":      result.Output,
		"duration_ms": result.EndTime.Sub(result.StartTime).Milliseconds(),
	}

	a.sendActionResult(req.ActionID, result.Success, result.Message, result.Error, details)
	status := "success"
	if !result.Success {
		status = "failed"
	}
	a.sendActionProgress(req.ActionID, status, 100, result.Message)
}

// handleDockerLogs fetches container logs and streams them back as LOG_DATA.
func (a *WSAgent) handleDockerLogs(ctx context.Context, requestID, rawEntityID string, params map[string]string) {
	if a.actionExecutor == nil {
		a.sendLogData(requestID, rawEntityID, nil, "action executor not available", true)
		return
	}

	containerID := normalizeEntityID(rawEntityID)
	tail := 200
	if t, ok := params["tail"]; ok {
		if n := atoi(t); n > 0 {
			tail = n
		}
	}

	// Use a short-lived context for the log fetch
	logCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	reader, err := a.actionExecutor.DockerLogs(logCtx, containerID, tail)
	if err != nil {
		a.sendLogData(requestID, rawEntityID, nil, err.Error(), true)
		return
	}
	defer reader.Close()

	var lines []string
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		// Docker multiplexes stdout/stderr with an 8-byte header — strip it.
		if len(line) > 8 {
			h := line[0]
			if h == 1 || h == 2 {
				line = line[8:]
			}
		}
		lines = append(lines, line)
		// Send in batches of 50 lines
		if len(lines) >= 50 {
			a.sendLogData(requestID, rawEntityID, lines, "", false)
			lines = lines[:0]
		}
	}
	// Send remaining lines + done signal
	a.sendLogData(requestID, rawEntityID, lines, "", true)
}

func (a *WSAgent) sendLogData(requestID, containerID string, lines []string, errMsg string, done bool) {
	payload := map[string]interface{}{
		"request_id":   requestID,
		"container_id": containerID,
		"lines":        lines,
		"done":         done,
		"error":        errMsg,
	}
	raw, err := marshalEnvelope("LOG_DATA", payload)
	if err != nil {
		return
	}
	_ = a.sendRaw(raw)
}

// handleExecStart creates an interactive terminal session.
// For layer "host" it spawns a PTY shell on the VM itself.
// For layer "docker" (or empty) it runs docker exec inside the container.
func (a *WSAgent) handleExecStart(ctx context.Context, data json.RawMessage) {
	var req struct {
		SessionID   string   `json:"session_id"`
		ContainerID string   `json:"container_id"` // only for docker layer
		Layer       string   `json:"layer"`         // "host" or "docker"
		Cmd         []string `json:"cmd"`
		Rows        uint     `json:"rows"`
		Cols        uint     `json:"cols"`
	}
	if err := json.Unmarshal(data, &req); err != nil {
		log.Printf("[exec] bad EXEC_START: %v", err)
		return
	}
	if req.SessionID == "" {
		return
	}
	if req.Rows == 0 {
		req.Rows = 24
	}
	if req.Cols == 0 {
		req.Cols = 80
	}

	execCtx, cancel := context.WithCancel(ctx)

	if req.Layer == "host" {
		a.startHostExec(execCtx, cancel, req.SessionID, req.Cmd, req.Rows, req.Cols)
	} else {
		a.startDockerExec(execCtx, cancel, req.SessionID, req.ContainerID, req.Cmd, req.Rows, req.Cols)
	}
}

// startHostExec opens a PTY shell on the host VM.
func (a *WSAgent) startHostExec(ctx context.Context, cancel context.CancelFunc, sessionID string, cmd []string, rows, cols uint) {
	if len(cmd) == 0 {
		for _, sh := range []string{"/bin/bash", "/bin/sh"} {
			if _, err := os.Stat(sh); err == nil {
				cmd = []string{sh, "-l"} // login shell → starts in HOME
				break
			}
		}
	}

	home := os.Getenv("HOME")
	if home == "" {
		home = "/root"
	}
	c := exec.CommandContext(ctx, cmd[0], cmd[1:]...)
	c.Dir = home
	c.Env = append(os.Environ(), "TERM=xterm-256color", "COLORTERM=truecolor")

	ptmx, err := pty.Start(c)
	if err != nil {
		cancel()
		a.sendExecData(sessionID, nil, fmt.Sprintf("failed to open host terminal: %v", err))
		return
	}

	// Set initial window size
	_ = pty.Setsize(ptmx, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})

	a.execSessions.Store(sessionID, &execSession{ptmx: ptmx, cancel: cancel})

	go func() {
		defer func() {
			ptmx.Close()
			_ = c.Process.Kill()
			a.execSessions.Delete(sessionID)
			a.sendExecEnd(sessionID)
		}()
		buf := make([]byte, 4096)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			n, err := ptmx.Read(buf)
			if n > 0 {
				a.sendExecData(sessionID, buf[:n], "")
			}
			if err != nil {
				return
			}
		}
	}()
}

// startDockerExec opens an exec session inside a container.
func (a *WSAgent) startDockerExec(ctx context.Context, cancel context.CancelFunc, sessionID, rawContainerID string, cmd []string, rows, cols uint) {
	if len(cmd) == 0 {
		cmd = []string{"/bin/sh"}
	}
	containerID := normalizeEntityID(rawContainerID)

	if a.actionExecutor == nil {
		cancel()
		a.sendExecData(sessionID, nil, "action executor not available")
		return
	}

	sess, err := a.actionExecutor.DockerExec(ctx, containerID, cmd, rows, cols)
	if err != nil {
		cancel()
		a.sendExecData(sessionID, nil, fmt.Sprintf("docker exec failed: %v", err))
		return
	}

	a.execSessions.Store(sessionID, &execSession{dockerSess: sess, cancel: cancel})

	go func() {
		defer func() {
			sess.Attach.Close()
			a.execSessions.Delete(sessionID)
			a.sendExecEnd(sessionID)
		}()
		buf := make([]byte, 4096)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			n, err := sess.Attach.Reader.Read(buf)
			if n > 0 {
				a.sendExecData(sessionID, buf[:n], "")
			}
			if err != nil {
				if err != io.EOF {
					log.Printf("[exec] docker read error session=%s: %v", sessionID, err)
				}
				return
			}
		}
	}()
}

func (a *WSAgent) handleExecInput(data json.RawMessage) {
	var req struct {
		SessionID string `json:"session_id"`
		Data      string `json:"data"` // base64-encoded
	}
	if err := json.Unmarshal(data, &req); err != nil {
		return
	}
	val, ok := a.execSessions.Load(req.SessionID)
	if !ok {
		return
	}
	es := val.(*execSession)
	decoded, err := base64.StdEncoding.DecodeString(req.Data)
	if err != nil {
		return
	}
	if es.ptmx != nil {
		_, _ = es.ptmx.Write(decoded)
	} else if es.dockerSess != nil {
		_, _ = es.dockerSess.Attach.Conn.Write(decoded)
	}
}

func (a *WSAgent) handleExecResize(data json.RawMessage) {
	var req struct {
		SessionID string `json:"session_id"`
		Rows      uint   `json:"rows"`
		Cols      uint   `json:"cols"`
	}
	if err := json.Unmarshal(data, &req); err != nil {
		return
	}
	val, ok := a.execSessions.Load(req.SessionID)
	if !ok {
		return
	}
	es := val.(*execSession)
	if es.ptmx != nil {
		_ = pty.Setsize(es.ptmx, &pty.Winsize{Rows: uint16(req.Rows), Cols: uint16(req.Cols)})
	} else if es.dockerSess != nil && a.actionExecutor != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = a.actionExecutor.DockerExecResize(ctx, es.dockerSess.ExecID, req.Rows, req.Cols)
	}
}

func (a *WSAgent) handleExecEnd(data json.RawMessage) {
	var req struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(data, &req); err != nil {
		return
	}
	val, ok := a.execSessions.Load(req.SessionID)
	if !ok {
		return
	}
	es := val.(*execSession)
	es.cancel()
	if es.ptmx != nil {
		es.ptmx.Close()
	} else if es.dockerSess != nil {
		es.dockerSess.Attach.Close()
	}
	a.execSessions.Delete(req.SessionID)
}

func (a *WSAgent) sendExecData(sessionID string, data []byte, errMsg string) {
	payload := map[string]interface{}{
		"session_id": sessionID,
		"data":       base64.StdEncoding.EncodeToString(data),
		"error":      errMsg,
	}
	raw, err := marshalEnvelope("EXEC_DATA", payload)
	if err != nil {
		return
	}
	_ = a.sendRaw(raw)
}

func (a *WSAgent) sendExecEnd(sessionID string) {
	payload := map[string]interface{}{"session_id": sessionID}
	raw, err := marshalEnvelope("EXEC_END", payload)
	if err != nil {
		return
	}
	_ = a.sendRaw(raw)
}

// mapFrontendActionType maps the frontend action type string to an ActionType constant.
// Returns the ActionType and an optional layer override (empty = keep original).
func mapFrontendActionType(frontendType string) (actions.ActionType, string) {
	switch frontendType {
	case "docker_restart_container":
		return actions.ActionRestartContainer, "docker"
	case "docker_stop_container":
		return actions.ActionStopContainer, "docker"
	case "docker_start_container":
		return actions.ActionStartContainer, "docker"
	case "k8s_restart_deployment", "k8s_restart_statefulset", "k8s_restart_daemonset":
		return actions.ActionK8sRolloutRestart, "kubernetes"
	case "k8s_update_image":
		return actions.ActionK8sUpdateImage, "kubernetes"
	case "k8s_get_logs":
		return actions.ActionK8sGetLogs, "kubernetes"
	case "k8s_rollout_restart":
		return actions.ActionK8sRolloutRestart, "kubernetes"
	default:
		return actions.ActionType(frontendType), ""
	}
}

// normalizeEntityID strips type prefixes like "container/", "container:", "pod/", "deployment/" from entity IDs.
func normalizeEntityID(id string) string {
	if i := strings.Index(id, "/"); i >= 0 {
		return id[i+1:]
	}
	if i := strings.Index(id, ":"); i >= 0 {
		return id[i+1:]
	}
	return id
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
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
