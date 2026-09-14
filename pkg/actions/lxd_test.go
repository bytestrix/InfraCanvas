package actions

import "testing"

func TestLXDOperationID(t *testing.T) {
	got := lxdOperationID("/1.0/operations/abcd-1234/")
	if got != "abcd-1234" {
		t.Fatalf("lxdOperationID() = %q, want %q", got, "abcd-1234")
	}
}

func TestLXDExecSecrets(t *testing.T) {
	t.Run("with control channel", func(t *testing.T) {
		data, control := lxdExecSecrets(map[string]string{
			"0":       "data-secret",
			"control": "control-secret",
		})
		if data != "data-secret" || control != "control-secret" {
			t.Fatalf("got (%q,%q), want (%q,%q)", data, control, "data-secret", "control-secret")
		}
	})

	t.Run("fallback to fd1 without control", func(t *testing.T) {
		data, control := lxdExecSecrets(map[string]string{
			"1": "data-secret",
		})
		if data != "data-secret" || control != "" {
			t.Fatalf("got (%q,%q), want (%q,%q)", data, control, "data-secret", "")
		}
	})

	t.Run("nil map", func(t *testing.T) {
		data, control := lxdExecSecrets(nil)
		if data != "" || control != "" {
			t.Fatalf("got (%q,%q), want (%q,%q)", data, control, "", "")
		}
	})

	t.Run("missing data fds", func(t *testing.T) {
		data, control := lxdExecSecrets(map[string]string{
			"control": "control-secret",
		})
		if data != "" || control != "control-secret" {
			t.Fatalf("got (%q,%q), want (%q,%q)", data, control, "", "control-secret")
		}
	})
}
