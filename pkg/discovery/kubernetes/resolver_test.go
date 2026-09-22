package kubernetes

import (
	"os"
	"path/filepath"
	"testing"

	"k8s.io/client-go/rest"
)

func TestResolveKubeConfig_DisabledLocalAutoDiscovery(t *testing.T) {
	origInCluster := inClusterConfig
	inClusterConfig = func() (*rest.Config, error) {
		return nil, os.ErrNotExist
	}
	defer func() { inClusterConfig = origInCluster }()

	_, info, err := ResolveKubeConfig(false)
	if err == nil {
		t.Fatal("expected error when local kubeconfig auto-discovery is disabled")
	}
	if info.Enabled {
		t.Fatal("expected local kubeconfig auto-discovery to be disabled")
	}
	if info.Ran {
		t.Fatal("expected local kubeconfig discovery not to run when disabled")
	}
	if info.DiscoveredContexts != 0 {
		t.Fatalf("expected 0 discovered contexts, got %d", info.DiscoveredContexts)
	}
}

func TestResolveKubeConfig_CountsLocalContexts(t *testing.T) {
	origInCluster := inClusterConfig
	inClusterConfig = func() (*rest.Config, error) {
		return nil, os.ErrNotExist
	}
	defer func() { inClusterConfig = origInCluster }()

	tempDir := t.TempDir()
	kubeconfigPath := filepath.Join(tempDir, "config")
	writeKubeconfig(t, kubeconfigPath, `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://cluster-a.example.com
  name: cluster-a
- cluster:
    server: https://cluster-b.example.com
  name: cluster-b
contexts:
- context:
    cluster: cluster-a
    user: user-a
  name: context-a
- context:
    cluster: cluster-b
    user: user-b
  name: context-b
current-context: context-a
users:
- name: user-a
  user:
    token: token-a
- name: user-b
  user:
    token: token-b
`)
	t.Setenv("KUBECONFIG", kubeconfigPath)

	_, info, err := ResolveKubeConfig(true)
	if err != nil {
		t.Fatalf("ResolveKubeConfig returned error: %v", err)
	}
	if !info.Enabled {
		t.Fatal("expected local kubeconfig auto-discovery to be enabled")
	}
	if !info.Ran {
		t.Fatal("expected local kubeconfig discovery to run")
	}
	if info.DiscoveredContexts != 2 {
		t.Fatalf("expected 2 discovered contexts, got %d", info.DiscoveredContexts)
	}
	if info.ConnectedContexts != 0 {
		t.Fatalf("expected 0 connected contexts before discovery success, got %d", info.ConnectedContexts)
	}
}

func TestResolveKubeConfig_KubeconfigPathListCountsMergedContexts(t *testing.T) {
	origInCluster := inClusterConfig
	inClusterConfig = func() (*rest.Config, error) {
		return nil, os.ErrNotExist
	}
	defer func() { inClusterConfig = origInCluster }()

	tempDir := t.TempDir()
	kubeconfigPathA := filepath.Join(tempDir, "config-a")
	kubeconfigPathB := filepath.Join(tempDir, "config-b")
	writeKubeconfig(t, kubeconfigPathA, `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://cluster-a.example.com
  name: cluster-a
contexts:
- context:
    cluster: cluster-a
    user: user-a
  name: context-a
users:
- name: user-a
  user:
    token: token-a
`)
	writeKubeconfig(t, kubeconfigPathB, `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://cluster-b.example.com
  name: cluster-b
contexts:
- context:
    cluster: cluster-b
    user: user-b
  name: context-b
current-context: context-b
users:
- name: user-b
  user:
    token: token-b
`)

	t.Setenv("KUBECONFIG", kubeconfigPathA+string(os.PathListSeparator)+kubeconfigPathB)
	_, info, err := ResolveKubeConfig(true)
	if err != nil {
		t.Fatalf("ResolveKubeConfig returned error: %v", err)
	}
	if info.DiscoveredContexts != 2 {
		t.Fatalf("expected merged context count 2, got %d", info.DiscoveredContexts)
	}
}

func writeKubeconfig(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}
}
