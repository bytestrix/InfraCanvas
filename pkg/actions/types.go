package actions

import "time"

// ActionType represents the type of action to execute
type ActionType string

const (
	// Host actions
	ActionRestartService   ActionType = "restart_service"
	ActionUpdateAgent      ActionType = "update_agent"
	ActionHostRunCommand   ActionType = "host_run_command"
	ActionHostDiskUsage    ActionType = "host_disk_usage"
	ActionHostTopProcesses ActionType = "host_top_processes"
	ActionHostSvcStatus    ActionType = "host_service_status"
	ActionHostJournalctl   ActionType = "host_journalctl"

	// Docker actions
	ActionRestartContainer  ActionType = "restart_container"
	ActionStopContainer     ActionType = "stop_container"
	ActionStartContainer    ActionType = "start_container"
	ActionDockerBuild       ActionType = "docker_build"
	ActionDockerPush        ActionType = "docker_push"
	ActionDockerPull        ActionType = "docker_pull"
	ActionDockerTag         ActionType = "docker_tag"
	ActionDockerImageRemove ActionType = "docker_image_remove"
	ActionDockerLogs        ActionType = "docker_logs"
	ActionDockerExec        ActionType = "docker_exec"

	// Docker extended actions
	ActionDockerUpdateImage ActionType = "docker_update_image"
	ActionDockerPullImage   ActionType = "docker_pull_image"
	ActionDockerRemoveImage ActionType = "docker_remove_image"

	// Kubernetes actions
	ActionScaleDeployment     ActionType = "scale_deployment"
	ActionScaleStatefulSet    ActionType = "scale_statefulset"
	ActionRestartPod          ActionType = "restart_pod"
	ActionK8sUpdateImage      ActionType = "k8s_update_image"
	ActionK8sRolloutRestart   ActionType = "k8s_rollout_restart"
	ActionK8sRolloutUndo      ActionType = "k8s_rollout_undo"
	ActionK8sRolloutStatus    ActionType = "k8s_rollout_status"
	ActionK8sGetLogs          ActionType = "k8s_get_logs"
	ActionK8sExec             ActionType = "k8s_exec"
	ActionK8sApplyManifest    ActionType = "k8s_apply_manifest"
	ActionK8sDeleteResource   ActionType = "k8s_delete_resource"
	ActionK8sGetConfigMap     ActionType = "k8s_get_configmap"
	ActionK8sUpdateConfigMap  ActionType = "k8s_update_configmap"
	ActionK8sGetSecret        ActionType = "k8s_get_secret"
	ActionK8sUpdateSecret     ActionType = "k8s_update_secret"
	ActionK8sPortForward      ActionType = "k8s_port_forward"
	ActionK8sStopPortForward  ActionType = "k8s_stop_port_forward"
	ActionK8sDeletePod        ActionType = "k8s_delete_pod"
	ActionK8sDeleteJob        ActionType = "k8s_delete_job"
	ActionK8sDeleteDeployment ActionType = "k8s_delete_deployment"
	ActionK8sDeleteService    ActionType = "k8s_delete_service"
	ActionK8sCordonNode       ActionType = "k8s_cordon_node"
	ActionK8sUnCordonNode     ActionType = "k8s_uncordon_node"
	ActionK8sDrainNode        ActionType = "k8s_drain_node"
	ActionK8sScaleDeployment    ActionType = "k8s_scale_deployment"
	ActionK8sScaleStatefulSet   ActionType = "k8s_scale_statefulset"
	ActionK8sDeleteStatefulSet  ActionType = "k8s_delete_statefulset"
	ActionK8sRestartDeployment  ActionType = "k8s_restart_deployment"
	ActionK8sRestartStatefulSet ActionType = "k8s_restart_statefulset"
	ActionK8sRestartDaemonSet   ActionType = "k8s_restart_daemonset"
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
	Status    string    `json:"status"`   // pending, in_progress, success, failed
	Progress  int       `json:"progress"` // 0-100
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

