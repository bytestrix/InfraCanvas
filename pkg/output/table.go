package output

import (
	"fmt"
	"infracanvas/internal/models"
	"sort"
	"strings"
	"time"
)

// TableFormatter formats InfraSnapshot as human-readable tables
type TableFormatter struct {
	MaxColumnWidth int
}

// Format renders the InfraSnapshot as tables
func (f *TableFormatter) Format(snapshot *models.InfraSnapshot) ([]byte, error) {
	if f.MaxColumnWidth == 0 {
		f.MaxColumnWidth = 50
	}

	var output strings.Builder

	// Header
	output.WriteString(fmt.Sprintf("Infrastructure Snapshot - %s\n", snapshot.Timestamp.Format(time.RFC3339)))
	output.WriteString(fmt.Sprintf("Host ID: %s\n", snapshot.HostID))
	output.WriteString(fmt.Sprintf("Collection Duration: %s\n", snapshot.Metadata.CollectionDuration))
	output.WriteString(fmt.Sprintf("Scope: %s\n\n", strings.Join(snapshot.Metadata.Scope, ", ")))

	// Group entities by type
	entityGroups := f.groupEntitiesByType(snapshot.Entities)

	// Render each entity type
	for _, entityType := range f.sortedEntityTypes(entityGroups) {
		entities := entityGroups[entityType]
		if len(entities) == 0 {
			continue
		}

		output.WriteString(f.renderEntityTable(entityType, entities))
		output.WriteString("\n")
	}

	// Render relationships summary
	if len(snapshot.Relations) > 0 {
		output.WriteString(f.renderRelationsSummary(snapshot.Relations))
		output.WriteString("\n")
	}

	// Render errors if any
	if len(snapshot.Metadata.Errors) > 0 {
		output.WriteString(f.renderErrors(snapshot.Metadata.Errors))
		output.WriteString("\n")
	}

	return []byte(output.String()), nil
}

// groupEntitiesByType groups entities by their type
func (f *TableFormatter) groupEntitiesByType(entities map[string]models.Entity) map[models.EntityType][]models.Entity {
	groups := make(map[models.EntityType][]models.Entity)
	for _, entity := range entities {
		entityType := entity.GetType()
		groups[entityType] = append(groups[entityType], entity)
	}
	return groups
}

// sortedEntityTypes returns entity types in a logical order
func (f *TableFormatter) sortedEntityTypes(groups map[models.EntityType][]models.Entity) []models.EntityType {
	order := []models.EntityType{
		models.EntityTypeHost,
		models.EntityTypeProcess,
		models.EntityTypeService,
		models.EntityTypeContainerRuntime,
		models.EntityTypeContainer,
		models.EntityTypeImage,
		models.EntityTypeVolume,
		models.EntityTypeNetwork,
		models.EntityTypeCluster,
		models.EntityTypeNode,
		models.EntityTypeNamespace,
		models.EntityTypeDeployment,
		models.EntityTypeStatefulSet,
		models.EntityTypeDaemonSet,
		models.EntityTypeJob,
		models.EntityTypeCronJob,
		models.EntityTypePod,
		models.EntityTypeK8sService,
		models.EntityTypeIngress,
		models.EntityTypeConfigMap,
		models.EntityTypeSecret,
		models.EntityTypePVC,
		models.EntityTypePV,
		models.EntityTypeStorageClass,
		models.EntityTypeEvent,
	}

	var result []models.EntityType
	for _, entityType := range order {
		if _, exists := groups[entityType]; exists {
			result = append(result, entityType)
		}
	}
	return result
}

// renderEntityTable renders a table for a specific entity type
func (f *TableFormatter) renderEntityTable(entityType models.EntityType, entities []models.Entity) string {
	var output strings.Builder

	output.WriteString(fmt.Sprintf("=== %s (%d) ===\n", strings.ToUpper(string(entityType)), len(entities)))

	switch entityType {
	case models.EntityTypeHost:
		output.WriteString(f.renderHostTable(entities))
	case models.EntityTypeProcess:
		output.WriteString(f.renderProcessTable(entities))
	case models.EntityTypeService:
		output.WriteString(f.renderServiceTable(entities))
	case models.EntityTypeContainer:
		output.WriteString(f.renderContainerTable(entities))
	case models.EntityTypeImage:
		output.WriteString(f.renderImageTable(entities))
	case models.EntityTypePod:
		output.WriteString(f.renderPodTable(entities))
	case models.EntityTypeDeployment:
		output.WriteString(f.renderDeploymentTable(entities))
	case models.EntityTypeNode:
		output.WriteString(f.renderNodeTable(entities))
	case models.EntityTypeK8sService:
		output.WriteString(f.renderK8sServiceTable(entities))
	default:
		output.WriteString(f.renderGenericTable(entities))
	}

	return output.String()
}

