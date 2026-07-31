package agent

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"

	"infracanvas/pkg/runstate"
)

// MachineID returns a stable identifier for this machine, generated once and
// persisted in the runtime state directory. It lets the relay recognize a
// reconnecting agent and resume its session instead of creating a new one.
// Any failure returns "" — the agent then falls back to per-connection
// pair-code identity, which always works.
func MachineID() string {
	path := filepath.Join(runstate.Dir(), "machine-id")
	if b, err := os.ReadFile(path); err == nil {
		if id := strings.TrimSpace(string(b)); id != "" {
			return id
		}
	}

	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	id := hex.EncodeToString(buf)

	if err := os.MkdirAll(runstate.Dir(), 0o755); err != nil {
		return ""
	}
	if err := os.WriteFile(path, []byte(id+"\n"), 0o600); err != nil {
		return ""
	}
	return id
}
