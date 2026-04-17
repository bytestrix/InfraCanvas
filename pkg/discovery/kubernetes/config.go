package kubernetes

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"infracanvas/internal/models"
)

// GetConfigMaps collects all configmaps in the specified namespace
func (d *Discovery) GetConfigMaps(ctx context.Context, namespace string) ([]models.ConfigMap, error) {
	cacheKey := fmt.Sprintf("configmaps:%s", namespace)
	
	// Check cache first
	if cached, found := d.cache.Get(cacheKey); found {
		if configmaps, ok := cached.([]models.ConfigMap); ok {
			return configmaps, nil
		}
	}
	
	cmList, err := d.clientset.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list configmaps: %w", err)
	}

	configmaps := make([]models.ConfigMap, 0, len(cmList.Items))
	for _, cm := range cmList.Items {
		configmap := d.parseConfigMap(&cm)
		configmaps = append(configmaps, configmap)
	}

	// Cache the result
	d.cache.Set(cacheKey, configmaps)

	return configmaps, nil
}

func (d *Discovery) parseConfigMap(cm *corev1.ConfigMap) models.ConfigMap {
	// Extract data keys only (not values)
	dataKeys := make([]string, 0, len(cm.Data)+len(cm.BinaryData))
	for key := range cm.Data {
		dataKeys = append(dataKeys, key)
	}
	for key := range cm.BinaryData {
		dataKeys = append(dataKeys, key)
	}

	configmap := models.ConfigMap{
		BaseEntity: models.BaseEntity{
			ID:          fmt.Sprintf("configmap/%s/%s", cm.Namespace, cm.Name),
			Type:        models.EntityTypeConfigMap,
			Labels:      cm.Labels,
			Annotations: cm.Annotations,
			Health:      models.HealthHealthy,
			Timestamp:   time.Now(),
		},
		Name:      cm.Name,
		Namespace: cm.Namespace,
		DataKeys:  dataKeys,
	}

	return configmap
}

// GetSecrets collects all secrets in the specified namespace
func (d *Discovery) GetSecrets(ctx context.Context, namespace string) ([]models.Secret, error) {
	cacheKey := fmt.Sprintf("secrets:%s", namespace)
	
	// Check cache first
	if cached, found := d.cache.Get(cacheKey); found {
		if secrets, ok := cached.([]models.Secret); ok {
			return secrets, nil
		}
	}
	
	secretList, err := d.clientset.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets: %w", err)
	}

	secrets := make([]models.Secret, 0, len(secretList.Items))
	for _, s := range secretList.Items {
		secret := d.parseSecret(&s)
		secrets = append(secrets, secret)
	}

	// Cache the result
	d.cache.Set(cacheKey, secrets)

	return secrets, nil
}

func (d *Discovery) parseSecret(s *corev1.Secret) models.Secret {
	// Extract data keys only (not values) - security requirement
	dataKeys := make([]string, 0, len(s.Data)+len(s.StringData))
	for key := range s.Data {
		dataKeys = append(dataKeys, key)
	}
	for key := range s.StringData {
		dataKeys = append(dataKeys, key)
	}

	secret := models.Secret{
		BaseEntity: models.BaseEntity{
			ID:          fmt.Sprintf("secret/%s/%s", s.Namespace, s.Name),
			Type:        models.EntityTypeSecret,
			Labels:      s.Labels,
			Annotations: s.Annotations,
			Health:      models.HealthHealthy,
			Timestamp:   time.Now(),
		},
		Name:      s.Name,
		Namespace: s.Namespace,
		Type:      string(s.Type),
		DataKeys:  dataKeys,
	}

	return secret
}
