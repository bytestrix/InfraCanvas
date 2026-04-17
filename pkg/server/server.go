package server

import (
	"encoding/json"
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
	MsgHello         = "HELLO"
	MsgGraphSnapshot = "GRAPH_SNAPSHOT"
	MsgGraphDiff     = "GRAPH_DIFF"
	MsgHeartbeat     = "HEARTBEAT"
	MsgActionResult  = "ACTION_RESULT"
	MsgActionProgress = "ACTION_PROGRESS"

	// Server → Agent
	MsgPairCode      = "PAIR_CODE"
	MsgPaired        = "PAIRED"
	MsgCommand       = "COMMAND"
	MsgActionRequest = "ACTION_REQUEST"

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

// Server is the InfraCanvas WebSocket relay server.
type Server struct {
	sessions       *SessionStore
	upgrader       websocket.Upgrader
	mux            *http.ServeMux
	token          string // shared secret; empty = auth disabled (dev mode)
	allowedOrigins map[string]bool
}

// New creates and configures a Server.
func New() *Server {
	token := os.Getenv("INFRACANVAS_TOKEN")
	if token == "" {
		log.Println("[WARN] INFRACANVAS_TOKEN is not set — auth disabled. Set it in production!")
	} else {
		log.Println("[INFO] Agent auth enabled via INFRACANVAS_TOKEN")
	}

	// Build allowed-origins map from INFRACANVAS_ALLOWED_ORIGINS (comma-separated).
	// Empty = allow all (dev mode).
	allowedOrigins := map[string]bool{}
	if raw := os.Getenv("INFRACANVAS_ALLOWED_ORIGINS"); raw != "" {
		for _, o := range strings.Split(raw, ",") {
			if o = strings.TrimSpace(o); o != "" {
				allowedOrigins[o] = true
			}
		}
		log.Printf("[INFO] Allowed origins: %v", raw)
	}

	s := &Server{
		sessions:       NewSessionStore(),
		token:          token,
		allowedOrigins: allowedOrigins,
	}
	s.upgrader = websocket.Upgrader{
		ReadBufferSize:  64 * 1024,
		WriteBufferSize: 64 * 1024,
		CheckOrigin: func(r *http.Request) bool {
			if len(s.allowedOrigins) == 0 {
				return true // dev mode
			}
			origin := r.Header.Get("Origin")
			return s.allowedOrigins[origin]
		},
	}
	s.mux = http.NewServeMux()
	s.mux.HandleFunc("/ws/agent", s.handleAgentWS)
	s.mux.HandleFunc("/ws/canvas", s.handleBrowserWS)
	s.mux.HandleFunc("/api/health", s.handleHealth)
	s.mux.HandleFunc("/api/sessions", s.requireToken(s.handleSessions))
	// Catch-all for debugging
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[debug] unhandled request: %s %s", r.Method, r.RequestURI)
		http.NotFound(w, r)
	})
	return s
}

// requireToken is middleware that checks the Authorization header for the shared token.
func (s *Server) requireToken(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.token != "" {
			auth := r.Header.Get("Authorization")
			if auth != "Bearer "+s.token {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		next(w, r)
	}
}

// checkAgentToken validates the Authorization header on a WebSocket upgrade request.
// Returns true if the connection should be allowed.
func (s *Server) checkAgentToken(r *http.Request) bool {
	if s.token == "" {
		return true // auth disabled
	}
	auth := r.Header.Get("Authorization")
	return auth == "Bearer "+s.token
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
	json.NewEncoder(w).Encode(map[string]interface{}{
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
	json.NewEncoder(w).Encode(info)
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

	// Immediately tell the agent its pair code.
	if err := writeMsg(conn, MsgPairCode, PairCodeData{Code: sess.PairCode}); err != nil {
		log.Printf("[agent] failed to send PAIR_CODE: %v", err)
		conn.Close()
		s.sessions.Delete(sess)
		return
	}

	defer func() {
		conn.Close()
		s.sessions.Delete(sess)
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
		// Forward action results to browsers
		broadcastToBrowsers(sess, raw)
		log.Printf("[agent] ACTION_RESULT  → %d browsers", sess.BrowserCount())

	case MsgActionProgress:
		// Forward action progress to browsers
		broadcastToBrowsers(sess, raw)

	case MsgHeartbeat:
		// Nothing to do — gorilla handles ping/pong separately.

	default:
		log.Printf("[agent] unknown message type: %s", env.Type)
	}
}

// ── Browser WebSocket handler ─────────────────────────────────────────────────

func (s *Server) handleBrowserWS(w http.ResponseWriter, r *http.Request) {
	raw, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("browser upgrade error: %v", err)
		return
	}
	conn := newSafeConn(raw)
	defer conn.Close()

	// First message from browser must be PAIR.
	_, payload, err := conn.ReadMessage()
	if err != nil {
		return
	}
	var env Envelope
	if err := json.Unmarshal(payload, &env); err != nil || env.Type != "PAIR" {
		writeMsg(conn, MsgError, map[string]string{"message": "first message must be PAIR"})
		return
	}
	var req PairRequest
	if err := json.Unmarshal(env.Data, &req); err != nil || req.Code == "" {
		writeMsg(conn, MsgError, map[string]string{"message": "missing pair code"})
		return
	}

	sess, ok := s.sessions.AddBrowser(req.Code, conn)
	if !ok {
		writeMsg(conn, MsgError, map[string]string{"message": "unknown pair code"})
		return
	}
	log.Printf("[browser] paired  session=%s  code=%s  browsers=%d",
		sess.ID, req.Code, sess.BrowserCount())

	// Notify agent it has a new viewer.
	go writeMsg(sess.AgentConn, MsgPaired, PairedData{BrowserCount: sess.BrowserCount()})

	// Replay the last cached snapshot so the browser doesn't wait for the next tick.
	sess.mu.RLock()
	lastSnap := sess.LastSnapshot
	sess.mu.RUnlock()
	if lastSnap != nil {
		conn.WriteMessage(websocket.TextMessage, lastSnap)
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
		if env.Type == MsgCommand || env.Type == MsgBrowserAction {
			sess.mu.RLock()
			agentConn := sess.AgentConn
			sess.mu.RUnlock()
			if agentConn != nil {
				// Forward to agent as ACTION_REQUEST
				if env.Type == MsgBrowserAction {
					env.Type = MsgActionRequest
					payload, _ = json.Marshal(env)
				}
				agentConn.WriteMessage(websocket.TextMessage, payload)
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
		go c.WriteMessage(websocket.TextMessage, msg)
	}
}
