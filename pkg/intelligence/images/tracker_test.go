package images

import (
	"testing"
	"time"

	"infracanvas/internal/models"
)

func TestNewTracker(t *testing.T) {
	tracker := NewTracker()
	if tracker == nil {
		t.Fatal("NewTracker returned nil")
	}
	if tracker.images == nil {
		t.Fatal("tracker.images is nil")
	}
}

func TestTrackDockerImages(t *testing.T) {
	tracker := NewTracker()
	
	dockerImages := []models.Image{
		{
			BaseEntity: models.BaseEntity{
				ID:   "image:abc123",
				Type: models.EntityTypeImage,
			},
			ImageID:    "sha256:abc123",
			Registry:   "docker.io",
			Repository: "library/nginx",
			Tag:        "1.21",
		},
		{
			BaseEntity: models.BaseEntity{
				ID:   "image:def456",
				Type: models.EntityTypeImage,
			},
			ImageID:    "sha256:def456",
			Registry:   "docker.io",
			Repository: "library/redis",
			Tag:        "latest",
		},
	}
	
	tracker.TrackDockerImages(dockerImages)
	
	images := tracker.GetAllImages()
	if len(images) != 2 {
		t.Errorf("Expected 2 images, got %d", len(images))
	}
}

func TestTrackDockerContainers(t *testing.T) {
	tracker := NewTracker()
	
	// Add initial images
	dockerImages := []models.Image{
		{
			BaseEntity: models.BaseEntity{
				ID:   "image:abc123",
				Type: models.EntityTypeImage,
			},
			ImageID:          "sha256:abc123",
			Registry:         "docker.io",
			Repository:       "library/nginx",
			Tag:              "1.21",
			UsedByContainers: []string{},
		},
	}
	tracker.TrackDockerImages(dockerImages)
	
	// Track containers
	containers := []models.Container{
		{
			ContainerID: "container1",
			ImageID:     "sha256:abc123",
			Image:       "nginx:1.21",
		},
		{
			ContainerID: "container2",
			ImageID:     "sha256:abc123",
			Image:       "nginx:1.21",
		},
	}
	
	tracker.TrackDockerContainers(containers)
	
	img := tracker.images["sha256:abc123"]
	if img == nil {
		t.Fatal("Image not found")
	}
	
	if len(img.UsedByContainers) != 2 {
		t.Errorf("Expected 2 containers, got %d", len(img.UsedByContainers))
	}
	
	// Verify container IDs
	expectedContainers := map[string]bool{"container1": true, "container2": true}
	for _, cid := range img.UsedByContainers {
		if !expectedContainers[cid] {
			t.Errorf("Unexpected container ID: %s", cid)
		}
	}
}

func TestTrackDockerContainers_NewImage(t *testing.T) {
	tracker := NewTracker()
	
	// Track container with image not in tracker
	containers := []models.Container{
		{
			ContainerID: "container1",
			ImageID:     "sha256:xyz789",
			Image:       "postgres:13",
		},
	}
	
	tracker.TrackDockerContainers(containers)
	
	images := tracker.GetAllImages()
	if len(images) != 1 {
		t.Errorf("Expected 1 image, got %d", len(images))
	}
	
	img := images[0]
	if img.Registry != "docker.io" {
		t.Errorf("Expected registry docker.io, got %s", img.Registry)
	}
	if img.Repository != "library/postgres" {
		t.Errorf("Expected repository library/postgres, got %s", img.Repository)
	}
	if img.Tag != "13" {
		t.Errorf("Expected tag 13, got %s", img.Tag)
	}
}

func TestTrackKubernetesPods(t *testing.T) {
	tracker := NewTracker()
	
	pods := []models.Pod{
		{
			BaseEntity: models.BaseEntity{
				ID:   "pod/default/nginx-pod",
				Type: models.EntityTypePod,
			},
			Name:      "nginx-pod",
			Namespace: "default",
			Containers: []models.PodContainer{
				{
					Name:    "nginx",
					Image:   "nginx:1.21",
					ImageID: "docker-pullable://nginx@sha256:abc123",
				},
			},
		},
		{
			BaseEntity: models.BaseEntity{
				ID:   "pod/default/redis-pod",
				Type: models.EntityTypePod,
			},
			Name:      "redis-pod",
			Namespace: "default",
			Containers: []models.PodContainer{
				{
					Name:    "redis",
					Image:   "redis:latest",
					ImageID: "docker-pullable://redis@sha256:def456",
				},
			},
		},
	}
	
	tracker.TrackKubernetesPods(pods)
	
	images := tracker.GetAllImages()
	if len(images) != 2 {
		t.Errorf("Expected 2 images, got %d", len(images))
	}
	
	// Check that pods are tracked
	for _, img := range images {
		if len(img.UsedByPods) != 1 {
			t.Errorf("Expected 1 pod for image %s, got %d", img.ImageID, len(img.UsedByPods))
		}
	}
}

