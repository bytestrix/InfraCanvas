package output

import (
	"encoding/json"
	"fmt"
	"infracanvas/internal/models"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// createTestSnapshot creates a test InfraSnapshot for testing
func createTestSnapshot() *models.InfraSnapshot {
	now := time.Now()

	host := &models.Host{
		BaseEntity: models.BaseEntity{
			ID:        "host-1",
			Type:      models.EntityTypeHost,
			Health:    models.HealthHealthy,
			Timestamp: now,
			Labels:    map[string]string{"env": "test"},
		},
		Hostname:           "test-host",
		OS:                 "Linux",
		OSVersion:          "Ubuntu 22.04",
		CPUUsagePercent:    45.5,
		MemoryUsagePercent: 60.2,
	}

	container := &models.Container{
		BaseEntity: models.BaseEntity{
			ID:        "container-1",
			Type:      models.EntityTypeContainer,
			Health:    models.HealthHealthy,
			Timestamp: now,
		},
		ContainerID: "abc123",
		Name:        "test-container",
		Image:       "nginx:latest",
		State:       "running",
		CPUPercent:  10.5,
		MemoryUsage: 104857600, // 100MB
	}

	pod := &models.Pod{
		BaseEntity: models.BaseEntity{
			ID:        "pod-1",
			Type:      models.EntityTypePod,
			Health:    models.HealthHealthy,
			Timestamp: now,
		},
		Name:      "test-pod",
		Namespace: "default",
		Phase:     "Running",
		NodeName:  "node-1",
		Containers: []models.PodContainer{
			{
				Name:         "app",
				Image:        "app:v1",
				State:        "running",
				Ready:        true,
				RestartCount: 0,
			},
		},
	}

	entities := map[string]models.Entity{
		"host-1":      host,
		"container-1": container,
		"pod-1":       pod,
	}

	relations := []models.Relation{
		{
			SourceID: "container-1",
			TargetID: "host-1",
			Type:     models.RelationRunsOn,
		},
		{
			SourceID: "pod-1",
			TargetID: "node-1",
			Type:     models.RelationRunsOn,
		},
	}

	return &models.InfraSnapshot{
		Timestamp: now,
		HostID:    "host-1",
		Entities:  entities,
		Relations: relations,
		Metadata: models.SnapshotMetadata{
			CollectionDuration: 2 * time.Second,
			Scope:              []string{"host", "docker", "kubernetes"},
			Errors:             []models.CollectionError{},
		},
	}
}

func TestJSONFormatter(t *testing.T) {
	snapshot := createTestSnapshot()
	formatter := &JSONFormatter{PrettyPrint: true}

	output, err := formatter.Format(snapshot)
	if err != nil {
		t.Fatalf("JSONFormatter.Format() error = %v", err)
	}

	// Verify it's valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("Output is not valid JSON: %v", err)
	}

	// Verify key fields exist
	if _, ok := result["timestamp"]; !ok {
		t.Error("JSON output missing 'timestamp' field")
	}
	if _, ok := result["entities"]; !ok {
		t.Error("JSON output missing 'entities' field")
	}
	if _, ok := result["relations"]; !ok {
		t.Error("JSON output missing 'relations' field")
	}
}

func TestJSONFormatterCompact(t *testing.T) {
	snapshot := createTestSnapshot()
	formatter := &JSONFormatter{PrettyPrint: false}

	output, err := formatter.Format(snapshot)
	if err != nil {
		t.Fatalf("JSONFormatter.Format() error = %v", err)
	}

	// Compact JSON should not have newlines (except at the end)
	lines := strings.Split(string(output), "\n")
	if len(lines) > 2 {
		t.Error("Compact JSON should not have multiple lines")
	}
}

func TestYAMLFormatter(t *testing.T) {
	snapshot := createTestSnapshot()
	formatter := &YAMLFormatter{}

	output, err := formatter.Format(snapshot)
	if err != nil {
		t.Fatalf("YAMLFormatter.Format() error = %v", err)
	}

	// Verify it's valid YAML
	var result map[string]interface{}
	if err := yaml.Unmarshal(output, &result); err != nil {
		t.Fatalf("Output is not valid YAML: %v", err)
	}

	// Verify key fields exist
	if _, ok := result["timestamp"]; !ok {
		t.Error("YAML output missing 'timestamp' field")
	}
	if _, ok := result["entities"]; !ok {
		t.Error("YAML output missing 'entities' field")
	}
	if _, ok := result["relations"]; !ok {
		t.Error("YAML output missing 'relations' field")
	}
}

