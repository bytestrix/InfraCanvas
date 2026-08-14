package agent

import (
	"os"
	"path/filepath"
	"strings"

	"infracanvas/pkg/runstate"
)

// loadResumeSecret reads the locally persisted resume secret, or "" if none
// has been issued yet (first-ever connect). Mirrors MachineID()'s pattern —
// a real VM agent needs this to survive process restarts (e.g. a reboot)
// so it can keep resuming the same server-side session instead of the
// server treating every reboot as a brand-new, unauthenticated machine.
func loadResumeSecret() string {
	b, err := os.ReadFile(filepath.Join(runstate.Dir(), "resume-secret"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// saveResumeSecret persists a newly issued resume secret. Best-effort: if it
// fails, the agent still works, it just won't be able to prove ownership of
// its MachineID on the next process restart (the server will treat it as a
// new machine and issue a fresh secret then — a safe fallback, not a hole).
func saveResumeSecret(secret string) {
	path := filepath.Join(runstate.Dir(), "resume-secret")
	if err := os.MkdirAll(runstate.Dir(), 0o700); err != nil {
		return
	}
	_ = os.WriteFile(path, []byte(secret+"\n"), 0o600)
}
