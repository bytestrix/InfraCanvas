package agent

import "testing"

// Regression test: pod graph-node IDs are "pod/<namespace>/<name>" — three
// segments, not the "type/name" two-segment shape most other resource IDs
// use. Stripping only through the first separator used to leave
// "<namespace>/<name>", which Kubernetes rejects as a resource name (found
// via a live browser test against a real cluster: pod log fetches and
// port-forwards both failed with "invalid resource name ... '/'").
func TestNormalizeEntityID(t *testing.T) {
	cases := []struct {
		id   string
		want string
	}{
		{"pod/kube-system/coredns-abc123", "coredns-abc123"},
		{"deployment/local-path-storage/local-path-provisioner", "local-path-provisioner"},
		{"container/abc123def456", "abc123def456"},
		{"host:my-service", "my-service"},
		{"bare-name-no-prefix", "bare-name-no-prefix"},
	}
	for _, c := range cases {
		if got := normalizeEntityID(c.id); got != c.want {
			t.Errorf("normalizeEntityID(%q) = %q, want %q", c.id, got, c.want)
		}
	}
}
