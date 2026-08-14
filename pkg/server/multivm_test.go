package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// dialAgent connects a fake agent, sends HELLO (optionally presenting a
// previously issued resumeSecret to reconnect as an existing MachineID),
// and waits for its pair code. Returns the connection, pair code, and
// whatever ResumeSecret the server assigned (only set on a machine-
// identified agent's first-ever connect).
func dialAgent(t *testing.T, srv *httptest.Server, hostname, machineID, resumeSecret string) (*websocket.Conn, string, string) {
	t.Helper()
	conn := dialWS(t, srv, "/ws/agent")
	t.Cleanup(func() { conn.Close() })
	sendEnvelope(t, conn, MsgHello, HelloData{Hostname: hostname, Scope: []string{"docker"}, MachineID: machineID, ResumeSecret: resumeSecret})
	env := readUntil(t, conn, MsgPairCode)
	var pc PairCodeData
	if err := json.Unmarshal(env.Data, &pc); err != nil {
		t.Fatalf("unmarshal PAIR_CODE: %v", err)
	}
	return conn, pc.Code, pc.ResumeSecret
}

// dialAgentExpectRejected attempts a HELLO the server should refuse (e.g. a
// MachineID claim with a wrong/missing resume secret) and asserts the
// connection is closed without ever receiving a PAIR_CODE.
func dialAgentExpectRejected(t *testing.T, srv *httptest.Server, hostname, machineID, resumeSecret string) {
	t.Helper()
	conn := dialWS(t, srv, "/ws/agent")
	defer conn.Close()
	sendEnvelope(t, conn, MsgHello, HelloData{Hostname: hostname, Scope: []string{"docker"}, MachineID: machineID, ResumeSecret: resumeSecret})
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Fatalf("expected connection to be closed/rejected, but it stayed open")
	}
}

// waitFor polls cond for up to 3 seconds.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("condition not met within 3s")
}

func TestMachineSessionResume(t *testing.T) {
	s := NewWithOptions(Options{LocalMode: true})
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)

	agent, _, resumeSecret := dialAgent(t, srv, "vm-a", "machine-aaa", "")
	if resumeSecret == "" {
		t.Fatalf("expected a resume secret to be issued on first connect")
	}

	browser := dialWS(t, srv, "/ws/canvas")
	t.Cleanup(func() { browser.Close() })
	sendEnvelope(t, browser, "PAIR", PairRequest{Code: "local"})
	readUntil(t, browser, MsgAgentConnected)

	// Agent drops: session must survive (machine-identified) and browsers get notified.
	agent.Close()
	readUntil(t, browser, MsgAgentDisconnected)
	if got := s.sessions.ActiveCount(); got != 1 {
		t.Fatalf("machine session should be retained offline, ActiveCount=%d", got)
	}

	// Agent returns with the same machine ID *and* the resume secret it was
	// issued: same session resumes, the already-attached browser sees
	// AGENT_CONNECTED again without re-pairing.
	dialAgent(t, srv, "vm-a", "machine-aaa", resumeSecret)
	env := readUntil(t, browser, MsgAgentConnected)
	var d AgentConnectedData
	_ = json.Unmarshal(env.Data, &d)
	if d.Hostname != "vm-a" {
		t.Fatalf("expected resumed agent hostname vm-a, got %q", d.Hostname)
	}
	if got := s.sessions.ActiveCount(); got != 1 {
		t.Fatalf("resume must not create a second session, ActiveCount=%d", got)
	}
}

// TestMachineSessionHijackRejected is the regression test for the
// disclosed vulnerability: in hub mode every VM shares one join token, so
// that token alone must never be enough to claim another machine's session.
// A second connection claiming an existing MachineID without its resume
// secret must be rejected outright — not silently swapped in as the new
// AgentConn for a session with browsers already attached to it, which
// would hand the attacker the operator's live EXEC_START traffic.
func TestMachineSessionHijackRejected(t *testing.T) {
	s := NewWithOptions(Options{LocalMode: true})
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)

	victim, _, resumeSecret := dialAgent(t, srv, "victim-vm", "machine-victim", "")
	if resumeSecret == "" {
		t.Fatalf("expected a resume secret to be issued on first connect")
	}

	browser := dialWS(t, srv, "/ws/canvas")
	t.Cleanup(func() { browser.Close() })
	sendEnvelope(t, browser, "PAIR", PairRequest{Code: "local"})
	readUntil(t, browser, MsgAgentConnected)

	// Attacker knows the MachineID (e.g. leaked via logs, or from any other
	// channel) and holds a valid shared join token — same as any legitimate
	// hub agent — but does not know the resume secret.
	dialAgentExpectRejected(t, srv, "attacker-vm", "machine-victim", "wrong-secret")
	dialAgentExpectRejected(t, srv, "attacker-vm", "machine-victim", "")

	// The victim's own connection must be completely unaffected: closing it
	// (not the rejected attacker attempt) is what should disconnect the
	// browser, proving the session was never swapped out from under it.
	victim.Close()
	readUntil(t, browser, MsgAgentDisconnected)
	if got := s.sessions.ActiveCount(); got != 1 {
		t.Fatalf("hijack attempt must not create or destroy sessions, ActiveCount=%d", got)
	}
}

