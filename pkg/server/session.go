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
	AgentConn    *SafeConn
	Browsers     []*SafeConn
	Hostname     string
	Scope        []string
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
	byCode map[string]*Session
	byID   map[string]*Session
	mu     sync.RWMutex
}

func NewSessionStore() *SessionStore {
	return &SessionStore{
		byCode: make(map[string]*Session),
		byID:   make(map[string]*Session),
	}
}

func (s *SessionStore) Create(conn *SafeConn) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()

	code := generatePairCode()
	for _, taken := s.byCode[code]; taken; _, taken = s.byCode[code] {
		code = generatePairCode()
	}

	sess := &Session{
		ID:        fmt.Sprintf("sess-%d", time.Now().UnixNano()),
		PairCode:  code,
		AgentConn: conn,
		Browsers:  make([]*SafeConn, 0),
	}
	s.byCode[code] = sess
	s.byID[sess.ID] = sess
	return sess
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
}

func (s *SessionStore) AddBrowser(code string, conn *SafeConn) (*Session, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.byCode[code]
	if !ok {
		return nil, false
	}
	sess.mu.Lock()
	sess.Browsers = append(sess.Browsers, conn)
	if sess.PairedAt.IsZero() {
		sess.PairedAt = time.Now()
	}
	sess.mu.Unlock()
	return sess, true
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
