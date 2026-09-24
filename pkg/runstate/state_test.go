package runstate

import (
	"os"
	"testing"
)

func TestReadMissingFileReturnsZeroState(t *testing.T) {
	t.Setenv("INFRACANVAS_STATE_DIR", t.TempDir())
	s, err := Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Clusters) != 0 {
		t.Fatalf("want empty state, got %+v", s)
	}
}

func TestUpdateRoundTripAndPermissions(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("INFRACANVAS_STATE_DIR", dir)

	if err := Update(func(s *State) {
		s.Clusters = append(s.Clusters, ClusterEntry{ID: "a", Name: "one"})
	}); err != nil {
		t.Fatal(err)
	}
	if err := Update(func(s *State) {
		s.Clusters = append(s.Clusters, ClusterEntry{ID: "b", Name: "two"})
	}); err != nil {
		t.Fatal(err)
	}

	s, err := Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Clusters) != 2 || s.Clusters[0].ID != "a" || s.Clusters[1].ID != "b" {
		t.Fatalf("unexpected clusters: %+v", s.Clusters)
	}
	if s.UpdatedAt.IsZero() {
		t.Error("UpdatedAt not set")
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Errorf("state dir perm = %o, want 700", perm)
	}
	fi, err := os.Stat(Path())
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm&0o077 != 0 {
		t.Errorf("state file readable by others: %o", perm)
	}
}

func TestDirPrefersEnv(t *testing.T) {
	t.Setenv("INFRACANVAS_STATE_DIR", "/custom/dir")
	if got := Dir(); got != "/custom/dir" {
		t.Fatalf("Dir() = %q", got)
	}
}
