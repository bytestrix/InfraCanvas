package server

import (
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Message type constants — shared with agent and browser clients.
const (
	// Agent → Server
	MsgHello          = "HELLO"
	MsgGraphSnapshot  = "GRAPH_SNAPSHOT"
	MsgGraphDiff      = "GRAPH_DIFF"
	MsgHeartbeat      = "HEARTBEAT"
	MsgActionResult   = "ACTION_RESULT"
	MsgActionProgress = "ACTION_PROGRESS"
	MsgLogData        = "LOG_DATA"
	MsgExecData       = "EXEC_DATA"
	MsgExecEnd        = "EXEC_END"

	// Server → Agent (and Browser → Server → Agent)
	MsgPairCode      = "PAIR_CODE"
	MsgPaired        = "PAIRED"
	MsgCommand       = "COMMAND"
	MsgActionRequest = "ACTION_REQUEST"
	MsgExecStart     = "EXEC_START"
	MsgExecInput     = "EXEC_INPUT"
	MsgExecResize    = "EXEC_RESIZE"

	// Browser → Server
	MsgBrowserAction = "BROWSER_ACTION"

	// Server → Browser
	MsgAgentConnected    = "AGENT_CONNECTED"
	MsgAgentDisconnected = "AGENT_DISCONNECTED"
	MsgError             = "ERROR"
)

// SafeConn wraps a websocket.Conn with a write mutex.
// gorilla/websocket allows concurrent reads but not concurrent writes.
type SafeConn struct {
	conn *websocket.Conn
	wmu  sync.Mutex
}

func newSafeConn(c *websocket.Conn) *SafeConn {
	return &SafeConn{conn: c}
}

func (sc *SafeConn) WriteJSON(v interface{}) error {
	sc.wmu.Lock()
	defer sc.wmu.Unlock()
	return sc.conn.WriteJSON(v)
}

func (sc *SafeConn) WriteMessage(t int, data []byte) error {
	sc.wmu.Lock()
	defer sc.wmu.Unlock()
	return sc.conn.WriteMessage(t, data)
}

func (sc *SafeConn) ReadMessage() (int, []byte, error) {
	return sc.conn.ReadMessage()
}

func (sc *SafeConn) Close() error {
	return sc.conn.Close()
}

// Envelope is the wire format for all WebSocket messages.
type Envelope struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

// HelloData is sent by the agent on connect.
type HelloData struct {
	Hostname string   `json:"hostname"`
	Scope    []string `json:"scope"`
	Version  string   `json:"version"`
}

// PairCodeData is sent to the agent after connection.
type PairCodeData struct {
	Code string `json:"code"`
}

// PairedData is sent to the agent when a browser pairs.
type PairedData struct {
	BrowserCount int `json:"browserCount"`
}

// AgentConnectedData is broadcast to browsers when agent info is known.
type AgentConnectedData struct {
	Hostname string   `json:"hostname"`
	Scope    []string `json:"scope"`
}

// PairRequest is the first message browsers send after connecting.
type PairRequest struct {
	Code string `json:"code"`
}

// Options configures a Server.
type Options struct {
	// AgentToken is the shared secret an agent presents on /ws/agent
	// (Authorization: Bearer <token>). Empty disables agent auth.
	AgentToken string

	// UIToken gates browser access to /ws/canvas and the static UI.
	// Empty disables UI auth (only safe when bound to loopback).
	UIToken string

	// LocalMode bypasses pair-code lookup: the first agent connection
	// becomes the implicit local session and any browser connecting to
	// /ws/canvas auto-binds to it. Used by `infracanvas serve`.
	LocalMode bool

	// AllowedOrigins, if non-empty, restricts CORS for WS upgrades.
	AllowedOrigins []string
}

// Server is the InfraCanvas WebSocket relay server.
type Server struct {
	sessions       *SessionStore
	upgrader       websocket.Upgrader
	mux            *http.ServeMux
	agentToken     string
	uiToken        string
	localMode      bool
	allowedOrigins map[string]bool

	localMu      sync.RWMutex
	localSession *Session
}

// New creates a Server using environment-based config (legacy SaaS-style).
func New() *Server {
	opts := Options{
		AgentToken: os.Getenv("INFRACANVAS_TOKEN"),
	}
	if raw := os.Getenv("INFRACANVAS_ALLOWED_ORIGINS"); raw != "" {
		for _, o := range strings.Split(raw, ",") {
			if o = strings.TrimSpace(o); o != "" {
				opts.AllowedOrigins = append(opts.AllowedOrigins, o)
			}
		}
	}
	if opts.AgentToken == "" {
		log.Println("[WARN] INFRACANVAS_TOKEN is not set — agent auth disabled.")
	}
	return NewWithOptions(opts)
}

// NewLocal creates a Server in local-mode for `infracanvas serve`:
// auto-pairing, UI gated by a token, no agent auth (the agent is in-process).
func NewLocal(uiToken string) *Server {
	return NewWithOptions(Options{
		LocalMode: true,
		UIToken:   uiToken,
	})
}

// NewWithOptions creates a Server with explicit configuration.
func NewWithOptions(opts Options) *Server {
	s := &Server{
		sessions:       NewSessionStore(),
		agentToken:     opts.AgentToken,
		uiToken:        opts.UIToken,
		localMode:      opts.LocalMode,
		allowedOrigins: map[string]bool{},
	}
	for _, o := range opts.AllowedOrigins {
		s.allowedOrigins[o] = true
	}
	s.upgrader = websocket.Upgrader{
		ReadBufferSize:  64 * 1024,
		WriteBufferSize: 64 * 1024,
		CheckOrigin: func(r *http.Request) bool {
			if len(s.allowedOrigins) == 0 {
				return true
			}
			origin := r.Header.Get("Origin")
			return s.allowedOrigins[origin]
		},
	}
	s.mux = http.NewServeMux()
	s.mux.HandleFunc("/ws/agent", s.handleAgentWS)
	s.mux.HandleFunc("/ws/canvas", s.handleBrowserWS)
	s.mux.HandleFunc("/api/health", s.handleHealth)
	s.mux.HandleFunc("/api/sessions", s.requireAgentToken(s.handleSessions))
	return s
}

// MountUI serves the embedded dashboard at /, gated by the UI token.
// Requests with ?token=<UIToken> set a cookie and redirect to a clean URL.
// Subsequent requests use the cookie.
func (s *Server) MountUI(fsys fs.FS) {
	fileServer := http.FileServer(http.FS(fsys))
	s.mux.Handle("/", s.requireUIAuth(fileServer))
}

// requireAgentToken protects API routes with the agent shared secret.
func (s *Server) requireAgentToken(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.agentToken != "" {
			auth := r.Header.Get("Authorization")
			if auth != "Bearer "+s.agentToken {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		next(w, r)
	}
}

// checkAgentToken validates the Authorization header on /ws/agent.
func (s *Server) checkAgentToken(r *http.Request) bool {
	if s.agentToken == "" {
		return true
	}
	auth := r.Header.Get("Authorization")
	return auth == "Bearer "+s.agentToken
}

// checkUIToken accepts the token from the cookie or ?token= query param.
func (s *Server) checkUIToken(r *http.Request) bool {
	if s.uiToken == "" {
		return true
	}
	if c, err := r.Cookie("infracanvas_token"); err == nil && c.Value == s.uiToken {
		return true
	}
	return r.URL.Query().Get("token") == s.uiToken
}

// requireUIAuth gates static-UI requests. On first visit with ?token=…
// it sets a cookie and redirects to a clean URL; thereafter the cookie carries auth.
// Missing/invalid auth returns a small HTML page asking for the token.
func (s *Server) requireUIAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.uiToken == "" {
			next.ServeHTTP(w, r)
			return
		}
		if q := r.URL.Query().Get("token"); q != "" {
			if q == s.uiToken {
				http.SetCookie(w, &http.Cookie{
					Name:     "infracanvas_token",
					Value:    q,
					Path:     "/",
					HttpOnly: true,
					SameSite: http.SameSiteStrictMode,
					MaxAge:   60 * 60 * 24 * 30,
				})
				u := *r.URL
				qq := u.Query()
				qq.Del("token")
				u.RawQuery = qq.Encode()
				http.Redirect(w, r, u.RequestURI(), http.StatusSeeOther)
				return
			}
			s.writeUnauthorizedHTML(w, "Invalid token.")
			return
		}
		if c, err := r.Cookie("infracanvas_token"); err == nil && c.Value == s.uiToken {
			next.ServeHTTP(w, r)
			return
		}
		s.writeUnauthorizedHTML(w, "Auth token required.")
	})
}

