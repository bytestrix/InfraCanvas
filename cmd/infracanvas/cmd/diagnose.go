package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"infracanvas/internal/models"
	"infracanvas/pkg/health"
	"infracanvas/pkg/orchestrator"
)

// diagnoseCmd represents the diagnose command
var diagnoseCmd = &cobra.Command{
	Use:   "diagnose",
	Short: "Diagnose infrastructure health",
	Long: `Diagnose performs a full infrastructure discovery and calculates health status 
for all entities. It displays a health summary and provides actionable recommendations 
for any issues found.`,
	RunE: runDiagnose,
}

func init() {
	rootCmd.AddCommand(diagnoseCmd)
}

func runDiagnose(cmd *cobra.Command, args []string) error {
	if !quiet {
		fmt.Fprintln(os.Stderr, "Running infrastructure diagnostics...")
	}

	// Execute full discovery
	orch := orchestrator.NewOrchestrator(true)
	ctx := context.Background()
	snapshot, err := orch.Discover(ctx, scope)
	if err != nil {
		return fmt.Errorf("discovery failed: %w", err)
	}

	// Calculate health status
	healthCalc := health.NewCalculator()

	// Collect entities by health status
	healthyEntities := []models.Entity{}
	degradedEntities := []models.Entity{}
	unhealthyEntities := []models.Entity{}
	unknownEntities := []models.Entity{}

	for _, entity := range snapshot.Entities {
		switch entity.GetHealth() {
		case models.HealthHealthy:
			healthyEntities = append(healthyEntities, entity)
		case models.HealthDegraded:
			degradedEntities = append(degradedEntities, entity)
		case models.HealthUnhealthy:
			unhealthyEntities = append(unhealthyEntities, entity)
		case models.HealthUnknown:
			unknownEntities = append(unknownEntities, entity)
		}
	}

	// Calculate aggregate health
	allEntities := make([]models.Entity, 0, len(snapshot.Entities))
	for _, entity := range snapshot.Entities {
		allEntities = append(allEntities, entity)
	}
	aggregateHealth := healthCalc.CalculateAggregateHealth(allEntities)

	// Display health summary
	fmt.Println("=== Infrastructure Health Summary ===")
	fmt.Printf("\nOverall Status: %s\n", aggregateHealth)
	fmt.Printf("\nEntity Health Breakdown:\n")
	fmt.Printf("  Healthy:   %d\n", len(healthyEntities))
	fmt.Printf("  Degraded:  %d\n", len(degradedEntities))
	fmt.Printf("  Unhealthy: %d\n", len(unhealthyEntities))
	fmt.Printf("  Unknown:   %d\n", len(unknownEntities))

	// Display issues and recommendations
	if len(unhealthyEntities) > 0 {
		fmt.Println("\n=== Unhealthy Entities ===")
		for _, entity := range unhealthyEntities {
			displayEntityHealth(entity, healthCalc)
		}
	}

	if len(degradedEntities) > 0 {
		fmt.Println("\n=== Degraded Entities ===")
		for _, entity := range degradedEntities {
			displayEntityHealth(entity, healthCalc)
		}
	}

	// Display recommendations
	if len(unhealthyEntities) > 0 || len(degradedEntities) > 0 {
		fmt.Println("\n=== Recommendations ===")
		displayRecommendations(unhealthyEntities, degradedEntities)
	}

	if aggregateHealth == models.HealthHealthy {
		fmt.Println("\n✓ All systems healthy")
		return nil
	}

	return nil
}

func displayEntityHealth(entity models.Entity, healthCalc *health.Calculator) {
	fmt.Printf("\n%s: %s\n", entity.GetType(), entity.GetID())
	fmt.Printf("  Status: %s\n", entity.GetHealth())

	reasons := healthCalc.GetHealthReasons(entity)
	if len(reasons) > 0 {
		fmt.Println("  Reasons:")
		for _, reason := range reasons {
			fmt.Printf("    - %s\n", reason)
		}
	}
}

func displayRecommendations(unhealthy, degraded []models.Entity) {
	recommendations := []string{}

	// Analyze unhealthy entities
	for _, entity := range unhealthy {
		switch e := entity.(type) {
		case *models.Container:
			if e.State == "exited" {
				recommendations = append(recommendations, fmt.Sprintf("Restart container: %s", e.Name))
			} else if e.State == "dead" {
				recommendations = append(recommendations, fmt.Sprintf("Remove and recreate container: %s", e.Name))
			}
		case *models.Pod:
			if e.Phase == "Failed" {
				recommendations = append(recommendations, fmt.Sprintf("Check pod logs: kubectl logs %s -n %s", e.Name, e.Namespace))
			}
		case *models.Deployment:
			if e.AvailableReplicas == 0 {
				recommendations = append(recommendations, fmt.Sprintf("Check deployment status: kubectl describe deployment %s -n %s", e.Name, e.Namespace))
			}
		case *models.Node:
			if e.Status == "NotReady" {
				recommendations = append(recommendations, fmt.Sprintf("Investigate node: kubectl describe node %s", e.Name))
			}
		}
	}

	// Analyze degraded entities
	for _, entity := range degraded {
		switch e := entity.(type) {
		case *models.Host:
			recommendations = append(recommendations, fmt.Sprintf("Review host resource usage on %s", e.Hostname))
		case *models.Deployment:
			if e.AvailableReplicas < e.Replicas {
				recommendations = append(recommendations, fmt.Sprintf("Scale deployment or investigate pod failures: %s", e.Name))
			}
		}
	}

	// Display recommendations
	for i, rec := range recommendations {
		fmt.Printf("%d. %s\n", i+1, rec)
	}
}
