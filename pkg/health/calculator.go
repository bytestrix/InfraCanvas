package health

import (
	"fmt"
	"strings"

	"infracanvas/internal/models"
)

// Calculator implements the HealthCalculator interface
type Calculator struct{}

// Compile-time check to ensure Calculator implements HealthCalculator
var _ HealthCalculator = (*Calculator)(nil)

// NewCalculator creates a new health calculator
func NewCalculator() *Calculator {
	return &Calculator{}
}

// CalculateHealth calculates the health status for a given entity
func (c *Calculator) CalculateHealth(entity models.Entity) models.HealthStatus {
	switch e := entity.(type) {
	case *models.Host:
		return c.calculateHostHealth(e)
	case *models.Container:
		return c.calculateContainerHealth(e)
	case *models.Deployment:
		return c.calculateDeploymentHealth(e)
	case *models.StatefulSet:
		return c.calculateStatefulSetHealth(e)
	case *models.DaemonSet:
		return c.calculateDaemonSetHealth(e)
	case *models.Pod:
		return c.calculatePodHealth(e)
	case *models.Node:
		return c.calculateNodeHealth(e)
	default:
		return models.HealthUnknown
	}
}

// calculateHostHealth calculates health for a host based on resource usage thresholds
func (c *Calculator) calculateHostHealth(host *models.Host) models.HealthStatus {
	// Requirements:
	// - Healthy: CPU < 90%, memory < 90%, disk < 85%
	// - Degraded: exceeds thresholds but operational
	
	cpuExceeded := host.CPUUsagePercent >= 90.0
	memoryExceeded := host.MemoryUsagePercent >= 90.0
	
	diskExceeded := false
	for _, fs := range host.Filesystems {
		if fs.UsagePercent >= 85.0 {
			diskExceeded = true
			break
		}
	}
	
	if cpuExceeded || memoryExceeded || diskExceeded {
		return models.HealthDegraded
	}
	
	return models.HealthHealthy
}

// calculateContainerHealth calculates health for a Docker container
func (c *Calculator) calculateContainerHealth(container *models.Container) models.HealthStatus {
	// Requirements:
	// - Healthy: state is running and health check passes
	// - Unhealthy: state is exited, dead, or health check fails
	
	state := strings.ToLower(container.State)
	
	switch state {
	case "running":
		// Check if status indicates health check failure
		statusLower := strings.ToLower(container.Status)
		if strings.Contains(statusLower, "unhealthy") {
			return models.HealthUnhealthy
		}
		return models.HealthHealthy
	case "exited", "dead":
		return models.HealthUnhealthy
	case "paused":
		return models.HealthDegraded
	default:
		return models.HealthUnknown
	}
}

// calculateDeploymentHealth calculates health for a Kubernetes Deployment
func (c *Calculator) calculateDeploymentHealth(deployment *models.Deployment) models.HealthStatus {
	// Requirements:
	// - Healthy: available replicas == desired replicas
	// - Degraded: available < desired but > 0
	// - Unhealthy: available == 0
	
	desired := deployment.Replicas
	available := deployment.AvailableReplicas
	
	if available == 0 && desired > 0 {
		return models.HealthUnhealthy
	}
	
	if available < desired {
		return models.HealthDegraded
	}
	
	return models.HealthHealthy
}

// calculateStatefulSetHealth calculates health for a Kubernetes StatefulSet
func (c *Calculator) calculateStatefulSetHealth(statefulSet *models.StatefulSet) models.HealthStatus {
	// Requirements:
	// - Healthy: ready replicas == desired replicas
	// - Degraded: ready < desired but > 0
	// - Unhealthy: ready == 0
	
	desired := statefulSet.Replicas
	ready := statefulSet.ReadyReplicas
	
	if ready == 0 && desired > 0 {
		return models.HealthUnhealthy
	}
	
	if ready < desired {
		return models.HealthDegraded
	}
	
	return models.HealthHealthy
}

// calculateDaemonSetHealth calculates health for a Kubernetes DaemonSet
func (c *Calculator) calculateDaemonSetHealth(daemonSet *models.DaemonSet) models.HealthStatus {
	// Requirements:
	// - Healthy: number ready == desired number scheduled
	// - Degraded: number ready < desired but > 0
	// - Unhealthy: number ready == 0
	
	desired := daemonSet.DesiredNumberScheduled
	ready := daemonSet.NumberReady
	
	if ready == 0 && desired > 0 {
		return models.HealthUnhealthy
	}
	
	if ready < desired {
		return models.HealthDegraded
	}
	
	return models.HealthHealthy
}

// calculatePodHealth calculates health for a Kubernetes Pod
func (c *Calculator) calculatePodHealth(pod *models.Pod) models.HealthStatus {
	// Requirements:
	// - Healthy: phase is Running and all containers are ready
	// - Unhealthy: phase is Failed, Unknown, or CrashLoopBackOff
	
	phase := pod.Phase
	
	// Check for unhealthy phases
	switch phase {
	case "Failed", "Unknown":
		return models.HealthUnhealthy
	}
	
	// Check for CrashLoopBackOff in status or conditions
	statusLower := strings.ToLower(pod.Status)
	if strings.Contains(statusLower, "crashloopbackoff") {
		return models.HealthUnhealthy
	}
	
	// Check conditions for CrashLoopBackOff
	for _, condition := range pod.Conditions {
		if strings.Contains(strings.ToLower(condition.Reason), "crashloopbackoff") {
			return models.HealthUnhealthy
		}
	}
	
	// If phase is Running, check if all containers are ready
	if phase == "Running" {
		allReady := true
		for _, container := range pod.Containers {
			if !container.Ready {
				allReady = false
				break
			}
		}
		
		if allReady && len(pod.Containers) > 0 {
			return models.HealthHealthy
		}
		
		// Some containers not ready
		return models.HealthDegraded
	}
	
	// Pending or other states
	if phase == "Pending" {
		return models.HealthDegraded
	}
	
	// Succeeded is healthy (completed pods)
	if phase == "Succeeded" {
		return models.HealthHealthy
	}
	
	return models.HealthUnknown
}

