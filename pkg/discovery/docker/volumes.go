package docker

import (
	"context"
	"fmt"
	"time"

	"github.com/docker/docker/api/types/volume"
	"infracanvas/internal/models"
)

// GetVolumes collects all Docker volumes
func (d *Discovery) GetVolumes(ctx context.Context) ([]models.Volume, error) {
	volumeList, err := d.client.VolumeList(ctx, volume.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list volumes: %w", err)
	}
	
	volumes := make([]models.Volume, 0, len(volumeList.Volumes))
	
	for _, vol := range volumeList.Volumes {
		volume := d.parseVolume(vol)
		volumes = append(volumes, volume)
	}
	
	return volumes, nil
}

// parseVolume parses volume information from Docker API
func (d *Discovery) parseVolume(vol *volume.Volume) models.Volume {
	// Parse created time
	created, _ := time.Parse(time.RFC3339, vol.CreatedAt)
	
	return models.Volume{
		BaseEntity: models.BaseEntity{
			ID:          fmt.Sprintf("volume:%s", vol.Name),
			Type:        models.EntityTypeVolume,
			Labels:      vol.Labels,
			Annotations: make(map[string]string),
			Health:      models.HealthHealthy,
			Timestamp:   time.Now(),
		},
		Name:             vol.Name,
		Driver:           vol.Driver,
		MountPoint:       vol.Mountpoint,
		Created:          created,
		UsedByContainers: []string{},
	}
}

// TrackVolumeUsage updates volume usage tracking based on containers
func TrackVolumeUsage(volumes []models.Volume, containers []models.Container) []models.Volume {
	// Create a map for quick lookup
	volumeMap := make(map[string]*models.Volume)
	for i := range volumes {
		volumeMap[volumes[i].Name] = &volumes[i]
	}
	
	// Track which containers use each volume
	for _, container := range containers {
		for _, mount := range container.Mounts {
			if mount.Type == "volume" {
				// Extract volume name from source
				volumeName := mount.Source
				if vol, exists := volumeMap[volumeName]; exists {
					vol.UsedByContainers = append(vol.UsedByContainers, container.ContainerID)
				}
			}
		}
	}
	
	// Convert map back to slice
	result := make([]models.Volume, 0, len(volumeMap))
	for _, vol := range volumeMap {
		result = append(result, *vol)
	}
	
	return result
}
