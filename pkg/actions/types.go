package actions

import "time"

// ActionType represents the type of action to execute
type ActionType string

const (
	// Host actions
	ActionRestartService ActionType = "restart_service"

	// Docker actions
	ActionRestartContainer ActionType = "restart_container"
	ActionStopContainer    ActionType = "stop_container"
	ActionStartContainer   ActionType = "start_container"
	ActionDockerBuild      ActionType = "docker_build"
	ActionDockerPush       ActionType = "docker_push"
	ActionDockerPull       ActionType = "docker_pull"
	ActionDockerTag        ActionType = "docker_tag"
	ActionDockerImageRemove ActionType = "docker_image_remove"
	ActionDockerLogs       ActionType = "docker_logs"
	ActionDockerExec       ActionType = "docker_exec"

	// Kubernetes actions
	ActionScaleDeployment    ActionType = "scale_deployment"
	ActionScaleStatefulSet   ActionType = "scale_statefulset"
	ActionRestartPod         ActionType = "restart_pod"
	ActionK8sUpdateImage     ActionType = "k8s_update_image"      // ⭐ Main use case
	ActionK8sRolloutRestart  ActionType = "k8s_rollout_restart"
	ActionK8sRolloutUndo     ActionType = "k8s_rollout_undo"
	ActionK8sRolloutStatus   ActionType = "k8s_rollout_status"
	ActionK8sGetLogs         ActionType = "k8s_get_logs"
	ActionK8sExec            ActionType = "k8s_exec"
	ActionK8sApplyManifest   ActionType = "k8s_apply_manifest"
	ActionK8sDeleteResource  ActionType = "k8s_delete_resource"

	// Batch operations
	ActionBatchUpdateImage  ActionType = "batch_update_image"   // ⭐ Multi-VM image update
	ActionBatchScale        ActionType = "batch_scale"
	ActionBatchRestart      ActionType = "batch_restart"
)

// ActionTarget identifies the target entity for an action
type ActionTarget struct {
	Layer      string `json:"layer"`       // host, docker, kubernetes
	EntityType string `json:"entity_type"` // service, container, deployment, etc.
	EntityID   string `json:"entity_id"`   // name or ID of the entity
	Namespace  string `json:"namespace,omitempty"`
}

// Action represents an action to be executed
type Action struct {
	ID          string            `json:"id"`
	Type        ActionType        `json:"type"`
	Target      ActionTarget      `json:"target"`
	Parameters  map[string]string `json:"parameters"`
	RequestedBy string            `json:"requested_by"`
	RequestedAt time.Time         `json:"requested_at"`
}

// ActionResult represents the result of an action execution
type ActionResult struct {
	Success   bool                   `json:"success"`
	Message   string                 `json:"message"`
	Output    string                 `json:"output,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
	StartTime time.Time              `json:"start_time"`
	EndTime   time.Time              `json:"end_time"`
}

// ActionProgress represents progress updates during action execution
type ActionProgress struct {
	ActionID  string    `json:"action_id"`
	Status    string    `json:"status"` // pending, in_progress, success, failed
	Progress  int       `json:"progress"` // 0-100
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// BatchAction represents an action to be executed on multiple targets
type BatchAction struct {
	ID          string            `json:"id"`
	Type        ActionType        `json:"type"`
	Targets     []ActionTarget    `json:"targets"` // Multiple targets
	Parameters  map[string]string `json:"parameters"`
	Options     BatchOptions      `json:"options"`
	RequestedBy string            `json:"requested_by"`
	RequestedAt time.Time         `json:"requested_at"`
}

// BatchOptions configures batch execution behavior
type BatchOptions struct {
	DryRun              bool `json:"dry_run"`
	AutoRollback        bool `json:"auto_rollback"`
	HealthCheckTimeout  int  `json:"health_check_timeout"` // seconds
	MaxParallel         int  `json:"max_parallel"`         // max concurrent executions
	StopOnFirstFailure  bool `json:"stop_on_first_failure"`
}

// BatchResult aggregates results from multiple action executions
type BatchResult struct {
	BatchID      string                    `json:"batch_id"`
	TotalTargets int                       `json:"total_targets"`
	Successful   int                       `json:"successful"`
	Failed       int                       `json:"failed"`
	Results      map[string]*ActionResult  `json:"results"` // target ID -> result
	StartTime    time.Time                 `json:"start_time"`
	EndTime      time.Time                 `json:"end_time"`
}
