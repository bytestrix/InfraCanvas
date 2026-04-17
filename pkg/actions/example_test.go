package actions_test

import (
	"fmt"
	"time"

	"infracanvas/pkg/actions"
)

// Example demonstrates how to use the action executor to restart a systemd service
func Example_restartService() {
	// Create an action executor
	executor, err := actions.NewActionExecutor()
	if err != nil {
		fmt.Printf("Failed to create executor: %v\n", err)
		return
	}

	// Define an action to restart a service
	action := &actions.Action{
		ID:   "action-001",
		Type: actions.ActionRestartService,
		Target: actions.ActionTarget{
			Layer:      "host",
			EntityType: "service",
			EntityID:   "nginx",
		},
		RequestedBy: "admin",
		RequestedAt: time.Now(),
	}

	// Check if confirmation is required
	if executor.RequiresConfirmation(action) {
		fmt.Println("This action requires confirmation")
	}

	// In a real scenario, you would:
	// 1. Validate the action
	// 2. Get user confirmation
	// 3. Execute the action
	// 4. Display the result

	fmt.Println("Action created successfully")
	// Output: This action requires confirmation
	// Action created successfully
}

// Example demonstrates how to use the action executor with Docker
func Example_dockerAction() {
	_, err := actions.NewActionExecutor()
	if err != nil {
		fmt.Printf("Failed to create executor: %v\n", err)
		return
	}

	action := &actions.Action{
		ID:   "action-002",
		Type: actions.ActionRestartContainer,
		Target: actions.ActionTarget{
			Layer:      "docker",
			EntityType: "container",
			EntityID:   "my-container",
		},
		RequestedBy: "admin",
		RequestedAt: time.Now(),
	}

	fmt.Printf("Action type: %s\n", action.Type)
	fmt.Printf("Target layer: %s\n", action.Target.Layer)
	// Output: Action type: restart_container
	// Target layer: docker
}

// Example demonstrates how to use the action executor with Kubernetes
func Example_kubernetesAction() {
	_, err := actions.NewActionExecutor()
	if err != nil {
		fmt.Printf("Failed to create executor: %v\n", err)
		return
	}

	action := &actions.Action{
		ID:   "action-003",
		Type: actions.ActionScaleDeployment,
		Target: actions.ActionTarget{
			Layer:      "kubernetes",
			EntityType: "deployment",
			EntityID:   "my-app",
			Namespace:  "production",
		},
		Parameters: map[string]string{
			"replicas": "3",
		},
		RequestedBy: "admin",
		RequestedAt: time.Now(),
	}

	fmt.Printf("Action type: %s\n", action.Type)
	fmt.Printf("Target: %s/%s\n", action.Target.Namespace, action.Target.EntityID)
	fmt.Printf("Replicas: %s\n", action.Parameters["replicas"])
	// Output: Action type: scale_deployment
	// Target: production/my-app
	// Replicas: 3
}

// Example demonstrates action result handling
func Example_actionResult() {
	startTime := time.Now()
	endTime := startTime.Add(500 * time.Millisecond)

	result := &actions.ActionResult{
		Success:   true,
		Message:   "Service restarted successfully",
		Output:    "systemctl restart nginx completed",
		StartTime: startTime,
		EndTime:   endTime,
	}

	fmt.Printf("Success: %v\n", result.Success)
	fmt.Printf("Message: %s\n", result.Message)
	duration := result.EndTime.Sub(result.StartTime)
	fmt.Printf("Duration: %v\n", duration)
	// Output: Success: true
	// Message: Service restarted successfully
	// Duration: 500ms
}
