package health

import "infracanvas/internal/models"

// HealthCalculator defines the interface for calculating entity health status
type HealthCalculator interface {
	// CalculateHealth calculates the health status for a given entity
	CalculateHealth(entity models.Entity) models.HealthStatus
	
	// CalculateAggregateHealth calculates overall infrastructure health from all entities
	CalculateAggregateHealth(entities []models.Entity) models.HealthStatus
	
	// GetHealthReasons returns human-readable reasons for an entity's health status
	GetHealthReasons(entity models.Entity) []string
}
