package kubernetes

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"infracanvas/internal/models"
)

// Discovery implements Kubernetes-level infrastructure discovery
type Discovery struct {
	clientset *kubernetes.Clientset
	config    *rest.Config
	cache     *Cache
}

// NewDiscovery creates a new Kubernetes discovery instance
func NewDiscovery() (*Discovery, error) {
	config, err := getKubeConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	return &Discovery{
		clientset: clientset,
		config:    config,
		cache:     NewCache(30 * time.Second),
	}, nil
}

// getKubeConfig attempts to load kubeconfig from multiple sources
func getKubeConfig() (*rest.Config, error) {
	// 1. Try in-cluster config first
	config, err := rest.InClusterConfig()
	if err == nil {
		return config, nil
	}

	// 2. Try KUBECONFIG environment variable
	kubeconfigPath := os.Getenv("KUBECONFIG")
	if kubeconfigPath != "" {
		config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err == nil {
			return config, nil
		}
	}

	// 3. Try default kubeconfig location
	homeDir, err := os.UserHomeDir()
	if err == nil {
		defaultKubeconfig := filepath.Join(homeDir, ".kube", "config")
		if _, err := os.Stat(defaultKubeconfig); err == nil {
			config, err := clientcmd.BuildConfigFromFlags("", defaultKubeconfig)
			if err == nil {
				return config, nil
			}
		}
	}

	return nil, fmt.Errorf("unable to load kubeconfig from any source")
}

// IsAvailable checks if Kubernetes is available and accessible
func (d *Discovery) IsAvailable() bool {
	if d.clientset == nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Try to list nodes as a connectivity check
	_, err := d.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{Limit: 1})
	return err == nil
}

// DiscoverAll performs a complete Kubernetes discovery
func (d *Discovery) DiscoverAll() (*models.Cluster, []models.Node, []models.Namespace, []models.Deployment, []models.StatefulSet, []models.DaemonSet, []models.Job, []models.CronJob, []models.Pod, []models.K8sService, []models.Ingress, []models.ConfigMap, []models.Secret, []models.PersistentVolumeClaim, []models.PersistentVolume, []models.StorageClass, []models.Event, error) {
	if !d.IsAvailable() {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("Kubernetes is not available")
	}

	// Get cluster info
	cluster, err := d.GetClusterInfo(context.Background())
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to get cluster info: %w", err)
	}

	// Get nodes
	nodes, err := d.GetNodes(context.Background())
	if err != nil {
		return cluster, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to get nodes: %w", err)
	}

	// Get namespaces
	namespaces, err := d.GetNamespaces(context.Background())
	if err != nil {
		return cluster, nodes, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to get namespaces: %w", err)
	}

	// Get workloads across all namespaces
	deployments, err := d.GetDeployments(context.Background(), "")
	if err != nil {
		return cluster, nodes, namespaces, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to get deployments: %w", err)
	}

	statefulsets, err := d.GetStatefulSets(context.Background(), "")
	if err != nil {
		return cluster, nodes, namespaces, deployments, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to get statefulsets: %w", err)
	}

	daemonsets, err := d.GetDaemonSets(context.Background(), "")
	if err != nil {
		return cluster, nodes, namespaces, deployments, statefulsets, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to get daemonsets: %w", err)
	}

	jobs, err := d.GetJobs(context.Background(), "")
	if err != nil {
		return cluster, nodes, namespaces, deployments, statefulsets, daemonsets, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to get jobs: %w", err)
	}

	cronjobs, err := d.GetCronJobs(context.Background(), "")
	if err != nil {
		return cluster, nodes, namespaces, deployments, statefulsets, daemonsets, jobs, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to get cronjobs: %w", err)
	}

	// Get pods
	pods, err := d.GetPods(context.Background(), "")
	if err != nil {
		return cluster, nodes, namespaces, deployments, statefulsets, daemonsets, jobs, cronjobs, nil, nil, nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to get pods: %w", err)
	}

	// Get services and ingress
	services, err := d.GetServices(context.Background(), "")
	if err != nil {
		return cluster, nodes, namespaces, deployments, statefulsets, daemonsets, jobs, cronjobs, pods, nil, nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to get services: %w", err)
	}

	ingresses, err := d.GetIngresses(context.Background(), "")
	if err != nil {
		return cluster, nodes, namespaces, deployments, statefulsets, daemonsets, jobs, cronjobs, pods, services, nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to get ingresses: %w", err)
	}

	// Get config and secrets
	configmaps, err := d.GetConfigMaps(context.Background(), "")
	if err != nil {
		return cluster, nodes, namespaces, deployments, statefulsets, daemonsets, jobs, cronjobs, pods, services, ingresses, nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to get configmaps: %w", err)
	}

	secrets, err := d.GetSecrets(context.Background(), "")
	if err != nil {
		return cluster, nodes, namespaces, deployments, statefulsets, daemonsets, jobs, cronjobs, pods, services, ingresses, configmaps, nil, nil, nil, nil, nil, fmt.Errorf("failed to get secrets: %w", err)
	}

	// Get storage
	pvcs, err := d.GetPVCs(context.Background(), "")
	if err != nil {
		return cluster, nodes, namespaces, deployments, statefulsets, daemonsets, jobs, cronjobs, pods, services, ingresses, configmaps, secrets, nil, nil, nil, nil, fmt.Errorf("failed to get pvcs: %w", err)
	}

	pvs, err := d.GetPVs(context.Background())
	if err != nil {
		return cluster, nodes, namespaces, deployments, statefulsets, daemonsets, jobs, cronjobs, pods, services, ingresses, configmaps, secrets, pvcs, nil, nil, nil, fmt.Errorf("failed to get pvs: %w", err)
	}

	storageclasses, err := d.GetStorageClasses(context.Background())
	if err != nil {
		return cluster, nodes, namespaces, deployments, statefulsets, daemonsets, jobs, cronjobs, pods, services, ingresses, configmaps, secrets, pvcs, pvs, nil, nil, fmt.Errorf("failed to get storageclasses: %w", err)
	}

	// Get events
	events, err := d.GetEvents(context.Background(), "")
	if err != nil {
		return cluster, nodes, namespaces, deployments, statefulsets, daemonsets, jobs, cronjobs, pods, services, ingresses, configmaps, secrets, pvcs, pvs, storageclasses, nil, fmt.Errorf("failed to get events: %w", err)
	}

	return cluster, nodes, namespaces, deployments, statefulsets, daemonsets, jobs, cronjobs, pods, services, ingresses, configmaps, secrets, pvcs, pvs, storageclasses, events, nil
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
