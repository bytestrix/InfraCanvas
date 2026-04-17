package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"infracanvas/internal/models"
	"infracanvas/internal/redactor"
	"infracanvas/pkg/discovery/docker"
	"infracanvas/pkg/discovery/host"
	"infracanvas/pkg/discovery/kubernetes"
	"infracanvas/pkg/health"
	"infracanvas/pkg/relationships"
)

// Orchestrator coordinates discovery across multiple layers
type Orchestrator struct {
	hostDiscovery       *host.Discovery
	dockerDiscovery     *docker.Discovery
	kubernetesDiscovery *kubernetes.Discovery
	relationshipBuilder *relationships.Builder
	healthCalculator    *health.Calculator
	redactor            *redactor.Redactor
}

// NewOrchestrator creates a new discovery orchestrator
func NewOrchestrator(enableRedaction bool) *Orchestrator {
	return &Orchestrator{
		hostDiscovery:       host.NewDiscovery(),
		relationshipBuilder: relationships.NewBuilder(),
		healthCalculator:    health.NewCalculator(),
		redactor:            redactor.NewRedactor(enableRedaction),
	}
}

// Discover performs discovery across the specified scope
func (o *Orchestrator) Discover(ctx context.Context, scope []string) (*models.InfraSnapshot, error) {
	startTime := time.Now()

	snapshot := &models.InfraSnapshot{
		Timestamp: startTime,
		Entities:  make(map[string]models.Entity),
		Relations: []models.Relation{},
		Metadata: models.SnapshotMetadata{
			Scope:  scope,
			Errors: []models.CollectionError{},
		},
	}

	// Use WaitGroup for parallel execution
	var wg sync.WaitGroup
	var mu sync.Mutex // Protect shared snapshot

	// Discover host layer
	if contains(scope, "host") {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := o.discoverHost(snapshot, &mu); err != nil {
				mu.Lock()
				snapshot.Metadata.Errors = append(snapshot.Metadata.Errors, models.CollectionError{
					Layer:   "host",
					Message: err.Error(),
				})
				mu.Unlock()
			}
		}()
	}

	// Discover Docker layer
	if contains(scope, "docker") {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := o.discoverDocker(snapshot, &mu); err != nil {
				mu.Lock()
				snapshot.Metadata.Errors = append(snapshot.Metadata.Errors, models.CollectionError{
					Layer:   "docker",
					Message: err.Error(),
				})
				mu.Unlock()
			}
		}()
	}

	// Discover Kubernetes layer
	if contains(scope, "kubernetes") {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := o.discoverKubernetes(snapshot, &mu); err != nil {
				mu.Lock()
				snapshot.Metadata.Errors = append(snapshot.Metadata.Errors, models.CollectionError{
					Layer:   "kubernetes",
					Message: err.Error(),
				})
				mu.Unlock()
			}
		}()
	}

	// Wait for all discovery operations to complete
	wg.Wait()

	// Build relationships after all entities are collected
	snapshot.Relations = o.relationshipBuilder.BuildRelationships(snapshot.Entities)

	// Calculate health status for all entities
	o.calculateHealthForAll(snapshot)

	// Apply sensitive data redaction
	o.applyRedaction(snapshot)

	// Track collection duration
	snapshot.Metadata.CollectionDuration = time.Since(startTime)

	// Set host ID from host entity if available
	for _, entity := range snapshot.Entities {
		if entity.GetType() == models.EntityTypeHost {
			if hostEntity, ok := entity.(*models.Host); ok {
				snapshot.HostID = hostEntity.Hostname
				break
			}
		}
	}

	return snapshot, nil
}

// discoverHost performs host-level discovery
func (o *Orchestrator) discoverHost(snapshot *models.InfraSnapshot, mu *sync.Mutex) error {
	hostEntity, processes, services, err := o.hostDiscovery.DiscoverAll()
	if err != nil {
		return fmt.Errorf("host discovery failed: %w", err)
	}

	mu.Lock()
	defer mu.Unlock()

	// Add host entity
	if hostEntity != nil {
		snapshot.Entities[hostEntity.ID] = hostEntity
	}

	// Add processes
	for i := range processes {
		snapshot.Entities[processes[i].ID] = &processes[i]
	}

	// Add services
	for i := range services {
		snapshot.Entities[services[i].ID] = &services[i]
	}

	return nil
}

// discoverDocker performs Docker-level discovery
func (o *Orchestrator) discoverDocker(snapshot *models.InfraSnapshot, mu *sync.Mutex) error {
	// Initialize Docker discovery if not already done
	if o.dockerDiscovery == nil {
		var err error
		o.dockerDiscovery, err = docker.NewDiscovery(o.redactor.IsEnabled())
		if err != nil {
			return fmt.Errorf("failed to initialize Docker discovery: %w", err)
		}
	}

	// Check if Docker is available
	if !o.dockerDiscovery.IsAvailable() {
		return fmt.Errorf("Docker is not available")
	}

	runtime, containers, images, volumes, networks, err := o.dockerDiscovery.DiscoverAll()
	if err != nil {
		return fmt.Errorf("Docker discovery failed: %w", err)
	}

	mu.Lock()
	defer mu.Unlock()

	// Add runtime entity
	if runtime != nil {
		snapshot.Entities[runtime.ID] = runtime
	}

	// Add containers
	for i := range containers {
		snapshot.Entities[containers[i].ID] = &containers[i]
	}

	// Add images
	for i := range images {
		snapshot.Entities[images[i].ID] = &images[i]
	}

	// Add volumes
	for i := range volumes {
		snapshot.Entities[volumes[i].ID] = &volumes[i]
	}

	// Add networks
	for i := range networks {
		snapshot.Entities[networks[i].ID] = &networks[i]
	}

	return nil
}

