package clustermgr

import (
	"errors"
	"testing"

	"k8s.io/client-go/rest"

	"infracanvas/pkg/runstate"
)

const testKubeconfig = `apiVersion: v1
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
- name: other
  context:
    cluster: dev
    user: dev
users:
- name: dev
  user:
    token: abc
`

func stubPing(t *testing.T, err error) {
	t.Helper()
	orig := pingCluster
	pingCluster = func(*rest.Config) error { return err }
	t.Cleanup(func() { pingCluster = orig })
}

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	t.Setenv("INFRACANVAS_STATE_DIR", t.TempDir())
	m := NewManager("ws://127.0.0.1:1", "tok", 30)
	t.Cleanup(func() {
		entries, _ := m.List()
		for _, e := range entries {
			_ = m.Remove(e.ID)
		}
	})
	return m
}

func TestParseContexts(t *testing.T) {
	ctxs, err := ParseContexts([]byte(testKubeconfig))
	if err != nil {
		t.Fatal(err)
	}
	if len(ctxs) != 2 {
		t.Fatalf("want 2 contexts, got %d", len(ctxs))
	}
	for _, c := range ctxs {
		if c.ServerURL != "https://127.0.0.1:1" {
			t.Errorf("context %s: server %q", c.Name, c.ServerURL)
		}
		if c.Current != (c.Name == "dev") {
			t.Errorf("context %s: current=%v", c.Name, c.Current)
		}
	}
	if _, err := ParseContexts([]byte("not yaml: [")); err == nil {
		t.Error("expected error for invalid kubeconfig")
	}
}

func TestAddUnreachableClusterIsNotPersisted(t *testing.T) {
	m := newTestManager(t)
	stubPing(t, errors.New("connection refused"))

	if _, err := m.Add("", []byte(testKubeconfig), "dev", false); err == nil {
		t.Fatal("expected Add to fail for an unreachable cluster")
	}
	s, err := runstate.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Clusters) != 0 {
		t.Fatalf("unreachable cluster was persisted: %+v", s.Clusters)
	}
}

func TestAddSameContextTwiceUpdatesInPlace(t *testing.T) {
	m := newTestManager(t)
	stubPing(t, nil)

	first, err := m.Add("", []byte(testKubeconfig), "dev", false)
	if err != nil {
		t.Fatal(err)
	}
	second, err := m.Add("renamed", []byte(testKubeconfig), "dev", true)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID {
		t.Fatalf("re-adding the same context created a new entry: %s vs %s", first.ID, second.ID)
	}

	entries, err := m.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 cluster, got %d", len(entries))
	}
	if entries[0].Name != "renamed" || !entries[0].ReadOnly {
		t.Errorf("entry not updated: %+v", entries[0])
	}

	// A different context on the same server is a different cluster entry.
	if _, err := m.Add("", []byte(testKubeconfig), "other", false); err != nil {
		t.Fatal(err)
	}
	entries, _ = m.List()
	if len(entries) != 2 {
		t.Fatalf("want 2 clusters, got %d", len(entries))
	}
}

func TestAddManyConnectsEachContext(t *testing.T) {
	m := newTestManager(t)
	stubPing(t, nil)

	// "dev" twice: duplicates are connected once.
	added, failed, err := m.AddMany([]byte(testKubeconfig), []string{"dev", "other", "dev"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(failed) != 0 {
		t.Fatalf("unexpected failures: %+v", failed)
	}
	if len(added) != 2 {
		t.Fatalf("want 2 added, got %d", len(added))
	}

	entries, err := m.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 clusters, got %d", len(entries))
	}
	ids := map[string]bool{}
	for _, e := range entries {
		if e.Name != e.ContextName {
			t.Errorf("entry %s: name %q, want its context name", e.ID, e.Name)
		}
		if !e.ReadOnly {
			t.Errorf("entry %s: readOnly not applied", e.ContextName)
		}
		ids[e.ID] = true
	}
	if len(ids) != 2 {
		t.Errorf("entries share an ID: %+v", entries)
	}

	// Adding the same contexts again updates in place, like Add.
	if _, _, err := m.AddMany([]byte(testKubeconfig), []string{"dev", "other"}, false); err != nil {
		t.Fatal(err)
	}
	if entries, _ := m.List(); len(entries) != 2 {
		t.Fatalf("re-adding created duplicates: %d entries", len(entries))
	}
}

func TestAddManyReportsFailuresPerContext(t *testing.T) {
	m := newTestManager(t)
	stubPing(t, nil)

	added, failed, err := m.AddMany([]byte(testKubeconfig), []string{"dev", "missing"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(added) != 1 || added[0].ContextName != "dev" {
		t.Fatalf("want only dev added, got %+v", added)
	}
	if len(failed) != 1 || failed[0].Context != "missing" || failed[0].Error == "" {
		t.Fatalf("want one failure for missing, got %+v", failed)
	}
}

func TestAddManyUnreachableIsNotPersisted(t *testing.T) {
	m := newTestManager(t)
	stubPing(t, errors.New("connection refused"))

	added, failed, err := m.AddMany([]byte(testKubeconfig), []string{"dev", "other"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if added == nil || len(added) != 0 {
		t.Fatalf("want empty, non-nil added, got %#v", added)
	}
	if len(failed) != 2 {
		t.Fatalf("want 2 failures, got %+v", failed)
	}
	s, err := runstate.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Clusters) != 0 {
		t.Fatalf("unreachable clusters were persisted: %+v", s.Clusters)
	}

	if _, _, err := m.AddMany([]byte("not yaml: ["), []string{"dev"}, false); err == nil {
		t.Error("expected error for invalid kubeconfig")
	}
}