func (s *Server) writeUnauthorizedHTML(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`<!doctype html><html><head><meta charset="utf-8"><title>InfraCanvas — Auth required</title>` +
		`<style>body{background:#08080E;color:#EEE8FF;font-family:system-ui,-apple-system,sans-serif;min-height:100vh;margin:0;display:flex;align-items:center;justify-content:center}` +
		`.card{max-width:440px;padding:32px;text-align:center}` +
		`h1{margin:0 0 12px;font-size:22px;font-weight:600;letter-spacing:-.3px}` +
		`p{margin:0 0 12px;color:#8B82B0;font-size:14px;line-height:1.6}` +
		`code{background:#0E0E1C;border:1px solid rgba(138,92,246,.18);padding:2px 8px;border-radius:6px;font-family:ui-monospace,monospace;color:#C026D3;font-size:13px}</style></head>` +
		`<body><div class="card"><h1>InfraCanvas</h1><p>` + msg +
		`</p><p>Append <code>?token=&lt;your-token&gt;</code> to the URL — the token was printed when InfraCanvas started.</p></div></body></html>`))
}

// Handler returns the HTTP handler (useful for testing or custom listeners).
func (s *Server) Handler() http.Handler {
	return s.mux
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe(addr string) error {
	log.Printf("InfraCanvas server listening on %s", addr)
	log.Printf("  Agent endpoint:   ws://%s/ws/agent", addr)
	log.Printf("  Canvas endpoint:  ws://%s/ws/canvas", addr)
	log.Printf("  Health:           http://%s/api/health", addr)
	return http.ListenAndServe(addr, s.mux)
}

// ── HTTP endpoints ────────────────────────────────────────────────────────────

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "ok",
		"sessions": s.sessions.ActiveCount(),
		"time":     time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	s.sessions.mu.RLock()
	info := make([]map[string]interface{}, 0, len(s.sessions.byCode))
	for code, sess := range s.sessions.byCode {
		sess.mu.RLock()
		info = append(info, map[string]interface{}{
			"code":         code,
			"id":           sess.ID,
			"hostname":     sess.Hostname,
			"scope":        sess.Scope,
			"browserCount": len(sess.Browsers),
			"paired":       !sess.PairedAt.IsZero(),
			"pairedAt":     sess.PairedAt,
		})
		sess.mu.RUnlock()
	}
	s.sessions.mu.RUnlock()
	_ = json.NewEncoder(w).Encode(info)
}

