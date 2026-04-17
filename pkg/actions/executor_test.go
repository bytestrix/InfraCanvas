package actions

import (
	"testing"
	"time"
)

func TestActionValidation(t *testing.T) {
	executor, err := NewActionExecutor()
	if err != nil {
		t.Fatalf("Failed to create action executor: %v", err)
	}

	tests := []struct {
		name        string
		action      *Action
		expectError bool
	}{
		{
			name:        "nil action",
			action:      nil,
			expectError: true,
		},
		{
			name: "missing action type",
			action: &Action{
				Target: ActionTarget{
					Layer:    "host",
					EntityID: "test-service",
				},
			},
			expectError: true,
		},
		{
			name: "missing layer",
			action: &Action{
				Type: ActionRestartService,
				Target: ActionTarget{
					EntityID: "test-service",
				},
			},
			expectError: true,
		},
		{
			name: "missing entity ID",
			action: &Action{
				Type: ActionRestartService,
				Target: ActionTarget{
					Layer: "host",
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := executor.ValidateAction(tt.action)
			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestRequiresConfirmation(t *testing.T) {
	executor, err := NewActionExecutor()
	if err != nil {
		t.Fatalf("Failed to create action executor: %v", err)
	}

	action := &Action{
		Type: ActionRestartService,
		Target: ActionTarget{
			Layer:    "host",
			EntityID: "test-service",
		},
		RequestedAt: time.Now(),
	}

	if !executor.RequiresConfirmation(action) {
		t.Error("Expected action to require confirmation")
	}
}

func TestActionResult(t *testing.T) {
	startTime := time.Now()
	time.Sleep(10 * time.Millisecond)
	endTime := time.Now()

	result := &ActionResult{
		Success:   true,
		Message:   "Test action completed",
		StartTime: startTime,
		EndTime:   endTime,
	}

	if !result.Success {
		t.Error("Expected success to be true")
	}

	if result.Message != "Test action completed" {
		t.Errorf("Expected message 'Test action completed', got '%s'", result.Message)
	}

	duration := result.EndTime.Sub(result.StartTime)
	if duration < 10*time.Millisecond {
		t.Errorf("Expected duration >= 10ms, got %v", duration)
	}
}
