// Package clustermgr manages Clusters connections: Kubernetes clusters
// reached directly via an uploaded kubeconfig, with no VM agent installed
// anywhere. Each connected cluster runs as an in-process "virtual agent" that
// self-dials the local server's own /ws/agent endpoint and speaks the exact
// same protocol a real remote agent would — the relay/session/canvas code
// needs no awareness that there's no separate process on the other end.
package clustermgr

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"infracanvas/pkg/agent"
	"infracanvas/pkg/runstate"
)

// ContextOption describes one context found in an uploaded kubeconfig, for
// the "which cluster(s) do you want to add" picker.
type ContextOption struct {
	Name      string `json:"name"`
	ServerURL string `json:"serverUrl"`
	Current   bool   `json:"current"`
}

// ParseContexts lists every context in a kubeconfig without connecting to
// anything, so the caller can present a picker before committing to a
// cluster. Returns an error if the bytes aren't a valid kubeconfig.
func ParseContexts(kubeconfigBytes []byte) ([]ContextOption, error) {
	cfg, err := clientcmd.Load(kubeconfigBytes)
	if err != nil {
		return nil, fmt.Errorf("invalid kubeconfig: %w", err)
	}
	if len(cfg.Contexts) == 0 {
		return nil, fmt.Errorf("kubeconfig has no contexts")
	}
	out := make([]ContextOption, 0, len(cfg.Contexts))
	for name, c := range cfg.Contexts {
		serverURL := ""
		if cluster, ok := cfg.Clusters[c.Cluster]; ok {
			serverURL = cluster.Server
		}
		out = append(out, ContextOption{
			Name:      name,
			ServerURL: serverURL,
			Current:   name == cfg.CurrentContext,
		})
	}
	return out, nil
}

// restConfigForContext builds a *rest.Config for one named context within a
// (possibly multi-context) kubeconfig.
func restConfigForContext(cfg *clientcmdapi.Config, contextName string) (*rest.Config, error) {
	if _, ok := cfg.Contexts[contextName]; !ok {
		return nil, fmt.Errorf("context %q not found in kubeconfig", contextName)
	}
	clientCfg := clientcmd.NewNonInteractiveClientConfig(*cfg, contextName, &clientcmd.ConfigOverrides{}, nil)
	return clientCfg.ClientConfig()
}

// Entry is a Clusters connection, its metadata plus live status.
type Entry struct {
	runstate.ClusterEntry
	Online bool `json:"online"`
}

// Manager owns the lifecycle of every connected cluster's virtual agent.
type Manager struct {
	backendURL     string
	agentToken     string
	refreshSeconds int
	enableRedact   bool

	mu      sync.Mutex
	cancels map[string]context.CancelFunc
	// lastError tracks each cluster's most recent discovery outcome (nil =
	// last attempt succeeded). Online in List() used to mean only "the
	// goroutine is still alive," which stays true forever for a cluster
	// pointed at an unreachable/misconfigured API server — the WS handshake
	// to the local server always succeeds regardless of whether the cluster
	// itself is reachable. This tracks the thing that actually matters.
	lastError map[string]error
	rootCtx   context.Context // long-lived; never an inbound request's context
}

// NewManager creates a Manager. backendURL is the local server's own
// ws://127.0.0.1:<port> address — every virtual agent self-dials it exactly
// like the existing in-process host agent does.
func NewManager(backendURL, agentToken string, refreshSeconds int) *Manager {
	return &Manager{
		backendURL:     backendURL,
		agentToken:     agentToken,
		refreshSeconds: refreshSeconds,
		enableRedact:   true,
		cancels:        make(map[string]context.CancelFunc),
		lastError:      make(map[string]error),
	}
}

func clusterDir() string {
	return filepath.Join(runstate.Dir(), "clusters")
}

// LoadPersisted starts a virtual agent for every cluster already recorded in
// runstate (call once at `infracanvas serve` startup). ctx must be the
// process's long-lived lifetime context (cancelled only on shutdown) — it
// becomes the parent for every virtual agent goroutine, including ones
// started later by Add, which must never be tied to a single HTTP request's
// context.
func (m *Manager) LoadPersisted(ctx context.Context) {
	m.rootCtx = ctx
	s, err := runstate.Read()
	if err != nil {
		log.Printf("[clustermgr] read state: %v", err)
		return
	}
	for _, entry := range s.Clusters {
		m.startAgent(ctx, entry)
	}
}

// List returns every connected cluster with live online status.
func (m *Manager) List() ([]Entry, error) {
	s, err := runstate.Read()
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Entry, 0, len(s.Clusters))
	for _, e := range s.Clusters {
		_, running := m.cancels[e.ID]
		online := running && m.lastError[e.ID] == nil
		out = append(out, Entry{ClusterEntry: e, Online: online})
	}
	return out, nil
}

