// Package runstate persists per-process facts that other commands (or a human
// running `infracanvas url`) need to recover after the launching shell is gone.
//
// The most important one is the live Cloudflare quick-tunnel URL: it changes
// every time cloudflared respawns, so the URL printed at install time goes
// stale on the first hiccup. Writing it here lets `infracanvas url` always
// report the current one.
package runstate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// State is the on-disk snapshot. Fields are optional; readers tolerate zero values.
type State struct {
	TunnelURL  string         `json:"tunnel_url,omitempty"`
	Port       int            `json:"port,omitempty"`
	Token      string         `json:"token,omitempty"`
	AgentToken string         `json:"agent_token,omitempty"`
	Clusters   []ClusterEntry `json:"clusters,omitempty"`
	UpdatedAt  time.Time      `json:"updated_at,omitempty"`
}

// ClusterEntry records a Clusters connection (an uploaded kubeconfig context)
// persisted across `infracanvas serve` restarts. Only metadata lives here —
// the actual kubeconfig bytes are copied to KubeconfigPath (0600), never
// inlined in state.json.
type ClusterEntry struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	ContextName    string    `json:"context_name"`
	ServerURL      string    `json:"server_url,omitempty"`
	KubeconfigPath string    `json:"kubeconfig_path"`
	AddedAt        time.Time `json:"added_at"`
}

// Dir returns the directory used to hold runtime state.
//
// Resolution order:
//  1. $INFRACANVAS_STATE_DIR if set
//  2. $STATE_DIRECTORY (set by systemd when StateDirectory= is in the unit)
//  3. /var/lib/infracanvas if it already exists or can be created
//  4. user cache dir (~/.cache/infracanvas)
func Dir() string {
	if d := os.Getenv("INFRACANVAS_STATE_DIR"); d != "" {
		return d
	}
	if d := os.Getenv("STATE_DIRECTORY"); d != "" {
		return d
	}
	const sysDir = "/var/lib/infracanvas"
	if info, err := os.Stat(sysDir); err == nil && info.IsDir() {
		return sysDir
	}
	if base, err := os.UserCacheDir(); err == nil {
		return filepath.Join(base, "infracanvas")
	}
	return "/tmp/infracanvas"
}

// Path returns the absolute path to the state file.
func Path() string { return filepath.Join(Dir(), "state.json") }

// Read loads the state file, returning a zero State if it doesn't exist.
func Read() (State, error) {
	var s State
	b, err := os.ReadFile(Path())
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return s, err
	}
	if len(b) == 0 {
		return s, nil
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return s, fmt.Errorf("parse state: %w", err)
	}
	return s, nil
}

// Update merges fields from the provided mutator into the existing state and
// writes atomically. Pass empty values to leave a field untouched.
func Update(mut func(*State)) error {
	s, _ := Read()
	mut(&s)
	s.UpdatedAt = time.Now().UTC()
	return write(s)
}

func write(s State) error {
	dir := Dir()
	// 0700, not 0755: State.Token and State.AgentToken are live credentials
	// (the UI token doubles as the SSH-equivalent dashboard key; AgentToken
	// is the hub join secret) — no reason for any other local user to even
	// list this directory, let alone read the file inside it.
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	_ = os.Chmod(dir, 0o700) // tighten if the dir pre-existed with looser perms from an older version
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".state-*.json")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, Path())
}
