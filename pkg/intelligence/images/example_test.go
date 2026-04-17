package images_test

import (
	"fmt"

	"infracanvas/internal/models"
	"infracanvas/pkg/intelligence/images"
)

// Example demonstrates basic usage of the image tracker
func Example() {
	// Create a new tracker
	tracker := images.NewTracker()

	// Track Docker images
	dockerImages := []models.Image{
		{
			ImageID:    "sha256:abc123",
			Registry:   "docker.io",
			Repository: "library/nginx",
			Tag:        "1.21",
		},
	}
	tracker.TrackDockerImages(dockerImages)

	// Track Docker containers
	containers := []models.Container{
		{
			ContainerID: "container1",
			ImageID:     "sha256:abc123",
			Image:       "nginx:1.21",
		},
	}
	tracker.TrackDockerContainers(containers)

	// Get all images
	allImages := tracker.GetAllImages()
	fmt.Printf("Total images: %d\n", len(allImages))

	// Output:
	// Total images: 1
}

// ExampleParseImageReference demonstrates image reference parsing
func ExampleParseImageReference() {
	registry, repository, tag := images.ParseImageReference("nginx:1.21")
	fmt.Printf("Registry: %s\n", registry)
	fmt.Printf("Repository: %s\n", repository)
	fmt.Printf("Tag: %s\n", tag)

	// Output:
	// Registry: docker.io
	// Repository: library/nginx
	// Tag: 1.21
}
