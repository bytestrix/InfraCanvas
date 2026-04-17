package docker

import (
	"context"
	"fmt"
	"time"

	"infracanvas/internal/models"
)

// GetRuntimeInfo collects Docker runtime information
func (d *Discovery) GetRuntimeInfo(ctx context.Context) (*models.ContainerRuntime, error) {
	info, err := d.client.Info(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get Docker info: %w", err)
	}
	
	version, err := d.client.ServerVersion(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get Docker version: %w", err)
	}
	
	runtime := &models.ContainerRuntime{
		BaseEntity: models.BaseEntity{
			ID:        "docker-runtime",
			Type:      models.EntityTypeContainerRuntime,
			Labels:    make(map[string]string),
			Health:    models.HealthHealthy,
			Timestamp: time.Now(),
		},
		RuntimeType:   "docker",
		Version:       version.Version,
		StorageDriver: info.Driver,
		CgroupDriver:  info.CgroupDriver,
		SocketPath:    "/var/run/docker.sock",
	}
	
	return runtime, nil
}
