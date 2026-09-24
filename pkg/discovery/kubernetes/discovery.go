package kubernetes

import (
	"context"
	"fmt"
	"time"

	"infracanvas/internal/models"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Discovery implements Kubernetes-level infrastructure discovery
type Discovery struct {
	clientset         *kubernetes.Clientset
	config            *rest.Config
	cache             *Cache
	connectedContexts int
	skipped           []string
}

// NewDiscovery creates a new Kubernetes discovery instance, resolving the
// kubeconfig from the local host (in-cluster config, $KUBECONFIG, or
// ~/.kube/config).
func NewDiscovery() (*Discovery, error) {
	config, _, err := ResolveKubeConfig(true)
	if err != nil {
		return nil, fmt.Errorf("failed to get kubeconfig: %w", err)
	}
	return NewDiscoveryFromConfig(config)
}

// NewDiscoveryWithLocalKubeconfigAutoDiscovery creates a new Kubernetes
// discovery instance and returns details about local kubeconfig
// auto-discovery behavior.
func NewDiscoveryWithLocalKubeconfigAutoDiscovery(enabled bool) (*Discovery, models.LocalKubeconfigAutoDiscovery, error) {
	config, info, err := ResolveKubeConfig(enabled)
	if err != nil {
		return nil, info, fmt.Errorf("failed to get kubeconfig: %w", err)
	}
	d, err := NewDiscoveryFromConfig(config)
	return d, info, err
}

// NewDiscoveryFromConfig creates a Discovery against an explicit *rest.Config
// instead of resolving one from the local host. Used for Clusters connections
// (an uploaded kubeconfig, parsed via clientcmd.RESTConfigFromKubeConfig) —
// the target cluster may have nothing to do with the machine this process
// runs on.
func NewDiscoveryFromConfig(config *rest.Config) (*Discovery, error) {
	// Clusters virtual agents share this *rest.Config pointer with the action
	// executor (used for pods/exec terminal sessions and log streaming, which
	// legitimately need to stay open well past any discovery-sized timeout) —
	// copy before mutating so Timeout below only ever applies to discovery's
	// own clientset, never leaks into exec/logs through the shared pointer.
	config = rest.CopyConfig(config)

	// Suppress "v1 Endpoints is deprecated" and similar API server warnings
	// that flood the logs on every discovery cycle.
	config.WarningHandler = rest.NoWarnings{}

	// A per-request context timeout isn't reliably honored during the dial
	// phase across every client-go transport path, so an unreachable API
	// server (wrong endpoint, dropped packets, no route) can hang each
	// discovery call far longer than IsAvailable's own short context ever
	// intends. rest.Config.Timeout is a hard http.Client-level bound
	// client-go always applies, so set one here regardless of the caller's
	// context.
	// 10s was sized for a local/in-cluster API server (near-zero latency).
	// Clusters direct-connect reaches a remote API server over the public
	// internet by design — real round-trip latency plus a full namespace
	// listing (e.g. secrets across every namespace) can legitimately run
	// past 10s on an ordinary home connection, which was surfacing as a
	// spurious "error" status on clusters that were actually fine, just
	// slower to reach than a local one.
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	return &Discovery{
		clientset:         clientset,
		config:            config,
		cache:             NewCache(30 * time.Second),
		connectedContexts: 0,
	}, nil
}

// IsAvailable checks if Kubernetes is available and accessible
func (d *Discovery) IsAvailable() bool {
	return d.Ping() == nil
}

// Ping checks that the API server is reachable and accepts our credentials.
// It hits /version, which every authenticated user can read, instead of
// listing Nodes: a namespace-scoped or read-only credential can't list
// cluster-scoped resources, and that used to mark a working cluster as
// unavailable.
func (d *Discovery) Ping() error {
	if d.clientset == nil {
		return fmt.Errorf("kubernetes client not initialized")
	}

	// 8s covers a remote API server over an ordinary home/office connection
	// (TLS handshake + auth) while still catching an unreachable endpoint well
	// before the 30s discovery timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	if _, err := d.clientset.Discovery().RESTClient().Get().AbsPath("/version").Do(ctx).Raw(); err != nil {
		return fmt.Errorf("cannot reach Kubernetes API server %s: %w", d.config.Host, err)
	}
	return nil
}

// Skipped returns the resource kinds the last DiscoverAll pass could not list
// because the credential lacks RBAC access to them.
func (d *Discovery) Skipped() []string {
	return d.skipped
}

