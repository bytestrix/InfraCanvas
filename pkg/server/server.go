package server

import (
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"infracanvas/pkg/audit"
	"infracanvas/pkg/clustermgr"
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

// SafeConn wraps a websocket.Conn with an ordered, single-writer outbound
// queue. gorilla/websocket allows concurrent reads but not concurrent
// writes, and a mutex alone only guarantees mutual exclusion — it does not
// guarantee that writes land in the order callers issued them, since
// multiple goroutines racing for the lock can be resumed in any order.
// Terminal/log/diff streams are order-sensitive, so every write is enqueued
// here and drained by a single dedicated goroutine per connection, which
// guarantees FIFO delivery regardless of how many goroutines call
// WriteMessage/WriteJSON concurrently.
type SafeConn struct {
	conn      *websocket.Conn
	wmu       sync.Mutex // guards the real conn write call itself
	outbox    chan wsFrame
	closed    chan struct{}
	closeOnce sync.Once
}

type wsFrame struct {
	msgType int
	data    []byte
	json    interface{}
}

func newSafeConn(c *websocket.Conn) *SafeConn {
	sc := &SafeConn{
		conn:   c,
		outbox: make(chan wsFrame, 256),
		closed: make(chan struct{}),
	}
	go sc.writeLoop()
	return sc
}

func (sc *SafeConn) writeLoop() {
	for {
		select {
		case f := <-sc.outbox:
			sc.wmu.Lock()
			if f.json != nil {
				_ = sc.conn.WriteJSON(f.json)
			} else {
				_ = sc.conn.WriteMessage(f.msgType, f.data)
			}
			sc.wmu.Unlock()
		case <-sc.closed:
			return
		}
	}
}

// enqueue is non-blocking so a slow/stalled connection can never stall the
// caller (e.g. the agent's single message-read loop, which calls
// broadcastToBrowsers for every fan-out target in sequence — blocking on
// one browser would delay delivery to every other browser on the same
// session). If the outbox is saturated, the connection is behind and gets
// closed rather than silently dropping or reordering messages.
func (sc *SafeConn) enqueue(f wsFrame) error {
	select {
	case sc.outbox <- f:
		return nil
	case <-sc.closed:
		return fmt.Errorf("connection closed")
	default:
		go sc.Close()
		return fmt.Errorf("write queue full, closing slow connection")
	}
}

func (sc *SafeConn) WriteJSON(v interface{}) error {
	return sc.enqueue(wsFrame{json: v})
}

func (sc *SafeConn) WriteMessage(t int, data []byte) error {
	return sc.enqueue(wsFrame{msgType: t, data: data})
}

func (sc *SafeConn) ReadMessage() (int, []byte, error) {
	return sc.conn.ReadMessage()
}

func (sc *SafeConn) Close() error {
	sc.closeOnce.Do(func() { close(sc.closed) })
	return sc.conn.Close()
}

// Envelope is the wire format for all WebSocket messages.
type Envelope struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

// HelloData is sent by the agent on connect.
type HelloData struct {
	Hostname  string   `json:"hostname"`
	Scope     []string `json:"scope"`
	Version   string   `json:"version"`
	MachineID string   `json:"machineId,omitempty"`
	// ResumeSecret proves this connection actually owns MachineID — required
	// because the hub join token is shared across every VM, so it alone
	// can't be used to tell agents apart. See Session.ResumeSecret.
	ResumeSecret string `json:"resumeSecret,omitempty"`
}

// PairCodeData is sent to the agent after connection.
type PairCodeData struct {
	Code string `json:"code"`
	// ResumeSecret is set only the first time a MachineID connects — the
	// agent must persist it locally and present it on every future HELLO
	// claiming this MachineID.
	ResumeSecret string `json:"resumeSecret,omitempty"`
}

// PairedData is sent to the agent when a browser pairs.
type PairedData struct {
	BrowserCount int `json:"browserCount"`
}

// AgentConnectedData is broadcast to browsers when agent info is known.
type AgentConnectedData struct {
	Hostname string   `json:"hostname"`
	Scope    []string `json:"scope"`
	ReadOnly bool     `json:"readOnly,omitempty"`
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

	// LocalMachineID pins the implicit local session to the in-process
	// agent's machine ID, so a remote agent joining the hub can never
	// displace the local canvas. Empty falls back to first-agent-wins.
	LocalMachineID string

	// ReadOnly blocks all mutating browser messages at the relay: actions
	// and exec/terminal sessions are rejected before they reach the agent.
	// Viewing the graph and fetching logs still work. Used for public demos.
	ReadOnly bool

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
	localMachineID string
	readOnly       bool
	allowedOrigins map[string]bool

	localMu      sync.RWMutex
	localSession *Session

	joinMu     sync.RWMutex
	joinURL    string // reachable address other VMs should dial; "" hides Add machine
	joinCaveat string // human note, e.g. quick-tunnel URLs change on restart

	clusterMgr *clustermgr.Manager // nil until SetClusterManager is called (local-mode only)

	authLimiter *authRateLimiter
}

// authRateLimiter is a minimal per-IP sliding-window lockout for failed auth
// attempts against the UI/agent token and resume-secret checks. Token/secret
// entropy already makes brute force impractical over a real network, but
// there's otherwise zero defense-in-depth against a fast local/LAN attacker
// with unlimited guesses — this closes that gap cheaply.
type authRateLimiter struct {
	mu       sync.Mutex
	failures map[string][]time.Time
}

func newAuthRateLimiter() *authRateLimiter {
	return &authRateLimiter{failures: make(map[string][]time.Time)}
}

const (
	authRateLimitWindow      = time.Minute
	authRateLimitMaxFailures = 20
)

// allow reports whether ip is currently permitted to attempt auth. Call
// recordFailure after every failed attempt, not on success.
func (l *authRateLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := time.Now().Add(-authRateLimitWindow)
	recent := l.failures[ip][:0]
	for _, t := range l.failures[ip] {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}
	l.failures[ip] = recent
	return len(recent) < authRateLimitMaxFailures
}

func (l *authRateLimiter) recordFailure(ip, reason string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.failures[ip] = append(l.failures[ip], time.Now())
	log.Printf("[auth] failed attempt from %s: %s", ip, reason)
}

// clientIP extracts the request's remote IP without the port, for rate-limit
// bucketing. Not used for any security decision beyond throttling.
// clientIP extracts the request's real IP for rate-limit bucketing. In the
// default deployment mode (bundled cloudflared quick-tunnel), every request
// arrives at this process via the local cloudflared connector, so
// RemoteAddr is always 127.0.0.1 for every real visitor — bucketing the
// rate limiter on that alone would lump every tunnel visitor into one
// shared quota, letting anyone with the (documented "unguessable but not
// secret") tunnel URL lock out the legitimate operator. Only when the
// immediate peer is loopback (i.e. genuinely our own bundled cloudflared,
// not an arbitrary client) do we trust the header it sets.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	// Cf-Connecting-Ip is set by Cloudflare's actual edge network on every
	// real cloudflared quick-tunnel connection — this is a genuine external
	// signal, not something the bundled cloudflared process fabricates or
	// passes through unchecked. X-Forwarded-For is deliberately NOT used as
	// a fallback here: for an arbitrary third-party reverse proxy a
	// self-hoster might put in front instead, there's no guarantee it
	// appends rather than passes the client's own value through untouched,
	// and taking "the first entry" (or any fixed position) in an unknown
	// proxy's XFF chain can just be reading back what the client sent.
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		if cf := r.Header.Get("Cf-Connecting-Ip"); cf != "" && net.ParseIP(cf) != nil {
			return cf
		}
	}
	return host
}

