package models

import "time"

// Entity is the base interface that all infrastructure entities implement
type Entity interface {
	GetID() string
	GetType() EntityType
	GetLabels() map[string]string
	GetAnnotations() map[string]string
	GetHealth() HealthStatus
	GetTimestamp() time.Time
}

// EntityType represents the type of infrastructure entity
type EntityType string

const (
	EntityTypeHost             EntityType = "host"
	EntityTypeProcess          EntityType = "process"
	EntityTypeService          EntityType = "service"
	EntityTypeContainerRuntime EntityType = "container_runtime"
	EntityTypeContainer        EntityType = "container"
	EntityTypeImage            EntityType = "image"
	EntityTypeVolume           EntityType = "volume"
	EntityTypeNetwork          EntityType = "network"
	EntityTypeCluster          EntityType = "cluster"
	EntityTypeNode             EntityType = "node"
	EntityTypeNamespace        EntityType = "namespace"
	EntityTypeDeployment       EntityType = "deployment"
	EntityTypeStatefulSet      EntityType = "statefulset"
	EntityTypeDaemonSet        EntityType = "daemonset"
	EntityTypeJob              EntityType = "job"
	EntityTypeCronJob          EntityType = "cronjob"
	EntityTypePod              EntityType = "pod"
	EntityTypeK8sService       EntityType = "k8s_service"
	EntityTypeIngress          EntityType = "ingress"
	EntityTypeConfigMap        EntityType = "configmap"
	EntityTypeSecret           EntityType = "secret"
	EntityTypePVC              EntityType = "pvc"
	EntityTypePV               EntityType = "pv"
	EntityTypeStorageClass     EntityType = "storageclass"
	EntityTypeEvent            EntityType = "event"
)

// HealthStatus represents the health state of an entity
type HealthStatus string

const (
	HealthHealthy   HealthStatus = "healthy"
	HealthDegraded  HealthStatus = "degraded"
	HealthUnhealthy HealthStatus = "unhealthy"
	HealthUnknown   HealthStatus = "unknown"
)

// BaseEntity provides common fields for all entities
type BaseEntity struct {
	ID          string            `json:"id"`
	Type        EntityType        `json:"type"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	Health      HealthStatus      `json:"health"`
	Timestamp   time.Time         `json:"timestamp"`
}

// GetID returns the entity ID
func (b *BaseEntity) GetID() string {
	return b.ID
}

// GetType returns the entity type
func (b *BaseEntity) GetType() EntityType {
	return b.Type
}

// GetLabels returns the entity labels
func (b *BaseEntity) GetLabels() map[string]string {
	return b.Labels
}

// GetAnnotations returns the entity annotations
func (b *BaseEntity) GetAnnotations() map[string]string {
	return b.Annotations
}

// GetHealth returns the entity health status
func (b *BaseEntity) GetHealth() HealthStatus {
	return b.Health
}

// GetTimestamp returns the entity timestamp
func (b *BaseEntity) GetTimestamp() time.Time {
	return b.Timestamp
}