// calculateNodeHealth calculates health for a Kubernetes Node
func (c *Calculator) calculateNodeHealth(node *models.Node) models.HealthStatus {
	// Requirements:
	// - Unhealthy: status is NotReady
	// - Healthy: status is Ready
	
	if node.Status == "NotReady" {
		return models.HealthUnhealthy
	}
	
	if node.Status == "Ready" {
		return models.HealthHealthy
	}
	
	return models.HealthUnknown
}

// CalculateAggregateHealth calculates overall infrastructure health from all entities
func (c *Calculator) CalculateAggregateHealth(entities []models.Entity) models.HealthStatus {
	if len(entities) == 0 {
		return models.HealthUnknown
	}
	
	healthCounts := map[models.HealthStatus]int{
		models.HealthHealthy:   0,
		models.HealthDegraded:  0,
		models.HealthUnhealthy: 0,
		models.HealthUnknown:   0,
	}
	
	for _, entity := range entities {
		health := c.CalculateHealth(entity)
		healthCounts[health]++
	}
	
	// If any entity is unhealthy, overall is unhealthy
	if healthCounts[models.HealthUnhealthy] > 0 {
		return models.HealthUnhealthy
	}
	
	// If any entity is degraded, overall is degraded
	if healthCounts[models.HealthDegraded] > 0 {
		return models.HealthDegraded
	}
	
	// If all entities are healthy
	if healthCounts[models.HealthHealthy] == len(entities) {
		return models.HealthHealthy
	}
	
	// Otherwise unknown
	return models.HealthUnknown
}

// GetHealthReasons returns human-readable reasons for an entity's health status
func (c *Calculator) GetHealthReasons(entity models.Entity) []string {
	reasons := []string{}
	
	switch e := entity.(type) {
	case *models.Host:
		if e.CPUUsagePercent >= 90.0 {
			reasons = append(reasons, fmt.Sprintf("CPU usage is high: %.1f%%", e.CPUUsagePercent))
		}
		if e.MemoryUsagePercent >= 90.0 {
			reasons = append(reasons, fmt.Sprintf("Memory usage is high: %.1f%%", e.MemoryUsagePercent))
		}
		for _, fs := range e.Filesystems {
			if fs.UsagePercent >= 85.0 {
				reasons = append(reasons, fmt.Sprintf("Disk usage is high on %s: %.1f%%", fs.MountPoint, fs.UsagePercent))
			}
		}
		
	case *models.Container:
		state := strings.ToLower(e.State)
		if state == "exited" {
			reasons = append(reasons, "Container has exited")
		} else if state == "dead" {
			reasons = append(reasons, "Container is dead")
		} else if strings.Contains(strings.ToLower(e.Status), "unhealthy") {
			reasons = append(reasons, "Health check failed")
		}
		
	case *models.Deployment:
		if e.AvailableReplicas == 0 && e.Replicas > 0 {
			reasons = append(reasons, "No replicas are available")
		} else if e.AvailableReplicas < e.Replicas {
			reasons = append(reasons, fmt.Sprintf("Only %d of %d replicas are available", e.AvailableReplicas, e.Replicas))
		}
		
	case *models.StatefulSet:
		if e.ReadyReplicas == 0 && e.Replicas > 0 {
			reasons = append(reasons, "No replicas are ready")
		} else if e.ReadyReplicas < e.Replicas {
			reasons = append(reasons, fmt.Sprintf("Only %d of %d replicas are ready", e.ReadyReplicas, e.Replicas))
		}
		
	case *models.DaemonSet:
		if e.NumberReady == 0 && e.DesiredNumberScheduled > 0 {
			reasons = append(reasons, "No pods are ready")
		} else if e.NumberReady < e.DesiredNumberScheduled {
			reasons = append(reasons, fmt.Sprintf("Only %d of %d pods are ready", e.NumberReady, e.DesiredNumberScheduled))
		}
		
	case *models.Pod:
		if e.Phase == "Failed" {
			reasons = append(reasons, "Pod has failed")
		} else if e.Phase == "Unknown" {
			reasons = append(reasons, "Pod status is unknown")
		} else if strings.Contains(strings.ToLower(e.Status), "crashloopbackoff") {
			reasons = append(reasons, "Pod is in CrashLoopBackOff")
		} else if e.Phase == "Running" {
			notReadyCount := 0
			for _, container := range e.Containers {
				if !container.Ready {
					notReadyCount++
				}
			}
			if notReadyCount > 0 {
				reasons = append(reasons, fmt.Sprintf("%d of %d containers are not ready", notReadyCount, len(e.Containers)))
			}
		} else if e.Phase == "Pending" {
			reasons = append(reasons, "Pod is pending")
		}
		
	case *models.Node:
		if e.Status == "NotReady" {
			reasons = append(reasons, "Node is not ready")
		}
		// Check conditions for more details
		for _, condition := range e.Conditions {
			if condition.Type == "Ready" && condition.Status != "True" {
				if condition.Reason != "" {
					reasons = append(reasons, fmt.Sprintf("Node not ready: %s", condition.Reason))
				}
			}
		}
	}
	
	if len(reasons) == 0 {
		reasons = append(reasons, "All checks passed")
	}
	
	return reasons
}
