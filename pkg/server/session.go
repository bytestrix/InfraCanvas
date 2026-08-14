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
	ID        string
	PairCode  string
	MachineID string // stable agent identity; empty for legacy agents
	// ResumeSecret is issued the first time this MachineID connects and must
	// be presented on every later HELLO claiming the same MachineID before
	// UpsertByMachine will swap that connection in. Without this, the shared
	// hub join token alone "authenticates" any agent to claim any other
	// machine's MachineID and hijack its session — including the browsers
	// already attached to it. Empty for non-machine-identified (legacy)
	// sessions, which never resume by MachineID at all.
	ResumeSecret string
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
// connection, keeping attached browsers) or creates a fresh one issuing a new
// ResumeSecret. The second return value reports whether an existing session
// was found for this machineID; the third reports whether the caller is
// authorized to use it — false means an existing session was found but
// presentedSecret didn't match its ResumeSecret, and the caller must reject
// the connection rather than use the returned Session (which is the real,
// still-attached victim session — returned only so the caller can log which
// machine was targeted, never so it can be connected to).
func (s *SessionStore) UpsertByMachine(conn *SafeConn, machineID, presentedSecret string) (sess *Session, found bool, authorized bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.byMachine[machineID]; ok {
		if existing.ResumeSecret != "" && existing.ResumeSecret != presentedSecret {
			return existing, true, false
		}
		existing.mu.Lock()
		existing.AgentConn = conn
		existing.Online = true
		existing.LastSeen = time.Now()
		existing.mu.Unlock()
		return existing, true, true
	}

	sess = s.createLocked(conn)
	sess.MachineID = machineID
	sess.ResumeSecret = generateResumeSecret()
	s.byMachine[machineID] = sess
	return sess, false, true
}

func generateResumeSecret() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failing is effectively unrecoverable on any real system;
		// an empty secret would disable the check entirely, so panic instead
		// of silently downgrading to "any machine can claim any session."
		panic("resume secret generation failed: " + err.Error())
	}
	return fmt.Sprintf("%x", b)
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
	Kind         string    `json:"kind"` // "machine" | "cluster" — derived from Scope
	Online       bool      `json:"online"`
	NodeCount    int       `json:"nodeCount"`
	BrowserCount int       `json:"browserCount"`
	Paired       bool      `json:"paired"`
	PairedAt     time.Time `json:"pairedAt"`
	LastSeen     time.Time `json:"lastSeen"`
	Local        bool      `json:"local"`
}

// sessionKind derives whether a session is a VM Machine or a Clusters
// connection purely from its reported scope — a session with kubernetes as
// its only scope is a Clusters connection (kubeconfig direct-connect or an
// in-cluster relay pod); anything with host/docker is a Machine. No separate
// wire concept is needed for this.
func sessionKind(scope []string) string {
	if len(scope) == 1 && scope[0] == "kubernetes" {
		return "cluster"
	}
	return "machine"
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
			Kind:         sessionKind(sess.Scope),
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