// ── Agent WebSocket handler ───────────────────────────────────────────────────

func (s *Server) handleAgentWS(w http.ResponseWriter, r *http.Request) {
	if !s.checkAgentToken(r) {
		log.Printf("[agent] rejected unauthorized connection from %s", r.RemoteAddr)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	log.Printf("[agent] upgrade attempt from %s", r.RemoteAddr)
	raw, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("agent upgrade error: %v (remote: %s)", err, r.RemoteAddr)
		return
	}
	conn := newSafeConn(raw)

	sess := s.sessions.Create(conn)
	log.Printf("[agent] connected  session=%s  code=%s", sess.ID, sess.PairCode)

	// In local mode the first agent becomes the implicit local session;
	// browsers auto-bind to it without a pair code.
	if s.localMode {
		s.localMu.Lock()
		s.localSession = sess
		s.localMu.Unlock()
	}

	// Send the pair code (still useful in shared-relay mode; harmless locally).
	if err := writeMsg(conn, MsgPairCode, PairCodeData{Code: sess.PairCode}); err != nil {
		log.Printf("[agent] failed to send PAIR_CODE: %v", err)
		conn.Close()
		s.sessions.Delete(sess)
		return
	}

	defer func() {
		conn.Close()
		s.sessions.Delete(sess)
		if s.localMode {
			s.localMu.Lock()
			if s.localSession == sess {
				s.localSession = nil
			}
			s.localMu.Unlock()
		}
		log.Printf("[agent] disconnected  session=%s", sess.ID)
		// Notify all paired browsers.
		msg := mustMarshalEnvelope(MsgAgentDisconnected, struct{}{})
		broadcastToBrowsers(sess, msg)
	}()

	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var env Envelope
		if err := json.Unmarshal(payload, &env); err != nil {
			continue
		}
		s.routeAgentMessage(sess, env, payload)
	}
}

