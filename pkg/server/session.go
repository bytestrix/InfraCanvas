package server

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"sync"
	"time"
)

var pairWords = []string{
	"WOLF", "BEAR", "HAWK", "LION", "DEER", "CROW", "LYNX",
	"SEAL", "IBIS", "KITE", "ORCA", "WREN", "APEX", "BOLT",
	"CYAN", "DUSK", "ECHO", "FLUX", "GLOW", "HAZE", "IRIS",
	"JADE", "NOVA", "ONYX", "PIKE", "REEF", "SAGE", "TIDE",
	"VIBE", "ZINC", "ATOM", "BYTE", "CORE", "DISK", "EDGE",
	"FIBER", "GRID", "HOST", "INIT", "JUMP", "KERN", "LOOP",
}

// Session represents a paired agent↔browser session.
type Session struct {
	ID           string
	PairCode     string
	MachineID    string // stable agent identity; empty for legacy agents
	AgentConn    *SafeConn
	Browsers     []*SafeConn
	Hostname     string
	Scope        []string
	Online       bool
	LastSeen     time.Time
	NodeCount    int
	PairedAt     time.Time
	LastSnapshot []byte // cached for late-joining browsers
	mu           sync.RWMutex
}

func (s *Session) BrowserCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.Browsers)
}

// SessionStore is a thread-safe store for active sessions.
type SessionStore struct {
	byCode    map[string]*Session
	byID      map[string]*Session
	byMachine map[string]*Session
	mu        sync.RWMutex
}

func NewSessionStore() *SessionStore {
	return &SessionStore{
		byCode:    make(map[string]*Session),
		byID:      make(map[string]*Session),
		byMachine: make(map[string]*Session),
	}
}

func (s *SessionStore) Create(conn *SafeConn) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.createLocked(conn)
}

func (s *SessionStore) createLocked(conn *SafeConn) *Session {
	code := generatePairCode()
	for _, taken := s.byCode[code]; taken; _, taken = s.byCode[code] {
		code = generatePairCode()
	}

	sess := &Session{
		ID:        fmt.Sprintf("sess-%d", time.Now().UnixNano()),
		PairCode:  code,
		AgentConn: conn,
		Online:    true,
		LastSeen:  time.Now(),
		Browsers:  make([]*SafeConn, 0),
	}
	s.byCode[code] = sess
	s.byID[sess.ID] = sess
	return sess
}

// UpsertByMachine resumes the session owned by machineID (swapping in the new
// connection, keeping attached browsers) or creates a fresh one. The second
// return value reports whether an existing session was resumed.
func (s *SessionStore) UpsertByMachine(conn *SafeConn, machineID string) (*Session, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if sess, ok := s.byMachine[machineID]; ok {
		sess.mu.Lock()
		sess.AgentConn = conn
		sess.Online = true
		sess.LastSeen = time.Now()
		sess.mu.Unlock()
		return sess, true
	}

	sess := s.createLocked(conn)
	sess.MachineID = machineID
	s.byMachine[machineID] = sess
	return sess, false
}

// FindByAny resolves a browser-supplied key against session ID, machine ID,
// then pair code.
func (s *SessionStore) FindByAny(key string) (*Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if sess, ok := s.byID[key]; ok {
		return sess, true
	}
	if sess, ok := s.byMachine[key]; ok {
		return sess, true
	}
	sess, ok := s.byCode[key]
	return sess, ok
}

// MarkOffline records an agent disconnect on a machine-identified session,
// which is retained so browsers can reattach when the agent returns.
func (s *SessionStore) MarkOffline(sess *Session) {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	sess.AgentConn = nil
	sess.Online = false
	sess.LastSeen = time.Now()
}

func (s *SessionStore) FindByCode(code string) (*Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.byCode[code]
	return sess, ok
}

func (s *SessionStore) Delete(sess *Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.byCode, sess.PairCode)
	delete(s.byID, sess.ID)
	if sess.MachineID != "" {
		delete(s.byMachine, sess.MachineID)
	}
}

func (s *SessionStore) AddBrowser(code string, conn *SafeConn) (*Session, bool) {
	s.mu.RLock()
	sess, ok := s.byCode[code]
	s.mu.RUnlock()
	if !ok {
		return nil, false
	}
	s.Attach(sess, conn)
	return sess, true
}

// Attach adds a browser connection to an already-resolved session.
func (s *SessionStore) Attach(sess *Session, conn *SafeConn) {
	sess.mu.Lock()
	sess.Browsers = append(sess.Browsers, conn)
	if sess.PairedAt.IsZero() {
		sess.PairedAt = time.Now()
	}
	sess.mu.Unlock()
}

func (s *SessionStore) RemoveBrowser(sess *Session, conn *SafeConn) {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	for i, b := range sess.Browsers {
		if b == conn {
			sess.Browsers = append(sess.Browsers[:i], sess.Browsers[i+1:]...)
			return
		}
	}
}

// SessionInfo is a lock-free snapshot of a session for the /api/sessions endpoint.
type SessionInfo struct {
	ID           string    `json:"id"`
	Code         string    `json:"code"`
	MachineID    string    `json:"machineId,omitempty"`
	Hostname     string    `json:"hostname"`
	Scope        []string  `json:"scope"`
	Online       bool      `json:"online"`
	NodeCount    int       `json:"nodeCount"`
	BrowserCount int       `json:"browserCount"`
	Paired       bool      `json:"paired"`
	PairedAt     time.Time `json:"pairedAt"`
	LastSeen     time.Time `json:"lastSeen"`
	Local        bool      `json:"local"`
}

// List snapshots all sessions. The caller marks the local session.
func (s *SessionStore) List() []SessionInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]SessionInfo, 0, len(s.byID))
	for _, sess := range s.byID {
		sess.mu.RLock()
		out = append(out, SessionInfo{
			ID:           sess.ID,
			Code:         sess.PairCode,
			MachineID:    sess.MachineID,
			Hostname:     sess.Hostname,
			Scope:        sess.Scope,
			Online:       sess.Online,
			NodeCount:    sess.NodeCount,
			BrowserCount: len(sess.Browsers),
			Paired:       !sess.PairedAt.IsZero(),
			PairedAt:     sess.PairedAt,
			LastSeen:     sess.LastSeen,
		})
		sess.mu.RUnlock()
	}
	return out
}

func (s *SessionStore) ActiveCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.byCode)
}

func cryptoRandN(n int) int {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	return int(binary.LittleEndian.Uint64(b[:]) % uint64(n))
}

func generatePairCode() string {
	w1 := pairWords[cryptoRandN(len(pairWords))]
	w2 := pairWords[cryptoRandN(len(pairWords))]
	num := cryptoRandN(900000) + 100000
	return fmt.Sprintf("%s-%s-%06d", w1, w2, num)
}
