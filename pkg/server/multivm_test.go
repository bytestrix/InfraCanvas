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

// dialAgent connects a fake agent, sends HELLO, and waits for its pair code.
func dialAgent(t *testing.T, srv *httptest.Server, hostname, machineID string) (*websocket.Conn, string) {
	t.Helper()
	conn := dialWS(t, srv, "/ws/agent")
	t.Cleanup(func() { conn.Close() })
	sendEnvelope(t, conn, MsgHello, HelloData{Hostname: hostname, Scope: []string{"docker"}, MachineID: machineID})
	env := readUntil(t, conn, MsgPairCode)
	var pc PairCodeData
	if err := json.Unmarshal(env.Data, &pc); err != nil {
		t.Fatalf("unmarshal PAIR_CODE: %v", err)
	}
	return conn, pc.Code
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

	agent, _ := dialAgent(t, srv, "vm-a", "machine-aaa")

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

	// Agent returns with the same machine ID: same session resumes, the
	// already-attached browser sees AGENT_CONNECTED again without re-pairing.
	dialAgent(t, srv, "vm-a", "machine-aaa")
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

func TestLegacyAgentSessionDeletedOnDisconnect(t *testing.T) {
	s := NewWithOptions(Options{LocalMode: true})
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)

	agent, _ := dialAgent(t, srv, "old-vm", "") // no machine ID = legacy agent
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

	dialAgent(t, srv, "hub-vm", "machine-local")
	dialAgent(t, srv, "remote-vm", "machine-remote")

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
	dialAgent(t, srv, "remote-vm", "machine-remote")
	dialAgent(t, srv, "hub-vm", "machine-local")

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
	if got := get(func(r *http.Request) { r.Header.Set("Authorization", "Bearer agent-tok") }); got != http.StatusOK {
		t.Fatalf("agent bearer: %d, want 200", got)
	}
	if got := get(func(r *http.Request) { r.URL.RawQuery = "token=wrong" }); got != http.StatusUnauthorized {
		t.Fatalf("wrong token: %d, want 401", got)
	}
}
