package agent

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"infracanvas/pkg/actions"
)

func TestMapFrontendActionTypeLXDContainerActions(t *testing.T) {
	tests := []struct {
		frontend  string
		wantType  actions.ActionType
		wantLayer string
	}{
		{frontend: "lxd_restart_container", wantType: actions.ActionRestartContainer, wantLayer: "lxd"},
		{frontend: "lxd_stop_container", wantType: actions.ActionStopContainer, wantLayer: "lxd"},
		{frontend: "lxd_start_container", wantType: actions.ActionStartContainer, wantLayer: "lxd"},
	}

	for _, tt := range tests {
		gotType, gotLayer := mapFrontendActionType(tt.frontend)
		if gotType != tt.wantType || gotLayer != tt.wantLayer {
			t.Fatalf("%s => (%s,%s), want (%s,%s)", tt.frontend, gotType, gotLayer, tt.wantType, tt.wantLayer)
		}
	}
}

func TestExecInputQueuesPreserveOrder(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	a := &WSAgent{}
	a.execSessions.Store("s1", &execSession{ptmx: w})

	q := newExecInputQueues(a)
	var want strings.Builder
	for i := 0; i < 300; i++ {
		chunk := fmt.Sprintf("%d,", i)
		want.WriteString(chunk)
		data, _ := json.Marshal(map[string]string{
			"session_id": "s1",
			"data":       base64.StdEncoding.EncodeToString([]byte(chunk)),
		})
		q.enqueue(data)
	}

	got := make([]byte, want.Len())
	if _, err := io.ReadFull(r, got); err != nil {
		t.Fatal(err)
	}
	q.closeAll()
	_ = w.Close()

	if string(got) != want.String() {
		t.Fatalf("terminal input reordered:\n got %q\nwant %q", got, want.String())
	}
}
