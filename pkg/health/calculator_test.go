package health

import (
	"testing"

	"infracanvas/internal/models"
)

func TestCalculateHostHealth(t *testing.T) {
	calc := NewCalculator()

	tests := []struct {
		name     string
		host     *models.Host
		expected models.HealthStatus
	}{
		{
			name: "healthy host - all metrics below thresholds",
			host: &models.Host{
				BaseEntity: models.BaseEntity{
					ID:   "host1",
					Type: models.EntityTypeHost,
				},
				CPUUsagePercent:    50.0,
				MemoryUsagePercent: 60.0,
				Filesystems: []models.Filesystem{
					{MountPoint: "/", UsagePercent: 70.0},
					{MountPoint: "/home", UsagePercent: 50.0},
				},
			},
			expected: models.HealthHealthy,
		},
		{
			name: "degraded host - CPU exceeds threshold",
			host: &models.Host{
				BaseEntity: models.BaseEntity{
					ID:   "host2",
					Type: models.EntityTypeHost,
				},
				CPUUsagePercent:    95.0,
				MemoryUsagePercent: 60.0,
				Filesystems: []models.Filesystem{
					{MountPoint: "/", UsagePercent: 70.0},
				},
			},
			expected: models.HealthDegraded,
		},
		{
			name: "degraded host - memory exceeds threshold",
			host: &models.Host{
				BaseEntity: models.BaseEntity{
					ID:   "host3",
					Type: models.EntityTypeHost,
				},
				CPUUsagePercent:    50.0,
				MemoryUsagePercent: 92.0,
				Filesystems: []models.Filesystem{
					{MountPoint: "/", UsagePercent: 70.0},
				},
			},
			expected: models.HealthDegraded,
		},
		{
			name: "degraded host - disk exceeds threshold",
			host: &models.Host{
				BaseEntity: models.BaseEntity{
					ID:   "host4",
					Type: models.EntityTypeHost,
				},
				CPUUsagePercent:    50.0,
				MemoryUsagePercent: 60.0,
				Filesystems: []models.Filesystem{
					{MountPoint: "/", UsagePercent: 88.0},
					{MountPoint: "/home", UsagePercent: 50.0},
				},
			},
			expected: models.HealthDegraded,
		},
		{
			name: "degraded host - multiple thresholds exceeded",
			host: &models.Host{
				BaseEntity: models.BaseEntity{
					ID:   "host5",
					Type: models.EntityTypeHost,
				},
				CPUUsagePercent:    91.0,
				MemoryUsagePercent: 93.0,
				Filesystems: []models.Filesystem{
					{MountPoint: "/", UsagePercent: 90.0},
				},
			},
			expected: models.HealthDegraded,
		},
		{
			name: "healthy host - at threshold boundaries",
			host: &models.Host{
				BaseEntity: models.BaseEntity{
					ID:   "host6",
					Type: models.EntityTypeHost,
				},
				CPUUsagePercent:    89.9,
				MemoryUsagePercent: 89.9,
				Filesystems: []models.Filesystem{
					{MountPoint: "/", UsagePercent: 84.9},
				},
			},
			expected: models.HealthHealthy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calc.calculateHostHealth(tt.host)
			if result != tt.expected {
				t.Errorf("calculateHostHealth() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCalculateContainerHealth(t *testing.T) {
	calc := NewCalculator()

	tests := []struct {
		name      string
		container *models.Container
		expected  models.HealthStatus
	}{
		{
			name: "healthy container - running",
			container: &models.Container{
				BaseEntity: models.BaseEntity{
					ID:   "container1",
					Type: models.EntityTypeContainer,
				},
				State:  "running",
				Status: "Up 2 hours",
			},
			expected: models.HealthHealthy,
		},
		{
			name: "unhealthy container - exited",
			container: &models.Container{
				BaseEntity: models.BaseEntity{
					ID:   "container2",
					Type: models.EntityTypeContainer,
				},
				State:  "exited",
				Status: "Exited (1) 5 minutes ago",
			},
			expected: models.HealthUnhealthy,
		},
		{
			name: "unhealthy container - dead",
			container: &models.Container{
				BaseEntity: models.BaseEntity{
					ID:   "container3",
					Type: models.EntityTypeContainer,
				},
				State:  "dead",
				Status: "Dead",
			},
			expected: models.HealthUnhealthy,
		},
		{
			name: "unhealthy container - health check failed",
			container: &models.Container{
				BaseEntity: models.BaseEntity{
					ID:   "container4",
					Type: models.EntityTypeContainer,
				},
				State:  "running",
				Status: "Up 1 hour (unhealthy)",
			},
			expected: models.HealthUnhealthy,
		},
		{
			name: "degraded container - paused",
			container: &models.Container{
				BaseEntity: models.BaseEntity{
					ID:   "container5",
					Type: models.EntityTypeContainer,
				},
				State:  "paused",
				Status: "Paused",
			},
			expected: models.HealthDegraded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calc.calculateContainerHealth(tt.container)
			if result != tt.expected {
				t.Errorf("calculateContainerHealth() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCalculateDeploymentHealth(t *testing.T) {
	calc := NewCalculator()

	tests := []struct {
		name       string
		deployment *models.Deployment
		expected   models.HealthStatus
	}{
		{
			name: "healthy deployment - all replicas available",
			deployment: &models.Deployment{
				BaseEntity: models.BaseEntity{
					ID:   "deploy1",
					Type: models.EntityTypeDeployment,
				},
				Replicas:          3,
				AvailableReplicas: 3,
			},
			expected: models.HealthHealthy,
		},
		{
			name: "degraded deployment - some replicas available",
			deployment: &models.Deployment{
				BaseEntity: models.BaseEntity{
					ID:   "deploy2",
					Type: models.EntityTypeDeployment,
				},
				Replicas:          3,
				AvailableReplicas: 2,
			},
			expected: models.HealthDegraded,
		},
		{
			name: "unhealthy deployment - no replicas available",
			deployment: &models.Deployment{
				BaseEntity: models.BaseEntity{
					ID:   "deploy3",
					Type: models.EntityTypeDeployment,
				},
				Replicas:          3,
				AvailableReplicas: 0,
			},
			expected: models.HealthUnhealthy,
		},
		{
			name: "healthy deployment - zero replicas desired",
			deployment: &models.Deployment{
				BaseEntity: models.BaseEntity{
					ID:   "deploy4",
					Type: models.EntityTypeDeployment,
				},
				Replicas:          0,
				AvailableReplicas: 0,
			},
			expected: models.HealthHealthy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calc.calculateDeploymentHealth(tt.deployment)
			if result != tt.expected {
				t.Errorf("calculateDeploymentHealth() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCalculateStatefulSetHealth(t *testing.T) {
	calc := NewCalculator()

	tests := []struct {
		name        string
		statefulSet *models.StatefulSet
		expected    models.HealthStatus
	}{
		{
			name: "healthy statefulset - all replicas ready",
			statefulSet: &models.StatefulSet{
				BaseEntity: models.BaseEntity{
					ID:   "sts1",
					Type: models.EntityTypeStatefulSet,
				},
				Replicas:      3,
				ReadyReplicas: 3,
			},
			expected: models.HealthHealthy,
		},
		{
			name: "degraded statefulset - some replicas ready",
			statefulSet: &models.StatefulSet{
				BaseEntity: models.BaseEntity{
					ID:   "sts2",
					Type: models.EntityTypeStatefulSet,
				},
				Replicas:      3,
				ReadyReplicas: 1,
			},
			expected: models.HealthDegraded,
		},
		{
			name: "unhealthy statefulset - no replicas ready",
			statefulSet: &models.StatefulSet{
				BaseEntity: models.BaseEntity{
					ID:   "sts3",
					Type: models.EntityTypeStatefulSet,
				},
				Replicas:      3,
				ReadyReplicas: 0,
			},
			expected: models.HealthUnhealthy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calc.calculateStatefulSetHealth(tt.statefulSet)
			if result != tt.expected {
				t.Errorf("calculateStatefulSetHealth() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCalculatePodHealth(t *testing.T) {
	calc := NewCalculator()

	tests := []struct {
		name     string
		pod      *models.Pod
		expected models.HealthStatus
	}{
		{
			name: "healthy pod - running with all containers ready",
			pod: &models.Pod{
				BaseEntity: models.BaseEntity{
					ID:   "pod1",
					Type: models.EntityTypePod,
				},
				Phase:  "Running",
				Status: "Running",
				Containers: []models.PodContainer{
					{Name: "app", Ready: true},
					{Name: "sidecar", Ready: true},
				},
			},
			expected: models.HealthHealthy,
		},
		{
			name: "degraded pod - running with some containers not ready",
			pod: &models.Pod{
				BaseEntity: models.BaseEntity{
					ID:   "pod2",
					Type: models.EntityTypePod,
				},
				Phase:  "Running",
				Status: "Running",
				Containers: []models.PodContainer{
					{Name: "app", Ready: true},
					{Name: "sidecar", Ready: false},
				},
			},
			expected: models.HealthDegraded,
		},
		{
			name: "unhealthy pod - failed phase",
			pod: &models.Pod{
				BaseEntity: models.BaseEntity{
					ID:   "pod3",
					Type: models.EntityTypePod,
				},
				Phase:  "Failed",
				Status: "Failed",
			},
			expected: models.HealthUnhealthy,
		},
		{
			name: "unhealthy pod - unknown phase",
			pod: &models.Pod{
				BaseEntity: models.BaseEntity{
					ID:   "pod4",
					Type: models.EntityTypePod,
				},
				Phase:  "Unknown",
				Status: "Unknown",
			},
			expected: models.HealthUnhealthy,
		},
		{
			name: "unhealthy pod - crashloopbackoff in status",
			pod: &models.Pod{
				BaseEntity: models.BaseEntity{
					ID:   "pod5",
					Type: models.EntityTypePod,
				},
				Phase:  "Running",
				Status: "CrashLoopBackOff",
			},
			expected: models.HealthUnhealthy,
		},
		{
			name: "degraded pod - pending",
			pod: &models.Pod{
				BaseEntity: models.BaseEntity{
					ID:   "pod6",
					Type: models.EntityTypePod,
				},
				Phase:  "Pending",
				Status: "Pending",
			},
			expected: models.HealthDegraded,
		},
		{
			name: "healthy pod - succeeded",
			pod: &models.Pod{
				BaseEntity: models.BaseEntity{
					ID:   "pod7",
					Type: models.EntityTypePod,
				},
				Phase:  "Succeeded",
				Status: "Completed",
			},
			expected: models.HealthHealthy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calc.calculatePodHealth(tt.pod)
			if result != tt.expected {
				t.Errorf("calculatePodHealth() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCalculateNodeHealth(t *testing.T) {
	calc := NewCalculator()

	tests := []struct {
		name     string
		node     *models.Node
		expected models.HealthStatus
	}{
		{
			name: "healthy node - ready",
			node: &models.Node{
				BaseEntity: models.BaseEntity{
					ID:   "node1",
					Type: models.EntityTypeNode,
				},
				Status: "Ready",
			},
			expected: models.HealthHealthy,
		},
		{
			name: "unhealthy node - not ready",
			node: &models.Node{
				BaseEntity: models.BaseEntity{
					ID:   "node2",
					Type: models.EntityTypeNode,
				},
				Status: "NotReady",
			},
			expected: models.HealthUnhealthy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calc.calculateNodeHealth(tt.node)
			if result != tt.expected {
				t.Errorf("calculateNodeHealth() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCalculateAggregateHealth(t *testing.T) {
	calc := NewCalculator()

	tests := []struct {
		name     string
		entities []models.Entity
		expected models.HealthStatus
	}{
		{
			name:     "empty entities",
			entities: []models.Entity{},
			expected: models.HealthUnknown,
		},
		{
			name: "all healthy",
			entities: []models.Entity{
				&models.Host{
					BaseEntity:         models.BaseEntity{ID: "h1", Type: models.EntityTypeHost},
					CPUUsagePercent:    50.0,
					MemoryUsagePercent: 60.0,
					Filesystems:        []models.Filesystem{{UsagePercent: 70.0}},
				},
				&models.Container{
					BaseEntity: models.BaseEntity{ID: "c1", Type: models.EntityTypeContainer},
					State:      "running",
					Status:     "Up",
				},
			},
			expected: models.HealthHealthy,
		},
		{
			name: "one degraded",
			entities: []models.Entity{
				&models.Host{
					BaseEntity:         models.BaseEntity{ID: "h1", Type: models.EntityTypeHost},
					CPUUsagePercent:    50.0,
					MemoryUsagePercent: 60.0,
					Filesystems:        []models.Filesystem{{UsagePercent: 70.0}},
				},
				&models.Deployment{
					BaseEntity:        models.BaseEntity{ID: "d1", Type: models.EntityTypeDeployment},
					Replicas:          3,
					AvailableReplicas: 2,
				},
			},
			expected: models.HealthDegraded,
		},
		{
			name: "one unhealthy",
			entities: []models.Entity{
				&models.Host{
					BaseEntity:         models.BaseEntity{ID: "h1", Type: models.EntityTypeHost},
					CPUUsagePercent:    50.0,
					MemoryUsagePercent: 60.0,
					Filesystems:        []models.Filesystem{{UsagePercent: 70.0}},
				},
				&models.Container{
					BaseEntity: models.BaseEntity{ID: "c1", Type: models.EntityTypeContainer},
					State:      "exited",
					Status:     "Exited",
				},
			},
			expected: models.HealthUnhealthy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calc.CalculateAggregateHealth(tt.entities)
			if result != tt.expected {
				t.Errorf("CalculateAggregateHealth() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetHealthReasons(t *testing.T) {
	calc := NewCalculator()

	tests := []struct {
		name            string
		entity          models.Entity
		expectedReasons int
		containsText    string
	}{
		{
			name: "host with high CPU",
			entity: &models.Host{
				BaseEntity:         models.BaseEntity{ID: "h1", Type: models.EntityTypeHost},
				CPUUsagePercent:    95.0,
				MemoryUsagePercent: 60.0,
				Filesystems:        []models.Filesystem{{MountPoint: "/", UsagePercent: 70.0}},
			},
			expectedReasons: 1,
			containsText:    "CPU usage is high",
		},
		{
			name: "container exited",
			entity: &models.Container{
				BaseEntity: models.BaseEntity{ID: "c1", Type: models.EntityTypeContainer},
				State:      "exited",
				Status:     "Exited",
			},
			expectedReasons: 1,
			containsText:    "Container has exited",
		},
		{
			name: "deployment with no replicas",
			entity: &models.Deployment{
				BaseEntity:        models.BaseEntity{ID: "d1", Type: models.EntityTypeDeployment},
				Replicas:          3,
				AvailableReplicas: 0,
			},
			expectedReasons: 1,
			containsText:    "No replicas are available",
		},
		{
			name: "pod in crashloopbackoff",
			entity: &models.Pod{
				BaseEntity: models.BaseEntity{ID: "p1", Type: models.EntityTypePod},
				Phase:      "Running",
				Status:     "CrashLoopBackOff",
			},
			expectedReasons: 1,
			containsText:    "CrashLoopBackOff",
		},
		{
			name: "healthy host",
			entity: &models.Host{
				BaseEntity:         models.BaseEntity{ID: "h1", Type: models.EntityTypeHost},
				CPUUsagePercent:    50.0,
				MemoryUsagePercent: 60.0,
				Filesystems:        []models.Filesystem{{MountPoint: "/", UsagePercent: 70.0}},
			},
			expectedReasons: 1,
			containsText:    "All checks passed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reasons := calc.GetHealthReasons(tt.entity)
			if len(reasons) != tt.expectedReasons {
				t.Errorf("GetHealthReasons() returned %d reasons, want %d", len(reasons), tt.expectedReasons)
			}
			found := false
			for _, reason := range reasons {
				if containsIgnoreCase(reason, tt.containsText) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("GetHealthReasons() reasons %v do not contain expected text %q", reasons, tt.containsText)
			}
		})
	}
}

func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && contains(toLower(s), toLower(substr))))
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if 'A' <= c && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestCalculateHealth_InterfaceMethod(t *testing.T) {
	calc := NewCalculator()

	// Test that CalculateHealth works with the Entity interface
	entities := []models.Entity{
		&models.Host{
			BaseEntity:         models.BaseEntity{ID: "h1", Type: models.EntityTypeHost},
			CPUUsagePercent:    95.0,
			MemoryUsagePercent: 60.0,
			Filesystems:        []models.Filesystem{{UsagePercent: 70.0}},
		},
		&models.Container{
			BaseEntity: models.BaseEntity{ID: "c1", Type: models.EntityTypeContainer},
			State:      "running",
			Status:     "Up",
		},
		&models.Node{
			BaseEntity: models.BaseEntity{ID: "n1", Type: models.EntityTypeNode},
			Status:     "Ready",
		},
	}

	for _, entity := range entities {
		health := calc.CalculateHealth(entity)
		if health == "" {
			t.Errorf("CalculateHealth() returned empty health status for entity %s", entity.GetID())
		}
	}
}

func TestDaemonSetHealth(t *testing.T) {
	calc := NewCalculator()

	tests := []struct {
		name      string
		daemonSet *models.DaemonSet
		expected  models.HealthStatus
	}{
		{
			name: "healthy daemonset - all pods ready",
			daemonSet: &models.DaemonSet{
				BaseEntity: models.BaseEntity{
					ID:   "ds1",
					Type: models.EntityTypeDaemonSet,
				},
				DesiredNumberScheduled: 3,
				NumberReady:            3,
			},
			expected: models.HealthHealthy,
		},
		{
			name: "degraded daemonset - some pods ready",
			daemonSet: &models.DaemonSet{
				BaseEntity: models.BaseEntity{
					ID:   "ds2",
					Type: models.EntityTypeDaemonSet,
				},
				DesiredNumberScheduled: 3,
				NumberReady:            2,
			},
			expected: models.HealthDegraded,
		},
		{
			name: "unhealthy daemonset - no pods ready",
			daemonSet: &models.DaemonSet{
				BaseEntity: models.BaseEntity{
					ID:   "ds3",
					Type: models.EntityTypeDaemonSet,
				},
				DesiredNumberScheduled: 3,
				NumberReady:            0,
			},
			expected: models.HealthUnhealthy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calc.calculateDaemonSetHealth(tt.daemonSet)
			if result != tt.expected {
				t.Errorf("calculateDaemonSetHealth() = %v, want %v", result, tt.expected)
			}
		})
	}
}
