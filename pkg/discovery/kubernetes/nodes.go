package kubernetes

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"infracanvas/internal/models"
)

// GetNodes collects all nodes in the cluster
func (d *Discovery) GetNodes(ctx context.Context) ([]models.Node, error) {
	cacheKey := "nodes"
	
	// Check cache first
	if cached, found := d.cache.Get(cacheKey); found {
		if nodes, ok := cached.([]models.Node); ok {
			return nodes, nil
		}
	}
	
	nodeList, err := d.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	nodes := make([]models.Node, 0, len(nodeList.Items))
	for _, n := range nodeList.Items {
		node := d.parseNode(&n)
		nodes = append(nodes, node)
	}

	// Cache the result
	d.cache.Set(cacheKey, nodes)

	return nodes, nil
}

func (d *Discovery) parseNode(n *corev1.Node) models.Node {
	// Determine node status
	status := "NotReady"
	for _, cond := range n.Status.Conditions {
		if cond.Type == "Ready" {
			if cond.Status == "True" {
				status = "Ready"
			}
			break
		}
	}

	// Extract roles
	roles := []string{}
	for label := range n.Labels {
		if strings.HasPrefix(label, "node-role.kubernetes.io/") {
			role := strings.TrimPrefix(label, "node-role.kubernetes.io/")
			if role != "" {
				roles = append(roles, role)
			}
		}
	}
	if len(roles) == 0 {
		roles = append(roles, "worker")
	}

	// Parse conditions
	conditions := make([]models.NodeCondition, 0, len(n.Status.Conditions))
	for _, cond := range n.Status.Conditions {
		conditions = append(conditions, models.NodeCondition{
			Type:           string(cond.Type),
			Status:         string(cond.Status),
			Reason:         cond.Reason,
			Message:        cond.Message,
			LastTransition: cond.LastTransitionTime.Time,
		})
	}

	// Parse capacity
	cpuCapacity := n.Status.Capacity.Cpu().String()
	memoryCapacity := n.Status.Capacity.Memory().String()
	podsCapacity := int(n.Status.Capacity.Pods().Value())

	// Parse allocatable
	cpuAllocatable := n.Status.Allocatable.Cpu().String()
	memoryAllocatable := n.Status.Allocatable.Memory().String()
	podsAllocatable := int(n.Status.Allocatable.Pods().Value())

	// Determine health
	health := models.HealthHealthy
	if status == "NotReady" {
		health = models.HealthUnhealthy
	}

	node := models.Node{
		BaseEntity: models.BaseEntity{
			ID:          fmt.Sprintf("node/%s", n.Name),
			Type:        models.EntityTypeNode,
			Labels:      n.Labels,
			Annotations: n.Annotations,
			Health:      health,
			Timestamp:   time.Now(),
		},
		Name:              n.Name,
		Status:            status,
		Roles:             roles,
		KubernetesVersion: n.Status.NodeInfo.KubeletVersion,
		ContainerRuntime:  n.Status.NodeInfo.ContainerRuntimeVersion,
		OSImage:           n.Status.NodeInfo.OSImage,
		KernelVersion:     n.Status.NodeInfo.KernelVersion,
		CPUCapacity:       cpuCapacity,
		MemoryCapacity:    memoryCapacity,
		PodsCapacity:      podsCapacity,
		CPUAllocatable:    cpuAllocatable,
		MemoryAllocatable: memoryAllocatable,
		PodsAllocatable:   podsAllocatable,
		Conditions:        conditions,
	}

	return node
}
