package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"infracanvas/pkg/clustermgr"
)

// Points at a port nothing listens on, so connecting fails right away.
const unreachableKubeconfig = `apiVersion: v1
kind: Config
current-context: dev
clusters:
- name: dev
  cluster:
    server: https://127.0.0.1:1
contexts:
- name: dev
  context:
    cluster: dev
    user: dev
users:
- name: dev
  user:
    token: abc
`

func newClustersServer(t *testing.T, readOnly bool) *httptest.Server {
	t.Helper()
	t.Setenv("INFRACANVAS_STATE_DIR", t.TempDir())
	s := NewWithOptions(Options{LocalMode: true, UIToken: "ui", ReadOnly: readOnly})
	s.SetClusterManager(clustermgr.NewManager("ws://127.0.0.1:1", "", 30))
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)
	return srv
}

func postClusters(t *testing.T, srv *httptest.Server, body map[string]interface{}) (int, string) {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Post(srv.URL+"/api/clusters?token=ui", "application/json", strings.NewReader(string(b)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(out)
}

func TestAddClustersRejectsInvalidContexts(t *testing.T) {
	srv := newClustersServer(t, false)
	cases := map[string]map[string]interface{}{
		"context and contexts": {"kubeconfig": unreachableKubeconfig, "context": "dev", "contexts": []string{"dev"}},
		"name with contexts":   {"kubeconfig": unreachableKubeconfig, "name": "x", "contexts": []string{"dev"}},
		"empty context name":   {"kubeconfig": unreachableKubeconfig, "contexts": []string{"dev", ""}},
	}
	for name, body := range cases {
		if code, _ := postClusters(t, srv, body); code != http.StatusBadRequest {
			t.Errorf("%s: got %d, want 400", name, code)
		}
	}
}

func TestReadOnlyBlocksAddingContexts(t *testing.T) {
	srv := newClustersServer(t, true)
	code, _ := postClusters(t, srv, map[string]interface{}{"kubeconfig": unreachableKubeconfig, "contexts": []string{"dev"}})
	if code != http.StatusForbidden {
		t.Fatalf("contexts in read-only mode: got %d, want 403", code)
	}
}

func TestAddClustersReportsEachFailure(t *testing.T) {
	srv := newClustersServer(t, false)
	code, body := postClusters(t, srv, map[string]interface{}{
		"kubeconfig": unreachableKubeconfig,
		"contexts":   []string{"dev", "missing"},
	})
	if code != http.StatusBadRequest {
		t.Fatalf("nothing added: got %d, want 400 (body %s)", code, body)
	}
	var res struct {
		Added  []json.RawMessage `json:"added"`
		Failed []struct {
			Context string `json:"context"`
			Error   string `json:"error"`
		} `json:"failed"`
	}
	if err := json.Unmarshal([]byte(body), &res); err != nil {
		t.Fatalf("response is not JSON: %v (%s)", err, body)
	}
	if res.Added == nil || len(res.Added) != 0 {
		t.Errorf("want added to be an empty list, got %s", body)
	}
	if len(res.Failed) != 2 || res.Failed[0].Context != "dev" || res.Failed[1].Context != "missing" {
		t.Fatalf("want failures for dev and missing in order, got %+v", res.Failed)
	}
	for _, f := range res.Failed {
		if f.Error == "" {
			t.Errorf("failure for %s has no error text", f.Context)
		}
	}
}
