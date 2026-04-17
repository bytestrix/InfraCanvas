package images

import (
	"fmt"
	"strings"

	"infracanvas/internal/models"
)

// Tracker tracks and analyzes container images across Docker and Kubernetes layers
type Tracker struct {
	images map[string]*models.Image // Key: ImageID
}

// NewTracker creates a new image tracker
func NewTracker() *Tracker {
	return &Tracker{
		images: make(map[string]*models.Image),
	}
}

// TrackDockerImages adds Docker images to the tracker
func (t *Tracker) TrackDockerImages(dockerImages []models.Image) {
	for i := range dockerImages {
		img := &dockerImages[i]
		t.images[img.ImageID] = img
	}
}

// TrackDockerContainers maps Docker containers to their images
func (t *Tracker) TrackDockerContainers(containers []models.Container) {
	for _, container := range containers {
		if img, exists := t.images[container.ImageID]; exists {
			// Check if container ID is already tracked
			found := false
			for _, cid := range img.UsedByContainers {
				if cid == container.ContainerID {
					found = true
					break
				}
			}
			if !found {
				img.UsedByContainers = append(img.UsedByContainers, container.ContainerID)
			}
		} else {
			// Create a new image entry if not found
			img := t.createImageFromContainer(container)
			t.images[img.ImageID] = &img
		}
	}
}

// TrackKubernetesPods maps Kubernetes pods to their images
func (t *Tracker) TrackKubernetesPods(pods []models.Pod) {
	for _, pod := range pods {
		podID := pod.GetID()
		for _, container := range pod.Containers {
			// Parse the image reference
			imageID := container.ImageID
			imageName := container.Image
			
			// Find or create image entry
			img := t.findOrCreateImageFromK8s(imageID, imageName)
			
			// Track pod usage
			found := false
			for _, pid := range img.UsedByPods {
				if pid == podID {
					found = true
					break
				}
			}
			if !found {
				img.UsedByPods = append(img.UsedByPods, podID)
			}
		}
	}
}

// TrackKubernetesWorkloads maps Kubernetes workloads to their images
// This tracks which workloads (Deployments, StatefulSets, etc.) use which images
func (t *Tracker) TrackKubernetesWorkloads(
	deployments []models.Deployment,
	statefulSets []models.StatefulSet,
	daemonSets []models.DaemonSet,
) {
	// Track deployment images
	for _, deploy := range deployments {
		for _, container := range deploy.Containers {
			t.trackWorkloadImage(container.Image)
		}
	}
	
	// Track statefulset images
	for _, sts := range statefulSets {
		for _, container := range sts.Containers {
			t.trackWorkloadImage(container.Image)
		}
	}
	
	// Track daemonset images
	for _, ds := range daemonSets {
		for _, container := range ds.Containers {
			t.trackWorkloadImage(container.Image)
		}
	}
}

// trackWorkloadImage ensures an image entry exists for a workload image reference
func (t *Tracker) trackWorkloadImage(imageName string) {
	// Parse the image reference
	registry, repository, tag := ParseImageReference(imageName)
	
	// Try to find existing image by matching registry/repository/tag
	for _, img := range t.images {
		if img.Registry == registry && img.Repository == repository && img.Tag == tag {
			return // Already tracked
		}
	}
	
	// Create a new image entry if not found
	imageID := fmt.Sprintf("image:%s/%s:%s", registry, repository, tag)
	img := &models.Image{
		BaseEntity: models.BaseEntity{
			ID:   imageID,
			Type: models.EntityTypeImage,
		},
		ImageID:          imageID,
		Registry:         registry,
		Repository:       repository,
		Tag:              tag,
		UsedByContainers: []string{},
		UsedByPods:       []string{},
	}
	t.images[imageID] = img
}

// GetAllImages returns all tracked images
func (t *Tracker) GetAllImages() []models.Image {
	images := make([]models.Image, 0, len(t.images))
	for _, img := range t.images {
		images = append(images, *img)
	}
	return images
}