// SetClusterManager wires up the Clusters (kubeconfig direct-connect) REST
// endpoints. Called once by `infracanvas serve` after the manager is created
// (it needs the server's own chosen port to self-dial).
func (s *Server) SetClusterManager(mgr *clustermgr.Manager) {
	s.clusterMgr = mgr
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
		localMachineID: opts.LocalMachineID,
		readOnly:       opts.ReadOnly,
		allowedOrigins: map[string]bool{},
		authLimiter:    newAuthRateLimiter(),
	}
	for _, o := range opts.AllowedOrigins {
		s.allowedOrigins[o] = true
	}
	s.upgrader = websocket.Upgrader{
		ReadBufferSize:  64 * 1024,
		WriteBufferSize: 64 * 1024,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			// Empty origin = non-browser client (curl, native agent) — allow.
			if origin == "" {
				return true
			}
			// Explicit allowlist takes priority.
			if len(s.allowedOrigins) > 0 {
				return s.allowedOrigins[origin]
			}
			// Default: same-origin check — strip scheme and compare host.
			bare := strings.TrimPrefix(strings.TrimPrefix(origin, "https://"), "http://")
			return bare == r.Host
		},
	}
	s.mux = http.NewServeMux()
	s.mux.HandleFunc("/ws/agent", s.handleAgentWS)
	s.mux.HandleFunc("/ws/canvas", s.handleBrowserWS)
	s.mux.HandleFunc("/api/health", s.handleHealth)
	s.mux.HandleFunc("/api/sessions", s.requireUIToken(s.handleSessions))
	s.mux.HandleFunc("/api/audit", s.requireUIToken(s.handleAudit))
	s.mux.HandleFunc("/api/join-info", s.requireUIOrAgentToken(s.handleJoinInfo))
	s.mux.HandleFunc("/api/clusters", s.requireUIOrAgentToken(s.handleClusters))
	s.mux.HandleFunc("/api/clusters/preview", s.requireUIOrAgentToken(s.handleClusterPreview))
	s.mux.HandleFunc("/api/clusters/", s.requireUIOrAgentToken(s.handleClusterByID))
	return s
}

