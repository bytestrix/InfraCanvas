package server

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
)

// TestExecDataDeliveredInOrder guards against the regression this file was
// added to fix: broadcastToBrowsers used to spawn a new, unordered goroutine
// per browser per message. SafeConn's mutex only prevented concurrent writes
// from corrupting a single WriteMessage call — it never guaranteed *ordering*
// across racing goroutines. A burst of same-size EXEC_DATA chunks (typical of
// a fast-printing terminal command) could therefore reach the browser out of
// sequence, which is exactly the "text doesn't come properly" symptom. This
// test fires many EXEC_DATA messages back-to-back and asserts they arrive at
// the browser in the exact order the agent sent them.
func TestExecDataDeliveredInOrder(t *testing.T) {
	s := NewWithOptions(Options{LocalMode: true})
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)

	agent := dialWS(t, srv, "/ws/agent")
	t.Cleanup(func() { agent.Close() })
	sendEnvelope(t, agent, MsgHello, HelloData{Hostname: "demo-vm", Scope: []string{"docker"}})
	readUntil(t, agent, MsgPairCode)

	browser := dialWS(t, srv, "/ws/canvas")
	t.Cleanup(func() { browser.Close() })
	sendEnvelope(t, browser, "PAIR", PairRequest{Code: "local"})
	readUntil(t, browser, MsgAgentConnected)

	const n = 500
	for i := 0; i < n; i++ {
		sendEnvelope(t, agent, MsgExecData, map[string]string{
			"session_id": "sess-1",
			"data":       fmt.Sprintf("chunk-%04d", i),
		})
	}

	for i := 0; i < n; i++ {
		env := readUntil(t, browser, MsgExecData)
		var d struct {
			Data string `json:"data"`
		}
		if err := json.Unmarshal(env.Data, &d); err != nil {
			t.Fatalf("unmarshal EXEC_DATA #%d: %v", i, err)
		}
		want := fmt.Sprintf("chunk-%04d", i)
		if d.Data != want {
			t.Fatalf("out-of-order delivery at index %d: got %q, want %q", i, d.Data, want)
		}
	}
}