// discoverKubernetes performs Kubernetes-level discovery
func (o *Orchestrator) discoverKubernetes(snapshot *models.InfraSnapshot, mu *sync.Mutex) error {
	// Initialize Kubernetes discovery if not already done
	if o.kubernetesDiscovery == nil {
		var err error
		o.kubernetesDiscovery, err = kubernetes.NewDiscovery()
		if err != nil {
			return fmt.Errorf("failed to initialize Kubernetes discovery: %w", err)
		}
	}

	// Check if Kubernetes is available
	if !o.kubernetesDiscovery.IsAvailable() {
		return fmt.Errorf("Kubernetes is not available")
	}

	cluster, nodes, namespaces, deployments, statefulsets, daemonsets, jobs, cronjobs, pods, services, ingresses, configmaps, secrets, pvcs, pvs, storageclasses, events, err := o.kubernetesDiscovery.DiscoverAll()
	if err != nil {
		return fmt.Errorf("Kubernetes discovery failed: %w", err)
	}

	mu.Lock()
	defer mu.Unlock()

	// Add cluster entity
	if cluster != nil {
		snapshot.Entities[cluster.ID] = cluster
	}

	// Add nodes
	for i := range nodes {
		snapshot.Entities[nodes[i].ID] = &nodes[i]
	}

	// Add namespaces
	for i := range namespaces {
		snapshot.Entities[namespaces[i].ID] = &namespaces[i]
	}

	// Add deployments
	for i := range deployments {
		snapshot.Entities[deployments[i].ID] = &deployments[i]
	}

	// Add statefulsets
	for i := range statefulsets {
		snapshot.Entities[statefulsets[i].ID] = &statefulsets[i]
	}

	// Add daemonsets
	for i := range daemonsets {
		snapshot.Entities[daemonsets[i].ID] = &daemonsets[i]
	}

	// Add jobs
	for i := range jobs {
		snapshot.Entities[jobs[i].ID] = &jobs[i]
	}

	// Add cronjobs
	for i := range cronjobs {
		snapshot.Entities[cronjobs[i].ID] = &cronjobs[i]
	}

	// Add pods
	for i := range pods {
		snapshot.Entities[pods[i].ID] = &pods[i]
	}

	// Add services
	for i := range services {
		snapshot.Entities[services[i].ID] = &services[i]
	}

	// Add ingresses
	for i := range ingresses {
		snapshot.Entities[ingresses[i].ID] = &ingresses[i]
	}

	// Add configmaps
	for i := range configmaps {
		snapshot.Entities[configmaps[i].ID] = &configmaps[i]
	}

	// Add secrets
	for i := range secrets {
		snapshot.Entities[secrets[i].ID] = &secrets[i]
	}

	// Add PVCs
	for i := range pvcs {
		snapshot.Entities[pvcs[i].ID] = &pvcs[i]
	}

	// Add PVs
	for i := range pvs {
		snapshot.Entities[pvs[i].ID] = &pvs[i]
	}

	// Add storage classes
	for i := range storageclasses {
		snapshot.Entities[storageclasses[i].ID] = &storageclasses[i]
	}

	// Add events
	for i := range events {
		snapshot.Entities[events[i].ID] = &events[i]
	}

	return nil
}

// calculateHealthForAll calculates health status for all entities
func (o *Orchestrator) calculateHealthForAll(snapshot *models.InfraSnapshot) {
	for id, entity := range snapshot.Entities {
		health := o.healthCalculator.CalculateHealth(entity)

		// Update entity health based on type
		switch e := entity.(type) {
		case *models.Host:
			e.Health = health
			snapshot.Entities[id] = e
		case *models.Container:
			e.Health = health
			snapshot.Entities[id] = e
		case *models.Deployment:
			e.Health = health
			snapshot.Entities[id] = e
		case *models.StatefulSet:
			e.Health = health
			snapshot.Entities[id] = e
		case *models.DaemonSet:
			e.Health = health
			snapshot.Entities[id] = e
		case *models.Pod:
			e.Health = health
			snapshot.Entities[id] = e
		case *models.Node:
			e.Health = health
			snapshot.Entities[id] = e
		}
	}
}

// applyRedaction applies sensitive data redaction to the snapshot
func (o *Orchestrator) applyRedaction(snapshot *models.InfraSnapshot) {
	if !o.redactor.IsEnabled() {
		return
	}

	for id, entity := range snapshot.Entities {
		switch e := entity.(type) {
		case *models.Container:
			// Redact environment variables
			e.Environment = o.redactor.RedactEnvVars(e.Environment)
			snapshot.Entities[id] = e

		case *models.Process:
			// Redact command line
			e.CommandLine = o.redactor.RedactCommandLine(e.CommandLine)
			snapshot.Entities[id] = e
		}
	}
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// GetKubernetesDiscovery returns the Kubernetes discovery instance
func (o *Orchestrator) GetKubernetesDiscovery() *kubernetes.Discovery {
	return o.kubernetesDiscovery
}