// MountUI serves the embedded dashboard at /, gated by the UI token.
// Requests with ?token=<UIToken> set a cookie and redirect to a clean URL.
// Subsequent requests use the cookie.
func (s *Server) MountUI(fsys fs.FS) {
	fileServer := http.FileServer(http.FS(fsys))
	s.mux.Handle("/", s.requireUIAuth(fileServer))
}

// requireUIToken protects routes meant for the dashboard only — never the
// shared agent/hub token, which every VM in a hub holds identically and so
// grants no real per-caller identity. Use this for anything that discloses
// data about *other* machines (e.g. the session roster); agents have no
// legitimate reason to enumerate each other.
func (s *Server) requireUIToken(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.authLimiter.allow(clientIP(r)) {
			http.Error(w, "too many failed auth attempts, slow down", http.StatusTooManyRequests)
			return
		}
		uiOK := s.uiToken != "" && s.checkUIToken(r)
		open := s.uiToken == ""
		if uiOK || open {
			next(w, r)
			return
		}
		s.authLimiter.recordFailure(clientIP(r), "invalid UI token on "+r.URL.Path)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}
}

// requireUIOrAgentToken protects API routes readable by both the dashboard
// (UI token via cookie/query) and agents/automation (bearer token). When both
// tokens are configured, either grants access; a route stays open only if
// neither token is set (loopback-only deployments).
func (s *Server) requireUIOrAgentToken(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.authLimiter.allow(clientIP(r)) {
			http.Error(w, "too many failed auth attempts, slow down", http.StatusTooManyRequests)
			return
		}
		uiOK := s.uiToken != "" && s.checkUIToken(r)
		agentOK := s.agentToken != "" && secureEquals(r.Header.Get("Authorization"), "Bearer "+s.agentToken)
		open := s.uiToken == "" && s.agentToken == ""
		if uiOK || agentOK || open {
			next(w, r)
			return
		}
		s.authLimiter.recordFailure(clientIP(r), "invalid UI/agent token on "+r.URL.Path)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}
}