func TestLegacyAgentSessionDeletedOnDisconnect(t *testing.T) {
	s := NewWithOptions(Options{LocalMode: true})
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)

	agent, _, _ := dialAgent(t, srv, "old-vm", "", "") // no machine ID = legacy agent
	if got := s.sessions.ActiveCount(); got != 1 {
		t.Fatalf("ActiveCount=%d, want 1", got)
	}
	agent.Close()
	waitFor(t, func() bool { return s.sessions.ActiveCount() == 0 })
}

func TestBrowserPairBySessionAndMachineID(t *testing.T) {
	s := NewWithOptions(Options{LocalMode: true, LocalMachineID: "machine-local"})
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)

	dialAgent(t, srv, "hub-vm", "machine-local", "")
	dialAgent(t, srv, "remote-vm", "machine-remote", "")

	// PAIR by machine ID reaches the remote agent's session.
	browser := dialWS(t, srv, "/ws/canvas")
	t.Cleanup(func() { browser.Close() })
	sendEnvelope(t, browser, "PAIR", PairRequest{Code: "machine-remote"})
	env := readUntil(t, browser, MsgAgentConnected)
	var d AgentConnectedData
	_ = json.Unmarshal(env.Data, &d)
	if d.Hostname != "remote-vm" {
		t.Fatalf("expected remote-vm, got %q", d.Hostname)
	}

	// 'local' still resolves to the hub's own agent even though the remote
	// agent connected after it.
	local := dialWS(t, srv, "/ws/canvas")
	t.Cleanup(func() { local.Close() })
	sendEnvelope(t, local, "PAIR", PairRequest{Code: "local"})
	env = readUntil(t, local, MsgAgentConnected)
	_ = json.Unmarshal(env.Data, &d)
	if d.Hostname != "hub-vm" {
		t.Fatalf("expected hub-vm for 'local', got %q", d.Hostname)
	}
}

func TestRemoteAgentCannotDisplaceLocalSession(t *testing.T) {
	s := NewWithOptions(Options{LocalMode: true, LocalMachineID: "machine-local"})
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)

	// Remote agent connects FIRST — must not become the local session.
	dialAgent(t, srv, "remote-vm", "machine-remote", "")
	dialAgent(t, srv, "hub-vm", "machine-local", "")

	browser := dialWS(t, srv, "/ws/canvas")
	t.Cleanup(func() { browser.Close() })
	sendEnvelope(t, browser, "PAIR", PairRequest{Code: "local"})
	env := readUntil(t, browser, MsgAgentConnected)
	var d AgentConnectedData
	_ = json.Unmarshal(env.Data, &d)
	if d.Hostname != "hub-vm" {
		t.Fatalf("'local' bound to %q, want hub-vm", d.Hostname)
	}
}

func TestAgentWSRequiresToken(t *testing.T) {
	s := NewWithOptions(Options{LocalMode: true, AgentToken: "sekrit"})
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)

	resp, err := http.Get(srv.URL + "/ws/agent")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no-token agent connect: status=%d, want 401", resp.StatusCode)
	}
}

func TestSessionsAPIAuth(t *testing.T) {
	s := NewWithOptions(Options{LocalMode: true, UIToken: "ui-tok", AgentToken: "agent-tok"})
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)

	get := func(mutate func(*http.Request)) int {
		req, _ := http.NewRequest("GET", srv.URL+"/api/sessions", nil)
		if mutate != nil {
			mutate(req)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)
		return resp.StatusCode
	}

	if got := get(nil); got != http.StatusUnauthorized {
		t.Fatalf("unauthenticated: %d, want 401", got)
	}
	if got := get(func(r *http.Request) { r.URL.RawQuery = "token=ui-tok" }); got != http.StatusOK {
		t.Fatalf("UI token: %d, want 200", got)
	}
	// /api/sessions discloses every machine's MachineID — the shared agent
	// token must NOT grant access to it (it's part of what made the session-
	// hijack disclosure practical: an attacker's own valid join token could
	// otherwise be used to enumerate every other machine's MachineID before
	// attempting to claim one). Only the UI token may read it.
	if got := get(func(r *http.Request) { r.Header.Set("Authorization", "Bearer agent-tok") }); got != http.StatusUnauthorized {
		t.Fatalf("agent bearer: %d, want 401 (agent token must not grant access to the session roster)", got)
	}
	if got := get(func(r *http.Request) { r.URL.RawQuery = "token=wrong" }); got != http.StatusUnauthorized {
		t.Fatalf("wrong token: %d, want 401", got)
	}
}