// renderHostTable renders host entities
func (f *TableFormatter) renderHostTable(entities []models.Entity) string {
	headers := []string{"HOSTNAME", "OS", "CPU", "MEMORY", "HEALTH"}
	rows := [][]string{}

	for _, entity := range entities {
		host, ok := entity.(*models.Host)
		if !ok {
			continue
		}
		rows = append(rows, []string{
			f.truncate(host.Hostname, 20),
			f.truncate(fmt.Sprintf("%s %s", host.OS, host.OSVersion), 20),
			fmt.Sprintf("%.1f%%", host.CPUUsagePercent),
			fmt.Sprintf("%.1f%%", host.MemoryUsagePercent),
			f.colorizeHealth(host.Health),
		})
	}

	return f.renderTable(headers, rows)
}

// renderProcessTable renders process entities
func (f *TableFormatter) renderProcessTable(entities []models.Entity) string {
	headers := []string{"PID", "NAME", "USER", "CPU%", "MEM%", "TYPE"}
	rows := [][]string{}

	for _, entity := range entities {
		proc, ok := entity.(*models.Process)
		if !ok {
			continue
		}
		rows = append(rows, []string{
			fmt.Sprintf("%d", proc.PID),
			f.truncate(proc.Name, 25),
			f.truncate(proc.User, 15),
			fmt.Sprintf("%.1f", proc.CPUPercent),
			fmt.Sprintf("%.1f", proc.MemoryPercent),
			f.truncate(proc.ProcessType, 15),
		})
	}

	return f.renderTable(headers, rows)
}

// renderServiceTable renders service entities
func (f *TableFormatter) renderServiceTable(entities []models.Entity) string {
	headers := []string{"NAME", "STATUS", "ENABLED", "CRITICAL", "HEALTH"}
	rows := [][]string{}

	for _, entity := range entities {
		svc, ok := entity.(*models.Service)
		if !ok {
			continue
		}
		rows = append(rows, []string{
			f.truncate(svc.Name, 30),
			f.colorizeStatus(svc.Status),
			f.boolToString(svc.Enabled),
			f.boolToString(svc.IsCritical),
			f.colorizeHealth(svc.Health),
		})
	}

	return f.renderTable(headers, rows)
}

// renderContainerTable renders container entities
func (f *TableFormatter) renderContainerTable(entities []models.Entity) string {
	headers := []string{"NAME", "IMAGE", "STATE", "CPU%", "MEMORY", "HEALTH"}
	rows := [][]string{}

	for _, entity := range entities {
		container, ok := entity.(*models.Container)
		if !ok {
			continue
		}
		rows = append(rows, []string{
			f.truncate(container.Name, 25),
			f.truncate(container.Image, 30),
			f.colorizeContainerState(container.State),
			fmt.Sprintf("%.1f", container.CPUPercent),
			f.formatBytes(container.MemoryUsage),
			f.colorizeHealth(container.Health),
		})
	}

	return f.renderTable(headers, rows)
}

// renderImageTable renders image entities
func (f *TableFormatter) renderImageTable(entities []models.Entity) string {
	headers := []string{"REPOSITORY", "TAG", "SIZE", "CREATED"}
	rows := [][]string{}

	for _, entity := range entities {
		image, ok := entity.(*models.Image)
		if !ok {
			continue
		}
		rows = append(rows, []string{
			f.truncate(image.Repository, 35),
			f.truncate(image.Tag, 15),
			f.formatBytes(image.Size),
			f.formatTime(image.Created),
		})
	}

	return f.renderTable(headers, rows)
}

// renderPodTable renders pod entities
func (f *TableFormatter) renderPodTable(entities []models.Entity) string {
	headers := []string{"NAMESPACE", "NAME", "PHASE", "NODE", "RESTARTS", "HEALTH"}
	rows := [][]string{}

	for _, entity := range entities {
		pod, ok := entity.(*models.Pod)
		if !ok {
			continue
		}
		restarts := f.countPodRestarts(pod)
		rows = append(rows, []string{
			f.truncate(pod.Namespace, 15),
			f.truncate(pod.Name, 30),
			f.colorizePodPhase(pod.Phase),
			f.truncate(pod.NodeName, 20),
			fmt.Sprintf("%d", restarts),
			f.colorizeHealth(pod.Health),
		})
	}

	return f.renderTable(headers, rows)
}

