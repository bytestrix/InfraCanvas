package agent

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// WatcherManager manages all event watchers
type WatcherManager struct {
	config        *Config
	backendClient *BackendClient
	dockerClient  *client.Client
	k8sClient     *kubernetes.Clientset
	watchers      map[string]context.CancelFunc
	cacheInvalidator CacheInvalidator
}

// CacheInvalidator is an interface for invalidating cached data
type CacheInvalidator interface {
	InvalidateCacheForResource(resourceType string)
}

// NewWatcherManager creates a new watcher manager
func NewWatcherManager(config *Config, backendClient *BackendClient, cacheInvalidator CacheInvalidator) (*WatcherManager, error) {
	wm := &WatcherManager{
		config:           config,
		backendClient:    backendClient,
		watchers:         make(map[string]context.CancelFunc),
		cacheInvalidator: cacheInvalidator,
	}

	// Initialize Docker client if Docker is in scope
	if contains(config.Scope, "docker") {
		dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err == nil {
			wm.dockerClient = dockerClient
		}
	}

	// Initialize Kubernetes client if Kubernetes is in scope
	if contains(config.Scope, "kubernetes") {
		k8sClient, err := getKubernetesClient()
		if err == nil {
			wm.k8sClient = k8sClient
		}
	}

	return wm, nil
}

// StartAll starts all configured watchers
func (wm *WatcherManager) StartAll(ctx context.Context) error {
	if wm.dockerClient != nil {
		if err := wm.StartDockerWatcher(ctx); err != nil {
			log.Printf("Failed to start Docker watcher: %v", err)
		}
	}

	if wm.k8sClient != nil {
		if err := wm.StartKubernetesPodWatcher(ctx); err != nil {
			log.Printf("Failed to start Kubernetes pod watcher: %v", err)
		}

		if err := wm.StartKubernetesEventWatcher(ctx); err != nil {
			log.Printf("Failed to start Kubernetes event watcher: %v", err)
		}
	}

	return nil
}

// StopAll stops all running watchers
func (wm *WatcherManager) StopAll() {
	for name, cancel := range wm.watchers {
		log.Printf("Stopping watcher: %s", name)
		cancel()
	}
	wm.watchers = make(map[string]context.CancelFunc)
}

// StartDockerWatcher starts watching Docker events
func (wm *WatcherManager) StartDockerWatcher(ctx context.Context) error {
	if wm.dockerClient == nil {
		return fmt.Errorf("Docker client not initialized")
	}

	watchCtx, cancel := context.WithCancel(ctx)
	wm.watchers["docker-events"] = cancel

	go func() {
		defer cancel()

		eventChan, errChan := wm.dockerClient.Events(watchCtx, events.ListOptions{})

		for {
			select {
			case <-watchCtx.Done():
				return
			case err := <-errChan:
				if err != nil {
					log.Printf("Docker event stream error: %v", err)
					// Attempt to reconnect after a delay
					time.Sleep(5 * time.Second)
					return
				}
			case event := <-eventChan:
				wm.handleDockerEvent(event)
			}
		}
	}()

	log.Println("Started Docker event watcher")
	return nil
}

// handleDockerEvent processes a Docker event and sends it to the backend
func (wm *WatcherManager) handleDockerEvent(event events.Message) {
	// Create event payload
	backendEvent := &Event{
		Timestamp: time.Unix(event.Time, 0),
		Type:      fmt.Sprintf("docker.%s.%s", event.Type, event.Action),
		Source:    "docker",
		Data: map[string]interface{}{
			"type":   string(event.Type),
			"action": string(event.Action),
			"actor":  event.Actor,
		},
	}

	// Send to backend
	if err := wm.backendClient.SendEvent(backendEvent); err != nil {
		log.Printf("Failed to send Docker event to backend: %v", err)
	}
}