func TestTableFormatter(t *testing.T) {
	snapshot := createTestSnapshot()
	formatter := &TableFormatter{}

	output, err := formatter.Format(snapshot)
	if err != nil {
		t.Fatalf("TableFormatter.Format() error = %v", err)
	}

	outputStr := string(output)

	// Verify output contains expected sections
	if !strings.Contains(outputStr, "Infrastructure Snapshot") {
		t.Error("Table output missing header")
	}
	if !strings.Contains(outputStr, "HOST") {
		t.Error("Table output missing HOST section")
	}
	if !strings.Contains(outputStr, "CONTAINER") {
		t.Error("Table output missing CONTAINER section")
	}
	if !strings.Contains(outputStr, "POD") {
		t.Error("Table output missing POD section")
	}
	if !strings.Contains(outputStr, "RELATIONSHIPS") {
		t.Error("Table output missing RELATIONSHIPS section")
	}

	// Verify entity data is present
	if !strings.Contains(outputStr, "test-host") {
		t.Error("Table output missing host name")
	}
	if !strings.Contains(outputStr, "test-container") {
		t.Error("Table output missing container name")
	}
	if !strings.Contains(outputStr, "test-pod") {
		t.Error("Table output missing pod name")
	}
}

func TestTableFormatterTruncation(t *testing.T) {
	formatter := &TableFormatter{MaxColumnWidth: 50}

	longString := strings.Repeat("a", 100)
	truncated := formatter.truncate(longString, 20)

	if len(truncated) > 20 {
		t.Errorf("Truncate failed: expected length <= 20, got %d", len(truncated))
	}
	if !strings.HasSuffix(truncated, "...") {
		t.Error("Truncated string should end with '...'")
	}
}

func TestTableFormatterColorization(t *testing.T) {
	formatter := &TableFormatter{}

	// Test health colorization
	healthy := formatter.colorizeHealth(models.HealthHealthy)
	if !strings.Contains(healthy, "\033[32m") {
		t.Error("Healthy status should be green")
	}

	degraded := formatter.colorizeHealth(models.HealthDegraded)
	if !strings.Contains(degraded, "\033[33m") {
		t.Error("Degraded status should be yellow")
	}

	unhealthy := formatter.colorizeHealth(models.HealthUnhealthy)
	if !strings.Contains(unhealthy, "\033[31m") {
		t.Error("Unhealthy status should be red")
	}
}

func TestTableFormatterFormatBytes(t *testing.T) {
	formatter := &TableFormatter{}

	tests := []struct {
		bytes    int64
		expected string
	}{
		{512, "512B"},
		{1024, "1.0KB"},
		{1048576, "1.0MB"},
		{1073741824, "1.0GB"},
	}

	for _, tt := range tests {
		result := formatter.formatBytes(tt.bytes)
		if result != tt.expected {
			t.Errorf("formatBytes(%d) = %s, want %s", tt.bytes, result, tt.expected)
		}
	}
}

func TestNewFormatter(t *testing.T) {
	tests := []struct {
		format   FormatType
		expected string
	}{
		{FormatJSON, "*output.JSONFormatter"},
		{FormatYAML, "*output.YAMLFormatter"},
		{FormatTable, "*output.TableFormatter"},
	}

	for _, tt := range tests {
		formatter := NewFormatter(tt.format)
		typeName := strings.TrimPrefix(strings.TrimPrefix(fmt.Sprintf("%T", formatter), "*"), "output.")
		if !strings.Contains(fmt.Sprintf("%T", formatter), typeName) {
			t.Errorf("NewFormatter(%s) returned unexpected type", tt.format)
		}
	}
}

func TestTableFormatterWithErrors(t *testing.T) {
	snapshot := createTestSnapshot()
	snapshot.Metadata.Errors = []models.CollectionError{
		{
			Layer:   "docker",
			Message: "Failed to connect to Docker socket",
		},
		{
			Layer:   "kubernetes",
			Message: "Kubeconfig not found",
		},
	}

	formatter := &TableFormatter{}
	output, err := formatter.Format(snapshot)
	if err != nil {
		t.Fatalf("TableFormatter.Format() error = %v", err)
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "ERRORS") {
		t.Error("Table output should contain ERRORS section when errors exist")
	}
	if !strings.Contains(outputStr, "docker") {
		t.Error("Table output should contain docker error")
	}
	if !strings.Contains(outputStr, "kubernetes") {
		t.Error("Table output should contain kubernetes error")
	}
}

func TestTableFormatterEmptySnapshot(t *testing.T) {
	snapshot := &models.InfraSnapshot{
		Timestamp: time.Now(),
		HostID:    "test-host",
		Entities:  map[string]models.Entity{},
		Relations: []models.Relation{},
		Metadata: models.SnapshotMetadata{
			CollectionDuration: 1 * time.Second,
			Scope:              []string{"host"},
		},
	}

	formatter := &TableFormatter{}
	output, err := formatter.Format(snapshot)
	if err != nil {
		t.Fatalf("TableFormatter.Format() error = %v", err)
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "Infrastructure Snapshot") {
		t.Error("Table output should contain header even for empty snapshot")
	}
}