// renderDeploymentTable renders deployment entities
func (f *TableFormatter) renderDeploymentTable(entities []models.Entity) string {
	headers := []string{"NAMESPACE", "NAME", "READY", "UP-TO-DATE", "AVAILABLE", "HEALTH"}
	rows := [][]string{}

	for _, entity := range entities {
		deploy, ok := entity.(*models.Deployment)
		if !ok {
			continue
		}
		rows = append(rows, []string{
			f.truncate(deploy.Namespace, 15),
			f.truncate(deploy.Name, 30),
			fmt.Sprintf("%d/%d", deploy.ReadyReplicas, deploy.Replicas),
			fmt.Sprintf("%d", deploy.UpdatedReplicas),
			fmt.Sprintf("%d", deploy.AvailableReplicas),
			f.colorizeHealth(deploy.Health),
		})
	}

	return f.renderTable(headers, rows)
}

// renderNodeTable renders node entities
func (f *TableFormatter) renderNodeTable(entities []models.Entity) string {
	headers := []string{"NAME", "STATUS", "ROLES", "VERSION", "HEALTH"}
	rows := [][]string{}

	for _, entity := range entities {
		node, ok := entity.(*models.Node)
		if !ok {
			continue
		}
		rows = append(rows, []string{
			f.truncate(node.Name, 25),
			f.colorizeNodeStatus(node.Status),
			f.truncate(strings.Join(node.Roles, ","), 20),
			f.truncate(node.KubernetesVersion, 15),
			f.colorizeHealth(node.Health),
		})
	}

	return f.renderTable(headers, rows)
}

// renderK8sServiceTable renders Kubernetes service entities
func (f *TableFormatter) renderK8sServiceTable(entities []models.Entity) string {
	headers := []string{"NAMESPACE", "NAME", "TYPE", "CLUSTER-IP", "ENDPOINTS"}
	rows := [][]string{}

	for _, entity := range entities {
		svc, ok := entity.(*models.K8sService)
		if !ok {
			continue
		}
		endpoints := "No"
		if svc.HasEndpoints {
			endpoints = "Yes"
		}
		rows = append(rows, []string{
			f.truncate(svc.Namespace, 15),
			f.truncate(svc.Name, 30),
			f.truncate(svc.ServiceType, 15),
			f.truncate(svc.ClusterIP, 15),
			endpoints,
		})
	}

	return f.renderTable(headers, rows)
}

// renderGenericTable renders a generic table for unknown entity types
func (f *TableFormatter) renderGenericTable(entities []models.Entity) string {
	headers := []string{"ID", "TYPE", "HEALTH"}
	rows := [][]string{}

	for _, entity := range entities {
		rows = append(rows, []string{
			f.truncate(entity.GetID(), 40),
			string(entity.GetType()),
			f.colorizeHealth(entity.GetHealth()),
		})
	}

	return f.renderTable(headers, rows)
}

// renderTable renders a table with headers and rows
func (f *TableFormatter) renderTable(headers []string, rows [][]string) string {
	if len(rows) == 0 {
		return "No data\n"
	}

	// Calculate column widths
	colWidths := make([]int, len(headers))
	for i, header := range headers {
		colWidths[i] = len(header)
	}
	for _, row := range rows {
		for i, cell := range row {
			// Strip ANSI codes for width calculation
			cleanCell := f.stripANSI(cell)
			if len(cleanCell) > colWidths[i] {
				colWidths[i] = len(cleanCell)
			}
		}
	}

	var output strings.Builder

	// Render header
	for i, header := range headers {
		output.WriteString(fmt.Sprintf("%-*s  ", colWidths[i], header))
	}
	output.WriteString("\n")

	// Render separator
	for _, width := range colWidths {
		output.WriteString(strings.Repeat("-", width))
		output.WriteString("  ")
	}
	output.WriteString("\n")

	// Render rows
	for _, row := range rows {
		for i, cell := range row {
			cleanCell := f.stripANSI(cell)
			padding := colWidths[i] - len(cleanCell)
			output.WriteString(cell)
			output.WriteString(strings.Repeat(" ", padding))
			output.WriteString("  ")
		}
		output.WriteString("\n")
	}

	return output.String()
}

