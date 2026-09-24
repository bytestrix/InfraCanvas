package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"infracanvas/pkg/clustermgr"
)

func postJSON(t *testing.T, url, body string, header map[string]string) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range header {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

// Cluster routes run kubeconfig auth on the hub, so the join token that
// every joined VM holds must not reach them.
func TestClusterRoutesRejectAgentToken(t *testing.T) {
	s := NewWithOptions(Options{LocalMode: true, UIToken: "ui", AgentToken: "agent"})
	s.SetClusterManager(clustermgr.NewManager("ws://127.0.0.1:1", "agent", 30))
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	bearer := map[string]string{"Authorization": "Bearer agent"}
	for _, path := range []string{"/api/clusters", "/api/clusters/preview"} {
		if code := postJSON(t, srv.URL+path, `{"kubeconfig":"x"}`, bearer); code != http.StatusUnauthorized {
			t.Errorf("%s with agent token: got %d, want 401", path, code)
		}
	}
}

func TestReadOnlyBlocksClusterPreview(t *testing.T) {
	s := NewWithOptions(Options{LocalMode: true, UIToken: "ui", ReadOnly: true})
	s.SetClusterManager(clustermgr.NewManager("ws://127.0.0.1:1", "", 30))
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	code := postJSON(t, srv.URL+"/api/clusters/preview?token=ui", `{"kubeconfig":"apiVersion: v1","context":"x"}`, nil)
	if code != http.StatusForbidden {
		t.Fatalf("preview in read-only mode: got %d, want 403", code)
	}
}