// Add parses kubeconfigBytes, validates contextName exists, persists it, and
// starts its virtual agent (which runs for the process's lifetime, not tied
// to the calling HTTP request). name defaults to contextName if empty.
func (m *Manager) Add(name string, kubeconfigBytes []byte, contextName string) (Entry, error) {
	cfg, err := clientcmd.Load(kubeconfigBytes)
	if err != nil {
		return Entry{}, fmt.Errorf("invalid kubeconfig: %w", err)
	}
	if contextName == "" {
		contextName = cfg.CurrentContext
	}
	restCfg, err := restConfigForContext(cfg, contextName)
	if err != nil {
		return Entry{}, err
	}
	// Fail fast on an unreachable/misconfigured cluster rather than persisting
	// a dead entry — mirrors kubernetes.Discovery.IsAvailable()'s connectivity probe.
	restCfg.Timeout = 5 * time.Second

	serverURL := ""
	if c, ok := cfg.Contexts[contextName]; ok {
		if cl, ok := cfg.Clusters[c.Cluster]; ok {
			serverURL = cl.Server
		}
	}

	id := randomID()
	if err := os.MkdirAll(clusterDir(), 0o700); err != nil {
		return Entry{}, fmt.Errorf("create cluster dir: %w", err)
	}
	path := filepath.Join(clusterDir(), id+".kubeconfig")
	if err := os.WriteFile(path, kubeconfigBytes, 0o600); err != nil {
		return Entry{}, fmt.Errorf("save kubeconfig: %w", err)
	}

	if name == "" {
		name = contextName
	}
	entry := runstate.ClusterEntry{
		ID:             id,
		Name:           name,
		ContextName:    contextName,
		ServerURL:      serverURL,
		KubeconfigPath: path,
		AddedAt:        time.Now().UTC(),
	}

	if err := runstate.Update(func(s *runstate.State) {
		s.Clusters = append(s.Clusters, entry)
	}); err != nil {
		_ = os.Remove(path)
		return Entry{}, fmt.Errorf("persist cluster: %w", err)
	}

	rootCtx := m.rootCtx
	if rootCtx == nil {
		rootCtx = context.Background()
	}
	m.startAgent(rootCtx, entry)
	return Entry{ClusterEntry: entry, Online: true}, nil
}

// Remove stops the virtual agent and deletes the cluster's persisted entry
// and on-disk kubeconfig file.
func (m *Manager) Remove(id string) error {
	m.mu.Lock()
	if cancel, ok := m.cancels[id]; ok {
		cancel()
		delete(m.cancels, id)
	}
	delete(m.lastError, id)
	m.mu.Unlock()

	var path string
	if err := runstate.Update(func(s *runstate.State) {
		kept := s.Clusters[:0]
		for _, e := range s.Clusters {
			if e.ID == id {
				path = e.KubeconfigPath
				continue
			}
			kept = append(kept, e)
		}
		s.Clusters = kept
	}); err != nil {
		return err
	}
	if path != "" {
		_ = os.Remove(path)
	}
	return nil
}

// startAgent launches (or restarts, on failure, with backoff) the virtual
// agent goroutine for one cluster entry.
func (m *Manager) startAgent(parent context.Context, entry runstate.ClusterEntry) {
	ctx, cancel := context.WithCancel(parent)
	m.mu.Lock()
	m.cancels[entry.ID] = cancel
	m.mu.Unlock()

	go func() {
		for {
			if ctx.Err() != nil {
				return
			}
			kubeconfigBytes, err := os.ReadFile(entry.KubeconfigPath)
			if err != nil {
				log.Printf("[clustermgr] cluster %s (%s): read kubeconfig: %v", entry.Name, entry.ID, err)
				return
			}
			cfg, err := clientcmd.Load(kubeconfigBytes)
			if err != nil {
				log.Printf("[clustermgr] cluster %s (%s): parse kubeconfig: %v", entry.Name, entry.ID, err)
				return
			}
			restCfg, err := restConfigForContext(cfg, entry.ContextName)
			if err != nil {
				log.Printf("[clustermgr] cluster %s (%s): %v", entry.Name, entry.ID, err)
				return
			}

			wsCfg := &agent.WSConfig{
				BackendURL:        m.backendURL,
				AuthToken:         m.agentToken,
				RefreshSeconds:    m.refreshSeconds,
				EnableRedaction:   m.enableRedact,
				QuietPairBanner:   true,
				KubeConfig:        restCfg,
				MachineIDOverride: "cluster-" + entry.ID,
				OnDiscoveryResult: func(discErr error) {
					m.mu.Lock()
					m.lastError[entry.ID] = discErr
					m.mu.Unlock()
				},
			}
			ag, err := agent.NewWSAgent(wsCfg)
			if err != nil {
				log.Printf("[clustermgr] cluster %s (%s): create agent: %v — retrying in 10s", entry.Name, entry.ID, err)
			} else if err := ag.Run(ctx); err != nil && ctx.Err() == nil {
				log.Printf("[clustermgr] cluster %s (%s): agent run ended: %v — retrying in 10s", entry.Name, entry.ID, err)
			}

			if ctx.Err() != nil {
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(10 * time.Second):
			}
		}
	}()
}

func randomID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("c%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
