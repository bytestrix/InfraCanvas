package dockerhost

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestResolve_RespectsDockerHost(t *testing.T) {
	t.Setenv("DOCKER_HOST", "tcp://example.invalid:2375")
	if got := Resolve(); got != "" {
		t.Fatalf("expected empty (caller handles DOCKER_HOST itself), got %q", got)
	}
}

func TestResolve_NoSocketsPresent(t *testing.T) {
	t.Setenv("DOCKER_HOST", "")
	// isSocket() on paths that don't exist should just fail closed.
	if got := Resolve(); got != "" {
		t.Fatalf("expected empty when nothing is listening, got %q", got)
	}
}

func TestIsSocket(t *testing.T) {
	dir := t.TempDir()

	regularFile := filepath.Join(dir, "not-a-socket")
	if err := os.WriteFile(regularFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if isSocket(regularFile) {
		t.Fatal("regular file should not be reported as a socket")
	}

	if isSocket(filepath.Join(dir, "does-not-exist")) {
		t.Fatal("missing path should not be reported as a socket")
	}

	sockPath := filepath.Join(dir, "test.sock")
	l, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	if !isSocket(sockPath) {
		t.Fatal("real unix socket should be reported as a socket")
	}
}
