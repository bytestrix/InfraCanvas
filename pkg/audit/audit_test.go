package audit

import (
	"bufio"
	"encoding/json"
	"os"
	"testing"
)

func reset() {
	mu.Lock()
	defer mu.Unlock()
	if file != nil {
		_ = file.Close()
	}
	file = nil
	recent = nil
}

func TestRecentNewestFirstAndPersisted(t *testing.T) {
	t.Setenv("INFRACANVAS_STATE_DIR", t.TempDir())
	reset()
	t.Cleanup(reset)
	Init()

	LogActionRequested("a1", "restart_container", "m1", "host1", "c1", "")
	LogActionCompleted("a1", true, "ok")
	LogActionBlocked("action_blocked", "stop_container", "m1", "host1")

	got := Recent(10)
	if len(got) != 3 {
		t.Fatalf("want 3 entries, got %d", len(got))
	}
	if got[0].Event != "action_blocked" || got[2].Event != "action_requested" {
		t.Fatalf("wrong order: %s, %s", got[0].Event, got[2].Event)
	}
	if got[0].Success == nil || *got[0].Success {
		t.Error("blocked action should record success=false")
	}

	f, err := os.Open(logPath())
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	lines := 0
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e Entry
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			t.Fatalf("bad log line %q: %v", sc.Text(), err)
		}
		lines++
	}
	if lines != 3 {
		t.Fatalf("want 3 lines on disk, got %d", lines)
	}
	if fi, _ := os.Stat(logPath()); fi.Mode().Perm() != 0o600 {
		t.Errorf("audit log perm = %o, want 600", fi.Mode().Perm())
	}
}

func TestRecentReloadsAfterRestart(t *testing.T) {
	t.Setenv("INFRACANVAS_STATE_DIR", t.TempDir())
	reset()
	t.Cleanup(reset)
	Init()
	LogExecRequested("m1", "host1", "pod/x", "default")

	reset() // simulate a process restart
	Init()
	got := Recent(0)
	if len(got) != 1 || got[0].Event != "exec_requested" {
		t.Fatalf("history not reloaded: %+v", got)
	}
}
