package output_test

import (
	"fmt"
	"infracanvas/internal/models"
	"infracanvas/pkg/output"
	"time"
)

// ExampleJSONFormatter demonstrates JSON output formatting
func ExampleJSONFormatter() {
	snapshot := createExampleSnapshot()
	
	formatter := &output.JSONFormatter{PrettyPrint: true}
	result, err := formatter.Format(snapshot)
	if err != nil {
		panic(err)
	}
	
	fmt.Println("JSON output generated:", len(result) > 0)
	// Output: JSON output generated: true
}

// ExampleYAMLFormatter demonstrates YAML output formatting
func ExampleYAMLFormatter() {
	snapshot := createExampleSnapshot()
	
	formatter := &output.YAMLFormatter{}
	result, err := formatter.Format(snapshot)
	if err != nil {
		panic(err)
	}
	
	fmt.Println("YAML output generated:", len(result) > 0)
	// Output: YAML output generated: true
}

// ExampleTableFormatter demonstrates table output formatting
func ExampleTableFormatter() {
	snapshot := createExampleSnapshot()
	
	formatter := &output.TableFormatter{}
	result, err := formatter.Format(snapshot)
	if err != nil {
		panic(err)
	}
	
	fmt.Println("Table output generated")
	fmt.Println(string(result)[:50]) // Print first 50 chars
}

// ExampleNewFormatter demonstrates the formatter factory
func ExampleNewFormatter() {
	snapshot := createExampleSnapshot()
	
	// Create JSON formatter
	jsonFormatter := output.NewFormatter(output.FormatJSON)
	jsonResult, _ := jsonFormatter.Format(snapshot)
	fmt.Println("JSON formatter created, output length:", len(jsonResult))
	
	// Create YAML formatter
	yamlFormatter := output.NewFormatter(output.FormatYAML)
	yamlResult, _ := yamlFormatter.Format(snapshot)
	fmt.Println("YAML formatter created, output length:", len(yamlResult))
	
	// Create Table formatter
	tableFormatter := output.NewFormatter(output.FormatTable)
	tableResult, _ := tableFormatter.Format(snapshot)
	fmt.Println("Table formatter created, output length:", len(tableResult))
}

// ExampleProgressIndicator demonstrates progress indicator usage
func ExampleProgressIndicator() {
	indicator := output.NewProgressIndicator("Discovering infrastructure...")
	
	indicator.Start()
	time.Sleep(100 * time.Millisecond)
	
	indicator.UpdateMessage("Collecting host information...")
	time.Sleep(100 * time.Millisecond)
	
	indicator.UpdateMessage("Collecting Docker containers...")
	time.Sleep(100 * time.Millisecond)
	
	indicator.Stop()
	
	fmt.Println("Discovery complete")
	// Output: Discovery complete
}

// ExampleProgressTracker demonstrates multi-stage progress tracking
func ExampleProgressTracker() {
	stages := []string{
		"Discovering host information",
		"Discovering Docker containers",
		"Discovering Kubernetes resources",
	}
	
	tracker := output.NewProgressTracker(stages)
	
	tracker.Start()
	time.Sleep(100 * time.Millisecond)
	
	tracker.NextStage()
	time.Sleep(100 * time.Millisecond)
	
	tracker.NextStage()
	time.Sleep(100 * time.Millisecond)
	
	tracker.Complete()
	
	fmt.Println("All stages complete")
}

// ExampleSimpleProgress demonstrates simple progress messages
func ExampleSimpleProgress() {
	progress := output.NewSimpleProgress()
	
	progress.Show("Starting discovery...")
	progress.Success("Host information collected")
	progress.Warning("Docker socket not accessible")
	progress.Error("Kubernetes API unreachable")
	
	fmt.Println("Progress messages displayed")
	// Output: Progress messages displayed
}

// createExampleSnapshot creates a sample snapshot for examples
func createExampleSnapshot() *models.InfraSnapshot {
	now := time.Now()
	
	host := &models.Host{
		BaseEntity: models.BaseEntity{
			ID:        "host-1",
			Type:      models.EntityTypeHost,
			Health:    models.HealthHealthy,
			Timestamp: now,
		},
		Hostname:           "example-host",
		OS:                 "Linux",
		CPUUsagePercent:    25.5,
		MemoryUsagePercent: 45.2,
	}
	
	entities := map[string]models.Entity{
		"host-1": host,
	}
	
	return &models.InfraSnapshot{
		Timestamp: now,
		HostID:    "host-1",
		Entities:  entities,
		Relations: []models.Relation{},
		Metadata: models.SnapshotMetadata{
			CollectionDuration: 1 * time.Second,
			Scope:              []string{"host"},
		},
	}
}
