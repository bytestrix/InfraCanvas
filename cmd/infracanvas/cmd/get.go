package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"infracanvas/internal/models"
	"infracanvas/pkg/orchestrator"
)

var (
	getNamespace string
	getLabels    []string
	getStatus    string
)

// getCmd represents the get command
var getCmd = &cobra.Command{
	Use:   "get",
	Short: "Get specific resource types",
	Long:  `Get retrieves specific resource types from the infrastructure.`,
}

// getPodsCmd represents the get pods command
var getPodsCmd = &cobra.Command{
	Use:   "pods",
	Short: "Get Kubernetes pods",
	RunE:  runGetPods,
}

// getContainersCmd represents the get containers command
var getContainersCmd = &cobra.Command{
	Use:   "containers",
	Short: "Get Docker containers",
	RunE:  runGetContainers,
}

// getServicesCmd represents the get services command
var getServicesCmd = &cobra.Command{
	Use:   "services",
	Short: "Get Kubernetes services",
	RunE:  runGetServices,
}

// getNodesCmd represents the get nodes command
var getNodesCmd = &cobra.Command{
	Use:   "nodes",
	Short: "Get Kubernetes nodes",
	RunE:  runGetNodes,
}

// getDeploymentsCmd represents the get deployments command
var getDeploymentsCmd = &cobra.Command{
	Use:   "deployments",
	Short: "Get Kubernetes deployments",
	RunE:  runGetDeployments,
}

// getImagesCmd represents the get images command
var getImagesCmd = &cobra.Command{
	Use:   "images",
	Short: "Get container images",
	RunE:  runGetImages,
}

func init() {
	rootCmd.AddCommand(getCmd)

	// Add subcommands
	getCmd.AddCommand(getPodsCmd)
	getCmd.AddCommand(getContainersCmd)
	getCmd.AddCommand(getServicesCmd)
	getCmd.AddCommand(getNodesCmd)
	getCmd.AddCommand(getDeploymentsCmd)
	getCmd.AddCommand(getImagesCmd)

	// Add flags to get command and all subcommands
	getCmd.PersistentFlags().StringVarP(&getNamespace, "namespace", "n", "", "Filter by namespace")
	getCmd.PersistentFlags().StringSliceVarP(&getLabels, "labels", "l", []string{}, "Filter by labels (key=value)")
	getCmd.PersistentFlags().StringVar(&getStatus, "status", "", "Filter by status")
}

func runGetPods(cmd *cobra.Command, args []string) error {
	snapshot, err := executeTargetedDiscovery([]string{"kubernetes"})
	if err != nil {
		return err
	}

	// Filter pods
	pods := filterEntitiesByType(snapshot, models.EntityTypePod)
	pods = applyFilters(pods)

	return formatOutput(pods)
}

func runGetContainers(cmd *cobra.Command, args []string) error {
	snapshot, err := executeTargetedDiscovery([]string{"docker"})
	if err != nil {
		return err
	}

	// Filter containers
	containers := filterEntitiesByType(snapshot, models.EntityTypeContainer)
	containers = applyFilters(containers)

	return formatOutput(containers)
}

func runGetServices(cmd *cobra.Command, args []string) error {
	snapshot, err := executeTargetedDiscovery([]string{"kubernetes"})
	if err != nil {
		return err
	}

	// Filter services
	services := filterEntitiesByType(snapshot, models.EntityTypeK8sService)
	services = applyFilters(services)

	return formatOutput(services)
}

func runGetNodes(cmd *cobra.Command, args []string) error {
	snapshot, err := executeTargetedDiscovery([]string{"kubernetes"})
	if err != nil {
		return err
	}

	// Filter nodes
	nodes := filterEntitiesByType(snapshot, models.EntityTypeNode)
	nodes = applyFilters(nodes)

	return formatOutput(nodes)
}

func runGetDeployments(cmd *cobra.Command, args []string) error {
	snapshot, err := executeTargetedDiscovery([]string{"kubernetes"})
	if err != nil {
		return err
	}

	// Filter deployments
	deployments := filterEntitiesByType(snapshot, models.EntityTypeDeployment)
	deployments = applyFilters(deployments)

	return formatOutput(deployments)
}

func runGetImages(cmd *cobra.Command, args []string) error {
	snapshot, err := executeTargetedDiscovery([]string{"docker", "kubernetes"})
	if err != nil {
		return err
	}

	// Filter images
	images := filterEntitiesByType(snapshot, models.EntityTypeImage)
	images = applyFilters(images)

	return formatOutput(images)
}

func executeTargetedDiscovery(targetScope []string) (*models.InfraSnapshot, error) {
	if !quiet {
		fmt.Fprintln(os.Stderr, "Executing discovery...")
	}

	orch := orchestrator.NewOrchestrator(true)
	ctx := context.Background()
	snapshot, err := orch.Discover(ctx, targetScope)
	if err != nil {
		return nil, fmt.Errorf("discovery failed: %w", err)
	}

	return snapshot, nil
}

func filterEntitiesByType(snapshot *models.InfraSnapshot, entityType models.EntityType) []models.Entity {
	var filtered []models.Entity
	for _, entity := range snapshot.Entities {
		if entity.GetType() == entityType {
			filtered = append(filtered, entity)
		}
	}
	return filtered
}

func applyFilters(entities []models.Entity) []models.Entity {
	var filtered []models.Entity

	for _, entity := range entities {
		// Filter by namespace
		if getNamespace != "" {
			labels := entity.GetLabels()
			if ns, ok := labels["namespace"]; !ok || ns != getNamespace {
				continue
			}
		}

		// Filter by status
		if getStatus != "" {
			// Status filtering depends on entity type
			switch e := entity.(type) {
			case *models.Container:
				if e.State != getStatus {
					continue
				}
			case *models.Pod:
				if e.Phase != getStatus {
					continue
				}
			}
		}

		// Filter by labels
		if len(getLabels) > 0 {
			entityLabels := entity.GetLabels()
			match := true
			for _, labelFilter := range getLabels {
				// Parse label filter (key=value)
				// Simple implementation - can be enhanced
				match = false
				for k, v := range entityLabels {
					if fmt.Sprintf("%s=%s", k, v) == labelFilter {
						match = true
						break
					}
				}
				if !match {
					break
				}
			}
			if !match {
				continue
			}
		}

		filtered = append(filtered, entity)
	}

	return filtered
}