func TestTrackKubernetesWorkloads(t *testing.T) {
	tracker := NewTracker()
	
	deployments := []models.Deployment{
		{
			BaseEntity: models.BaseEntity{
				ID: "deployment/default/nginx",
			},
			Name:      "nginx",
			Namespace: "default",
			Containers: []models.ContainerSpec{
				{
					Name:  "nginx",
					Image: "nginx:1.21",
				},
			},
		},
	}
	
	statefulSets := []models.StatefulSet{
		{
			BaseEntity: models.BaseEntity{
				ID: "statefulset/default/postgres",
			},
			Name:      "postgres",
			Namespace: "default",
			Containers: []models.ContainerSpec{
				{
					Name:  "postgres",
					Image: "postgres:13",
				},
			},
		},
	}
	
	daemonSets := []models.DaemonSet{
		{
			BaseEntity: models.BaseEntity{
				ID: "daemonset/kube-system/fluentd",
			},
			Name:      "fluentd",
			Namespace: "kube-system",
			Containers: []models.ContainerSpec{
				{
					Name:  "fluentd",
					Image: "fluentd:v1.14",
				},
			},
		},
	}
	
	tracker.TrackKubernetesWorkloads(deployments, statefulSets, daemonSets)
	
	images := tracker.GetAllImages()
	if len(images) != 3 {
		t.Errorf("Expected 3 images, got %d", len(images))
	}
}

func TestGroupByRepository(t *testing.T) {
	tracker := NewTracker()
	
	dockerImages := []models.Image{
		{
			ImageID:    "sha256:abc123",
			Registry:   "docker.io",
			Repository: "library/nginx",
			Tag:        "1.21",
		},
		{
			ImageID:    "sha256:def456",
			Registry:   "docker.io",
			Repository: "library/nginx",
			Tag:        "1.22",
		},
		{
			ImageID:    "sha256:ghi789",
			Registry:   "docker.io",
			Repository: "library/redis",
			Tag:        "latest",
		},
	}
	
	tracker.TrackDockerImages(dockerImages)
	
	groups := tracker.GroupByRepository()
	
	if len(groups) != 2 {
		t.Errorf("Expected 2 repository groups, got %d", len(groups))
	}
	
	nginxKey := "docker.io/library/nginx"
	if len(groups[nginxKey]) != 2 {
		t.Errorf("Expected 2 nginx images, got %d", len(groups[nginxKey]))
	}
	
	redisKey := "docker.io/library/redis"
	if len(groups[redisKey]) != 1 {
		t.Errorf("Expected 1 redis image, got %d", len(groups[redisKey]))
	}
}

func TestGetImagesWithLatestTag(t *testing.T) {
	tracker := NewTracker()
	
	dockerImages := []models.Image{
		{
			ImageID:    "sha256:abc123",
			Registry:   "docker.io",
			Repository: "library/nginx",
			Tag:        "1.21",
		},
		{
			ImageID:    "sha256:def456",
			Registry:   "docker.io",
			Repository: "library/redis",
			Tag:        "latest",
		},
		{
			ImageID:    "sha256:ghi789",
			Registry:   "docker.io",
			Repository: "library/postgres",
			Tag:        "latest",
		},
	}
	
	tracker.TrackDockerImages(dockerImages)
	
	latestImages := tracker.GetImagesWithLatestTag()
	
	if len(latestImages) != 2 {
		t.Errorf("Expected 2 images with latest tag, got %d", len(latestImages))
	}
	
	for _, img := range latestImages {
		if img.Tag != "latest" {
			t.Errorf("Expected tag 'latest', got '%s'", img.Tag)
		}
	}
}

