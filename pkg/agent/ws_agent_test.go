package agent

import (
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
