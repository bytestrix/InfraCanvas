package kubernetes

import (
	"fmt"
	"os"
	"path/filepath"

	"infracanvas/internal/models"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var inClusterConfig = rest.InClusterConfig

// ResolveKubeConfig resolves Kubernetes config from in-cluster config first,
// then optional local kubeconfig auto-discovery.
func ResolveKubeConfig(localKubeconfigAutoDiscovery bool) (*rest.Config, models.LocalKubeconfigAutoDiscovery, error) {
	info := models.LocalKubeconfigAutoDiscovery{
		Enabled: localKubeconfigAutoDiscovery,
	}

	// In-cluster config is not local kubeconfig auto-discovery.
	config, err := inClusterConfig()
	if err == nil {
		return config, info, nil
	}

	if !localKubeconfigAutoDiscovery {
		return nil, info, fmt.Errorf("local kubeconfig auto-discovery is disabled")
	}

	info.Ran = true
	config, discovered, err := loadLocalKubeconfigConfig()
	info.DiscoveredContexts = discovered
	if err != nil {
		return nil, info, err
	}
	return config, info, nil
}

func loadLocalKubeconfigConfig() (*rest.Config, int, error) {
	paths, err := localKubeconfigPaths()
	if err != nil {
		return nil, 0, err
	}

	rules := &clientcmd.ClientConfigLoadingRules{Precedence: paths}
	cfg, err := rules.Load()
	if err != nil {
		return nil, 0, fmt.Errorf("load local kubeconfig: %w", err)
	}

	discovered := len(cfg.Contexts)
	if discovered == 0 {
		return nil, 0, fmt.Errorf("local kubeconfig has no contexts")
	}

	clientCfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, &clientcmd.ConfigOverrides{})
	restCfg, err := clientCfg.ClientConfig()
	if err != nil {
		return nil, discovered, fmt.Errorf("build local kubeconfig client: %w", err)
	}

	return restCfg, discovered, nil
}

func localKubeconfigPaths() ([]string, error) {
	if raw := os.Getenv("KUBECONFIG"); raw != "" {
		paths := filepath.SplitList(raw)
		if len(paths) > 0 {
			return paths, nil
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home dir: %w", err)
	}
	defaultKubeconfig := filepath.Join(homeDir, ".kube", "config")
	if _, err := os.Stat(defaultKubeconfig); err != nil {
		return nil, fmt.Errorf("no local kubeconfig found")
	}
	return []string{defaultKubeconfig}, nil
}