func TestGetImagesWithoutExplicitTag(t *testing.T) {
	tracker := NewTracker()
	
	dockerImages := []models.Image{
		{
			ImageID:    "sha256:abc123",
			Registry:   "docker.io",
			Repository: "library/nginx",
			Tag:        "1.21",
		},
		{
			ImageID:    "sha256:def456",
			Registry:   "docker.io",
			Repository: "library/redis",
			Tag:        "latest",
		},
		{
			ImageID:    "sha256:ghi789",
			Registry:   "docker.io",
			Repository: "library/postgres",
			Tag:        "",
		},
		{
			ImageID:    "sha256:jkl012",
			Registry:   "docker.io",
			Repository: "library/mongo",
			Tag:        "<none>",
		},
	}
	
	tracker.TrackDockerImages(dockerImages)
	
	noTagImages := tracker.GetImagesWithoutExplicitTag()
	
	if len(noTagImages) != 3 {
		t.Errorf("Expected 3 images without explicit tag, got %d", len(noTagImages))
	}
}

func TestGetImagesByContainer(t *testing.T) {
	tracker := NewTracker()
	
	dockerImages := []models.Image{
		{
			ImageID:          "sha256:abc123",
			Registry:         "docker.io",
			Repository:       "library/nginx",
			Tag:              "1.21",
			UsedByContainers: []string{"container1", "container2"},
		},
	}
	
	tracker.TrackDockerImages(dockerImages)
	
	img := tracker.GetImagesByContainer("container1")
	if img == nil {
		t.Fatal("Image not found for container1")
	}
	
	if img.ImageID != "sha256:abc123" {
		t.Errorf("Expected image sha256:abc123, got %s", img.ImageID)
	}
	
	img2 := tracker.GetImagesByContainer("container999")
	if img2 != nil {
		t.Error("Expected nil for non-existent container")
	}
}

func TestGetImagesByPod(t *testing.T) {
	tracker := NewTracker()
	
	dockerImages := []models.Image{
		{
			ImageID:    "sha256:abc123",
			Registry:   "docker.io",
			Repository: "library/nginx",
			Tag:        "1.21",
			UsedByPods: []string{"pod/default/nginx-pod"},
		},
		{
			ImageID:    "sha256:def456",
			Registry:   "docker.io",
			Repository: "library/redis",
			Tag:        "latest",
			UsedByPods: []string{"pod/default/nginx-pod"},
		},
	}
	
	tracker.TrackDockerImages(dockerImages)
	
	images := tracker.GetImagesByPod("pod/default/nginx-pod")
	if len(images) != 2 {
		t.Errorf("Expected 2 images for pod, got %d", len(images))
	}
	
	images2 := tracker.GetImagesByPod("pod/default/nonexistent")
	if len(images2) != 0 {
		t.Errorf("Expected 0 images for non-existent pod, got %d", len(images2))
	}
}

func TestParseImageReference(t *testing.T) {
	tests := []struct {
		name       string
		imageName  string
		wantReg    string
		wantRepo   string
		wantTag    string
	}{
		{
			name:      "simple library image",
			imageName: "nginx",
			wantReg:   "docker.io",
			wantRepo:  "library/nginx",
			wantTag:   "latest",
		},
		{
			name:      "library image with tag",
			imageName: "nginx:1.21",
			wantReg:   "docker.io",
			wantRepo:  "library/nginx",
			wantTag:   "1.21",
		},
		{
			name:      "user image",
			imageName: "myuser/myapp:v1.0",
			wantReg:   "docker.io",
			wantRepo:  "myuser/myapp",
			wantTag:   "v1.0",
		},
		{
			name:      "custom registry",
			imageName: "gcr.io/myproject/myapp:latest",
			wantReg:   "gcr.io",
			wantRepo:  "myproject/myapp",
			wantTag:   "latest",
		},
		{
			name:      "registry with port",
			imageName: "localhost:5000/myapp:dev",
			wantReg:   "localhost:5000",
			wantRepo:  "myapp",
			wantTag:   "dev",
		},
		{
			name:      "image with digest",
			imageName: "nginx@sha256:abc123",
			wantReg:   "docker.io",
			wantRepo:  "library/nginx",
			wantTag:   "latest",
		},
		{
			name:      "full reference with digest",
			imageName: "gcr.io/myproject/myapp:v1.0@sha256:def456",
			wantReg:   "gcr.io",
			wantRepo:  "myproject/myapp",
			wantTag:   "v1.0",
		},
		{
			name:      "deep repository path",
			imageName: "myregistry.io/team/project/service:v2.1",
			wantReg:   "myregistry.io",
			wantRepo:  "team/project/service",
			wantTag:   "v2.1",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotReg, gotRepo, gotTag := ParseImageReference(tt.imageName)
			
			if gotReg != tt.wantReg {
				t.Errorf("ParseImageReference() registry = %v, want %v", gotReg, tt.wantReg)
			}
			if gotRepo != tt.wantRepo {
				t.Errorf("ParseImageReference() repository = %v, want %v", gotRepo, tt.wantRepo)
			}
			if gotTag != tt.wantTag {
				t.Errorf("ParseImageReference() tag = %v, want %v", gotTag, tt.wantTag)
			}
		})
	}
}

