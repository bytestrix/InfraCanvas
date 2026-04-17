package kubernetes

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"infracanvas/internal/models"
)

// GetPods collects all pods in the specified namespace (empty string for all namespaces)
func (d *Discovery) GetPods(ctx context.Context, namespace string) ([]models.Pod, error) {
	cacheKey := fmt.Sprintf("pods:%s", namespace)
	
	// Check cache first
	if cached, found := d.cache.Get(cacheKey); found {
		if pods, ok := cached.([]models.Pod); ok {
			return pods, nil
		}
	}
	
	podList, err := d.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	pods := make([]models.Pod, 0, len(podList.Items))
	for _, p := range podList.Items {
		pod := d.parsePod(&p)
		pods = append(pods, pod)
	}

	// Cache the result
	d.cache.Set(cacheKey, pods)

	return pods, nil
}

func (d *Discovery) parsePod(p *corev1.Pod) models.Pod {
	// Parse owner references
	ownerKind := ""
	ownerName := ""
	if len(p.OwnerReferences) > 0 {
		owner := p.OwnerReferences[0]
		ownerKind = owner.Kind
		ownerName = owner.Name
	}

	// Parse volume references
	volumeRefs := models.PodVolumeRefs{
		ConfigMaps: []string{},
		Secrets:    []string{},
		PVCs:       []string{},
	}

	for _, vol := range p.Spec.Volumes {
		if vol.ConfigMap != nil {
			volumeRefs.ConfigMaps = append(volumeRefs.ConfigMaps, vol.ConfigMap.Name)
		}
		if vol.Secret != nil {
			volumeRefs.Secrets = append(volumeRefs.Secrets, vol.Secret.SecretName)
		}
		if vol.PersistentVolumeClaim != nil {
			volumeRefs.PVCs = append(volumeRefs.PVCs, vol.PersistentVolumeClaim.ClaimName)
		}
	}

	// imagePullSecrets
	for _, ips := range p.Spec.ImagePullSecrets {
		if ips.Name != "" {
			volumeRefs.Secrets = append(volumeRefs.Secrets, ips.Name)
		}
	}

	// Also scan envFrom and env.valueFrom in all containers (including init containers)
	allContainers := append(p.Spec.Containers, p.Spec.InitContainers...)
	for _, c := range allContainers {
		for _, envFrom := range c.EnvFrom {
			if envFrom.ConfigMapRef != nil && envFrom.ConfigMapRef.Name != "" {
				volumeRefs.ConfigMaps = append(volumeRefs.ConfigMaps, envFrom.ConfigMapRef.Name)
			}
			if envFrom.SecretRef != nil && envFrom.SecretRef.Name != "" {
				volumeRefs.Secrets = append(volumeRefs.Secrets, envFrom.SecretRef.Name)
			}
		}
		for _, env := range c.Env {
			if env.ValueFrom != nil {
				if env.ValueFrom.ConfigMapKeyRef != nil && env.ValueFrom.ConfigMapKeyRef.Name != "" {
					volumeRefs.ConfigMaps = append(volumeRefs.ConfigMaps, env.ValueFrom.ConfigMapKeyRef.Name)
				}
				if env.ValueFrom.SecretKeyRef != nil && env.ValueFrom.SecretKeyRef.Name != "" {
					volumeRefs.Secrets = append(volumeRefs.Secrets, env.ValueFrom.SecretKeyRef.Name)
				}
			}
		}
	}

	// Parse containers
	containers := make([]models.PodContainer, 0, len(p.Spec.Containers))
	for _, c := range p.Spec.Containers {
		containerStatus := models.PodContainer{
			Name:  c.Name,
			Image: c.Image,
		}

		// Find matching container status
		for _, cs := range p.Status.ContainerStatuses {
			if cs.Name == c.Name {
				containerStatus.ImageID = cs.ImageID
				containerStatus.Ready = cs.Ready
				containerStatus.RestartCount = cs.RestartCount

				// Determine state
				if cs.State.Running != nil {
					containerStatus.State = "running"
				} else if cs.State.Waiting != nil {
					containerStatus.State = "waiting"
				} else if cs.State.Terminated != nil {
					containerStatus.State = "terminated"
				}

				break
			}
		}

		// Parse resources
		if c.Resources.Requests != nil {
			if cpu, ok := c.Resources.Requests[corev1.ResourceCPU]; ok {
				containerStatus.CPURequest = cpu.String()
			}
			if mem, ok := c.Resources.Requests[corev1.ResourceMemory]; ok {
				containerStatus.MemoryRequest = mem.String()
			}
		}
		if c.Resources.Limits != nil {
			if cpu, ok := c.Resources.Limits[corev1.ResourceCPU]; ok {
				containerStatus.CPULimit = cpu.String()
			}
			if mem, ok := c.Resources.Limits[corev1.ResourceMemory]; ok {
				containerStatus.MemoryLimit = mem.String()
			}
		}

		containers = append(containers, containerStatus)
	}

	// Parse conditions
	conditions := make([]models.PodCondition, 0, len(p.Status.Conditions))
	for _, cond := range p.Status.Conditions {
		conditions = append(conditions, models.PodCondition{
			Type:           string(cond.Type),
			Status:         string(cond.Status),
			Reason:         cond.Reason,
			Message:        cond.Message,
			LastTransition: cond.LastTransitionTime.Time,
		})
	}

	// Determine health
	health := models.HealthHealthy
	phase := string(p.Status.Phase)
	if phase == "Failed" || phase == "Unknown" {
		health = models.HealthUnhealthy
	} else if phase == "Pending" {
		health = models.HealthDegraded
	} else if phase == "Running" {
		// Check if all containers are ready
		allReady := true
		for _, c := range containers {
			if !c.Ready {
				allReady = false
				break
			}
		}
		if !allReady {
			health = models.HealthDegraded
		}
	}

	startTime := time.Time{}
	if p.Status.StartTime != nil {
		startTime = p.Status.StartTime.Time
	}

	pod := models.Pod{
		BaseEntity: models.BaseEntity{
			ID:          fmt.Sprintf("pod/%s/%s", p.Namespace, p.Name),
			Type:        models.EntityTypePod,
			Labels:      p.Labels,
			Annotations: p.Annotations,
			Health:      health,
			Timestamp:   time.Now(),
		},
		Name:       p.Name,
		Namespace:  p.Namespace,
		Status:     string(p.Status.Phase),
		Phase:      phase,
		NodeName:   p.Spec.NodeName,
		PodIP:      p.Status.PodIP,
		HostIP:     p.Status.HostIP,
		StartTime:  startTime,
		OwnerKind:  ownerKind,
		OwnerName:  ownerName,
		Containers: containers,
		Conditions: conditions,
		VolumeRefs: volumeRefs,
	}

	return pod
}
