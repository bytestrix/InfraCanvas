package actions

import (
	"context"
	"fmt"
	"time"
)

// Executor is the interface for executing actions on infrastructure
type Executor interface {
	// ValidateAction validates that an action is well-formed and can be executed
	ValidateAction(action *Action) error

	// RequiresConfirmation returns true if the action is destructive and requires confirmation
	RequiresConfirmation(action *Action) bool

	// ExecuteAction executes the given action and returns the result
	ExecuteAction(ctx context.Context, action *Action) (*ActionResult, error)
}

// ActionExecutor implements the Executor interface
type ActionExecutor struct {
	hostExecutor       *HostExecutor
	dockerExecutor     *DockerExecutor
	kubernetesExecutor *KubernetesExecutor
}

// NewActionExecutor creates a new action executor
func NewActionExecutor() (*ActionExecutor, error) {
	hostExec := NewHostExecutor()

	dockerExec, err := NewDockerExecutor()
	if err != nil {
		// Docker may not be available, that's okay
		dockerExec = nil
	}

	k8sExec, err := NewKubernetesExecutor()
	if err != nil {
		// Kubernetes may not be available, that's okay
		k8sExec = nil
	}

	return &ActionExecutor{
		hostExecutor:       hostExec,
		dockerExecutor:     dockerExec,
		kubernetesExecutor: k8sExec,
	}, nil
}

// ValidateAction validates that an action is well-formed and can be executed
func (e *ActionExecutor) ValidateAction(action *Action) error {
	if action == nil {
		return fmt.Errorf("action cannot be nil")
	}

	if action.Type == "" {
		return fmt.Errorf("action type is required")
	}

	if action.Target.Layer == "" {
		return fmt.Errorf("target layer is required")
	}

	if action.Target.EntityID == "" {
		return fmt.Errorf("target entity ID is required")
	}

	// Validate layer-specific requirements
	switch action.Target.Layer {
	case "host":
		if e.hostExecutor == nil {
			return fmt.Errorf("host executor not available")
		}
		return e.hostExecutor.ValidateAction(action)

	case "docker":
		if e.dockerExecutor == nil {
			return fmt.Errorf("docker is not available")
		}
		return e.dockerExecutor.ValidateAction(action)

	case "kubernetes":
		if e.kubernetesExecutor == nil {
			return fmt.Errorf("kubernetes is not available")
		}
		return e.kubernetesExecutor.ValidateAction(action)

	default:
		return fmt.Errorf("unknown layer: %s", action.Target.Layer)
	}
}

// RequiresConfirmation returns true if the action is destructive and requires confirmation
func (e *ActionExecutor) RequiresConfirmation(action *Action) bool {
	// All actions are considered potentially destructive and require confirmation
	// except for read-only operations (which we don't have in this implementation)
	return true
}

// ExecuteAction executes the given action and returns the result
func (e *ActionExecutor) ExecuteAction(ctx context.Context, action *Action) (*ActionResult, error) {
	startTime := time.Now()

	// Handle Kubernetes advanced actions first (before validation)
	if action.Target.Layer == "kubernetes" && e.kubernetesExecutor != nil {
		switch action.Type {
		case ActionK8sUpdateImage:
			namespace := action.Target.Namespace
			if namespace == "" {
				namespace = "default"
			}
			deploymentName := action.Target.EntityID
			containerName := action.Parameters["container"]
			newImage := action.Parameters["image"]
			
			if newImage == "" {
				return &ActionResult{
					Success:   false,
					Message:   "Image parameter is required",
					StartTime: startTime,
					EndTime:   time.Now(),
				}, fmt.Errorf("image parameter is required")
			}
			
			return e.kubernetesExecutor.UpdateDeploymentImage(ctx, namespace, deploymentName, containerName, newImage)
			
		case ActionK8sRolloutRestart:
			namespace := action.Target.Namespace
			if namespace == "" {
				namespace = "default"
			}
			deploymentName := action.Target.EntityID
			return e.kubernetesExecutor.RolloutRestart(ctx, namespace, deploymentName)
			
		case ActionK8sRolloutUndo:
			namespace := action.Target.Namespace
			if namespace == "" {
				namespace = "default"
			}
			deploymentName := action.Target.EntityID
			return e.kubernetesExecutor.RolloutUndo(ctx, namespace, deploymentName, 0)
			
		case ActionK8sRolloutStatus:
			namespace := action.Target.Namespace
			if namespace == "" {
				namespace = "default"
			}
			deploymentName := action.Target.EntityID
			return e.kubernetesExecutor.GetRolloutStatus(ctx, namespace, deploymentName)
			
		case ActionK8sGetLogs:
			namespace := action.Target.Namespace
			if namespace == "" {
				namespace = "default"
			}
			podName := action.Target.EntityID
			containerName := action.Parameters["container"]
			tailLines := int64(100) // default
			return e.kubernetesExecutor.GetPodLogs(ctx, namespace, podName, containerName, tailLines)
		}
	}

	// Validate the action first
	if err := e.ValidateAction(action); err != nil {
		return &ActionResult{
			Success:   false,
			Message:   "Action validation failed",
			Error:     err.Error(),
			StartTime: startTime,
			EndTime:   time.Now(),
		}, err
	}

	// Route to the appropriate executor
	var result *ActionResult
	var err error

	switch action.Target.Layer {
	case "host":
		result, err = e.hostExecutor.ExecuteAction(ctx, action)

	case "docker":
		result, err = e.dockerExecutor.ExecuteAction(ctx, action)

	case "kubernetes":
		result, err = e.kubernetesExecutor.ExecuteAction(ctx, action)

	default:
		err = fmt.Errorf("unknown layer: %s", action.Target.Layer)
		result = &ActionResult{
			Success:   false,
			Message:   "Unknown layer",
			Error:     err.Error(),
			StartTime: startTime,
			EndTime:   time.Now(),
		}
	}

	return result, err
}