func (s *Server) routeAgentMessage(sess *Session, env Envelope, raw []byte) {
	switch env.Type {
	case MsgHello:
		var hello HelloData
		if err := json.Unmarshal(env.Data, &hello); err != nil {
			return
		}
		sess.mu.Lock()
		sess.Hostname = hello.Hostname
		sess.Scope = hello.Scope
		sess.mu.Unlock()
		log.Printf("[agent] HELLO  host=%s  scope=%v", hello.Hostname, hello.Scope)

		// Tell already-connected browsers about this agent.
		msg := mustMarshalEnvelope(MsgAgentConnected, AgentConnectedData{
			Hostname: hello.Hostname,
			Scope:    hello.Scope,
		})
		broadcastToBrowsers(sess, msg)

	case MsgGraphSnapshot:
		// Cache for late joiners, then relay.
		sess.mu.Lock()
		sess.LastSnapshot = make([]byte, len(raw))
		copy(sess.LastSnapshot, raw)
		sess.mu.Unlock()

		broadcastToBrowsers(sess, raw)
		log.Printf("[agent] GRAPH_SNAPSHOT  %d bytes  → %d browsers", len(raw), sess.BrowserCount())

	case MsgGraphDiff:
		broadcastToBrowsers(sess, raw)
		log.Printf("[agent] GRAPH_DIFF  → %d browsers", sess.BrowserCount())

	case MsgActionResult:
		broadcastToBrowsers(sess, raw)
		log.Printf("[agent] ACTION_RESULT  → %d browsers", sess.BrowserCount())

	case MsgActionProgress:
		broadcastToBrowsers(sess, raw)

	case MsgLogData:
		// Forward streaming log data to browsers
		broadcastToBrowsers(sess, raw)

	case MsgExecData:
		// Forward exec output to browsers
		broadcastToBrowsers(sess, raw)

	case MsgExecEnd:
		// Forward exec session end notification to browsers
		broadcastToBrowsers(sess, raw)

	case MsgHeartbeat:
		// Nothing to do — gorilla handles ping/pong separately.

	default:
		log.Printf("[agent] unknown message type: %s", env.Type)
	}
}

// ── Browser WebSocket handler ─────────────────────────────────────────────────