func TestTrackDockerContainers_DuplicatePrevention(t *testing.T) {
	tracker := NewTracker()
	
	dockerImages := []models.Image{
		{
			ImageID:          "sha256:abc123",
			Registry:         "docker.io",
			Repository:       "library/nginx",
			Tag:              "1.21",
			UsedByContainers: []string{},
		},
	}
	tracker.TrackDockerImages(dockerImages)
	
	containers := []models.Container{
		{
			ContainerID: "container1",
			ImageID:     "sha256:abc123",
			Image:       "nginx:1.21",
		},
	}
	
	// Track the same container twice
	tracker.TrackDockerContainers(containers)
	tracker.TrackDockerContainers(containers)
	
	img := tracker.images["sha256:abc123"]
	if len(img.UsedByContainers) != 1 {
		t.Errorf("Expected 1 container (no duplicates), got %d", len(img.UsedByContainers))
	}
}

func TestTrackKubernetesPods_DuplicatePrevention(t *testing.T) {
	tracker := NewTracker()
	
	pods := []models.Pod{
		{
			BaseEntity: models.BaseEntity{
				ID:   "pod/default/nginx-pod",
				Type: models.EntityTypePod,
			},
			Name:      "nginx-pod",
			Namespace: "default",
			Containers: []models.PodContainer{
				{
					Name:    "nginx",
					Image:   "nginx:1.21",
					ImageID: "docker-pullable://nginx@sha256:abc123",
				},
			},
		},
	}
	
	// Track the same pod twice
	tracker.TrackKubernetesPods(pods)
	tracker.TrackKubernetesPods(pods)
	
	images := tracker.GetAllImages()
	if len(images) != 1 {
		t.Errorf("Expected 1 image, got %d", len(images))
	}
	
	img := images[0]
	if len(img.UsedByPods) != 1 {
		t.Errorf("Expected 1 pod (no duplicates), got %d", len(img.UsedByPods))
	}
}

func TestIntegration_DockerAndKubernetes(t *testing.T) {
	tracker := NewTracker()
	
	// Track Docker images
	dockerImages := []models.Image{
		{
			ImageID:    "sha256:abc123",
			Registry:   "docker.io",
			Repository: "library/nginx",
			Tag:        "1.21",
			Created:    time.Now(),
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
	
	// Track Kubernetes pods using the same image
	pods := []models.Pod{
		{
			BaseEntity: models.BaseEntity{
				ID: "pod/default/nginx-pod",
			},
			Containers: []models.PodContainer{
				{
					Name:    "nginx",
					Image:   "nginx:1.21",
					ImageID: "docker-pullable://nginx@sha256:abc123",
				},
			},
		},
	}
	tracker.TrackKubernetesPods(pods)
	
	// Verify the image is tracked across both layers
	images := tracker.GetAllImages()
	if len(images) != 1 {
		t.Fatalf("Expected 1 image, got %d", len(images))
	}
	
	img := images[0]
	if len(img.UsedByContainers) != 1 {
		t.Errorf("Expected 1 container, got %d", len(img.UsedByContainers))
	}
	if len(img.UsedByPods) != 1 {
		t.Errorf("Expected 1 pod, got %d", len(img.UsedByPods))
	}
}
