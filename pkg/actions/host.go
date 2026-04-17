package actions

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// HostExecutor handles actions on the host system
type HostExecutor struct{}

// NewHostExecutor creates a new host executor
func NewHostExecutor() *HostExecutor {
	return &HostExecutor{}
}

// ValidateAction validates a host action
func (h *HostExecutor) ValidateAction(action *Action) error {
	switch action.Type {
	case ActionRestartService:
		if action.Target.EntityID == "" {
			return fmt.Errorf("service name is required")
		}
		// Check if service exists
		return h.validateServiceExists(action.Target.EntityID)

	default:
		return fmt.Errorf("unsupported host action type: %s", action.Type)
	}
}

// validateServiceExists checks if a systemd service exists
func (h *HostExecutor) validateServiceExists(serviceName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use systemctl show to check if service exists
	cmd := exec.CommandContext(ctx, "systemctl", "show", serviceName, "--property=LoadState")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to check service existence: %w", err)
	}

	// Parse output to check LoadState
	outputStr := strings.TrimSpace(string(output))
	if strings.Contains(outputStr, "LoadState=not-found") {
		return fmt.Errorf("service %s not found", serviceName)
	}

	return nil
}

// ExecuteAction executes a host action
func (h *HostExecutor) ExecuteAction(ctx context.Context, action *Action) (*ActionResult, error) {
	startTime := time.Now()

	switch action.Type {
	case ActionRestartService:
		return h.restartService(ctx, action.Target.EntityID, startTime)

	default:
		return &ActionResult{
			Success:   false,
			Message:   "Unsupported action type",
			Error:     fmt.Sprintf("unsupported host action type: %s", action.Type),
			StartTime: startTime,
			EndTime:   time.Now(),
		}, fmt.Errorf("unsupported host action type: %s", action.Type)
	}
}

// restartService restarts a systemd service
func (h *HostExecutor) restartService(ctx context.Context, serviceName string, startTime time.Time) (*ActionResult, error) {
	// Execute systemctl restart
	cmd := exec.CommandContext(ctx, "systemctl", "restart", serviceName)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return &ActionResult{
			Success:   false,
			Message:   fmt.Sprintf("Failed to restart service %s", serviceName),
			Output:    string(output),
			Error:     err.Error(),
			StartTime: startTime,
			EndTime:   time.Now(),
		}, err
	}

	// Verify service is running
	statusCmd := exec.CommandContext(ctx, "systemctl", "is-active", serviceName)
	statusOutput, statusErr := statusCmd.CombinedOutput()
	status := strings.TrimSpace(string(statusOutput))

	if statusErr != nil || status != "active" {
		return &ActionResult{
			Success:   false,
			Message:   fmt.Sprintf("Service %s restarted but is not active (status: %s)", serviceName, status),
			Output:    string(output),
			Error:     fmt.Sprintf("service status: %s", status),
			StartTime: startTime,
			EndTime:   time.Now(),
		}, fmt.Errorf("service not active after restart: %s", status)
	}

	return &ActionResult{
		Success:   true,
		Message:   fmt.Sprintf("Successfully restarted service %s", serviceName),
		Output:    string(output),
		StartTime: startTime,
		EndTime:   time.Now(),
	}, nil
}
