# Image Intelligence Package

The `images` package provides comprehensive image tracking and analysis capabilities across Docker and Kubernetes layers. It enables tracking of all unique container images, their usage patterns, and provides intelligent grouping and filtering capabilities.

## Features

### Core Capabilities

1. **Unified Image Tracking**: Track all unique images across Docker containers and Kubernetes pods
2. **Usage Mapping**: Map images to their consumers (Docker containers, Kubernetes pods)
3. **Image Parsing**: Parse image references to extract registry, repository, tag, and digest components
4. **Repository Grouping**: Group images by repository to identify different versions
5. **Tag Analysis**: Identify images with `latest` tag or no explicit tag
6. **Cross-Layer Intelligence**: Correlate images across Docker and Kubernetes layers

### Requirements Coverage

This package implements the following requirements from the Infrastructure Discovery CLI specification:

- **Requirement 13.1**: Identify all unique container images across Docker containers and Kubernetes Pods
- **Requirement 13.2**: Track which Docker containers use each image
- **Requirement 13.3**: Track which Kubernetes Pods use each image
- **Requirement 13.4**: Track which Kubernetes Workloads use each image
- **Requirement 13.5**: Extract image registry, repository, tag, and digest
- **Requirement 13.6**: Group images by repository and list all tags
- **Requirement 13.7**: Identify images with latest tag
- **Requirement 13.8**: Identify images without explicit tags

## Usage

### Basic Usage

```go
import "rix/pkg/intelligence/images"

// Create a new tracker
tracker := images.NewTracker()

// Track Docker images
dockerImages := []models.Image{...}
tracker.TrackDockerImages(dockerImages)

// Track Docker containers
containers := []models.Container{...}
tracker.TrackDockerContainers(containers)

// Track Kubernetes pods
pods := []models.Pod{...}
tracker.TrackKubernetesPods(pods)

// Track Kubernetes workloads
deployments := []models.Deployment{...}
statefulSets := []models.StatefulSet{...}
daemonSets := []models.DaemonSet{...}
tracker.TrackKubernetesWorkloads(deployments, statefulSets, daemonSets)

// Get all tracked images
allImages := tracker.GetAllImages()
```

### Querying Images

```go
// Group images by repository
groups := tracker.GroupByRepository()
for repoKey, images := range groups {
    fmt.Printf("Repository: %s\n", repoKey)
    for _, img := range images {
        fmt.Printf("  - Tag: %s\n", img.Tag)
    }
}

// Find images with latest tag
latestImages := tracker.GetImagesWithLatestTag()

// Find images without explicit tags
noTagImages := tracker.GetImagesWithoutExplicitTag()

// Find image used by a specific container
img := tracker.GetImagesByContainer("container-id")

// Find all images used by a specific pod
podImages := tracker.GetImagesByPod("pod/namespace/name")
```

### Image Reference Parsing

```go
// Parse an image reference
registry, repository, tag := images.ParseImageReference("gcr.io/myproject/myapp:v1.0")
// registry: "gcr.io"
// repository: "myproject/myapp"
// tag: "v1.0"

// Parse a simple library image
registry, repository, tag := images.ParseImageReference("nginx:1.21")
// registry: "docker.io"
// repository: "library/nginx"
// tag: "1.21"

// Parse an image with digest
registry, repository, tag := images.ParseImageReference("nginx@sha256:abc123")
// registry: "docker.io"
// repository: "library/nginx"
// tag: "latest"
```

## Architecture

### Tracker Structure

The `Tracker` maintains an internal map of images indexed by their ImageID. This allows for efficient lookups and prevents duplicate tracking.

```go
type Tracker struct {
    images map[string]*models.Image // Key: ImageID
}
```

### Image Deduplication

The tracker automatically handles deduplication:
- Docker images are tracked by their ImageID
- Kubernetes pod images are matched to existing Docker images when possible
- Workload images are tracked separately if not found in Docker or pod images

### Cross-Layer Correlation

The tracker intelligently correlates images across layers:
1. Docker images are added first with their full metadata
2. Docker containers are mapped to images by ImageID
3. Kubernetes pods are mapped to images by ImageID or image reference
4. Workload images are tracked to ensure all image references are captured

## Image Reference Parsing

The package includes a robust image reference parser that handles various formats:

### Supported Formats

| Format | Example | Registry | Repository | Tag |
|--------|---------|----------|------------|-----|
| Library image | `nginx` | `docker.io` | `library/nginx` | `latest` |
| Library with tag | `nginx:1.21` | `docker.io` | `library/nginx` | `1.21` |
| User image | `myuser/myapp:v1.0` | `docker.io` | `myuser/myapp` | `v1.0` |
| Custom registry | `gcr.io/project/app:v1` | `gcr.io` | `project/app` | `v1` |
| Registry with port | `localhost:5000/app:dev` | `localhost:5000` | `app` | `dev` |
| With digest | `nginx@sha256:abc123` | `docker.io` | `library/nginx` | `latest` |
| Deep path | `reg.io/team/proj/svc:v2` | `reg.io` | `team/proj/svc` | `v2` |