// GroupByRepository groups images by repository
func (t *Tracker) GroupByRepository() map[string][]models.Image {
	groups := make(map[string][]models.Image)
	
	for _, img := range t.images {
		repoKey := fmt.Sprintf("%s/%s", img.Registry, img.Repository)
		groups[repoKey] = append(groups[repoKey], *img)
	}
	
	return groups
}

// GetImagesWithLatestTag returns images with "latest" tag
func (t *Tracker) GetImagesWithLatestTag() []models.Image {
	var images []models.Image
	
	for _, img := range t.images {
		if img.Tag == "latest" {
			images = append(images, *img)
		}
	}
	
	return images
}

// GetImagesWithoutExplicitTag returns images without explicit tags (tag is empty or "latest")
func (t *Tracker) GetImagesWithoutExplicitTag() []models.Image {
	var images []models.Image
	
	for _, img := range t.images {
		if img.Tag == "" || img.Tag == "latest" || img.Tag == "<none>" {
			images = append(images, *img)
		}
	}
	
	return images
}

// GetImagesByContainer returns the image used by a specific container
func (t *Tracker) GetImagesByContainer(containerID string) *models.Image {
	for _, img := range t.images {
		for _, cid := range img.UsedByContainers {
			if cid == containerID {
				return img
			}
		}
	}
	return nil
}

// GetImagesByPod returns all images used by a specific pod
func (t *Tracker) GetImagesByPod(podID string) []models.Image {
	var images []models.Image
	
	for _, img := range t.images {
		for _, pid := range img.UsedByPods {
			if pid == podID {
				images = append(images, *img)
				break
			}
		}
	}
	
	return images
}

// createImageFromContainer creates an image entry from a Docker container
func (t *Tracker) createImageFromContainer(container models.Container) models.Image {
	registry, repository, tag := ParseImageReference(container.Image)
	
	return models.Image{
		BaseEntity: models.BaseEntity{
			ID:   fmt.Sprintf("image:%s", container.ImageID),
			Type: models.EntityTypeImage,
		},
		ImageID:          container.ImageID,
		Registry:         registry,
		Repository:       repository,
		Tag:              tag,
		UsedByContainers: []string{container.ContainerID},
		UsedByPods:       []string{},
	}
}

// findOrCreateImageFromK8s finds or creates an image entry from Kubernetes pod container
func (t *Tracker) findOrCreateImageFromK8s(imageID, imageName string) *models.Image {
	// Try to find by imageID first
	if imageID != "" {
		// Clean up imageID (remove docker-pullable:// prefix if present)
		cleanImageID := strings.TrimPrefix(imageID, "docker-pullable://")
		cleanImageID = strings.TrimPrefix(cleanImageID, "docker://")
		
		// Try exact match
		if img, exists := t.images[cleanImageID]; exists {
			return img
		}
		
		// Try to find by matching the digest part
		if strings.Contains(cleanImageID, "@sha256:") {
			for _, img := range t.images {
				if strings.Contains(img.ImageID, "@sha256:") && 
				   strings.HasSuffix(cleanImageID, strings.Split(img.ImageID, "@sha256:")[1]) {
					return img
				}
			}
		}
	}
	
	// Parse the image reference
	registry, repository, tag := ParseImageReference(imageName)
	
	// Try to find by registry/repository/tag
	for _, img := range t.images {
		if img.Registry == registry && img.Repository == repository && img.Tag == tag {
			return img
		}
	}
	
	// Create a new image entry
	imageIDKey := imageID
	if imageIDKey == "" {
		imageIDKey = fmt.Sprintf("image:%s/%s:%s", registry, repository, tag)
	}
	
	img := &models.Image{
		BaseEntity: models.BaseEntity{
			ID:   imageIDKey,
			Type: models.EntityTypeImage,
		},
		ImageID:          imageIDKey,
		Registry:         registry,
		Repository:       repository,
		Tag:              tag,
		UsedByContainers: []string{},
		UsedByPods:       []string{},
	}
	t.images[imageIDKey] = img
	return img
}

// ParseImageReference parses an image reference into registry, repository, and tag
func ParseImageReference(imageName string) (registry, repository, tag string) {
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