// renderRelationsSummary renders a summary of relationships
func (f *TableFormatter) renderRelationsSummary(relations []models.Relation) string {
	var output strings.Builder
	output.WriteString(fmt.Sprintf("=== RELATIONSHIPS (%d) ===\n", len(relations)))

	// Group by relation type
	typeCount := make(map[models.RelationType]int)
	for _, rel := range relations {
		typeCount[rel.Type]++
	}

	// Sort by count
	type kv struct {
		Key   models.RelationType
		Value int
	}
	var sorted []kv
	for k, v := range typeCount {
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Value > sorted[j].Value
	})

	headers := []string{"RELATION TYPE", "COUNT"}
	rows := [][]string{}
	for _, item := range sorted {
		rows = append(rows, []string{
			string(item.Key),
			fmt.Sprintf("%d", item.Value),
		})
	}

	output.WriteString(f.renderTable(headers, rows))
	return output.String()
}

// renderErrors renders collection errors
func (f *TableFormatter) renderErrors(errors []models.CollectionError) string {
	var output strings.Builder
	output.WriteString(fmt.Sprintf("=== ERRORS (%d) ===\n", len(errors)))

	headers := []string{"LAYER", "MESSAGE"}
	rows := [][]string{}
	for _, err := range errors {
		rows = append(rows, []string{
			err.Layer,
			f.truncate(err.Message, 60),
		})
	}

	output.WriteString(f.renderTable(headers, rows))
	return output.String()
}

// Helper functions

func (f *TableFormatter) truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func (f *TableFormatter) colorizeHealth(health models.HealthStatus) string {
	switch health {
	case models.HealthHealthy:
		return f.green(string(health))
	case models.HealthDegraded:
		return f.yellow(string(health))
	case models.HealthUnhealthy:
		return f.red(string(health))
	default:
		return string(health)
	}
}

func (f *TableFormatter) colorizeStatus(status string) string {
	switch status {
	case "active":
		return f.green(status)
	case "inactive":
		return f.yellow(status)
	case "failed":
		return f.red(status)
	default:
		return status
	}
}

func (f *TableFormatter) colorizeContainerState(state string) string {
	switch state {
	case "running":
		return f.green(state)
	case "exited", "dead":
		return f.red(state)
	case "paused":
		return f.yellow(state)
	default:
		return state
	}
}

func (f *TableFormatter) colorizePodPhase(phase string) string {
	switch phase {
	case "Running":
		return f.green(phase)
	case "Succeeded":
		return f.green(phase)
	case "Failed":
		return f.red(phase)
	case "Pending":
		return f.yellow(phase)
	case "Unknown":
		return f.yellow(phase)
	default:
		return phase
	}
}

func (f *TableFormatter) colorizeNodeStatus(status string) string {
	if status == "Ready" {
		return f.green(status)
	}
	return f.red(status)
}

func (f *TableFormatter) green(s string) string {
	return fmt.Sprintf("\033[32m%s\033[0m", s)
}

func (f *TableFormatter) yellow(s string) string {
	return fmt.Sprintf("\033[33m%s\033[0m", s)
}

func (f *TableFormatter) red(s string) string {
	return fmt.Sprintf("\033[31m%s\033[0m", s)
}

func (f *TableFormatter) stripANSI(s string) string {
	// Simple ANSI code stripper for width calculation
	result := strings.Builder{}
	inEscape := false
	for _, r := range s {
		if r == '\033' {
			inEscape = true
			continue
		}
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		result.WriteRune(r)
	}
	return result.String()
}

func (f *TableFormatter) formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%dB", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func (f *TableFormatter) formatTime(t time.Time) string {
	duration := time.Since(t)
	if duration < time.Minute {
		return fmt.Sprintf("%ds ago", int(duration.Seconds()))
	}
	if duration < time.Hour {
		return fmt.Sprintf("%dm ago", int(duration.Minutes()))
	}
	if duration < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(duration.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(duration.Hours()/24))
}

func (f *TableFormatter) boolToString(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}

func (f *TableFormatter) countPodRestarts(pod *models.Pod) int32 {
	var total int32
	for _, container := range pod.Containers {
		total += container.RestartCount
	}
	return total
}
