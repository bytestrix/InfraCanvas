package output

// This file contains integration examples showing how to use the output package
// in the context of the rix CLI application.

import (
	"fmt"
	"infracanvas/internal/models"
	"time"
)

// Example: Using formatters in a CLI discover command
func ExampleDiscoverCommand(snapshot *models.InfraSnapshot, format string) error {
	// Create progress tracker for discovery stages
	tracker := NewProgressTracker([]string{
		"Discovering host information",
		"Discovering Docker containers",
		"Discovering Kubernetes resources",
		"Building relationships",
		"Calculating health status",
	})
	
	tracker.Start()
	
	// Simulate discovery stages
	// (In real implementation, these would be actual discovery calls)
	time.Sleep(100 * time.Millisecond)
	tracker.NextStage()
	
	time.Sleep(100 * time.Millisecond)
	tracker.NextStage()
	
	time.Sleep(100 * time.Millisecond)
	tracker.NextStage()
	
	time.Sleep(100 * time.Millisecond)
	tracker.NextStage()
	
	tracker.Complete()
	
	// Format output based on user preference
	var formatter Formatter
	switch format {
	case "json":
		formatter = &JSONFormatter{PrettyPrint: true}
	case "yaml":
		formatter = &YAMLFormatter{}
	case "table":
		formatter = &TableFormatter{}
	default:
		formatter = &TableFormatter{} // Default to table
	}
	
	output, err := formatter.Format(snapshot)
	if err != nil {
		return fmt.Errorf("failed to format output: %w", err)
	}
	
	fmt.Println(string(output))
	return nil
}

// Example: Using progress indicators for long-running operations
func ExampleLongRunningOperation() error {
	indicator := NewProgressIndicator("Collecting container statistics...")
	indicator.Start()
	defer indicator.Stop()
	
	// Simulate long-running operation
	for i := 0; i < 10; i++ {
		time.Sleep(100 * time.Millisecond)
		indicator.UpdateMessage(fmt.Sprintf("Processing container %d/10...", i+1))
	}
	
	return nil
}

// Example: Using simple progress for quick feedback
func ExampleQuickFeedback() {
	progress := NewSimpleProgress()
	
	progress.Show("Checking Docker availability...")
	// Check Docker
	dockerAvailable := true
	if dockerAvailable {
		progress.Success("Docker is available")
	} else {
		progress.Warning("Docker is not available, skipping Docker discovery")
	}
	
	progress.Show("Checking Kubernetes availability...")
	// Check Kubernetes
	k8sAvailable := false
	if k8sAvailable {
		progress.Success("Kubernetes is available")
	} else {
		progress.Warning("Kubernetes is not available, skipping Kubernetes discovery")
	}
	
	progress.Show("Checking permissions...")
	// Check permissions
	hasPermissions := true
	if hasPermissions {
		progress.Success("All required permissions available")
	} else {
		progress.Error("Insufficient permissions for some operations")
	}
}

// Example: Formatting specific entity types
func ExampleFormatEntityType(entities []models.Entity, entityType models.EntityType) {
	// Filter entities by type
	filtered := make(map[string]models.Entity)
	for _, entity := range entities {
		if entity.GetType() == entityType {
			filtered[entity.GetID()] = entity
		}
	}
	
	// Create a snapshot with only the filtered entities
	snapshot := &models.InfraSnapshot{
		Timestamp: time.Now(),
		Entities:  filtered,
		Relations: []models.Relation{},
		Metadata: models.SnapshotMetadata{
			Scope: []string{string(entityType)},
		},
	}
	
	// Format as table
	formatter := &TableFormatter{}
	output, err := formatter.Format(snapshot)
	if err != nil {
		fmt.Printf("Error formatting output: %v\n", err)
		return
	}
	
	fmt.Println(string(output))
}

// Example: Handling errors during discovery
func ExampleErrorHandling(snapshot *models.InfraSnapshot) {
	progress := NewSimpleProgress()
	
	// Check for errors in metadata
	if len(snapshot.Metadata.Errors) > 0 {
		progress.Warning(fmt.Sprintf("Discovery completed with %d errors", len(snapshot.Metadata.Errors)))
		
		for _, err := range snapshot.Metadata.Errors {
			progress.Error(fmt.Sprintf("%s: %s", err.Layer, err.Message))
		}
	} else {
		progress.Success("Discovery completed successfully")
	}
	
	// Check for permission issues
	if len(snapshot.Metadata.PermissionIssues) > 0 {
		progress.Warning("Some operations were skipped due to insufficient permissions:")
		for _, issue := range snapshot.Metadata.PermissionIssues {
			fmt.Printf("  - %s\n", issue)
		}
	}
}

// Example: Streaming output for large datasets
func ExampleStreamingOutput(snapshot *models.InfraSnapshot) error {
	// For very large snapshots, you might want to stream JSON output
	// instead of loading everything into memory
	
	formatter := &JSONFormatter{PrettyPrint: false}
	output, err := formatter.Format(snapshot)
	if err != nil {
		return err
	}
	
	// In a real implementation, you might write directly to a file or stdout
	// in chunks to avoid memory issues
	fmt.Println(string(output))
	
	return nil
}

// Example: Custom table formatting options
func ExampleCustomTableFormatting(snapshot *models.InfraSnapshot) error {
	formatter := &TableFormatter{
		MaxColumnWidth: 80, // Increase max column width for detailed output
	}
	
	output, err := formatter.Format(snapshot)
	if err != nil {
		return err
	}
	
	fmt.Println(string(output))
	return nil
}
