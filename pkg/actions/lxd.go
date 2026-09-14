package actions

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/gorilla/websocket"

	lxddiscovery "infracanvas/pkg/discovery/lxd"
)

// LXDExecutor handles actions and interactive exec sessions for LXD/Incus instances.
type LXDExecutor struct {
	client       *http.Client
	socketPath   string
	controlConns map[string]*websocket.Conn // execID -> control websocket
	mu           sync.Mutex
}

type lxdResponse struct {
	Type       string          `json:"type"`
	StatusCode int             `json:"status_code"`
	Error      string          `json:"error"`
	Operation  string          `json:"operation"`
	Metadata   json.RawMessage `json:"metadata"`
}

type lxdExecMetadata struct {
	FDS map[string]string `json:"fds"`
}

// lxdWSStreamConn adapts a websocket to net.Conn-like stream semantics.
type lxdWSStreamConn struct {
	ws *websocket.Conn
	r  io.Reader
	wm sync.Mutex
}

func (c *lxdWSStreamConn) Read(p []byte) (int, error) {
	for {
		if c.r == nil {
			msgType, r, err := c.ws.NextReader()
			if err != nil {
				return 0, err
			}
			if msgType != websocket.TextMessage && msgType != websocket.BinaryMessage {
				continue
			}
			c.r = r
		}
		n, err := c.r.Read(p)
		if err == io.EOF {
			c.r = nil
			if n > 0 {
				return n, nil
			}
			continue
		}
		return n, err
	}
}

