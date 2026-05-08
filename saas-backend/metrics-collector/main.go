// metrics-collector — sidecar service for InfraCanvas SaaS.
// Opens persistent WebSocket connections to the relay per online agent,
// extracts host metrics from GRAPH_SNAPSHOT messages, writes to
// infracanvas_metrics.metrics table, and serves a query HTTP API on :8091.
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	_ "github.com/lib/pq"
)

// ---- Config ----------------------------------------------------------------

type Config struct {
	SaasDBDSN       string
	MetricsDBDSN    string
	SaasAPIURL      string
	LoginEmail      string
	LoginPassword   string
	CollectInterval time.Duration
	Port            string
}

func configFromEnv() Config {
	return Config{
		SaasDBDSN:       env("SAAS_DB_DSN", "host=localhost port=5433 user=infracanvas password=infracanvas_dev_password dbname=infracanvas_saas sslmode=disable"),
		MetricsDBDSN:    env("METRICS_DB_DSN", "host=localhost port=5433 user=infracanvas password=infracanvas_dev_password dbname=infracanvas_metrics sslmode=disable"),
		SaasAPIURL:      env("SAAS_API_URL", "http://localhost:8090"),
		LoginEmail:      env("LOGIN_EMAIL", ""),
		LoginPassword:   env("LOGIN_PASSWORD", ""),
		CollectInterval: 30 * time.Second,
		Port:            env("METRICS_PORT", "8091"),
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// ---- Models ----------------------------------------------------------------

type Agent struct {
	UUID      string // primary key UUID in agents table
	StringID  string // e.g. "ByteStrix-5bcfabe8"
	OrgID     string // organization UUID
}

type wsMsg struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type graphNode struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Metadata map[string]interface{} `json:"metadata"`
}

type graphSnapshot struct {
	Nodes []graphNode `json:"nodes"`
}

type hostMetrics struct {
	CPU    float64
	Memory float64
	Disk   float64
}

// ---- Collector -------------------------------------------------------------

type agentConn struct {
	agent  Agent
	conn   *websocket.Conn
	cancel chan struct{}
}

type Collector struct {
	cfg       Config
	saasDB    *sql.DB
	metricsDB *sql.DB

	tokenMu sync.RWMutex
	token   string

	agentsMu sync.Mutex
	agents   map[string]*agentConn // keyed by agent UUID
}

func newCollector(cfg Config) (*Collector, error) {
	saasDB, err := sql.Open("postgres", cfg.SaasDBDSN)
	if err != nil {
		return nil, fmt.Errorf("open saas db: %w", err)
	}
	if err := saasDB.Ping(); err != nil {
		return nil, fmt.Errorf("ping saas db: %w", err)
	}

	mDB, err := sql.Open("postgres", cfg.MetricsDBDSN)
	if err != nil {
		return nil, fmt.Errorf("open metrics db: %w", err)
	}
	if err := mDB.Ping(); err != nil {
		return nil, fmt.Errorf("ping metrics db: %w", err)
	}

	return &Collector{
		cfg:       cfg,
		saasDB:    saasDB,
		metricsDB: mDB,
		agents:    make(map[string]*agentConn),
	}, nil
}

// login calls the SaaS API and stores a fresh JWT.
func (c *Collector) login() error {
	body, _ := json.Marshal(map[string]string{
		"email":    c.cfg.LoginEmail,
		"password": c.cfg.LoginPassword,
	})
	resp, err := http.Post(
		c.cfg.SaasAPIURL+"/api/v1/auth/login",
		"application/json",
		strings.NewReader(string(body)),
	)
	if err != nil {
		return fmt.Errorf("login request: %w", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("login %d: %s", resp.StatusCode, b)
	}
	var res struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(b, &res); err != nil || res.Token == "" {
		return fmt.Errorf("login empty token: %s", b)
	}
	c.tokenMu.Lock()
	c.token = res.Token
	c.tokenMu.Unlock()
	log.Printf("[collector] authenticated as %s", c.cfg.LoginEmail)
	return nil
}

func (c *Collector) getToken() string {
	c.tokenMu.RLock()
	defer c.tokenMu.RUnlock()
	return c.token
}

// syncAgents reconciles WS connections with online agents from the DB.
func (c *Collector) syncAgents() {
	rows, err := c.saasDB.Query(`
		SELECT id, agent_id, organization_id FROM agents WHERE status = 'online'
	`)
	if err != nil {
		log.Printf("[collector] query agents: %v", err)
		return
	}
	defer rows.Close()

	online := make(map[string]Agent)
	for rows.Next() {
		var a Agent
		if err := rows.Scan(&a.UUID, &a.StringID, &a.OrgID); err == nil {
			online[a.UUID] = a
		}
	}

	c.agentsMu.Lock()
	defer c.agentsMu.Unlock()

	// Disconnect agents that went offline
	for uuid, ac := range c.agents {
		if _, ok := online[uuid]; !ok {
			log.Printf("[collector] agent %s offline, closing WS", ac.agent.StringID)
			close(ac.cancel)
			delete(c.agents, uuid)
		}
	}

	// Connect new online agents
	for uuid, agent := range online {
		if _, ok := c.agents[uuid]; !ok {
			ac := &agentConn{agent: agent, cancel: make(chan struct{})}
			c.agents[uuid] = ac
			go c.watchAgent(ac)
		}
	}
}

// watchAgent maintains a persistent WS connection to one agent.
func (c *Collector) watchAgent(ac *agentConn) {
	wsBase := strings.NewReplacer("http://", "ws://", "https://", "wss://").Replace(c.cfg.SaasAPIURL)
	url := fmt.Sprintf("%s/ws/canvas/%s", wsBase, ac.agent.StringID)

	for {
		select {
		case <-ac.cancel:
			return
		default:
		}

		conn, _, err := websocket.DefaultDialer.Dial(url, nil)
		if err != nil {
			log.Printf("[collector] dial %s: %v — retry in 15s", ac.agent.StringID, err)
			select {
			case <-ac.cancel:
				return
			case <-time.After(15 * time.Second):
			}
			continue
		}

		// First message: AUTH
		authMsg, _ := json.Marshal(wsMsg{
			Type: "AUTH",
			Data: json.RawMessage(fmt.Sprintf(`{"token":%q}`, c.getToken())),
		})
		if err := conn.WriteMessage(websocket.TextMessage, authMsg); err != nil {
			conn.Close()
			continue
		}

		log.Printf("[collector] watching agent %s", ac.agent.StringID)
		ac.conn = conn
		c.readLoop(ac)
		conn.Close()

		select {
		case <-ac.cancel:
			return
		case <-time.After(5 * time.Second):
		}
	}
}

// readLoop processes incoming WS messages for one agent.
func (c *Collector) readLoop(ac *agentConn) {
	for {
		_, data, err := ac.conn.ReadMessage()
		if err != nil {
			return
		}
		var msg wsMsg
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		if msg.Type != "GRAPH_SNAPSHOT" && msg.Type != "GRAPH_DIFF" {
			continue
		}
		if msg.Type == "GRAPH_SNAPSHOT" {
			var snap graphSnapshot
			if err := json.Unmarshal(msg.Data, &snap); err != nil {
				continue
			}
			if m := extractMetrics(snap.Nodes); m != nil {
				c.writeMetrics(ac.agent, *m)
			}
		}
	}
}

func extractMetrics(nodes []graphNode) *hostMetrics {
	for _, n := range nodes {
		if n.Type != "host" {
			continue
		}
		m := n.Metadata
		cpu := asFloat(m["cpu"])
		mem := asFloat(m["memory"])
		var disk float64
		if fs, ok := m["filesystems"].([]interface{}); ok {
			for _, f := range fs {
				fm, ok := f.(map[string]interface{})
				if !ok {
					continue
				}
				if mp, _ := fm["mount_point"].(string); mp == "/" {
					disk = asFloat(fm["usage_percent"])
					break
				}
			}
			if disk == 0 && len(fs) > 0 {
				if fm, ok := fs[0].(map[string]interface{}); ok {
					disk = asFloat(fm["usage_percent"])
				}
			}
		}
		return &hostMetrics{CPU: cpu, Memory: mem, Disk: disk}
	}
	return nil
}

func asFloat(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case json.Number:
		f, _ := n.Float64()
		return f
	}
	return 0
}

func (c *Collector) writeMetrics(agent Agent, m hostMetrics) {
	now := time.Now().UTC()
	type row struct {
		name   string
		value  float64
		labels string
	}
	rows := []row{
		{"cpu_percent", m.CPU, `{}`},
		{"memory_percent", m.Memory, `{}`},
		{"disk_percent", m.Disk, `{"mount":"/"}`},
	}
	for _, r := range rows {
		if _, err := c.metricsDB.Exec(`
			INSERT INTO metrics (time, agent_id, organization_id, metric_type, metric_name, value, labels, unit)
			VALUES ($1, $2::uuid, $3::uuid, 'host', $4, $5, $6::jsonb, 'percent')
		`, now, agent.UUID, agent.OrgID, r.name, r.value, r.labels); err != nil {
			log.Printf("[collector] insert metrics: %v", err)
		}
	}
	log.Printf("[collector] %s  cpu=%.1f%%  mem=%.1f%%  disk=%.1f%%",
		agent.StringID, m.CPU, m.Memory, m.Disk)
}

// ---- HTTP API --------------------------------------------------------------

type metricPoint struct {
	TS     time.Time `json:"ts"`
	CPU    float64   `json:"cpu"`
	Memory float64   `json:"mem"`
	Disk   float64   `json:"disk"`
}

func (c *Collector) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// GET /metrics/v1/agents/{id}?range=1h|24h|7d
	// {id} can be the UUID or the agent string ID (ByteStrix-5bcfabe8)
	p := strings.TrimPrefix(r.URL.Path, "/metrics/v1/agents/")
	agentParam := strings.Split(p, "/")[0]
	if agentParam == "" {
		http.Error(w, "missing agent id", http.StatusBadRequest)
		return
	}

	agentUUID, err := c.resolveAgentUUID(agentParam)
	if err != nil {
		http.Error(w, "agent not found", http.StatusNotFound)
		return
	}

	rangeStr := r.URL.Query().Get("range")
	if rangeStr == "" {
		rangeStr = "1h"
	}
	durSecs, stepSecs := parseRange(rangeStr)

	rows, err := c.metricsDB.Query(`
		SELECT
			TO_TIMESTAMP((EXTRACT(EPOCH FROM time)::bigint / $1) * $1) AS bucket,
			metric_name,
			ROUND(AVG(value)::numeric, 1)
		FROM metrics
		WHERE agent_id = $2::uuid
		  AND time > NOW() - ($3 * INTERVAL '1 second')
		GROUP BY 1, 2
		ORDER BY 1
	`, stepSecs, agentUUID, durSecs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	buckets := make(map[time.Time]*metricPoint)
	var order []time.Time

	for rows.Next() {
		var bucket time.Time
		var name string
		var val float64
		if err := rows.Scan(&bucket, &name, &val); err != nil {
			continue
		}
		pt, ok := buckets[bucket]
		if !ok {
			pt = &metricPoint{TS: bucket}
			buckets[bucket] = pt
			order = append(order, bucket)
		}
		val = math.Round(val*10) / 10
		switch name {
		case "cpu_percent":
			pt.CPU = val
		case "memory_percent":
			pt.Memory = val
		case "disk_percent":
			pt.Disk = val
		}
	}

	points := make([]metricPoint, 0, len(order))
	for _, t := range order {
		points = append(points, *buckets[t])
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"points": points,
		"range":  rangeStr,
	})
}

