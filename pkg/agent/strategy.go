package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"infracanvas/internal/models"
	"infracanvas/pkg/orchestrator"
)

// CollectionStrategy defines how data should be collected
type CollectionStrategy string

const (
	StrategyStatic   CollectionStrategy = "static"   // Collect once on startup
	StrategyPeriodic CollectionStrategy = "periodic" // Poll at regular intervals
	StrategyEvent    CollectionStrategy = "event"    // Watch for events
)

// DataClassification maps data types to collection strategies
type DataClassification struct {
	Layer    string
	Strategy CollectionStrategy
	Interval time.Duration
}

// StrategyManager manages collection strategies and scheduling
type StrategyManager struct {
	config       *Config
	orchestrator *orchestrator.Orchestrator
	lastSnapshot *models.InfraSnapshot
	staticCache  map[string]interface{}
	mu           sync.RWMutex
}

// NewStrategyManager creates a new collection strategy manager
func NewStrategyManager(config *Config, orch *orchestrator.Orchestrator) *StrategyManager {
	return &StrategyManager{
		config:       config,
		orchestrator: orch,
		staticCache:  make(map[string]interface{}),
	}
}

// CollectInitial performs the initial full discovery
func (sm *StrategyManager) CollectInitial(ctx context.Context) (*models.InfraSnapshot, error) {
	snapshot, err := sm.orchestrator.Discover(ctx, sm.config.Scope)
	if err != nil {
		return nil, fmt.Errorf("initial collection failed: %w", err)
	}

	sm.mu.Lock()
	sm.lastSnapshot = snapshot
	sm.mu.Unlock()

	return snapshot, nil
}

// CollectPeriodic performs periodic collection for a specific layer
func (sm *StrategyManager) CollectPeriodic(ctx context.Context, layer string) (*models.InfraSnapshot, error) {
	// Perform discovery for the specific layer
	snapshot, err := sm.orchestrator.Discover(ctx, []string{layer})
	if err != nil {
		return nil, fmt.Errorf("periodic collection for %s failed: %w", layer, err)
	}

	return snapshot, nil
}

// CalculateDelta calculates the difference between two snapshots
func (sm *StrategyManager) CalculateDelta(previous, current *models.InfraSnapshot) *models.Delta {
	if previous == nil {
		// If no previous snapshot, everything is new
		return &models.Delta{
			Added:    current.Entities,
			Modified: make(map[string]models.Entity),
			Removed:  []string{},
		}
	}

	delta := &models.Delta{
		Added:    make(map[string]models.Entity),
		Modified: make(map[string]models.Entity),
		Removed:  []string{},
	}

	// Find added and modified entities
	for id, entity := range current.Entities {
		if prevEntity, exists := previous.Entities[id]; !exists {
			// Entity is new
			delta.Added[id] = entity
		} else if !entitiesEqual(prevEntity, entity) {
			// Entity has changed
			delta.Modified[id] = entity
		}
	}

	// Find removed entities
	for id := range previous.Entities {
		if _, exists := current.Entities[id]; !exists {
			delta.Removed = append(delta.Removed, id)
		}
	}

	return delta
}

// UpdateLastSnapshot updates the stored last snapshot
func (sm *StrategyManager) UpdateLastSnapshot(snapshot *models.InfraSnapshot) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.lastSnapshot = snapshot
}

// GetLastSnapshot returns the last collected snapshot
func (sm *StrategyManager) GetLastSnapshot() *models.InfraSnapshot {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.lastSnapshot
}

// MergeSnapshots merges a partial snapshot into the full snapshot
func (sm *StrategyManager) MergeSnapshots(base, partial *models.InfraSnapshot) *models.InfraSnapshot {
	if base == nil {
		return partial
	}

	merged := &models.InfraSnapshot{
		Timestamp: partial.Timestamp,
		HostID:    base.HostID,
		Entities:  make(map[string]models.Entity),
		Relations: base.Relations,
		Metadata:  partial.Metadata,
	}

	// Copy all entities from base
	for id, entity := range base.Entities {
		merged.Entities[id] = entity
	}

	// Overlay entities from partial snapshot
	for id, entity := range partial.Entities {
		merged.Entities[id] = entity
	}

	return merged
}

// entitiesEqual compares two entities for equality
func entitiesEqual(a, b models.Entity) bool {
	// Simple comparison based on type and basic fields
	if a.GetType() != b.GetType() {
		return false
	}

	if a.GetHealth() != b.GetHealth() {
		return false
	}

	// For more detailed comparison, we'd need to compare specific fields
	// based on entity type. For now, this is a simplified version.
	return false
}

// CacheStatic stores static data in the cache
func (sm *StrategyManager) CacheStatic(key string, value interface{}) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.staticCache[key] = value
}

// GetCachedStatic retrieves static data from the cache
func (sm *StrategyManager) GetCachedStatic(key string) (interface{}, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	value, exists := sm.staticCache[key]
	return value, exists
}