func (s *Server) handleBrowserWS(w http.ResponseWriter, r *http.Request) {
	if !s.checkUIToken(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	raw, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("browser upgrade error: %v", err)
		return
	}
	conn := newSafeConn(raw)
	defer conn.Close()

	var sess *Session

	if s.localMode {
		// Auto-pair: bind to the local session without requiring a PAIR message.
		// If the agent hasn't connected yet, wait briefly for it.
		for i := 0; i < 50; i++ { // up to ~5s
			s.localMu.RLock()
			sess = s.localSession
			s.localMu.RUnlock()
			if sess != nil {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		if sess == nil {
			_ = writeMsg(conn, MsgError, map[string]string{"message": "agent not yet ready — refresh in a moment"})
			return
		}
		s.sessions.mu.Lock()
		sess.mu.Lock()
		sess.Browsers = append(sess.Browsers, conn)
		if sess.PairedAt.IsZero() {
			sess.PairedAt = time.Now()
		}
		sess.mu.Unlock()
		s.sessions.mu.Unlock()
		log.Printf("[browser] auto-paired (local)  session=%s  browsers=%d", sess.ID, sess.BrowserCount())
	} else {
		// Shared-relay mode: first message must be PAIR with a code.
		_, payload, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var env Envelope
		if err := json.Unmarshal(payload, &env); err != nil || env.Type != "PAIR" {
			_ = writeMsg(conn, MsgError, map[string]string{"message": "first message must be PAIR"})
			return
		}
		var req PairRequest
		if err := json.Unmarshal(env.Data, &req); err != nil || req.Code == "" {
			_ = writeMsg(conn, MsgError, map[string]string{"message": "missing pair code"})
			return
		}
		var ok bool
		sess, ok = s.sessions.AddBrowser(req.Code, conn)
		if !ok {
			_ = writeMsg(conn, MsgError, map[string]string{"message": "unknown pair code"})
			return
		}
		log.Printf("[browser] paired  session=%s  code=%s  browsers=%d",
			sess.ID, req.Code, sess.BrowserCount())
	}

	// Notify agent it has a new viewer.
	go func() { _ = writeMsg(sess.AgentConn, MsgPaired, PairedData{BrowserCount: sess.BrowserCount()}) }()

	// Replay the last cached snapshot so the browser doesn't wait for the next tick.
	sess.mu.RLock()
	lastSnap := sess.LastSnapshot
	sess.mu.RUnlock()
	if lastSnap != nil {
		_ = conn.WriteMessage(websocket.TextMessage, lastSnap)
	}

	defer func() {
		s.sessions.RemoveBrowser(sess, conn)
		log.Printf("[browser] disconnected  session=%s  browsers=%d",
			sess.ID, sess.BrowserCount())
	}()

	// Read loop: forward COMMAND and BROWSER_ACTION messages to the agent.
	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var env Envelope
		if err := json.Unmarshal(payload, &env); err != nil {
			continue
		}
		switch env.Type {
		case MsgBrowserAction:
			// Translate BROWSER_ACTION → ACTION_REQUEST before forwarding
			env.Type = MsgActionRequest
			payload, _ = json.Marshal(env)
			fallthrough
		case MsgCommand, MsgExecStart, MsgExecInput, MsgExecResize, MsgExecEnd:
			sess.mu.RLock()
			agentConn := sess.AgentConn
			sess.mu.RUnlock()
			if agentConn != nil {
				_ = agentConn.WriteMessage(websocket.TextMessage, payload)
			}
		}
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

func writeMsg(conn *SafeConn, msgType string, data interface{}) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return conn.WriteJSON(Envelope{Type: msgType, Data: payload})
}

func mustMarshalEnvelope(msgType string, data interface{}) []byte {
	payload, _ := json.Marshal(data)
	out, _ := json.Marshal(Envelope{Type: msgType, Data: payload})
	return out
}

func broadcastToBrowsers(sess *Session, msg []byte) {
	sess.mu.RLock()
	browsers := make([]*SafeConn, len(sess.Browsers))
	copy(browsers, sess.Browsers)
	sess.mu.RUnlock()

	for _, c := range browsers {
		go func(c *SafeConn) { _ = c.WriteMessage(websocket.TextMessage, msg) }(c)
	}
}