// resolveAgentUUID returns the DB UUID for either a UUID string or agent string ID.
func (c *Collector) resolveAgentUUID(param string) (string, error) {
	// Try as UUID directly first (by checking if it contains hyphens in UUID format)
	if len(param) == 36 && strings.Count(param, "-") == 4 {
		return param, nil
	}
	// Treat as string agent_id, look up in saas DB
	var uuid string
	err := c.saasDB.QueryRow(`SELECT id FROM agents WHERE agent_id = $1`, param).Scan(&uuid)
	if err != nil {
		return "", fmt.Errorf("agent %q not found: %w", param, err)
	}
	return uuid, nil
}

func parseRange(r string) (durSecs int64, stepSecs int64) {
	switch r {
	case "1h":
		return 3600, 120 // 2 min buckets
	case "24h":
		return 86400, 1800 // 30 min buckets
	case "7d":
		return 7 * 86400, 7200 // 2 hour buckets
	default:
		return 3600, 120
	}
}

// ---- Retention cleanup -----------------------------------------------------

func (c *Collector) startRetentionCleanup() {
	go func() {
		ticker := time.NewTicker(6 * time.Hour)
		for range ticker.C {
			res, err := c.metricsDB.Exec(
				`DELETE FROM metrics WHERE time < NOW() - INTERVAL '30 days'`,
			)
			if err != nil {
				log.Printf("[collector] cleanup: %v", err)
				continue
			}
			if n, _ := res.RowsAffected(); n > 0 {
				log.Printf("[collector] pruned %d rows older than 30d", n)
			}
		}
	}()
}

// ---- Main ------------------------------------------------------------------

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmsgprefix)

	cfg := configFromEnv()
	if cfg.LoginEmail == "" || cfg.LoginPassword == "" {
		log.Fatal("LOGIN_EMAIL and LOGIN_PASSWORD env vars required")
	}

	coll, err := newCollector(cfg)
	if err != nil {
		log.Fatalf("init: %v", err)
	}

	// Initial auth
	if err := coll.login(); err != nil {
		log.Fatalf("initial login: %v", err)
	}

	// Re-auth every 20h (JWT lifetime is 24h)
	go func() {
		for range time.Tick(20 * time.Hour) {
			if err := coll.login(); err != nil {
				log.Printf("[collector] re-auth: %v", err)
			}
		}
	}()

	// Initial agent sync + periodic re-sync
	coll.syncAgents()
	go func() {
		for range time.Tick(cfg.CollectInterval) {
			coll.syncAgents()
		}
	}()

	coll.startRetentionCleanup()

	log.Printf("[collector] HTTP API listening on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, coll); err != nil {
		log.Fatalf("http: %v", err)
	}
}
