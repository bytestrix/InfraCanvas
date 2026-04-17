package host

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"infracanvas/internal/models"
	"infracanvas/pkg/validation"
)

// getSystemdServices collects systemd services with validation (internal implementation)
func getSystemdServices() ([]models.Service, error) {
	// Check if systemd is available
	if !isSystemdAvailable() {
		return nil, fmt.Errorf("systemd not available")
	}

	// Execute systemctl list-units --type=service --all
	cmd := exec.Command("systemctl", "list-units", "--type=service", "--all", "--no-pager", "--no-legend")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute systemctl: %w", err)
	}

	// Validate command output
	if err := validation.ValidateCommandOutput(string(output), "", "systemctl list-units"); err != nil {
		validation.LogParseError(err, "systemctl output validation")
		return nil, err
	}

	services := []models.Service{}
	lines, err := validation.SafeSplitLines(string(output), "systemctl list-units")
	if err != nil {
		validation.LogParseError(err, "systemctl output parsing")
		return nil, err
	}

	for _, line := range lines {
		// Parse service line
		service, err := parseServiceLine(line)
		if err != nil {
			// Log but continue with other services
			validation.LogParseError(err, fmt.Sprintf("service line: %s", line))
			continue
		}

		// Get additional service details
		enrichServiceDetails(service)

		services = append(services, *service)
	}

	return services, nil
}

// isSystemdAvailable checks if systemd is available
func isSystemdAvailable() bool {
	cmd := exec.Command("systemctl", "--version")
	err := cmd.Run()
	return err == nil
}

// parseServiceLine parses a line from systemctl list-units output with validation
func parseServiceLine(line string) (*models.Service, error) {
	// Format: UNIT LOAD ACTIVE SUB DESCRIPTION
	// Example: docker.service loaded active running Docker Application Container Engine
	
	fields, err := validation.SafeSplitFields(line, 5, "systemctl list-units line")
	if err != nil {
		return nil, err
	}

	service := &models.Service{
		BaseEntity: models.BaseEntity{
			ID:        fmt.Sprintf("service:%s", fields[0]),
			Type:      models.EntityTypeService,
			Labels:    make(map[string]string),
			Timestamp: time.Now(),
		},
		Name: fields[0],
	}

	// Validate service name is not empty
	if err := validation.ValidateNotEmpty(service.Name, "service_name", "systemctl list-units"); err != nil {
		return nil, err
	}

	// Parse status
	loadState := fields[1]
	activeState := fields[2]
	subState := fields[3]

	// Combine active and sub state for status
	service.Status = fmt.Sprintf("%s/%s", activeState, subState)

	// Parse description (remaining fields)
	if len(fields) > 4 {
		service.Description = strings.Join(fields[4:], " ")
	}

	// Mark as critical if it's a known critical service
	service.IsCritical = isCriticalService(service.Name)

	// Set health based on status
	if activeState == "active" && subState == "running" {
		service.Health = models.HealthHealthy
	} else if activeState == "failed" {
		service.Health = models.HealthUnhealthy
	} else if activeState == "inactive" {
		service.Health = models.HealthUnknown
	} else {
		service.Health = models.HealthDegraded
	}

	_ = loadState // Unused for now

	return service, nil
}

// enrichServiceDetails gets additional details for a service with validation
func enrichServiceDetails(service *models.Service) {
	// Execute systemctl show <service> to get more details
	cmd := exec.Command("systemctl", "show", service.Name, "--no-pager")
	output, err := cmd.Output()
	if err != nil {
		return
	}

	lines, err := validation.SafeSplitLines(string(output), fmt.Sprintf("systemctl show %s", service.Name))
	if err != nil {
		validation.LogParseError(err, "systemctl show output parsing")
		return
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "UnitFileState=") {
			state := strings.TrimPrefix(line, "UnitFileState=")
			service.Enabled = (state == "enabled")
		} else if strings.HasPrefix(line, "NRestarts=") {
			restartStr := strings.TrimPrefix(line, "NRestarts=")
			restartCount, err := validation.SafeParseInt(restartStr, "restart_count", fmt.Sprintf("systemctl show %s", service.Name))
			if err != nil {
				// Log but continue
				service.RestartCount = 0
			} else {
				service.RestartCount = restartCount
			}
		}
	}

	// Get dependencies
	deps, err := getServiceDependencies(service.Name)
	if err == nil {
		service.Dependencies = deps
	}
}