// secureEquals compares two secrets in constant time.
func secureEquals(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// checkAgentToken validates the Authorization header on /ws/agent.
func (s *Server) checkAgentToken(r *http.Request) bool {
	if s.agentToken == "" {
		return true
	}
	auth := r.Header.Get("Authorization")
	return secureEquals(auth, "Bearer "+s.agentToken)
}

// checkUIToken accepts the token from the cookie or ?token= query param.
func (s *Server) checkUIToken(r *http.Request) bool {
	if s.uiToken == "" {
		return true
	}
	if c, err := r.Cookie("infracanvas_token"); err == nil && secureEquals(c.Value, s.uiToken) {
		return true
	}
	return secureEquals(r.URL.Query().Get("token"), s.uiToken)
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
		if !s.authLimiter.allow(clientIP(r)) {
			http.Error(w, "too many failed auth attempts, slow down", http.StatusTooManyRequests)
			return
		}
		if q := r.URL.Query().Get("token"); q != "" {
			if secureEquals(q, s.uiToken) {
				http.SetCookie(w, &http.Cookie{
					Name:     "infracanvas_token",
					Value:    q,
					Path:     "/",
					HttpOnly: true,
					Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
					SameSite: http.SameSiteStrictMode,
					MaxAge:   60 * 60 * 24 * 30,
				})
				// Build redirect from known-safe components to prevent
				// open-redirect via paths like "//evil.com".
				redirectPath := r.URL.Path
				if !strings.HasPrefix(redirectPath, "/") {
					redirectPath = "/"
				}
				qq := r.URL.Query()
				qq.Del("token")
				if enc := qq.Encode(); enc != "" {
					redirectPath += "?" + enc
				}
				http.Redirect(w, r, redirectPath, http.StatusSeeOther)
				return
			}
			s.authLimiter.recordFailure(clientIP(r), "invalid ?token= on "+r.URL.Path)
			s.writeUnauthorizedHTML(w, "Invalid token.")
			return
		}
		if c, err := r.Cookie("infracanvas_token"); err == nil && secureEquals(c.Value, s.uiToken) {
			next.ServeHTTP(w, r)
			return
		}
		s.authLimiter.recordFailure(clientIP(r), "missing/invalid auth cookie on "+r.URL.Path)
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

// securityHeaders sets baseline defense-in-depth headers on every response:
// no framing (clickjacking), no MIME sniffing, no active-content CSP beyond
// same-origin (the dashboard renders agent/cluster-supplied strings), and
// HSTS when we know the request arrived over TLS.
//
// script-src/style-src need 'unsafe-inline': the embedded dashboard is a
// Next.js static export, which boots via inline `self.__next_f.push(...)`
// hydration script blocks and has no nonce infrastructure (there's no
// per-request template rendering — index.html is a static file). A strict
// default-src with no override blocks that hydration entirely, breaking the
// dashboard outright. This still blocks framing, restricts every resource
// type to same-origin, and stops third-party script/style injection — just
// not inline-script injection, which is the tradeoff a static-export SPA
// with no server-side templating has to make short of a much bigger rewrite.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Frame-Options", "DENY")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; frame-ancestors 'none'; base-uri 'self'")
		h.Set("Referrer-Policy", "same-origin")
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

// Handler returns the HTTP handler (useful for testing or custom listeners).
func (s *Server) Handler() http.Handler {
	return securityHeaders(s.mux)
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe(addr string) error {
	log.Printf("InfraCanvas server listening on %s", addr)
	log.Printf("  Agent endpoint:   ws://%s/ws/agent", addr)
	log.Printf("  Canvas endpoint:  ws://%s/ws/canvas", addr)
	log.Printf("  Health:           http://%s/api/health", addr)
	return http.ListenAndServe(addr, securityHeaders(s.mux))
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

// SetJoinInfo publishes the address other VMs should use to join this hub.
// The dashboard's "Add machine" flow reads it via /api/join-info. An empty
// url hides the flow (e.g. --private with no reachable address). Safe to call
// again when the address changes (tunnel restarts with a new hostname).
func (s *Server) SetJoinInfo(url, caveat string) {
	s.joinMu.Lock()
	s.joinURL, s.joinCaveat = url, caveat
	s.joinMu.Unlock()
}

// handleJoinInfo returns everything the dashboard needs to render a
// copy-paste join command. Gated by the same auth as /api/sessions: the
// join token must never be readable without the UI token.
func (s *Server) handleJoinInfo(w http.ResponseWriter, r *http.Request) {
	s.joinMu.RLock()
	url, caveat := s.joinURL, s.joinCaveat
	s.joinMu.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"joinUrl": url,
		"token":   s.agentToken,
		"caveat":  caveat,
	})
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	s.localMu.RLock()
	localID := ""
	if s.localSession != nil {
		localID = s.localSession.ID
	}
	s.localMu.RUnlock()

	info := s.sessions.List()
	for i := range info {
		info[i].Local = info[i].ID == localID
		if s.readOnly {
			info[i].ReadOnly = true
		} else if s.clusterMgr != nil {
			if id, ok := strings.CutPrefix(info[i].MachineID, "cluster-"); ok {
				info[i].ReadOnly = s.clusterMgr.IsReadOnly(id)
			}
		}
	}
	sort.Slice(info, func(a, b int) bool {
		if info[a].Local != info[b].Local {
			return info[a].Local
		}
		if info[a].Hostname != info[b].Hostname {
			return info[a].Hostname < info[b].Hostname
		}
		return info[a].ID < info[b].ID
	})
	_ = json.NewEncoder(w).Encode(info)
}

// handleAudit serves GET /api/audit?limit=N — the most recent write-action
// and terminal-session events, newest first. Entries are attributed by
// session/machine, not by user identity: OSS has one shared UI token
// authenticating the whole dashboard, no per-user login, so that's the real
// ceiling on what can be logged here.
func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	limit := 200
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(audit.Recent(limit))
}

// ── Clusters (kubeconfig direct-connect) ──────────────────────────────────────

type addClusterRequest struct {
	Kubeconfig string `json:"kubeconfig"` // raw kubeconfig YAML/JSON text
	Name       string `json:"name,omitempty"`
	Context    string `json:"context,omitempty"` // omit to get the context picker back instead of creating
	ReadOnly   bool   `json:"readOnly,omitempty"`
}

// updateClusterRequest is the body for PATCH /api/clusters/{id}.
type updateClusterRequest struct {
	ReadOnly *bool `json:"readOnly,omitempty"` // pointer so "omitted" and "set to false" are distinguishable
}

// handleClusters serves GET (list) and POST (add) on /api/clusters.
func (s *Server) handleClusters(w http.ResponseWriter, r *http.Request) {
	if s.clusterMgr == nil {
		http.Error(w, "clusters not available", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		entries, err := s.clusterMgr.List()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(entries)

	case http.MethodPost:
		body, err := io.ReadAll(io.LimitReader(r.Body, 8<<20)) // 8MB cap
		if err != nil {
			http.Error(w, "failed to read body", http.StatusBadRequest)
			return
		}
		var req addClusterRequest
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(req.Kubeconfig) == "" {
			http.Error(w, "kubeconfig is required", http.StatusBadRequest)
			return
		}

		if req.Context == "" {
			// Pure parse — clientcmd.Load never touches the network or runs an
			// exec credential plugin, so this is safe to allow even read-only.
			contexts, err := clustermgr.ParseContexts([]byte(req.Kubeconfig))
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"contexts": contexts})
			return
		}

		// Actually connecting a cluster runs the kubeconfig's auth (including
		// any `exec:` credential plugin client-go executes verbatim) — a
		// mutating, code-executing action that read-only mode must block the
		// same as any other write.
		if s.readOnly {
			http.Error(w, "read-only mode: adding clusters is disabled", http.StatusForbidden)
			return
		}

		entry, err := s.clusterMgr.Add(req.Name, []byte(req.Kubeconfig), req.Context, req.ReadOnly)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(entry)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleClusterPreview serves POST /api/clusters/preview — checks what a
// kubeconfig context can actually do (view, exec, restart, scale, read
// secrets) before the user commits to connecting it. Read-only in every
// sense: no cluster is added, no virtual agent starts, nothing is persisted,
// so this is allowed even in --read-only mode.
func (s *Server) handleClusterPreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	var req addClusterRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Kubeconfig) == "" {
		http.Error(w, "kubeconfig is required", http.StatusBadRequest)
		return
	}
	preview, err := clustermgr.PreviewPermissions([]byte(req.Kubeconfig), req.Context)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(preview)
}

