package kubernetes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"k8s.io/client-go/rest"
)

// fakeAPIServer answers /version, returns empty lists for paths whose last
// segment is in allowed, and 403 for every other list call.
func fakeAPIServer(t *testing.T, allowed ...string) *httptest.Server {
	t.Helper()
	ok := map[string]bool{}
	for _, a := range allowed {
		ok[a] = true
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/version" {
			_, _ = w.Write([]byte(`{"major":"1","minor":"31","gitVersion":"v1.31.0"}`))
			return
		}
		parts := strings.Split(strings.TrimSuffix(r.URL.Path, "/"), "/")
		if ok[parts[len(parts)-1]] {
			_, _ = w.Write([]byte(`{"items":[]}`))
			return
		}
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"kind":"Status","apiVersion":"v1","status":"Failure","reason":"Forbidden","code":403}`))
	}))
}

func TestDiscoverAllSkipsForbiddenResources(t *testing.T) {
	srv := fakeAPIServer(t, "pods", "deployments", "services")
	defer srv.Close()

	d, err := NewDiscoveryFromConfig(&rest.Config{Host: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	cluster, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, err := d.DiscoverAll()
	if err != nil {
		t.Fatalf("DiscoverAll with forbidden nodes/secrets should succeed, got %v", err)
	}
	if cluster == nil || cluster.Version != "v1.31.0" {
		t.Fatalf("unexpected cluster: %+v", cluster)
	}
	skipped := d.Skipped()
	for _, want := range []string{"nodes", "secrets", "persistentvolumes"} {
		if !contains(skipped, want) {
			t.Errorf("expected %q in skipped, got %v", want, skipped)
		}
	}
	if contains(skipped, "pods") {
		t.Errorf("pods should not be skipped: %v", skipped)
	}
}

func TestDiscoverAllFailsWhenNoWorkloadsReadable(t *testing.T) {
	srv := fakeAPIServer(t)
	defer srv.Close()

	d, err := NewDiscoveryFromConfig(&rest.Config{Host: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, err := d.DiscoverAll(); err == nil {
		t.Fatal("expected an error when pods and deployments are both forbidden")
	}
}

func TestPingUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()

	d, err := NewDiscoveryFromConfig(&rest.Config{Host: url})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Ping(); err == nil || !strings.Contains(err.Error(), url) {
		t.Fatalf("expected error naming %s, got %v", url, err)
	}
}
