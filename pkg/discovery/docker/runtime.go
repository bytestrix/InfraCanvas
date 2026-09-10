package docker

import (
	"context"
	"fmt"
	"strings"
	"time"

	"infracanvas/internal/models"
)

// GetRuntimeInfo collects container runtime information. Works against
// either Docker or Podman (see pkg/dockerhost) — the two are told apart via
// the version endpoint's platform name, which Podman's Docker-API-compatible
// server deliberately reports as "Podman Engine" for exactly this purpose.
func (d *Discovery) GetRuntimeInfo(ctx context.Context) (*models.ContainerRuntime, error) {
	info, err := d.client.Info(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get container runtime info: %w", err)
	}

	version, err := d.client.ServerVersion(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get container runtime version: %w", err)
	}

	runtimeType := "docker"
	if strings.Contains(strings.ToLower(version.Platform.Name), "podman") {
		runtimeType = "podman"
	}

	socketPath := "/var/run/docker.sock"
	if d.resolvedHost != "" {
		socketPath = strings.TrimPrefix(d.resolvedHost, "unix://")
	}

	runtime := &models.ContainerRuntime{
		BaseEntity: models.BaseEntity{
			ID:        "docker-runtime",
			Type:      models.EntityTypeContainerRuntime,
			Labels:    make(map[string]string),
			Health:    models.HealthHealthy,
			Timestamp: time.Now(),
		},
		RuntimeType:   runtimeType,
		Version:       version.Version,
		StorageDriver: info.Driver,
		CgroupDriver:  info.CgroupDriver,
		SocketPath:    socketPath,
	}

	return runtime, nil
}