// handleClusterByID serves DELETE and PATCH on /api/clusters/{id}.
func (s *Server) handleClusterByID(w http.ResponseWriter, r *http.Request) {
	if s.clusterMgr == nil {
		http.Error(w, "clusters not available", http.StatusServiceUnavailable)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/clusters/")
	if id == "" {
		http.Error(w, "missing cluster id", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodDelete:
		if s.readOnly {
			http.Error(w, "read-only mode: removing clusters is disabled", http.StatusForbidden)
			return
		}
		if err := s.clusterMgr.Remove(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	case http.MethodPatch:
		if s.readOnly {
			http.Error(w, "read-only mode: editing clusters is disabled", http.StatusForbidden)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<16))
		if err != nil {
			http.Error(w, "failed to read body", http.StatusBadRequest)
			return
		}
		var req updateClusterRequest
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if req.ReadOnly == nil {
			http.Error(w, "nothing to update", http.StatusBadRequest)
			return
		}
		entry, err := s.clusterMgr.SetReadOnly(id, *req.ReadOnly)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(entry)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// ── Agent WebSocket handler ───────────────────────────────────────────────────

func (s *Server) handleAgentWS(w http.ResponseWriter, r *http.Request) {
	if !s.authLimiter.allow(clientIP(r)) {
		http.Error(w, "too many failed auth attempts, slow down", http.StatusTooManyRequests)
		return
	}
	if !s.checkAgentToken(r) {
		s.authLimiter.recordFailure(clientIP(r), "invalid agent token on /ws/agent")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	log.Printf("[agent] upgrade attempt from %s", r.RemoteAddr)
	raw, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("agent upgrade error: %v (remote: %s)", err, r.RemoteAddr)
		return
	}
	raw.SetReadLimit(32 << 20) // 32MB — generous headroom for large topology snapshots
	conn := newSafeConn(raw)

	// The agent's first message is HELLO. Read it before creating the session
	// so a machine-identified agent resumes its previous session (keeping any
	// attached browsers) instead of appearing as a new machine.
	_ = raw.SetReadDeadline(time.Now().Add(15 * time.Second))
	_, firstPayload, err := conn.ReadMessage()
	if err != nil {
		log.Printf("[agent] no HELLO from %s: %v", r.RemoteAddr, err)
		conn.Close()
		return
	}
	_ = raw.SetReadDeadline(time.Time{})

	var firstEnv Envelope
	var hello HelloData
	if json.Unmarshal(firstPayload, &firstEnv) == nil && firstEnv.Type == MsgHello {
		_ = json.Unmarshal(firstEnv.Data, &hello)
	}

	var sess *Session
	resumed := false
	newSecret := false
	if hello.MachineID != "" {
		var authorized bool
		sess, resumed, authorized = s.sessions.UpsertByMachine(conn, hello.MachineID, hello.ResumeSecret)
		if !authorized {
			log.Printf("[agent] REJECTED machine=%s from %s: resume secret mismatch (possible session-hijack attempt)", hello.MachineID, r.RemoteAddr)
			conn.Close()
			return
		}
		newSecret = !resumed
	} else {
		sess = s.sessions.Create(conn)
	}
	if resumed {
		log.Printf("[agent] resumed  session=%s  machine=%s", sess.ID, hello.MachineID)
	} else {
		log.Printf("[agent] connected  session=%s  code=%s", sess.ID, sess.PairCode)
	}

	// In local mode the in-process agent (matched by machine ID) owns the
	// implicit local session; without a configured local machine ID the first
	// agent wins. Remote agents joining the hub never displace it.
	if s.localMode {
		s.localMu.Lock()
		if s.localMachineID != "" {
			if hello.MachineID == s.localMachineID {
				s.localSession = sess
			}
		} else if s.localSession == nil || s.localSession == sess {
			s.localSession = sess
		}
		s.localMu.Unlock()
	}

	// Send the pair code (still useful in shared-relay mode; harmless locally).
	pairCodeMsg := PairCodeData{Code: sess.PairCode}
	if newSecret {
		pairCodeMsg.ResumeSecret = sess.ResumeSecret
	}
	if err := writeMsg(conn, MsgPairCode, pairCodeMsg); err != nil {
		log.Printf("[agent] failed to send PAIR_CODE: %v", err)
		conn.Close()
		s.dropAgentConn(sess, conn)
		return
	}

	// Process the HELLO we already consumed (updates hostname/scope and
	// re-announces the agent to any browsers attached from a previous run).
	if firstEnv.Type != "" {
		s.routeAgentMessage(sess, firstEnv, firstPayload)
	}

	defer func() {
		conn.Close()
		s.dropAgentConn(sess, conn)
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

// dropAgentConn handles an agent connection going away. Machine-identified
// sessions are retained offline (browsers stay attached and the host list
// keeps the machine with an offline marker); legacy pair-code sessions are
// deleted as before. If the session was already resumed by a newer
// connection, this is a no-op.
func (s *Server) dropAgentConn(sess *Session, conn *SafeConn) {
	sess.mu.Lock()
	if sess.AgentConn != conn {
		sess.mu.Unlock()
		return // a newer connection already took over
	}
	machineOwned := sess.MachineID != ""
	sess.AgentConn = nil
	sess.Online = false
	sess.LastSeen = time.Now()
	sess.mu.Unlock()

	if machineOwned {
		log.Printf("[agent] offline  session=%s  machine=%s", sess.ID, sess.MachineID)
	} else {
		s.sessions.Delete(sess)
		if s.localMode {
			s.localMu.Lock()
			if s.localSession == sess {
				s.localSession = nil
			}
			s.localMu.Unlock()
		}
		log.Printf("[agent] disconnected  session=%s", sess.ID)
	}

	// Notify all paired browsers.
	broadcastToBrowsers(sess, mustMarshalEnvelope(MsgAgentDisconnected, struct{}{}))
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
			ReadOnly: s.sessionReadOnly(sess),
		})
		broadcastToBrowsers(sess, msg)

	case MsgGraphSnapshot:
		// Cache for late joiners, count nodes for the host list, then relay.
		var snap struct {
			Data struct {
				Nodes []json.RawMessage `json:"nodes"`
			} `json:"data"`
		}
		_ = json.Unmarshal(raw, &snap)
		sess.mu.Lock()
		sess.LastSnapshot = make([]byte, len(raw))
		copy(sess.LastSnapshot, raw)
		sess.NodeCount = len(snap.Data.Nodes)
		sess.mu.Unlock()

		broadcastToBrowsers(sess, raw)
		log.Printf("[agent] GRAPH_SNAPSHOT  %d bytes  → %d browsers", len(raw), sess.BrowserCount())

	case MsgGraphDiff:
		broadcastToBrowsers(sess, raw)
		log.Printf("[agent] GRAPH_DIFF  → %d browsers", sess.BrowserCount())

	case MsgActionResult:
		broadcastToBrowsers(sess, raw)
		log.Printf("[agent] ACTION_RESULT  → %d browsers", sess.BrowserCount())
		var result struct {
			ActionID string `json:"action_id"`
			Success  bool   `json:"success"`
			Message  string `json:"message"`
			Error    string `json:"error"`
		}
		if json.Unmarshal(env.Data, &result) == nil && result.ActionID != "" {
			msg := result.Message
			if !result.Success && result.Error != "" {
				msg = result.Error
			}
			audit.LogActionCompleted(result.ActionID, result.Success, msg)
		}

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
		sess.mu.Lock()
		sess.LastSeen = time.Now()
		sess.mu.Unlock()

	default:
		log.Printf("[agent] unknown message type: %s", env.Type)
	}
}

// ── Browser WebSocket handler ─────────────────────────────────────────────────

func (s *Server) handleBrowserWS(w http.ResponseWriter, r *http.Request) {
	if !s.authLimiter.allow(clientIP(r)) {
		http.Error(w, "too many failed auth attempts, slow down", http.StatusTooManyRequests)
		return
	}
	if !s.checkUIToken(r) {
		s.authLimiter.recordFailure(clientIP(r), "invalid UI token on /ws/canvas")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	raw, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("browser upgrade error: %v", err)
		return
	}
	raw.SetReadLimit(4 << 20) // 4MB — plenty for any browser-originated action/exec payload
	conn := newSafeConn(raw)
	defer conn.Close()

	var sess *Session

	if s.localMode {
		// The UI opens one socket per machine and sends PAIR with a key:
		// the literal 'local' (this machine's canvas) or a session/machine
		// ID from /api/sessions. A non-PAIR first message is treated as
		// 'local' for back-compat and processed after attach.
		var pendingEnv *Envelope
		var pendingPayload []byte
		key := "local"

		_, payload, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var env Envelope
		if err := json.Unmarshal(payload, &env); err == nil {
			if env.Type == "PAIR" {
				var req PairRequest
				_ = json.Unmarshal(env.Data, &req)
				if req.Code != "" {
					key = req.Code
				}
			} else {
				pendingEnv, pendingPayload = &env, payload
			}
		}

		if key == "local" {
			// Bind to the local session; if the in-process agent hasn't
			// connected yet, wait briefly for it.
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
			s.sessions.Attach(sess, conn)
			log.Printf("[browser] paired (local)  session=%s  browsers=%d", sess.ID, sess.BrowserCount())
		} else {
			var ok bool
			sess, ok = s.sessions.FindByAny(key)
			if !ok {
				_ = writeMsg(conn, MsgError, map[string]string{"message": "unknown machine"})
				return
			}
			s.sessions.Attach(sess, conn)
			log.Printf("[browser] paired  session=%s  key=%s  browsers=%d", sess.ID, key, sess.BrowserCount())
		}

		if pendingEnv != nil {
			s.handleBrowserMessage(sess, conn, *pendingEnv, pendingPayload)
		}
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
		sess, ok = s.sessions.FindByAny(req.Code)
		if !ok {
			_ = writeMsg(conn, MsgError, map[string]string{"message": "unknown pair code"})
			return
		}
		s.sessions.Attach(sess, conn)
		log.Printf("[browser] paired  session=%s  code=%s  browsers=%d",
			sess.ID, req.Code, sess.BrowserCount())
	}

	// Notify agent it has a new viewer (if it's currently connected).
	sess.mu.RLock()
	agentConn := sess.AgentConn
	online := sess.Online
	sess.mu.RUnlock()
	if agentConn != nil {
		go func() { _ = writeMsg(agentConn, MsgPaired, PairedData{BrowserCount: sess.BrowserCount()}) }()
	}

	// Late joiners missed the HELLO broadcast — replay agent identity so the
	// browser learns hostname, scope, and the read-only flag immediately.
	sess.mu.RLock()
	hostname, scope := sess.Hostname, sess.Scope
	sess.mu.RUnlock()
	if hostname != "" {
		_ = writeMsg(conn, MsgAgentConnected, AgentConnectedData{
			Hostname: hostname,
			Scope:    scope,
			ReadOnly: s.sessionReadOnly(sess),
		})
	}

	// Replay the last cached snapshot so the browser doesn't wait for the next tick.
	sess.mu.RLock()
	lastSnap := sess.LastSnapshot
	sess.mu.RUnlock()
	if lastSnap != nil {
		_ = conn.WriteMessage(websocket.TextMessage, lastSnap)
	}

	// Attaching to an offline machine shows its last-known state; tell the
	// browser the agent is away so the UI can badge it.
	if !online {
		_ = writeMsg(conn, MsgAgentDisconnected, struct{}{})
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
		s.handleBrowserMessage(sess, conn, env, payload)
	}
}

// sessionReadOnly reports whether writes should be blocked for this specific
// session — either the server's global --read-only flag, or (for a Clusters
// virtual agent session) that one cluster's own read-only flag, set
// independently of the global one via POST/PATCH /api/clusters.
func (s *Server) sessionReadOnly(sess *Session) bool {
	if s.readOnly {
		return true
	}
	if s.clusterMgr == nil {
		return false
	}
	if id, ok := strings.CutPrefix(sess.MachineID, "cluster-"); ok {
		return s.clusterMgr.IsReadOnly(id)
	}
	return false
}

// handleBrowserMessage forwards COMMAND and BROWSER_ACTION messages to the agent.
func (s *Server) handleBrowserMessage(sess *Session, conn *SafeConn, env Envelope, payload []byte) {
	switch env.Type {
	case MsgBrowserAction:
		if s.sessionReadOnly(sess) && !isReadOnlySafeAction(env.Data) {
			s.rejectReadOnlyAction(sess, conn, env.Data)
			return
		}
		if !isReadOnlySafeAction(env.Data) {
			var req struct {
				ActionID string `json:"action_id"`
				Type     string `json:"type"`
				Target   struct {
					EntityID  string `json:"entity_id"`
					Namespace string `json:"namespace"`
				} `json:"target"`
			}
			if json.Unmarshal(env.Data, &req) == nil {
				sess.mu.RLock()
				hostname, machineID := sess.Hostname, sess.MachineID
				sess.mu.RUnlock()
				audit.LogActionRequested(req.ActionID, req.Type, machineID, hostname, req.Target.EntityID, req.Target.Namespace)
			}
		}
		// Translate BROWSER_ACTION → ACTION_REQUEST before forwarding
		env.Type = MsgActionRequest
		payload, _ = json.Marshal(env)
		fallthrough
	case MsgCommand, MsgExecStart, MsgExecInput, MsgExecResize, MsgExecEnd:
		if s.sessionReadOnly(sess) && (env.Type == MsgExecStart || env.Type == MsgExecInput || env.Type == MsgExecResize) {
			if env.Type == MsgExecStart {
				s.rejectReadOnlyExec(sess, conn, env.Data)
			}
			return
		}
		if env.Type == MsgExecStart {
			var req struct {
				ContainerID string `json:"container_id"`
				PodName     string `json:"pod_name"`
				Namespace   string `json:"namespace"`
			}
			if json.Unmarshal(env.Data, &req) == nil {
				sess.mu.RLock()
				hostname, machineID := sess.Hostname, sess.MachineID
				sess.mu.RUnlock()
				entity := req.PodName
				if entity == "" {
					entity = req.ContainerID
				}
				audit.LogExecRequested(machineID, hostname, entity, req.Namespace)
			}
		}
		sess.mu.RLock()
		agentConn := sess.AgentConn
		sess.mu.RUnlock()
		if agentConn != nil {
			_ = agentConn.WriteMessage(websocket.TextMessage, payload)
		}
	}
}

// ── read-only mode ───────────────────────────────────────────────────────────

// readOnlySafeActions are BROWSER_ACTION types that only read state.
var readOnlySafeActions = map[string]bool{
	"docker_logs": true,
	"k8s_logs":    true,
	"host_logs":   true,
}

func isReadOnlySafeAction(data json.RawMessage) bool {
	var req struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &req); err != nil {
		return false
	}
	return readOnlySafeActions[req.Type]
}

// rejectReadOnlyAction answers a blocked BROWSER_ACTION with a failed
// ACTION_RESULT in the same shape the agent would produce, so the UI
// surfaces it as a normal action error.
func (s *Server) rejectReadOnlyAction(sess *Session, conn *SafeConn, data json.RawMessage) {
	var req struct {
		ActionID string `json:"action_id"`
		Type     string `json:"type"`
	}
	_ = json.Unmarshal(data, &req)
	log.Printf("[browser] blocked action %q (read-only mode)", req.Type)
	sess.mu.RLock()
	hostname, machineID := sess.Hostname, sess.MachineID
	sess.mu.RUnlock()
	audit.LogActionBlocked("action_blocked", req.Type, machineID, hostname)
	_ = writeMsg(conn, MsgActionResult, map[string]interface{}{
		"action_id": req.ActionID,
		"success":   false,
		"message":   "",
		"error":     "This is a read-only demo — actions are disabled.",
		"details":   map[string]interface{}{},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// rejectReadOnlyExec answers a blocked EXEC_START with a short message on the
// terminal stream followed by EXEC_END, so the terminal panel closes cleanly.
func (s *Server) rejectReadOnlyExec(sess *Session, conn *SafeConn, data json.RawMessage) {
	var req struct {
		SessionID string `json:"session_id"`
	}
	_ = json.Unmarshal(data, &req)
	log.Printf("[browser] blocked exec session (read-only mode)")
	sess.mu.RLock()
	hostname, machineID := sess.Hostname, sess.MachineID
	sess.mu.RUnlock()
	audit.LogActionBlocked("exec_blocked", "", machineID, hostname)
	_ = writeMsg(conn, MsgExecData, map[string]interface{}{
		"session_id": req.SessionID,
		"data":       base64.StdEncoding.EncodeToString([]byte("This is a read-only demo — terminals are disabled.\r\n")),
		"error":      "",
	})
	_ = writeMsg(conn, MsgExecEnd, map[string]interface{}{"session_id": req.SessionID})
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

// broadcastToBrowsers fans a message out to every browser attached to sess.
// Enqueuing happens synchronously and in order here: SafeConn.WriteMessage
// only enqueues onto that connection's own ordered outbox (a fast,
// non-blocking channel send in the common case — see SafeConn.enqueue), and
// a dedicated per-connection writer goroutine drains it FIFO. Spawning a
// goroutine per message per browser here (as before) would let concurrent
// enqueues race for channel-send order, defeating the FIFO guarantee for
// exactly the order-sensitive streams (EXEC_DATA, LOG_DATA) this exists to
// protect.
func broadcastToBrowsers(sess *Session, msg []byte) {
	sess.mu.RLock()
	browsers := make([]*SafeConn, len(sess.Browsers))
	copy(browsers, sess.Browsers)
	sess.mu.RUnlock()

	for _, c := range browsers {
		_ = c.WriteMessage(websocket.TextMessage, msg)
	}
}