### Parsing Rules

1. **Digest Handling**: Digests are stripped before parsing (e.g., `@sha256:...`)
2. **Default Registry**: Images without a registry default to `docker.io`
3. **Library Images**: Single-part names are prefixed with `library/`
4. **Default Tag**: Images without a tag default to `latest`
5. **Registry Detection**: Registries are identified by the presence of `.` or `:` in the first path component

## API Reference

### Tracker Methods

#### `NewTracker() *Tracker`
Creates a new image tracker instance.

#### `TrackDockerImages(dockerImages []models.Image)`
Adds Docker images to the tracker.

#### `TrackDockerContainers(containers []models.Container)`
Maps Docker containers to their images. Creates new image entries if not found.

#### `TrackKubernetesPods(pods []models.Pod)`
Maps Kubernetes pods to their images. Attempts to correlate with existing Docker images.

#### `TrackKubernetesWorkloads(deployments, statefulSets, daemonSets)`
Tracks images used by Kubernetes workloads.

#### `GetAllImages() []models.Image`
Returns all tracked images.

#### `GroupByRepository() map[string][]models.Image`
Groups images by repository (registry/repository).

#### `GetImagesWithLatestTag() []models.Image`
Returns images with the `latest` tag.

#### `GetImagesWithoutExplicitTag() []models.Image`
Returns images with `latest`, empty, or `<none>` tags.

#### `GetImagesByContainer(containerID string) *models.Image`
Returns the image used by a specific container.

#### `GetImagesByPod(podID string) []models.Image`
Returns all images used by a specific pod.

### Utility Functions

#### `ParseImageReference(imageName string) (registry, repository, tag string)`
Parses an image reference into its components.

## Testing

The package includes comprehensive unit tests covering:
- Basic tracking operations
- Cross-layer correlation
- Deduplication logic
- Image reference parsing
- Query operations
- Integration scenarios

Run tests:
```bash
go test -v ./pkg/intelligence/images/...
```

## Integration Example

```go
package main

import (
    "context"
    "fmt"
    
    "rix/pkg/discovery/docker"
    "rix/pkg/discovery/kubernetes"
    "rix/pkg/intelligence/images"
)

func main() {
    // Initialize discoveries
    dockerDiscovery, _ := docker.NewDiscovery()
    k8sDiscovery, _ := kubernetes.NewDiscovery()
    
    // Create image tracker
    tracker := images.NewTracker()
    
    // Collect Docker data
    dockerImages, _ := dockerDiscovery.GetImages(context.Background())
    containers, _ := dockerDiscovery.GetContainers(context.Background())
    
    // Track Docker layer
    tracker.TrackDockerImages(dockerImages)
    tracker.TrackDockerContainers(containers)
    
    // Collect Kubernetes data
    pods, _ := k8sDiscovery.GetPods(context.Background(), "")
    deployments, _ := k8sDiscovery.GetDeployments(context.Background(), "")
    statefulSets, _ := k8sDiscovery.GetStatefulSets(context.Background(), "")
    daemonSets, _ := k8sDiscovery.GetDaemonSets(context.Background(), "")
    
    // Track Kubernetes layer
    tracker.TrackKubernetesPods(pods)
    tracker.TrackKubernetesWorkloads(deployments, statefulSets, daemonSets)
    
    // Analyze images
    fmt.Println("=== All Images ===")
    for _, img := range tracker.GetAllImages() {
        fmt.Printf("%s/%s:%s\n", img.Registry, img.Repository, img.Tag)
        fmt.Printf("  Used by %d containers, %d pods\n", 
            len(img.UsedByContainers), len(img.UsedByPods))
    }
    
    fmt.Println("\n=== Images by Repository ===")
    for repo, images := range tracker.GroupByRepository() {
        fmt.Printf("%s: %d versions\n", repo, len(images))
    }
    
    fmt.Println("\n=== Images with Latest Tag ===")
    for _, img := range tracker.GetImagesWithLatestTag() {
        fmt.Printf("%s/%s:%s\n", img.Registry, img.Repository, img.Tag)
    }
}
```

## Design Decisions

### Why Track Images Separately?

Images are tracked as first-class entities because:
1. They are shared across multiple containers and pods
2. Image metadata (size, creation time, layers) is valuable for analysis
3. Tracking usage patterns helps identify unused or over-used images
4. Repository grouping enables version analysis

### Why Parse Image References?

Parsing image references enables:
1. Consistent registry/repository/tag extraction
2. Grouping images by repository
3. Identifying images without explicit tags
4. Normalizing image references across Docker and Kubernetes

### Why Deduplicate Across Layers?

Deduplication is important because:
1. The same image may be used by both Docker containers and Kubernetes pods
2. Prevents double-counting in usage statistics
3. Provides a unified view of image usage across the infrastructure

## Future Enhancements

Potential future enhancements:
- Image vulnerability tracking
- Image size analysis and optimization recommendations
- Image pull policy analysis
- Image registry health checks
- Image lifecycle management (unused images, old versions)
- Image provenance tracking
