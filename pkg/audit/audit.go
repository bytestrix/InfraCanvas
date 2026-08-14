// Package audit records every write action and terminal session requested
// through the dashboard to an append-only local log — so "what changed, when,
// and from where" has a real answer instead of just being whatever's still in
// the agent's own stdout.
//
// OSS has no per-user login (one shared UI token authenticates the whole
// dashboard), so entries are attributed by session/machine, not by user
// identity — that's a real, honest limit of what can be logged here, not an
// oversight. The hosted SaaS product has real user accounts and its own
// per-user audit log; this package is OSS-only.
package audit

import (
	"bufio"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"infracanvas/pkg/runstate"
)

// Entry is one immutable audit event. A single logical action produces two
// entries — "requested" when the browser asks for it, "completed" when the
// agent's result comes back — correlated by ActionID. Never mutate a past
// entry; log a new one instead, same principle as the log file itself being
// append-only.
type Entry struct {
	Timestamp time.Time `json:"timestamp"`
	Event     string    `json:"event"` // "action_requested" | "action_completed" | "exec_requested"
	ActionID  string    `json:"action_id,omitempty"`
	Type      string    `json:"type,omitempty"` // action type, e.g. "restart_service"
	MachineID string    `json:"machine_id,omitempty"`
	Hostname  string    `json:"hostname,omitempty"`
	EntityID  string    `json:"entity_id,omitempty"`
	Namespace string    `json:"namespace,omitempty"`
	Success   *bool     `json:"success,omitempty"`
	Message   string    `json:"message,omitempty"`
}

const maxInMemory = 500

var (
	mu     sync.Mutex
	recent []Entry // ring buffer, newest last
	file   *os.File
)

func logPath() string {
	return filepath.Join(runstate.Dir(), "audit.log")
}

// Init opens the audit log for appending. Safe to call multiple times.
// Failure to open is logged but never fatal — a missing audit trail
// shouldn't take down the dashboard.
func Init() {
	mu.Lock()
	defer mu.Unlock()
	if file != nil {
		return
	}
	if err := os.MkdirAll(runstate.Dir(), 0o700); err != nil {
		log.Printf("[audit] mkdir: %v", err)
		return
	}
	f, err := os.OpenFile(logPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		log.Printf("[audit] open: %v", err)
		return
	}
	file = f
	loadRecent()
}

// loadRecent seeds the in-memory ring buffer from the tail of the log file
// on startup, so GET /api/audit has history immediately after a restart
// instead of starting empty.
func loadRecent() {
	f, err := os.Open(logPath())
	if err != nil {
		return
	}
	defer f.Close()
	var all []Entry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		var e Entry
		if json.Unmarshal(sc.Bytes(), &e) == nil {
			all = append(all, e)
		}
	}
	if len(all) > maxInMemory {
		all = all[len(all)-maxInMemory:]
	}
	recent = all
}

func write(e Entry) {
	e.Timestamp = time.Now().UTC()
	mu.Lock()
	defer mu.Unlock()

	recent = append(recent, e)
	if len(recent) > maxInMemory {
		recent = recent[len(recent)-maxInMemory:]
	}

	if file == nil {
		return
	}
	b, err := json.Marshal(e)
	if err != nil {
		return
	}
	b = append(b, '\n')
	if _, err := file.Write(b); err != nil {
		log.Printf("[audit] write: %v", err)
	}
}

// LogActionRequested records a write action as it's forwarded to the agent
// — before the result is known, so a crash mid-action still leaves a trail.
func LogActionRequested(actionID, actionType, machineID, hostname, entityID, namespace string) {
	write(Entry{
		Event: "action_requested", ActionID: actionID, Type: actionType,
		MachineID: machineID, Hostname: hostname, EntityID: entityID, Namespace: namespace,
	})
}

// LogActionCompleted records the outcome of a previously requested action.
func LogActionCompleted(actionID string, success bool, message string) {
	write(Entry{Event: "action_completed", ActionID: actionID, Success: &success, Message: message})
}

// LogActionBlocked records a write action or exec session that read-only
// mode refused to even forward — "attempted but blocked" is still worth a
// record, distinct from "never attempted."
func LogActionBlocked(event, actionType, machineID, hostname string) {
	f := false
	write(Entry{Event: event, Type: actionType, MachineID: machineID, Hostname: hostname, Success: &f, Message: "blocked by read-only mode"})
}

// LogExecRequested records a terminal/exec session being opened — sensitive
// even though "opening a terminal" itself isn't a mutation, since anything
// typed into it is unaudited by nature (it's a raw shell), so at minimum the
// fact that one was opened, by whom, and against what, should be on record.
func LogExecRequested(machineID, hostname, entityID, namespace string) {
	write(Entry{Event: "exec_requested", MachineID: machineID, Hostname: hostname, EntityID: entityID, Namespace: namespace})
}

// Recent returns up to limit most-recent entries, newest first.
func Recent(limit int) []Entry {
	mu.Lock()
	defer mu.Unlock()
	if limit <= 0 || limit > len(recent) {
		limit = len(recent)
	}
	out := make([]Entry, limit)
	for i := 0; i < limit; i++ {
		out[i] = recent[len(recent)-1-i]
	}
	return out
}