func (c *lxdWSStreamConn) Write(p []byte) (int, error) {
	c.wm.Lock()
	defer c.wm.Unlock()
	if err := c.ws.WriteMessage(websocket.BinaryMessage, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (c *lxdWSStreamConn) Close() error         { return c.ws.Close() }
func (c *lxdWSStreamConn) LocalAddr() net.Addr  { return c.ws.UnderlyingConn().LocalAddr() }
func (c *lxdWSStreamConn) RemoteAddr() net.Addr { return c.ws.UnderlyingConn().RemoteAddr() }
func (c *lxdWSStreamConn) SetDeadline(t time.Time) error {
	_ = c.ws.SetReadDeadline(t)
	return c.ws.SetWriteDeadline(t)
}
func (c *lxdWSStreamConn) SetReadDeadline(t time.Time) error  { return c.ws.SetReadDeadline(t) }
func (c *lxdWSStreamConn) SetWriteDeadline(t time.Time) error { return c.ws.SetWriteDeadline(t) }

// NewLXDExecutor creates a new LXD executor when the local daemon socket is available.
func NewLXDExecutor() (*LXDExecutor, error) {
	d := lxddiscovery.NewDiscovery()
	if !d.IsAvailable() {
		return nil, fmt.Errorf("lxd/incus is not available")
	}
	cli := d.HTTPClient()
	sock := d.SocketPath()
	if cli == nil || sock == "" {
		return nil, fmt.Errorf("lxd/incus client is not available")
	}
	return &LXDExecutor{
		client:       cli,
		socketPath:   sock,
		controlConns: make(map[string]*websocket.Conn),
	}, nil
}

// ValidateAction validates an LXD action.
func (l *LXDExecutor) ValidateAction(action *Action) error {
	if action.Target.EntityID == "" {
		return fmt.Errorf("entity ID is required")
	}
	return nil
}

// ExecuteAction executes an LXD action.
func (l *LXDExecutor) ExecuteAction(ctx context.Context, action *Action) (*ActionResult, error) {
	startTime := time.Now()
	id := action.Target.EntityID

	switch action.Type {
	case ActionRestartContainer:
		return l.changeState(ctx, id, "restart", startTime)
	case ActionStopContainer:
		return l.changeState(ctx, id, "stop", startTime)
	case ActionStartContainer:
		return l.changeState(ctx, id, "start", startTime)
	default:
		return &ActionResult{
			Success:   false,
			Message:   "Unsupported action type",
			Error:     fmt.Sprintf("unsupported lxd action type: %s", action.Type),
			StartTime: startTime,
			EndTime:   time.Now(),
		}, fmt.Errorf("unsupported lxd action type: %s", action.Type)
	}
}

// ExecCreate creates an interactive exec session and attaches to its data websocket.
func (l *LXDExecutor) ExecCreate(ctx context.Context, containerID string, cmd []string) (*ExecSession, error) {
	if len(cmd) == 0 {
		cmd = []string{"/bin/sh"}
	}

	var resp lxdResponse
	if err := l.doJSON(ctx, http.MethodPost, "/1.0/instances/"+url.PathEscape(containerID)+"/exec", map[string]interface{}{
		"command":            cmd,
		"interactive":        true,
		"wait-for-websocket": true,
		"environment":        map[string]string{"TERM": "xterm-256color", "COLORTERM": "truecolor"},
	}, &resp); err != nil {
		return nil, fmt.Errorf("exec create: %w", err)
	}
	if resp.Operation == "" {
		return nil, fmt.Errorf("exec create: missing operation id")
	}

	var meta lxdExecMetadata
	if err := json.Unmarshal(resp.Metadata, &meta); err != nil {
		return nil, fmt.Errorf("exec create: decode metadata: %w", err)
	}
	dataSecret, controlSecret := lxdExecSecrets(meta.FDS)
	if dataSecret == "" {
		return nil, fmt.Errorf("exec create: missing websocket secret")
	}

	execID := lxdOperationID(resp.Operation)

	dataWS, err := l.dialOpWebsocket(ctx, resp.Operation, dataSecret)
	if err != nil {
		return nil, fmt.Errorf("exec attach: %w", err)
	}

	if controlSecret != "" {
		controlWS, err := l.dialOpWebsocket(ctx, resp.Operation, controlSecret)
		if err != nil {
			_ = dataWS.Close()
			return nil, fmt.Errorf("exec control attach: %w", err)
		}
		l.mu.Lock()
		l.controlConns[execID] = controlWS
		l.mu.Unlock()
	}

	streamConn := &lxdWSStreamConn{ws: dataWS}
	attach := dockertypes.NewHijackedResponse(streamConn, "application/vnd.docker.raw-stream")
	return &ExecSession{ExecID: execID, Attach: attach}, nil
}

// ExecResize resizes the terminal for an active LXD exec session.
func (l *LXDExecutor) ExecResize(_ context.Context, execID string, rows, cols uint) error {
	if execID == "" {
		return fmt.Errorf("exec id is required")
	}
	l.mu.Lock()
	controlWS := l.controlConns[execID]
	if controlWS == nil {
		l.mu.Unlock()
		return fmt.Errorf("exec control channel not available")
	}
	msg := map[string]interface{}{
		"command": "window-resize",
		"args": map[string]string{
			"width":  strconv.FormatUint(uint64(cols), 10),
			"height": strconv.FormatUint(uint64(rows), 10),
		},
	}
	err := controlWS.WriteJSON(msg)
	l.mu.Unlock()
	return err
}

// CloseExec closes and forgets control channel resources for an LXD exec session.
func (l *LXDExecutor) CloseExec(execID string) {
	l.mu.Lock()
	controlWS := l.controlConns[execID]
	delete(l.controlConns, execID)
	l.mu.Unlock()
	if controlWS != nil {
		_ = controlWS.Close()
	}
}

func (l *LXDExecutor) changeState(ctx context.Context, containerID, action string, startTime time.Time) (*ActionResult, error) {
	verbPast := map[string]string{
		"start":   "started",
		"stop":    "stopped",
		"restart": "restarted",
	}[action]
	if verbPast == "" {
		verbPast = action
	}
	var resp lxdResponse
	err := l.doJSON(ctx, http.MethodPut, "/1.0/instances/"+url.PathEscape(containerID)+"/state", map[string]interface{}{
		"action":  action,
		"timeout": 30,
		"force":   action != "start",
	}, &resp)
	if err != nil {
		return &ActionResult{
			Success:   false,
			Message:   fmt.Sprintf("Failed to %s container %s", action, containerID),
			Error:     err.Error(),
			StartTime: startTime,
			EndTime:   time.Now(),
		}, err
	}
	if resp.Operation != "" {
		if err := l.waitOperation(ctx, resp.Operation, 30); err != nil {
			return &ActionResult{
				Success:   false,
				Message:   fmt.Sprintf("Failed to %s container %s", action, containerID),
				Error:     err.Error(),
				StartTime: startTime,
				EndTime:   time.Now(),
			}, err
		}
	}
	return &ActionResult{
		Success:   true,
		Message:   fmt.Sprintf("Successfully %s container %s", verbPast, containerID),
		StartTime: startTime,
		EndTime:   time.Now(),
	}, nil
}

func (l *LXDExecutor) waitOperation(ctx context.Context, opPath string, timeoutSeconds int) error {
	var resp lxdResponse
	waitPath := strings.TrimRight(opPath, "/") + "/wait?timeout=" + strconv.Itoa(timeoutSeconds)
	if err := l.doJSON(ctx, http.MethodGet, waitPath, nil, &resp); err != nil {
		return err
	}
	if resp.Error != "" {
		return errors.New(resp.Error)
	}
	return nil
}

func (l *LXDExecutor) doJSON(ctx context.Context, method, apiPath string, body interface{}, out interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://lxd"+apiPath, bodyReader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := l.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if len(b) == 0 {
			return fmt.Errorf("status %d", resp.StatusCode)
		}
		return fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return err
		}
	}
	var envelope lxdResponse
	if len(respBody) > 0 && json.Unmarshal(respBody, &envelope) == nil {
		if envelope.Type == "error" || envelope.Error != "" || envelope.StatusCode >= 400 {
			if envelope.Error != "" {
				return errors.New(envelope.Error)
			}
			return fmt.Errorf("lxd api %s (status_code=%d)", envelope.Type, envelope.StatusCode)
		}
	}
	return nil
}

func (l *LXDExecutor) dialOpWebsocket(ctx context.Context, opPath, secret string) (*websocket.Conn, error) {
	wsURL := "ws://lxd" + strings.TrimRight(opPath, "/") + "/websocket?secret=" + url.QueryEscape(secret)
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		NetDialContext: func(dialCtx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(dialCtx, "unix", l.socketPath)
		},
	}
	conn, _, err := dialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func lxdOperationID(opPath string) string {
	return path.Base(strings.TrimRight(opPath, "/"))
}

func lxdExecSecrets(fds map[string]string) (dataSecret, controlSecret string) {
	if fds == nil {
		return "", ""
	}
	dataSecret = fds["0"]
	if dataSecret == "" {
		dataSecret = fds["1"]
	}
	return dataSecret, fds["control"]
}
