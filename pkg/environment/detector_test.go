package environment

import (
	"testing"
	"time"

	"infracanvas/internal/models"
)

func TestDetectFromHost(t *testing.T) {
	detector := NewDetector()
	
	tests := []struct {
		name     string
		host     *models.Host
		expected Environment
	}{
		{
			name: "hostname with prod pattern",
			host: &models.Host{
				Hostname: "web-prod-01",
			},
			expected: EnvProduction,
		},
		{
			name: "hostname with production pattern",
			host: &models.Host{
				Hostname: "api-production-server",
			},
			expected: EnvProduction,
		},
		{
			name: "hostname with staging pattern",
			host: &models.Host{
				Hostname: "db-staging-01",
			},
			expected: EnvStaging,
		},
		{
			name: "hostname with dev pattern",
			host: &models.Host{
				Hostname: "dev-workstation",
			},
			expected: EnvDev,
		},
		{
			name: "hostname with qa pattern",
			host: &models.Host{
				Hostname: "qa-test-server",
			},
			expected: EnvQA,
		},
		{
			name: "FQDN with prod pattern",
			host: &models.Host{
				Hostname: "server01",
				FQDN:     "server01.prod.example.com",
			},
			expected: EnvProduction,
		},
		{
			name: "cloud tags with environment",
			host: &models.Host{
				Hostname: "generic-host",
				CloudTags: map[string]string{
					"environment": "production",
				},
			},
			expected: EnvProduction,
		},
		{
			name: "cloud tags with env key",
			host: &models.Host{
				Hostname: "generic-host",
				CloudTags: map[string]string{
					"env": "staging",
				},
			},
			expected: EnvStaging,
		},
		{
			name: "no environment indicators",
			host: &models.Host{
				Hostname: "generic-server",
			},
			expected: EnvUnknown,
		},
		{
			name:     "nil host",
			host:     nil,
			expected: EnvUnknown,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.DetectFromHost(tt.host)
			if result != tt.expected {
				t.Errorf("DetectFromHost() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDetectFromContainer(t *testing.T) {
	detector := NewDetector()
	
	tests := []struct {
		name      string
		container *models.Container
		expected  Environment
	}{
		{
			name: "container with environment label",
			container: &models.Container{
				BaseEntity: models.BaseEntity{
					Labels: map[string]string{
						"environment": "production",
					},
				},
				Name: "web-app",
			},
			expected: EnvProduction,
		},
		{
			name: "container with env label",
			container: &models.Container{
				BaseEntity: models.BaseEntity{
					Labels: map[string]string{
						"env": "dev",
					},
				},
				Name: "api-service",
			},
			expected: EnvDev,
		},
		{
			name: "container with tier label",
			container: &models.Container{
				BaseEntity: models.BaseEntity{
					Labels: map[string]string{
						"tier": "staging",
					},
				},
				Name: "cache",
			},
			expected: EnvStaging,
		},
		{
			name: "container name with prod pattern",
			container: &models.Container{
				BaseEntity: models.BaseEntity{
					Labels: map[string]string{},
				},
				Name: "nginx-prod",
			},
			expected: EnvProduction,
		},
		{
			name: "no environment indicators",
			container: &models.Container{
				BaseEntity: models.BaseEntity{
					Labels: map[string]string{},
				},
				Name: "my-app",
			},
			expected: EnvUnknown,
		},
		{
			name:      "nil container",
			container: nil,
			expected:  EnvUnknown,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.DetectFromContainer(tt.container)
			if result != tt.expected {
				t.Errorf("DetectFromContainer() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDetectFromNamespace(t *testing.T) {
	detector := NewDetector()
	
	tests := []struct {
		name      string
		namespace *models.Namespace
		expected  Environment
	}{
		{
			name: "namespace name with production",
			namespace: &models.Namespace{
				Name: "production",
			},
			expected: EnvProduction,
		},
		{
			name: "namespace name with prod",
			namespace: &models.Namespace{
				Name: "prod",
			},
			expected: EnvProduction,
		},
		{
			name: "namespace name with staging",
			namespace: &models.Namespace{
				Name: "staging",
			},
			expected: EnvStaging,
		},
		{
			name: "namespace name with dev",
			namespace: &models.Namespace{
				Name: "dev",
			},
			expected: EnvDev,
		},
		{
			name: "namespace name with qa",
			namespace: &models.Namespace{
				Name: "qa",
			},
			expected: EnvQA,
		},
		{
			name: "namespace with environment label",
			namespace: &models.Namespace{
				BaseEntity: models.BaseEntity{
					Labels: map[string]string{
						"environment": "staging",
					},
				},
				Name: "my-namespace",
			},
			expected: EnvStaging,
		},
		{
			name: "namespace with env label",
			namespace: &models.Namespace{
				BaseEntity: models.BaseEntity{
					Labels: map[string]string{
						"env": "prod",
					},
				},
				Name: "apps",
			},
			expected: EnvProduction,
		},
		{
			name: "no environment indicators",
			namespace: &models.Namespace{
				Name: "default",
			},
			expected: EnvUnknown,
		},
		{
			name:      "nil namespace",
			namespace: nil,
			expected:  EnvUnknown,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.DetectFromNamespace(tt.namespace)
			if result != tt.expected {
				t.Errorf("DetectFromNamespace() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDetectFromPod(t *testing.T) {
	detector := NewDetector()
	
	tests := []struct {
		name     string
		pod      *models.Pod
		expected Environment
	}{
		{
			name: "pod in production namespace",
			pod: &models.Pod{
				Namespace: "production",
			},
			expected: EnvProduction,
		},
		{
			name: "pod with environment label",
			pod: &models.Pod{
				BaseEntity: models.BaseEntity{
					Labels: map[string]string{
						"environment": "dev",
					},
				},
				Namespace: "default",
			},
			expected: EnvDev,
		},
		{
			name: "pod with tier label",
			pod: &models.Pod{
				BaseEntity: models.BaseEntity{
					Labels: map[string]string{
						"tier": "qa",
					},
				},
				Namespace: "default",
			},
			expected: EnvQA,
		},
		{
			name: "no environment indicators",
			pod: &models.Pod{
				Namespace: "kube-system",
			},
			expected: EnvUnknown,
		},
		{
			name:     "nil pod",
			pod:      nil,
			expected: EnvUnknown,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.DetectFromPod(tt.pod)
			if result != tt.expected {
				t.Errorf("DetectFromPod() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDetectFromDeployment(t *testing.T) {
	detector := NewDetector()
	
	tests := []struct {
		name       string
		deployment *models.Deployment
		expected   Environment
	}{
		{
			name: "deployment in prod namespace",
			deployment: &models.Deployment{
				Namespace: "prod",
			},
			expected: EnvProduction,
		},
		{
			name: "deployment with env label",
			deployment: &models.Deployment{
				BaseEntity: models.BaseEntity{
					Labels: map[string]string{
						"env": "staging",
					},
				},
				Namespace: "apps",
			},
			expected: EnvStaging,
		},
		{
			name: "no environment indicators",
			deployment: &models.Deployment{
				Namespace: "default",
			},
			expected: EnvUnknown,
		},
		{
			name:       "nil deployment",
			deployment: nil,
			expected:   EnvUnknown,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.DetectFromDeployment(tt.deployment)
			if result != tt.expected {
				t.Errorf("DetectFromDeployment() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDetectFromStatefulSet(t *testing.T) {
	detector := NewDetector()
	
	tests := []struct {
		name        string
		statefulset *models.StatefulSet
		expected    Environment
	}{
		{
			name: "statefulset in staging namespace",
			statefulset: &models.StatefulSet{
				Namespace: "staging",
			},
			expected: EnvStaging,
		},
		{
			name: "statefulset with environment label",
			statefulset: &models.StatefulSet{
				BaseEntity: models.BaseEntity{
					Labels: map[string]string{
						"environment": "qa",
					},
				},
				Namespace: "databases",
			},
			expected: EnvQA,
		},
		{
			name: "no environment indicators",
			statefulset: &models.StatefulSet{
				Namespace: "default",
			},
			expected: EnvUnknown,
		},
		{
			name:        "nil statefulset",
			statefulset: nil,
			expected:    EnvUnknown,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.DetectFromStatefulSet(tt.statefulset)
			if result != tt.expected {
				t.Errorf("DetectFromStatefulSet() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDetectFromDaemonSet(t *testing.T) {
	detector := NewDetector()
	
	tests := []struct {
		name      string
		daemonset *models.DaemonSet
		expected  Environment
	}{
		{
			name: "daemonset in dev namespace",
			daemonset: &models.DaemonSet{
				Namespace: "dev",
			},
			expected: EnvDev,
		},
		{
			name: "daemonset with stage label",
			daemonset: &models.DaemonSet{
				BaseEntity: models.BaseEntity{
					Labels: map[string]string{
						"stage": "production",
					},
				},
				Namespace: "monitoring",
			},
			expected: EnvProduction,
		},
		{
			name: "no environment indicators",
			daemonset: &models.DaemonSet{
				Namespace: "kube-system",
			},
			expected: EnvUnknown,
		},
		{
			name:      "nil daemonset",
			daemonset: nil,
			expected:  EnvUnknown,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.DetectFromDaemonSet(tt.daemonset)
			if result != tt.expected {
				t.Errorf("DetectFromDaemonSet() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestNormalizeEnvironment(t *testing.T) {
	detector := NewDetector()
	
	tests := []struct {
		name     string
		value    string
		expected Environment
	}{
		{"prod", "prod", EnvProduction},
		{"production", "production", EnvProduction},
		{"PRODUCTION", "PRODUCTION", EnvProduction},
		{"prd", "prd", EnvProduction},
		{"staging", "staging", EnvStaging},
		{"stage", "stage", EnvStaging},
		{"stg", "stg", EnvStaging},
		{"qa", "qa", EnvQA},
		{"QA", "QA", EnvQA},
		{"dev", "dev", EnvDev},
		{"development", "development", EnvDev},
		{"test", "test", EnvTest},
		{"testing", "testing", EnvTest},
		{"unknown value", "random", EnvUnknown},
		{"empty string", "", EnvUnknown},
		{"whitespace", "  ", EnvUnknown},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.normalizeEnvironment(tt.value)
			if result != tt.expected {
				t.Errorf("normalizeEnvironment(%q) = %v, want %v", tt.value, result, tt.expected)
			}
		})
	}
}

func TestDetectFromSnapshot(t *testing.T) {
	detector := NewDetector()
	
	snapshot := &models.InfraSnapshot{
		Timestamp: time.Now(),
		Entities: map[string]models.Entity{
			"host-1": &models.Host{
				BaseEntity: models.BaseEntity{
					ID:   "host-1",
					Type: models.EntityTypeHost,
				},
				Hostname: "prod-server-01",
			},
			"container-1": &models.Container{
				BaseEntity: models.BaseEntity{
					ID:   "container-1",
					Type: models.EntityTypeContainer,
					Labels: map[string]string{
						"environment": "staging",
					},
				},
				Name: "web-app",
			},
			"namespace-1": &models.Namespace{
				BaseEntity: models.BaseEntity{
					ID:   "namespace-1",
					Type: models.EntityTypeNamespace,
				},
				Name: "dev",
			},
			"pod-1": &models.Pod{
				BaseEntity: models.BaseEntity{
					ID:   "pod-1",
					Type: models.EntityTypePod,
					Labels: map[string]string{
						"env": "qa",
					},
				},
				Namespace: "default",
			},
			"deployment-1": &models.Deployment{
				BaseEntity: models.BaseEntity{
					ID:   "deployment-1",
					Type: models.EntityTypeDeployment,
				},
				Namespace: "production",
			},
		},
	}
	
	envMap := detector.DetectFromSnapshot(snapshot)
	
	// Verify expected environments
	expected := map[string]Environment{
		"host-1":       EnvProduction,
		"container-1":  EnvStaging,
		"namespace-1":  EnvDev,
		"pod-1":        EnvQA,
		"deployment-1": EnvProduction,
	}
	
	for id, expectedEnv := range expected {
		if env, exists := envMap[id]; !exists {
			t.Errorf("Expected environment for %s, but not found in map", id)
		} else if env != expectedEnv {
			t.Errorf("Environment for %s = %v, want %v", id, env, expectedEnv)
		}
	}
}

func TestDetectFromSnapshot_NilSnapshot(t *testing.T) {
	detector := NewDetector()
	
	envMap := detector.DetectFromSnapshot(nil)
	
	if envMap == nil {
		t.Error("DetectFromSnapshot(nil) returned nil, expected empty map")
	}
	
	if len(envMap) != 0 {
		t.Errorf("DetectFromSnapshot(nil) returned map with %d entries, expected 0", len(envMap))
	}
}

func TestMatchHostname(t *testing.T) {
	detector := NewDetector()
	
	tests := []struct {
		hostname string
		expected Environment
	}{
		{"prod-web-01", EnvProduction},
		{"production-api", EnvProduction},
		{"prd-db", EnvProduction},
		{"staging-server", EnvStaging},
		{"stage-app", EnvStaging},
		{"stg-cache", EnvStaging},
		{"dev-workstation", EnvDev},
		{"development-box", EnvDev},
		{"qa-server", EnvQA},
		{"quality-test", EnvQA},
		{"test-runner", EnvTest},
		{"tst-machine", EnvTest},
		{"generic-host", EnvUnknown},
		{"", EnvUnknown},
	}
	
	for _, tt := range tests {
		t.Run(tt.hostname, func(t *testing.T) {
			result := detector.matchHostname(tt.hostname)
			if result != tt.expected {
				t.Errorf("matchHostname(%q) = %v, want %v", tt.hostname, result, tt.expected)
			}
		})
	}
}

func TestMatchNamespace(t *testing.T) {
	detector := NewDetector()
	
	tests := []struct {
		namespace string
		expected  Environment
	}{
		{"production", EnvProduction},
		{"prod", EnvProduction},
		{"prd-apps", EnvProduction},
		{"staging", EnvStaging},
		{"stage", EnvStaging},
		{"stg-services", EnvStaging},
		{"dev", EnvDev},
		{"development", EnvDev},
		{"qa", EnvQA},
		{"qas-testing", EnvQA},
		{"test", EnvTest},
		{"testing", EnvTest},
		{"default", EnvUnknown},
		{"kube-system", EnvUnknown},
		{"", EnvUnknown},
	}
	
	for _, tt := range tests {
		t.Run(tt.namespace, func(t *testing.T) {
			result := detector.matchNamespace(tt.namespace)
			if result != tt.expected {
				t.Errorf("matchNamespace(%q) = %v, want %v", tt.namespace, result, tt.expected)
			}
		})
	}
}

func TestDetectFromLabels(t *testing.T) {
	detector := NewDetector()
	
	tests := []struct {
		name     string
		labels   map[string]string
		expected Environment
	}{
		{
			name: "environment label with production",
			labels: map[string]string{
				"environment": "production",
			},
			expected: EnvProduction,
		},
		{
			name: "env label with staging",
			labels: map[string]string{
				"env": "staging",
			},
			expected: EnvStaging,
		},
		{
			name: "tier label with dev",
			labels: map[string]string{
				"tier": "dev",
			},
			expected: EnvDev,
		},
		{
			name: "stage label with qa",
			labels: map[string]string{
				"stage": "qa",
			},
			expected: EnvQA,
		},
		{
			name: "multiple labels, environment takes precedence",
			labels: map[string]string{
				"environment": "prod",
				"tier":        "staging",
			},
			expected: EnvProduction,
		},
		{
			name: "no environment labels",
			labels: map[string]string{
				"app": "web",
				"version": "1.0",
			},
			expected: EnvUnknown,
		},
		{
			name:     "nil labels",
			labels:   nil,
			expected: EnvUnknown,
		},
		{
			name:     "empty labels",
			labels:   map[string]string{},
			expected: EnvUnknown,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.detectFromLabels(tt.labels)
			if result != tt.expected {
				t.Errorf("detectFromLabels() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCaseInsensitiveMatching(t *testing.T) {
	detector := NewDetector()
	
	// Test hostname patterns are case-insensitive
	hostnames := []string{"PROD-server", "Prod-Server", "prod-SERVER"}
	for _, hostname := range hostnames {
		result := detector.matchHostname(hostname)
		if result != EnvProduction {
			t.Errorf("matchHostname(%q) = %v, want %v (case-insensitive test)", hostname, result, EnvProduction)
		}
	}
	
	// Test label values are case-insensitive
	labels := []string{"PRODUCTION", "Production", "PrOdUcTiOn"}
	for _, label := range labels {
		result := detector.normalizeEnvironment(label)
		if result != EnvProduction {
			t.Errorf("normalizeEnvironment(%q) = %v, want %v (case-insensitive test)", label, result, EnvProduction)
		}
	}
}