// StartKubernetesPodWatcher starts watching Kubernetes pod events
func (wm *WatcherManager) StartKubernetesPodWatcher(ctx context.Context) error {
	if wm.k8sClient == nil {
		return fmt.Errorf("Kubernetes client not initialized")
	}

	watchCtx, cancel := context.WithCancel(ctx)
	wm.watchers["k8s-pods"] = cancel

	go func() {
		defer cancel()

		for {
			select {
			case <-watchCtx.Done():
				return
			default:
				watcher, err := wm.k8sClient.CoreV1().Pods("").Watch(watchCtx, metav1.ListOptions{})
				if err != nil {
					log.Printf("Failed to create pod watcher: %v", err)
					time.Sleep(5 * time.Second)
					continue
				}

				wm.watchPods(watchCtx, watcher)
				watcher.Stop()

				// If we exit the watch loop, wait before reconnecting
				time.Sleep(5 * time.Second)
			}
		}
	}()

	log.Println("Started Kubernetes pod watcher")
	return nil
}

// watchPods processes pod watch events
func (wm *WatcherManager) watchPods(ctx context.Context, watcher watch.Interface) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return
			}

			wm.handleKubernetesPodEvent(event)
		}
	}
}

// handleKubernetesPodEvent processes a Kubernetes pod event
func (wm *WatcherManager) handleKubernetesPodEvent(event watch.Event) {
	// Invalidate pods cache on any pod event
	if wm.cacheInvalidator != nil {
		wm.cacheInvalidator.InvalidateCacheForResource("pods:")
	}
	
	backendEvent := &Event{
		Timestamp: time.Now(),
		Type:      "kubernetes.pod." + string(event.Type),
		Source:    "kubernetes",
		Data: map[string]interface{}{
			"event_type": string(event.Type),
			"object":     event.Object,
		},
	}

	if err := wm.backendClient.SendEvent(backendEvent); err != nil {
		log.Printf("Failed to send Kubernetes pod event to backend: %v", err)
	}
}

// StartKubernetesEventWatcher starts watching Kubernetes events
func (wm *WatcherManager) StartKubernetesEventWatcher(ctx context.Context) error {
	if wm.k8sClient == nil {
		return fmt.Errorf("Kubernetes client not initialized")
	}

	watchCtx, cancel := context.WithCancel(ctx)
	wm.watchers["k8s-events"] = cancel

	go func() {
		defer cancel()

		for {
			select {
			case <-watchCtx.Done():
				return
			default:
				watcher, err := wm.k8sClient.CoreV1().Events("").Watch(watchCtx, metav1.ListOptions{})
				if err != nil {
					log.Printf("Failed to create event watcher: %v", err)
					time.Sleep(5 * time.Second)
					continue
				}

				wm.watchEvents(watchCtx, watcher)
				watcher.Stop()

				// If we exit the watch loop, wait before reconnecting
				time.Sleep(5 * time.Second)
			}
		}
	}()

	log.Println("Started Kubernetes event watcher")
	return nil
}

// watchEvents processes Kubernetes event watch events
func (wm *WatcherManager) watchEvents(ctx context.Context, watcher watch.Interface) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return
			}

			wm.handleKubernetesEvent(event)
		}
	}
}

// handleKubernetesEvent processes a Kubernetes event
func (wm *WatcherManager) handleKubernetesEvent(event watch.Event) {
	// Invalidate events cache on any event
	if wm.cacheInvalidator != nil {
		wm.cacheInvalidator.InvalidateCacheForResource("events:")
	}
	
	backendEvent := &Event{
		Timestamp: time.Now(),
		Type:      "kubernetes.event." + string(event.Type),
		Source:    "kubernetes",
		Data: map[string]interface{}{
			"event_type": string(event.Type),
			"object":     event.Object,
		},
	}

	if err := wm.backendClient.SendEvent(backendEvent); err != nil {
		log.Printf("Failed to send Kubernetes event to backend: %v", err)
	}
}

// getKubernetesClient creates a Kubernetes client
func getKubernetesClient() (*kubernetes.Clientset, error) {
	// Try in-cluster config first
	config, err := rest.InClusterConfig()
	if err != nil {
		// Fall back to kubeconfig
		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		configOverrides := &clientcmd.ConfigOverrides{}
		kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
		config, err = kubeConfig.ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	return clientset, nil
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