// DiscoverAll performs a complete Kubernetes discovery. A resource kind the
// credential isn't allowed to list (403) is skipped and recorded in Skipped()
// instead of failing the whole pass, so a narrowly scoped kubeconfig still
// gets a canvas. Any other error (network, timeout, 5xx) fails the pass.
func (d *Discovery) DiscoverAll() (*models.Cluster, []models.Node, []models.Namespace, []models.Deployment, []models.StatefulSet, []models.DaemonSet, []models.Job, []models.CronJob, []models.Pod, []models.K8sService, []models.Ingress, []models.ConfigMap, []models.Secret, []models.PersistentVolumeClaim, []models.PersistentVolume, []models.StorageClass, []models.Event, error) {
	d.connectedContexts = 0
	d.skipped = nil
	fail := func(err error) (*models.Cluster, []models.Node, []models.Namespace, []models.Deployment, []models.StatefulSet, []models.DaemonSet, []models.Job, []models.CronJob, []models.Pod, []models.K8sService, []models.Ingress, []models.ConfigMap, []models.Secret, []models.PersistentVolumeClaim, []models.PersistentVolume, []models.StorageClass, []models.Event, error) {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, err
	}

	if err := d.Ping(); err != nil {
		return fail(err)
	}

	ctx := context.Background()
	var firstErr error
	// check records a 403/404 as skipped and keeps any other error as the
	// pass's error.
	check := func(kind string, err error) {
		if err == nil || firstErr != nil {
			return
		}
		if apierrors.IsForbidden(err) || apierrors.IsNotFound(err) {
			d.skipped = append(d.skipped, kind)
			return
		}
		firstErr = fmt.Errorf("failed to get %s: %w", kind, err)
	}

	cluster, err := d.GetClusterInfo(ctx)
	if err != nil {
		return fail(fmt.Errorf("failed to get cluster info: %w", err))
	}

	nodes, err := d.GetNodes(ctx)
	check("nodes", err)
	namespaces, err := d.GetNamespaces(ctx)
	check("namespaces", err)
	deployments, err := d.GetDeployments(ctx, "")
	check("deployments", err)
	statefulsets, err := d.GetStatefulSets(ctx, "")
	check("statefulsets", err)
	daemonsets, err := d.GetDaemonSets(ctx, "")
	check("daemonsets", err)
	jobs, err := d.GetJobs(ctx, "")
	check("jobs", err)
	cronjobs, err := d.GetCronJobs(ctx, "")
	check("cronjobs", err)
	pods, err := d.GetPods(ctx, "")
	check("pods", err)
	services, err := d.GetServices(ctx, "")
	check("services", err)
	ingresses, err := d.GetIngresses(ctx, "")
	check("ingresses", err)
	configmaps, err := d.GetConfigMaps(ctx, "")
	check("configmaps", err)
	secrets, err := d.GetSecrets(ctx, "")
	check("secrets", err)
	pvcs, err := d.GetPVCs(ctx, "")
	check("persistentvolumeclaims", err)
	pvs, err := d.GetPVs(ctx)
	check("persistentvolumes", err)
	storageclasses, err := d.GetStorageClasses(ctx)
	check("storageclasses", err)
	events, err := d.GetEvents(ctx, "")
	check("events", err)

	if firstErr != nil {
		return fail(firstErr)
	}
	// Nothing workload-level was readable: the canvas would be empty with no
	// explanation, so report it as an error the UI can show.
	if contains(d.skipped, "pods") && contains(d.skipped, "deployments") {
		return fail(fmt.Errorf("credential cannot list pods or deployments cluster-wide (RBAC forbidden)"))
	}

	d.connectedContexts = 1
	return cluster, nodes, namespaces, deployments, statefulsets, daemonsets, jobs, cronjobs, pods, services, ingresses, configmaps, secrets, pvcs, pvs, storageclasses, events, nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// ConnectedContexts returns how many local kubeconfig contexts were
// successfully connected in the last discovery pass.
func (d *Discovery) ConnectedContexts() int {
	return d.connectedContexts
}

// InvalidateCache invalidates all cached Kubernetes data
func (d *Discovery) InvalidateCache() {
	if d.cache != nil {
		d.cache.Clear()
	}
}

// InvalidateCacheForResource invalidates cache for a specific resource type
func (d *Discovery) InvalidateCacheForResource(resourceType string) {
	if d.cache != nil {
		d.cache.InvalidatePattern(resourceType)
	}
}
