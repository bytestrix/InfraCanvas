package permissions

import (
	"testing"
)

func TestNewChecker(t *testing.T) {
	checker := NewChecker()
	if checker == nil {
		t.Fatal("NewChecker returned nil")
	}
	if checker.checks == nil {
		t.Error("checks slice should be initialized")
	}
}

func TestGetSummary(t *testing.T) {
	checker := NewChecker()
	
	checker.checks = []PermissionCheck{
		{Available: true, Level: PermissionFull},
		{Available: true, Level: PermissionPartial},
		{Available: false, Level: PermissionNone},
	}
	
	available, unavailable, partial := checker.GetSummary()
	
	if available != 1 {
		t.Errorf("Expected 1 available, got %d", available)
	}
	if partial != 1 {
		t.Errorf("Expected 1 partial, got %d", partial)
	}
	if unavailable != 1 {
		t.Errorf("Expected 1 unavailable, got %d", unavailable)
	}
}

func TestHasCriticalIssues(t *testing.T) {
	tests := []struct {
		name     string
		checks   []PermissionCheck
		expected bool
	}{
		{
			name: "no critical issues",
			checks: []PermissionCheck{
				{Required: true, Available: true},
				{Required: false, Available: false},
			},
			expected: false,
		},
		{
			name: "has critical issue",
			checks: []PermissionCheck{
				{Required: true, Available: false},
			},
			expected: true,
		},
		{
			name: "all optional unavailable",
			checks: []PermissionCheck{
				{Required: false, Available: false},
				{Required: false, Available: false},
			},
			expected: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewChecker()
			checker.checks = tt.checks
			
			result := checker.HasCriticalIssues()
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestCanReadFile(t *testing.T) {
	checker := NewChecker()
	
	// Test with /etc/os-release which should exist on most Linux systems
	result := checker.canReadFile("/etc/os-release")
	// We can't assert true/false as it depends on the system
	// Just ensure it doesn't panic
	_ = result
}

func TestIsCommandAvailable(t *testing.T) {
	checker := NewChecker()
	
	// Test with a command that should exist
	result := checker.isCommandAvailable("ls")
	if !result {
		t.Error("Expected 'ls' command to be available")
	}
	
	// Test with a command that shouldn't exist
	result = checker.isCommandAvailable("nonexistent-command-xyz")
	if result {
		t.Error("Expected nonexistent command to be unavailable")
	}
}

func TestGetLevel(t *testing.T) {
	checker := NewChecker()
	
	if level := checker.getLevel(true); level != PermissionFull {
		t.Errorf("Expected PermissionFull, got %s", level)
	}
	
	if level := checker.getLevel(false); level != PermissionNone {
		t.Errorf("Expected PermissionNone, got %s", level)
	}
}

func TestGetPartialLevel(t *testing.T) {
	checker := NewChecker()
	
	if level := checker.getPartialLevel(true); level != PermissionFull {
		t.Errorf("Expected PermissionFull, got %s", level)
	}
	
	if level := checker.getPartialLevel(false); level != PermissionPartial {
		t.Errorf("Expected PermissionPartial, got %s", level)
	}
}