// getServiceDependencies gets dependencies for a service (internal implementation)
func getServiceDependencies(serviceName string) ([]string, error) {
	// Execute systemctl show <service> to get dependencies
	cmd := exec.Command("systemctl", "show", serviceName, "--property=Requires", "--property=Wants", "--no-pager")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	dependencies := []string{}
	lines := strings.Split(string(output), "\n")

	for _, line := range lines {
		if strings.HasPrefix(line, "Requires=") {
			deps := strings.TrimPrefix(line, "Requires=")
			if deps != "" {
				dependencies = append(dependencies, strings.Split(deps, " ")...)
			}
		} else if strings.HasPrefix(line, "Wants=") {
			deps := strings.TrimPrefix(line, "Wants=")
			if deps != "" {
				dependencies = append(dependencies, strings.Split(deps, " ")...)
			}
		}
	}

	return dependencies, nil
}

// isCriticalService checks if a service is critical infrastructure
func isCriticalService(serviceName string) bool {
	criticalServices := []string{
		"docker.service",
		"dockerd.service",
		"kubelet.service",
		"containerd.service",
		"sshd.service",
		"ssh.service",
		"systemd-networkd.service",
		"NetworkManager.service",
		"systemd-resolved.service",
	}

	for _, critical := range criticalServices {
		if serviceName == critical {
			return true
		}
	}

	return false
}

// RestartService restarts a systemd service
func (d *Discovery) RestartService(serviceName string) error {
	if !isSystemdAvailable() {
		return fmt.Errorf("systemd not available")
	}

	cmd := exec.Command("systemctl", "restart", serviceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to restart service %s: %w, output: %s", serviceName, err, string(output))
	}

	return nil
}

// StopService stops a systemd service
func (d *Discovery) StopService(serviceName string) error {
	if !isSystemdAvailable() {
		return fmt.Errorf("systemd not available")
	}

	cmd := exec.Command("systemctl", "stop", serviceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to stop service %s: %w, output: %s", serviceName, err, string(output))
	}

	return nil
}

// StartService starts a systemd service
func (d *Discovery) StartService(serviceName string) error {
	if !isSystemdAvailable() {
		return fmt.Errorf("systemd not available")
	}

	cmd := exec.Command("systemctl", "start", serviceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to start service %s: %w, output: %s", serviceName, err, string(output))
	}

	return nil
}

// GetServiceStatus gets the status of a specific service
func (d *Discovery) GetServiceStatus(serviceName string) (*models.Service, error) {
	if !isSystemdAvailable() {
		return nil, fmt.Errorf("systemd not available")
	}

	cmd := exec.Command("systemctl", "status", serviceName, "--no-pager")
	output, err := cmd.Output()
	
	// Note: systemctl status returns non-zero exit code for inactive services
	// So we don't treat err as a fatal error here
	
	service := &models.Service{
		BaseEntity: models.BaseEntity{
			Type:      models.EntityTypeService,
			Labels:    make(map[string]string),
			Timestamp: time.Now(),
		},
		Name: serviceName,
	}
	service.ID = fmt.Sprintf("service:%s", serviceName)

	// Parse status output
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Active:") {
			// Parse active state
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				service.Status = fields[1]
			}
		} else if strings.HasPrefix(line, "Loaded:") {
			// Parse enabled state
			if strings.Contains(line, "enabled") {
				service.Enabled = true
			}
		}
	}

	// Enrich with additional details
	enrichServiceDetails(service)

	// Mark as critical
	service.IsCritical = isCriticalService(serviceName)

	// Set health
	if strings.Contains(service.Status, "active") {
		service.Health = models.HealthHealthy
	} else if strings.Contains(service.Status, "failed") {
		service.Health = models.HealthUnhealthy
	} else {
		service.Health = models.HealthUnknown
	}

	_ = err // Ignore error since status command returns non-zero for inactive services

	return service, nil
}
