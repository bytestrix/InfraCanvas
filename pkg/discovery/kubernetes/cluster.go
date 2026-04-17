package kubernetes

import (
	"context"
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"infracanvas/internal/models"
)

// GetClusterInfo collects cluster-level information
func (d *Discovery) GetClusterInfo(ctx context.Context) (*models.Cluster, error) {
	cacheKey := "cluster"
	
	// Check cache first
	if cached, found := d.cache.Get(cacheKey); found {
		if cluster, ok := cached.(*models.Cluster); ok {
			return cluster, nil
		}
	}
	
	// Get server version
	version, err := d.clientset.Discovery().ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("failed to get server version: %w", err)
	}

	// Detect platform from nodes
	platform := d.detectPlatform(ctx)

	cluster := &models.Cluster{
		BaseEntity: models.BaseEntity{
			ID:          "cluster",
			Type:        models.EntityTypeCluster,
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
			Health:      models.HealthHealthy,
			Timestamp:   time.Now(),
		},
		Name:      "kubernetes",
		Version:   version.GitVersion,
		APIServer: d.config.Host,
		Platform:  platform,
	}

	// Cache the result
	d.cache.Set(cacheKey, cluster)

	return cluster, nil
}

// detectPlatform attempts to detect the Kubernetes platform (EKS, GKE, AKS, etc.)
func (d *Discovery) detectPlatform(ctx context.Context) string {
	nodes, err := d.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil || len(nodes.Items) == 0 {
		return ""
	}

	node := nodes.Items[0]

	// Check node labels for platform indicators
	if _, ok := node.Labels["eks.amazonaws.com/nodegroup"]; ok {
		return "EKS"
	}
	if _, ok := node.Labels["cloud.google.com/gke-nodepool"]; ok {
		return "GKE"
	}
	if _, ok := node.Labels["kubernetes.azure.com/cluster"]; ok {
		return "AKS"
	}
	if _, ok := node.Labels["node.openshift.io/os_id"]; ok {
		return "OpenShift"
	}

	// Check provider ID
	if strings.Contains(node.Spec.ProviderID, "aws") {
		return "EKS"
	}
	if strings.Contains(node.Spec.ProviderID, "gce") {
		return "GKE"
	}
	if strings.Contains(node.Spec.ProviderID, "azure") {
		return "AKS"
	}

	// Detect kind (Kubernetes IN Docker) clusters
	if _, ok := node.Labels["io.x-k8s.kind.cluster"]; ok {
		return "kind"
	}
	if _, ok := node.Labels["ingress-ready"]; ok {
		if strings.Contains(node.Name, "kind") {
			return "kind"
		}
	}

	return ""
}
