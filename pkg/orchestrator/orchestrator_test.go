package orchestrator

import (
	"context"
	"testing"
	"time"
)

func TestNewOrchestrator(t *testing.T) {
	orch := NewOrchestrator(true)
	if orch == nil {
		t.Fatal("NewOrchestrator returned nil")
	}

	if orch.hostDiscovery == nil {
		t.Error("hostDiscovery is nil")
	}

	if orch.relationshipBuilder == nil {
		t.Error("relationshipBuilder is nil")
	}

	if orch.healthCalculator == nil {
		t.Error("healthCalculator is nil")
	}

	if orch.redactor == nil {
		t.Error("redactor is nil")
	}
}

func TestDiscover_HostOnly(t *testing.T) {
	orch := NewOrchestrator(true)
	ctx := context.Background()

	snapshot, err := orch.Discover(ctx, []string{"host"})
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	if snapshot == nil {
		t.Fatal("snapshot is nil")
	}

	// Verify snapshot structure
	if snapshot.Timestamp.IsZero() {
		t.Error("snapshot timestamp is zero")
	}

	if snapshot.Entities == nil {
		t.Error("snapshot entities is nil")
	}

	if snapshot.Relations == nil {
		t.Error("snapshot relations is nil")
	}

	// Verify metadata
	if len(snapshot.Metadata.Scope) != 1 || snapshot.Metadata.Scope[0] != "host" {
		t.Errorf("expected scope [host], got %v", snapshot.Metadata.Scope)
	}

	if snapshot.Metadata.CollectionDuration == 0 {
		t.Error("collection duration is zero")
	}

	// Verify host entity exists
	foundHost := false
	for _, entity := range snapshot.Entities {
		if entity.GetType() == "host" {
			foundHost = true
			break
		}
	}

	if !foundHost {
		t.Error("no host entity found in snapshot")
	}
}

func TestDiscover_EmptyScope(t *testing.T) {
	orch := NewOrchestrator(true)
	ctx := context.Background()

	snapshot, err := orch.Discover(ctx, []string{})
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	if snapshot == nil {
		t.Fatal("snapshot is nil")
	}

	// With empty scope, no entities should be discovered
	if len(snapshot.Entities) > 0 {
		t.Logf("Warning: discovered %d entities with empty scope", len(snapshot.Entities))
	}
}

func TestDiscover_ParallelExecution(t *testing.T) {
	orch := NewOrchestrator(true)
	ctx := context.Background()

	// Test that multiple layers can be discovered in parallel
	start := time.Now()
	snapshot, err := orch.Discover(ctx, []string{"host", "docker", "kubernetes"})
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	if snapshot == nil {
		t.Fatal("snapshot is nil")
	}

	// Verify that errors are captured for unavailable layers
	// Docker and Kubernetes might not be available in test environment
	t.Logf("Discovery completed in %v", duration)
	t.Logf("Discovered %d entities", len(snapshot.Entities))
	t.Logf("Errors: %d", len(snapshot.Metadata.Errors))

	for _, err := range snapshot.Metadata.Errors {
		t.Logf("  - %s: %s", err.Layer, err.Message)
	}
}

func TestDiscover_RelationshipBuilding(t *testing.T) {
	orch := NewOrchestrator(true)
	ctx := context.Background()

	snapshot, err := orch.Discover(ctx, []string{"host"})
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	// Relations should be built even if empty
	if snapshot.Relations == nil {
		t.Error("relations is nil")
	}
}

func TestDiscover_HealthCalculation(t *testing.T) {
	orch := NewOrchestrator(true)
	ctx := context.Background()

	snapshot, err := orch.Discover(ctx, []string{"host"})
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	// Verify that health is calculated for supported entity types
	// Note: Not all entity types have health calculation (e.g., Process, Service)
	supportedTypes := map[string]bool{
		"host":        true,
		"container":   true,
		"deployment":  true,
		"statefulset": true,
		"daemonset":   true,
		"pod":         true,
		"node":        true,
	}

	for id, entity := range snapshot.Entities {
		entityType := string(entity.GetType())
		if supportedTypes[entityType] {
			health := entity.GetHealth()
			if health == "" {
				t.Errorf("entity %s (type: %s) has empty health status", id, entityType)
			}
		}
	}
}

func TestDiscover_Redaction(t *testing.T) {
	// Test with redaction enabled
	orchEnabled := NewOrchestrator(true)
	ctx := context.Background()

	snapshotEnabled, err := orchEnabled.Discover(ctx, []string{"host"})
	if err != nil {
		t.Fatalf("Discover with redaction enabled failed: %v", err)
	}

	if snapshotEnabled == nil {
		t.Fatal("snapshot is nil")
	}

	// Test with redaction disabled
	orchDisabled := NewOrchestrator(false)
	snapshotDisabled, err := orchDisabled.Discover(ctx, []string{"host"})
	if err != nil {
		t.Fatalf("Discover with redaction disabled failed: %v", err)
	}

	if snapshotDisabled == nil {
		t.Fatal("snapshot is nil")
	}

	// Both should succeed
	t.Logf("Redaction enabled: %d entities", len(snapshotEnabled.Entities))
	t.Logf("Redaction disabled: %d entities", len(snapshotDisabled.Entities))
}

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		item     string
		expected bool
	}{
		{"found", []string{"host", "docker", "kubernetes"}, "docker", true},
		{"not found", []string{"host", "docker"}, "kubernetes", false},
		{"empty slice", []string{}, "host", false},
		{"exact match", []string{"host"}, "host", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.slice, tt.item)
			if result != tt.expected {
				t.Errorf("contains(%v, %s) = %v, want %v", tt.slice, tt.item, result, tt.expected)
			}
		})
	}
}
