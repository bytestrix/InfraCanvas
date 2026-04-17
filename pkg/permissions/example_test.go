package permissions_test

import (
	"fmt"
	"infracanvas/pkg/permissions"
)

func ExampleChecker_ValidatePermissions() {
	// Create a new permission checker
	checker := permissions.NewChecker()

	// Validate permissions for host, docker, and kubernetes layers
	checks := checker.ValidatePermissions([]string{"host", "docker", "kubernetes"})

	// Print results
	for _, check := range checks {
		status := "✓"
		if !check.Available {
			status = "✗"
		}
		fmt.Printf("%s [%s] %s: %s\n", status, check.Layer, check.Operation, check.Message)
		
		if !check.Available && check.Suggestion != "" {
			fmt.Printf("  → %s\n", check.Suggestion)
		}
	}

	// Check for critical issues
	if checker.HasCriticalIssues() {
		fmt.Println("\n⚠ Critical permissions are missing!")
	}

	// Get summary
	available, unavailable, partial := checker.GetSummary()
	fmt.Printf("\nSummary: %d available, %d unavailable, %d partial\n", available, unavailable, partial)
}

func ExampleChecker_HasCriticalIssues() {
	checker := permissions.NewChecker()
	checker.ValidatePermissions([]string{"host"})

	if checker.HasCriticalIssues() {
		fmt.Println("Cannot proceed: required permissions are missing")
		// Handle critical permission issues
	} else {
		fmt.Println("All required permissions are available")
		// Proceed with discovery
	}
}

func ExampleChecker_GetSummary() {
	checker := permissions.NewChecker()
	checker.ValidatePermissions([]string{"host", "docker", "kubernetes"})

	available, unavailable, partial := checker.GetSummary()
	
	fmt.Printf("Available operations: %d\n", available)
	fmt.Printf("Unavailable operations: %d\n", unavailable)
	fmt.Printf("Partially available operations: %d\n", partial)
	
	// Calculate percentage
	total := available + unavailable + partial
	if total > 0 {
		percentage := float64(available+partial) / float64(total) * 100
		fmt.Printf("Discovery coverage: %.1f%%\n", percentage)
	}
}
