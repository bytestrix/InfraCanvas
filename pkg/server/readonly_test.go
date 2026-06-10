package server

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// dialWS connects a websocket client to the test server at the given path.
func dialWS(t *testing.T, srv *httptest.Server, path string) *websocket.Conn {
	t.Helper()
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + path
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial %s: %v", path, err)
	}
	return conn
}

func readEnvelope(t *testing.T, conn *websocket.Conn) Envelope {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	var env Envelope
	if err := conn.ReadJSON(&env); err != nil {
		t.Fatalf("read envelope: %v", err)
	}
	return env
}

// readUntil reads envelopes until one of the wanted type arrives.
func readUntil(t *testing.T, conn *websocket.Conn, msgType string) Envelope {
	t.Helper()
	for i := 0; i < 10; i++ {
		env := readEnvelope(t, conn)
		if env.Type == msgType {
			return env
		}
	}
	t.Fatalf("never received %s", msgType)
	return Envelope{}
}

func sendEnvelope(t *testing.T, conn *websocket.Conn, msgType string, data interface{}) {
	t.Helper()
	payload, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := conn.WriteJSON(Envelope{Type: msgType, Data: payload}); err != nil {
		t.Fatalf("write %s: %v", msgType, err)
	}
}

// setupReadOnly boots a read-only local-mode server with a connected fake
// agent and a paired browser.
func setupReadOnly(t *testing.T) (agent, browser *websocket.Conn) {
	t.Helper()
	s := NewWithOptions(Options{LocalMode: true, ReadOnly: true})
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)

	agent = dialWS(t, srv, "/ws/agent")
	t.Cleanup(func() { agent.Close() })
	readUntil(t, agent, MsgPairCode)
	sendEnvelope(t, agent, MsgHello, HelloData{Hostname: "demo-vm", Scope: []string{"docker"}})

	browser = dialWS(t, srv, "/ws/canvas")
	t.Cleanup(func() { browser.Close() })

	env := readUntil(t, browser, MsgAgentConnected)
	var d AgentConnectedData
	if err := json.Unmarshal(env.Data, &d); err != nil {
		t.Fatalf("unmarshal AGENT_CONNECTED: %v", err)
	}
	if !d.ReadOnly {
		t.Fatal("AGENT_CONNECTED should advertise readOnly=true")
	}
	return agent, browser
}

func TestReadOnlyBlocksActions(t *testing.T) {
	_, browser := setupReadOnly(t)

	sendEnvelope(t, browser, MsgBrowserAction, map[string]interface{}{
		"action_id": "a1",
		"type":      "docker_restart_container",
	})

	env := readUntil(t, browser, MsgActionResult)
	var res struct {
		ActionID string `json:"action_id"`
		Success  bool   `json:"success"`
		Error    string `json:"error"`
	}
	if err := json.Unmarshal(env.Data, &res); err != nil {
		t.Fatalf("unmarshal ACTION_RESULT: %v", err)
	}
	if res.ActionID != "a1" || res.Success || res.Error == "" {
		t.Fatalf("expected failed result for a1 with error, got %+v", res)
	}
}

func TestReadOnlyBlocksExec(t *testing.T) {
	_, browser := setupReadOnly(t)

	sendEnvelope(t, browser, MsgExecStart, map[string]interface{}{
		"session_id": "s1",
		"cmd":        "/bin/sh",
	})

	data := readUntil(t, browser, MsgExecData)
	var ed struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(data.Data, &ed); err != nil || ed.SessionID != "s1" {
		t.Fatalf("expected EXEC_DATA for s1, got %s err=%v", data.Data, err)
	}
	end := readUntil(t, browser, MsgExecEnd)
	if err := json.Unmarshal(end.Data, &ed); err != nil || ed.SessionID != "s1" {
		t.Fatalf("expected EXEC_END for s1, got %s err=%v", end.Data, err)
	}
}

func TestReadOnlyAllowsLogs(t *testing.T) {
	agent, browser := setupReadOnly(t)

	sendEnvelope(t, browser, MsgBrowserAction, map[string]interface{}{
		"action_id": "logs1",
		"type":      "docker_logs",
	})

	// The relay should translate it to ACTION_REQUEST and forward to the agent.
	env := readUntil(t, agent, MsgActionRequest)
	var req struct {
		ActionID string `json:"action_id"`
		Type     string `json:"type"`
	}
	if err := json.Unmarshal(env.Data, &req); err != nil {
		t.Fatalf("unmarshal ACTION_REQUEST: %v", err)
	}
	if req.ActionID != "logs1" || req.Type != "docker_logs" {
		t.Fatalf("expected docker_logs forwarded, got %+v", req)
	}
}
