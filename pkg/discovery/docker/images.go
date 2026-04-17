package docker

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/docker/docker/api/types/image"
	"infracanvas/internal/models"
)

// GetImages collects all Docker images
func (d *Discovery) GetImages(ctx context.Context) ([]models.Image, error) {
	imageList, err := d.client.ImageList(ctx, image.ListOptions{
		All: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list images: %w", err)
	}
	
	images := make([]models.Image, 0, len(imageList))
	
	for _, img := range imageList {
		image := d.parseImage(img)
		images = append(images, image)
	}
	
	return images, nil
}

// parseImage parses image information from Docker API
func (d *Discovery) parseImage(img image.Summary) models.Image {
	// Parse repository and tag from RepoTags
	var registry, repository, tag string
	
	if len(img.RepoTags) > 0 {
		repoTag := img.RepoTags[0]
		registry, repository, tag = parseImageName(repoTag)
	} else if len(img.RepoDigests) > 0 {
		// Use digest if no tags available
		repoDigest := img.RepoDigests[0]
		registry, repository, _ = parseImageName(repoDigest)
		tag = "<none>"
	} else {
		registry = ""
		repository = "<none>"
		tag = "<none>"
	}
	
	// Parse created time
	created := time.Unix(img.Created, 0)
	
	// Extract digest from RepoDigests
	var digest string
	if len(img.RepoDigests) > 0 {
		parts := strings.Split(img.RepoDigests[0], "@")
		if len(parts) == 2 {
			digest = parts[1]
		}
	}
	
	// Truncate image ID for display
	imageIDShort := img.ID
	if strings.HasPrefix(imageIDShort, "sha256:") {
		imageIDShort = imageIDShort[7:19]
	}
	
	return models.Image{
		BaseEntity: models.BaseEntity{
			ID:          fmt.Sprintf("image:%s", imageIDShort),
			Type:        models.EntityTypeImage,
			Labels:      img.Labels,
			Annotations: make(map[string]string),
			Health:      models.HealthHealthy,
			Timestamp:   time.Now(),
		},
		ImageID:          img.ID,
		Registry:         registry,
		Repository:       repository,
		Tag:              tag,
		Digest:           digest,
		Size:             img.Size,
		Created:          created,
		UsedByContainers: []string{},
		UsedByPods:       []string{},
	}
}

// parseImageName parses registry, repository, and tag from image name
func parseImageName(imageName string) (registry, repository, tag string) {
	// Remove digest if present
	if strings.Contains(imageName, "@") {
		imageName = strings.Split(imageName, "@")[0]
	}
	
	// Default tag
	tag = "latest"
	
	// Find the last colon to check if it's a tag
	lastColon := strings.LastIndex(imageName, ":")
	lastSlash := strings.LastIndex(imageName, "/")
	
	var nameWithoutTag string
	
	// If there's a colon after the last slash, it's a tag
	if lastColon > lastSlash && lastColon != -1 {
		nameWithoutTag = imageName[:lastColon]
		tag = imageName[lastColon+1:]
	} else {
		nameWithoutTag = imageName
	}
	
	// Parse registry and repository
	parts := strings.Split(nameWithoutTag, "/")
	
	switch len(parts) {
	case 1:
		// No registry, library image (e.g., "nginx")
		registry = "docker.io"
		repository = "library/" + parts[0]
	case 2:
		// Could be registry/image or user/image
		if strings.Contains(parts[0], ".") || strings.Contains(parts[0], ":") {
			// Has domain or port, so it's a registry
			registry = parts[0]
			repository = parts[1]
		} else {
			// No domain, so it's docker.io/user/image
			registry = "docker.io"
			repository = nameWithoutTag
		}
	case 3:
		// registry/user/image
		registry = parts[0]
		repository = parts[1] + "/" + parts[2]
	default:
		// More than 3 parts, first is registry, rest is repository
		registry = parts[0]
		repository = strings.Join(parts[1:], "/")
	}
	
	return registry, repository, tag
}

// TrackImageUsage updates image usage tracking based on containers
func TrackImageUsage(images []models.Image, containers []models.Container) []models.Image {
	// Create a map for quick lookup
	imageMap := make(map[string]*models.Image)
	for i := range images {
		imageMap[images[i].ImageID] = &images[i]
	}
	
	// Track which containers use each image
	for _, container := range containers {
		if img, exists := imageMap[container.ImageID]; exists {
			img.UsedByContainers = append(img.UsedByContainers, container.ContainerID)
		}
	}
	
	// Convert map back to slice
	result := make([]models.Image, 0, len(imageMap))
	for _, img := range imageMap {
		result = append(result, *img)
	}
	
	return result
}
