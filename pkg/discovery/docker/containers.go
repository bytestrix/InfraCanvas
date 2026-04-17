package docker

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"infracanvas/internal/models"
)

// GetContainers collects all Docker containers
func (d *Discovery) GetContainers(ctx context.Context) ([]models.Container, error) {
	// List all containers (including stopped ones)
	containerList, err := d.client.ContainerList(ctx, container.ListOptions{
		All: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}
	
	var (
		containers []models.Container
		containersMu sync.Mutex
	)

	// Use worker pool for parallel container inspection and stats collection
	numWorkers := 10
	if len(containerList) < numWorkers {
		numWorkers = len(containerList)
	}
	if numWorkers == 0 {
		return containers, nil
	}

	type job struct {
		container types.Container
	}

	jobs := make(chan job, len(containerList))
	var wg sync.WaitGroup

	// Start workers
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				// Inspect container for detailed information
				inspect, err := d.client.ContainerInspect(ctx, j.container.ID)
				if err != nil {
					// Skip containers we can't inspect
					continue
				}

				c := d.parseContainer(j.container, inspect)

				// Collect stats for running containers
				if c.State == "running" {
					stats, err := d.GetContainerStats(ctx, j.container.ID)
					if err == nil {
						c.CPUPercent = stats.CPUPercent
						c.MemoryUsage = stats.MemoryUsage
						c.MemoryLimit = stats.MemoryLimit
						c.NetworkRxBytes = stats.NetworkRxBytes
						c.NetworkTxBytes = stats.NetworkTxBytes
						c.BlockReadBytes = stats.BlockReadBytes
						c.BlockWriteBytes = stats.BlockWriteBytes
					}
				}

				containersMu.Lock()
				containers = append(containers, c)
				containersMu.Unlock()
			}
		}()
	}

	// Send jobs to workers
	for _, c := range containerList {
		jobs <- job{container: c}
	}
	close(jobs)

	// Wait for all workers to complete
	wg.Wait()

	return containers, nil
}

// parseContainer parses container information from Docker API
func (d *Discovery) parseContainer(c types.Container, inspect types.ContainerJSON) models.Container {
	// Parse container name (remove leading slash)
	name := c.Names[0]
	if strings.HasPrefix(name, "/") {
		name = name[1:]
	}
	
	// Parse created time
	created, _ := time.Parse(time.RFC3339Nano, inspect.Created)
	
	// Parse started time
	var started time.Time
	if inspect.State.StartedAt != "" {
		started, _ = time.Parse(time.RFC3339Nano, inspect.State.StartedAt)
	}
	
	// Parse finished time
	var finished time.Time
	if inspect.State.FinishedAt != "" {
		finished, _ = time.Parse(time.RFC3339Nano, inspect.State.FinishedAt)
	}
	
	// Parse environment variables with redaction
	environment := make(map[string]string)
	for _, env := range inspect.Config.Env {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			key := parts[0]
			value := parts[1]
			environment[key] = d.redactor.RedactValue(key, value)
		}
	}
	
	// Parse port mappings
	portMappings := parsePortMappings(inspect.NetworkSettings.Ports)
	
	// Parse mounts
	mounts := parseMounts(inspect.Mounts)
	
	// Extract docker-compose labels
	composeProject := inspect.Config.Labels["com.docker.compose.project"]
	composeService := inspect.Config.Labels["com.docker.compose.service"]

	// Determine network mode
	networkMode := string(inspect.HostConfig.NetworkMode)

	// Collect all connected network names from NetworkSettings
	var networks []string
	for networkName := range inspect.NetworkSettings.Networks {
		networks = append(networks, networkName)
	}
	
	// Calculate health status
	health := calculateContainerHealth(inspect.State)
	
	container := models.Container{
		BaseEntity: models.BaseEntity{
			ID:          fmt.Sprintf("container:%s", c.ID[:12]),
			Type:        models.EntityTypeContainer,
			Labels:      inspect.Config.Labels,
			Annotations: make(map[string]string),
			Health:      health,
			Timestamp:   time.Now(),
		},
		ContainerID:    c.ID[:12],
		Name:           name,
		Image:          c.Image,
		ImageID:        c.ImageID,
		State:          inspect.State.Status,
		Status:         c.Status,
		Created:        created,
		Started:        started,
		Finished:       finished,
		RestartCount:   inspect.RestartCount,
		Environment:    environment,
		PortMappings:   portMappings,
		Mounts:         mounts,
		NetworkMode:    networkMode,
		Networks:       networks,
		ComposeProject: composeProject,
		ComposeService: composeService,
	}
	
	return container
}

// parsePortMappings parses Docker port mappings
func parsePortMappings(ports nat.PortMap) []models.PortMapping {
	mappings := []models.PortMapping{}
	
	for port, bindings := range ports {
		// Parse container port and protocol
		portStr := port.Port()
		protocol := port.Proto()
		
		containerPort, err := strconv.Atoi(portStr)
		if err != nil {
			continue
		}
		
		// Add each binding
		for _, binding := range bindings {
			hostPort, err := strconv.Atoi(binding.HostPort)
			if err != nil {
				continue
			}
			
			mappings = append(mappings, models.PortMapping{
				HostIP:        binding.HostIP,
				HostPort:      hostPort,
				ContainerPort: containerPort,
				Protocol:      protocol,
			})
		}
	}
	
	return mappings
}

// parseMounts parses Docker mounts
func parseMounts(dockerMounts []types.MountPoint) []models.Mount {
	mounts := make([]models.Mount, 0, len(dockerMounts))
	
	for _, m := range dockerMounts {
		mounts = append(mounts, models.Mount{
			Source:      m.Source,
			Destination: m.Destination,
			Mode:        m.Mode,
			Type:        string(m.Type),
		})
	}
	
	return mounts
}

// calculateContainerHealth calculates container health status
func calculateContainerHealth(state *container.State) models.HealthStatus {
	switch state.Status {
	case "running":
		// Check health check status if available
		if state.Health != nil {
			switch state.Health.Status {
			case "healthy":
				return models.HealthHealthy
			case "unhealthy":
				return models.HealthUnhealthy
			case "starting":
				return models.HealthUnknown
			}
		}
		return models.HealthHealthy
	case "exited", "dead":
		return models.HealthUnhealthy
	case "paused":
		return models.HealthDegraded
	default:
		return models.HealthUnknown
	}
}
