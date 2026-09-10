package docker

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/docker/docker/client"
	"infracanvas/internal/models"
	"infracanvas/internal/redactor"
	"infracanvas/pkg/dockerhost"
)

// Discovery implements Docker-level infrastructure discovery. Despite the
// package name, it works against any Docker-Engine-API-compatible socket —
// Podman's "podman system service" included — since it's the same client.Client
// from the Docker SDK either way; see pkg/dockerhost for how the socket is found.
type Discovery struct {
	client       *client.Client
	redactor     *redactor.Redactor
	resolvedHost string // "" when using DOCKER_HOST or the SDK's own default
}

// NewDiscovery creates a new Docker discovery instance
func NewDiscovery(enableRedaction bool) (*Discovery, error) {
	// Support DOCKER_HOST environment variable
	dockerHost := os.Getenv("DOCKER_HOST")
	if dockerHost == "" {
		// No explicit host and no standard Docker socket present — try
		// Podman's Docker-API-compatible socket before giving up.
		dockerHost = dockerhost.Resolve()
	}

	var cli *client.Client
	var err error

	if dockerHost != "" {
		cli, err = client.NewClientWithOpts(
			client.WithHost(dockerHost),
			client.WithAPIVersionNegotiation(),
		)
	} else {
		// Use default socket path
		cli, err = client.NewClientWithOpts(
			client.FromEnv,
			client.WithAPIVersionNegotiation(),
		)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	return &Discovery{
		client:       cli,
		redactor:     redactor.NewRedactor(enableRedaction),
		resolvedHost: dockerHost,
	}, nil
}

// IsAvailable checks if Docker is available and accessible
func (d *Discovery) IsAvailable() bool {
	if d.client == nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Try to ping Docker daemon
	_, err := d.client.Ping(ctx)
	return err == nil
}

// Close closes the Docker client connection
func (d *Discovery) Close() error {
	if d.client != nil {
		return d.client.Close()
	}
	return nil
}

// DiscoverAll performs a complete Docker discovery
func (d *Discovery) DiscoverAll() (*models.ContainerRuntime, []models.Container, []models.Image, []models.Volume, []models.Network, error) {
	if !d.IsAvailable() {
		return nil, nil, nil, nil, nil, fmt.Errorf("Docker is not available")
	}

	ctx := context.Background()

	// Get runtime info (required first)
	runtime, err := d.GetRuntimeInfo(ctx)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("failed to get runtime info: %w", err)
	}

	// Use goroutines for parallel collection
	var wg sync.WaitGroup
	var containers []models.Container
	var images []models.Image
	var volumes []models.Volume
	var networks []models.Network
	var containerErr, imageErr, volumeErr, networkErr error

	// Get containers
	wg.Add(1)
	go func() {
		defer wg.Done()
		containers, containerErr = d.GetContainers(ctx)
	}()

	// Get images
	wg.Add(1)
	go func() {
		defer wg.Done()
		images, imageErr = d.GetImages(ctx)
	}()

	// Get volumes
	wg.Add(1)
	go func() {
		defer wg.Done()
		volumes, volumeErr = d.GetVolumes(ctx)
	}()

	// Get networks
	wg.Add(1)
	go func() {
		defer wg.Done()
		networks, networkErr = d.GetNetworks(ctx)
	}()

	// Wait for all parallel operations to complete
	wg.Wait()

	// Check for errors
	if containerErr != nil {
		return runtime, nil, nil, nil, nil, fmt.Errorf("failed to get containers: %w", containerErr)
	}
	if imageErr != nil {
		return runtime, containers, nil, nil, nil, fmt.Errorf("failed to get images: %w", imageErr)
	}
	if volumeErr != nil {
		return runtime, containers, images, nil, nil, fmt.Errorf("failed to get volumes: %w", volumeErr)
	}
	if networkErr != nil {
		return runtime, containers, images, volumes, nil, fmt.Errorf("failed to get networks: %w", networkErr)
	}

	return runtime, containers, images, volumes, networks, nil
}
