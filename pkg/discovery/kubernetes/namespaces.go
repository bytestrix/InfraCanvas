package kubernetes

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"infracanvas/internal/models"
)

// GetNamespaces collects all namespaces in the cluster
func (d *Discovery) GetNamespaces(ctx context.Context) ([]models.Namespace, error) {
	cacheKey := "namespaces"
	
	// Check cache first
	if cached, found := d.cache.Get(cacheKey); found {
		if namespaces, ok := cached.([]models.Namespace); ok {
			return namespaces, nil
		}
	}
	
	nsList, err := d.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list namespaces: %w", err)
	}

	namespaces := make([]models.Namespace, 0, len(nsList.Items))
	for _, ns := range nsList.Items {
		namespace := d.parseNamespace(&ns)
		namespaces = append(namespaces, namespace)
	}

	// Cache the result
	d.cache.Set(cacheKey, namespaces)

	return namespaces, nil
}

func (d *Discovery) parseNamespace(ns *corev1.Namespace) models.Namespace {
	status := "Active"
	if ns.Status.Phase != "" {
		status = string(ns.Status.Phase)
	}

	health := models.HealthHealthy
	if ns.Status.Phase == "Terminating" {
		health = models.HealthDegraded
	}

	namespace := models.Namespace{
		BaseEntity: models.BaseEntity{
			ID:          fmt.Sprintf("namespace/%s", ns.Name),
			Type:        models.EntityTypeNamespace,
			Labels:      ns.Labels,
			Annotations: ns.Annotations,
			Health:      health,
			Timestamp:   time.Now(),
		},
		Name:   ns.Name,
		Status: status,
		Phase:  string(ns.Status.Phase),
	}

	return namespace
}
