package models

import "time"

// Relation represents a relationship between two entities
type Relation struct {
	SourceID   string            `json:"source_id"`
	TargetID   string            `json:"target_id"`
	Type       RelationType      `json:"type"`
	Properties map[string]string `json:"properties,omitempty"`
}

// RelationType represents the type of relationship between entities
type RelationType string

const (
	RelationRunsOn     RelationType = "RUNS_ON"      // Container -> Host, Pod -> Node
	RelationOwns       RelationType = "OWNS"         // Deployment -> ReplicaSet, ReplicaSet -> Pod
	RelationUses       RelationType = "USES"         // Container -> Image, Pod -> Image
	RelationMounts     RelationType = "MOUNTS"       // Container -> Volume, Pod -> PVC
	RelationExposes    RelationType = "EXPOSES"      // Service -> Pod
	RelationRoutesTo   RelationType = "ROUTES_TO"    // Ingress -> Service
	RelationTargets    RelationType = "TARGETS"      // Service -> Pod (via selector)
	RelationConnectsTo RelationType = "CONNECTS_TO"  // Container -> Network
	RelationReferences RelationType = "REFERENCES"   // Pod -> ConfigMap, Pod -> Secret
	RelationBindsTo    RelationType = "BINDS_TO"     // PVC -> PV
	RelationProvisions RelationType = "PROVISIONS"   // StorageClass -> PV
	RelationDependsOn  RelationType = "DEPENDS_ON"   // Service -> Service
	RelationContains   RelationType = "CONTAINS"     // Namespace -> Pod/Service/Deployment/etc.
	RelationRelatesTo  RelationType = "RELATES_TO"   // Event -> Pod/Deployment/etc.
)

// InfraSnapshot represents a complete snapshot of infrastructure at a point in time
type InfraSnapshot struct {
	Timestamp time.Time         `json:"timestamp"`
	HostID    string            `json:"host_id"`
	Entities  map[string]Entity `json:"entities"` // Key: Entity ID
	Relations []Relation        `json:"relations"`
	Metadata  SnapshotMetadata  `json:"metadata"`
}

// SnapshotMetadata contains metadata about the snapshot collection
type SnapshotMetadata struct {
	CollectionDuration time.Duration      `json:"collection_duration"`
	Scope              []string           `json:"scope"`
	Errors             []CollectionError  `json:"errors,omitempty"`
	PermissionIssues   []string           `json:"permission_issues,omitempty"`
}

// CollectionError represents an error that occurred during collection
type CollectionError struct {
	Layer   string `json:"layer"`
	Message string `json:"message"`
}

// Delta represents incremental changes between two snapshots
type Delta struct {
	Added    map[string]Entity `json:"added"`
	Modified map[string]Entity `json:"modified"`
	Removed  []string          `json:"removed"` // Entity IDs
}
