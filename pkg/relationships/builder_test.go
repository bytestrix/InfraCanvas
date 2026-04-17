package relationships

import (
	"testing"
	"time"

	"infracanvas/internal/models"
)

func TestBuildDockerRelationships(t *testing.T) {
	builder := NewBuilder()

	// Create test entities
	entities := make(map[string]models.Entity)

	// Create an image
	image := &models.Image{
		BaseEntity: models.BaseEntity{
			ID:        "image/test-image",
			Type:      models.EntityTypeImage,
			Timestamp: time.Now(),
		},
		ImageID:    "sha256:abc123",
		Repository: "nginx",
		Tag:        "latest",
	}
	entities[image.ID] = image

	// Create a volume
	volume := &models.Volume{
		BaseEntity: models.BaseEntity{
			ID:        "volume/test-volume",
			Type:      models.EntityTypeVolume,
			Timestamp: time.Now(),
		},
		Name:   "test-volume",
		Driver: "local",
	}
	entities[volume.ID] = volume

	// Create a network
	network := &models.Network{
		BaseEntity: models.BaseEntity{
			ID:        "network/test-network",
			Type:      models.EntityTypeNetwork,
			Timestamp: time.Now(),
		},
		NetworkID: "net123",
		Name:      "test-network",
		Driver:    "bridge",
	}
	entities[network.ID] = network

	// Create a container
	container := &models.Container{
		BaseEntity: models.BaseEntity{
			ID:        "container/test-container",
			Type:      models.EntityTypeContainer,
			Timestamp: time.Now(),
		},
		ContainerID: "cont123",
		Name:        "test-container",
		Image:       "nginx:latest",
		ImageID:     "sha256:abc123",
		Mounts: []models.Mount{
			{
				Source:      "test-volume",
				Destination: "/data",
				Mode:        "rw",
				Type:        "volume",
			},
		},
		NetworkMode: "test-network",
	}
	entities[container.ID] = container

	// Build relationships
	relations := builder.BuildRelationships(entities)

	// Verify relationships
	if len(relations) == 0 {
		t.Fatal("Expected relationships to be created")
	}

	// Check for USES relation (container -> image)
	foundUses := false
	for _, rel := range relations {
		if rel.Type == models.RelationUses && rel.SourceID == container.ID && rel.TargetID == image.ID {
			foundUses = true
			break
		}
	}
	if !foundUses {
		t.Error("Expected USES relation between container and image")
	}

	// Check for MOUNTS relation (container -> volume)
	foundMounts := false
	for _, rel := range relations {
		if rel.Type == models.RelationMounts && rel.SourceID == container.ID && rel.TargetID == volume.ID {
			foundMounts = true
			break
		}
	}
	if !foundMounts {
		t.Error("Expected MOUNTS relation between container and volume")
	}

	// Check for CONNECTS_TO relation (container -> network)
	foundConnects := false
	for _, rel := range relations {
		if rel.Type == models.RelationConnectsTo && rel.SourceID == container.ID && rel.TargetID == network.ID {
			foundConnects = true
			break
		}
	}
	if !foundConnects {
		t.Error("Expected CONNECTS_TO relation between container and network")
	}
}

func TestBuildKubernetesRelationships(t *testing.T) {
	builder := NewBuilder()

	// Create test entities
	entities := make(map[string]models.Entity)

	// Create a node
	node := &models.Node{
		BaseEntity: models.BaseEntity{
			ID:        "node/test-node",
			Type:      models.EntityTypeNode,
			Timestamp: time.Now(),
		},
		Name:   "test-node",
		Status: "Ready",
	}
	entities[node.ID] = node

	// Create a deployment
	deployment := &models.Deployment{
		BaseEntity: models.BaseEntity{
			ID:        "deployment/default/test-deployment",
			Type:      models.EntityTypeDeployment,
			Timestamp: time.Now(),
		},
		Name:      "test-deployment",
		Namespace: "default",
		Replicas:  3,
	}
	entities[deployment.ID] = deployment

	// Create a ConfigMap
	configMap := &models.ConfigMap{
		BaseEntity: models.BaseEntity{
			ID:        "configmap/default/test-config",
			Type:      models.EntityTypeConfigMap,
			Timestamp: time.Now(),
		},
		Name:      "test-config",
		Namespace: "default",
	}
	entities[configMap.ID] = configMap

	// Create a PVC
	pvc := &models.PersistentVolumeClaim{
		BaseEntity: models.BaseEntity{
			ID:        "pvc/default/test-pvc",
			Type:      models.EntityTypePVC,
			Timestamp: time.Now(),
		},
		Name:       "test-pvc",
		Namespace:  "default",
		VolumeName: "test-pv",
	}
	entities[pvc.ID] = pvc

	// Create a PV
	pv := &models.PersistentVolume{
		BaseEntity: models.BaseEntity{
			ID:        "pv/test-pv",
			Type:      models.EntityTypePV,
			Timestamp: time.Now(),
		},
		Name:         "test-pv",
		StorageClass: "standard",
	}
	entities[pv.ID] = pv

	// Create a StorageClass
	sc := &models.StorageClass{
		BaseEntity: models.BaseEntity{
			ID:        "storageclass/standard",
			Type:      models.EntityTypeStorageClass,
			Timestamp: time.Now(),
		},
		Name:        "standard",
		Provisioner: "kubernetes.io/gce-pd",
	}
	entities[sc.ID] = sc

	// Create a pod
	pod := &models.Pod{
		BaseEntity: models.BaseEntity{
			ID:        "pod/default/test-pod",
			Type:      models.EntityTypePod,
			Labels:    map[string]string{"app": "test"},
			Timestamp: time.Now(),
		},
		Name:      "test-pod",
		Namespace: "default",
		NodeName:  "test-node",
		OwnerKind: "Deployment",
		OwnerName: "test-deployment",
		VolumeRefs: models.PodVolumeRefs{
			ConfigMaps: []string{"test-config"},
			PVCs:       []string{"test-pvc"},
		},
	}
	entities[pod.ID] = pod

	// Create a service
	service := &models.K8sService{
		BaseEntity: models.BaseEntity{
			ID:        "service/default/test-service",
			Type:      models.EntityTypeK8sService,
			Timestamp: time.Now(),
		},
		Name:      "test-service",
		Namespace: "default",
		Selector:  map[string]string{"app": "test"},
	}
	entities[service.ID] = service

	// Build relationships
	relations := builder.BuildRelationships(entities)

	// Verify relationships
	if len(relations) == 0 {
		t.Fatal("Expected relationships to be created")
	}

	// Check for RUNS_ON relation (pod -> node)
	foundRunsOn := false
	for _, rel := range relations {
		if rel.Type == models.RelationRunsOn && rel.SourceID == pod.ID && rel.TargetID == node.ID {
			foundRunsOn = true
			break
		}
	}
	if !foundRunsOn {
		t.Error("Expected RUNS_ON relation between pod and node")
	}

	// Check for OWNS relation (deployment -> pod)
	foundOwns := false
	for _, rel := range relations {
		if rel.Type == models.RelationOwns && rel.SourceID == deployment.ID && rel.TargetID == pod.ID {
			foundOwns = true
			break
		}
	}
	if !foundOwns {
		t.Error("Expected OWNS relation between deployment and pod")
	}

	// Check for REFERENCES relation (pod -> configmap)
	foundReferences := false
	for _, rel := range relations {
		if rel.Type == models.RelationReferences && rel.SourceID == pod.ID && rel.TargetID == configMap.ID {
			foundReferences = true
			break
		}
	}
	if !foundReferences {
		t.Error("Expected REFERENCES relation between pod and configmap")
	}

	// Check for MOUNTS relation (pod -> pvc)
	foundMounts := false
	for _, rel := range relations {
		if rel.Type == models.RelationMounts && rel.SourceID == pod.ID && rel.TargetID == pvc.ID {
			foundMounts = true
			break
		}
	}
	if !foundMounts {
		t.Error("Expected MOUNTS relation between pod and pvc")
	}

	// Check for TARGETS relation (service -> pod)
	foundTargets := false
	for _, rel := range relations {
		if rel.Type == models.RelationTargets && rel.SourceID == service.ID && rel.TargetID == pod.ID {
			foundTargets = true
			break
		}
	}
	if !foundTargets {
		t.Error("Expected TARGETS relation between service and pod")
	}

	// Check for BINDS_TO relation (pvc -> pv)
	foundBindsTo := false
	for _, rel := range relations {
		if rel.Type == models.RelationBindsTo && rel.SourceID == pvc.ID && rel.TargetID == pv.ID {
			foundBindsTo = true
			break
		}
	}
	if !foundBindsTo {
		t.Error("Expected BINDS_TO relation between pvc and pv")
	}

	// Check for PROVISIONS relation (storageclass -> pv)
	foundProvisions := false
	for _, rel := range relations {
		if rel.Type == models.RelationProvisions && rel.SourceID == sc.ID && rel.TargetID == pv.ID {
			foundProvisions = true
			break
		}
	}
	if !foundProvisions {
		t.Error("Expected PROVISIONS relation between storageclass and pv")
	}
}

func TestGetDependencyGraph(t *testing.T) {
	builder := NewBuilder()

	// Create test relations
	relations := []models.Relation{
		{
			SourceID: "pod1",
			TargetID: "node1",
			Type:     models.RelationRunsOn,
		},
		{
			SourceID: "deployment1",
			TargetID: "pod1",
			Type:     models.RelationOwns,
		},
		{
			SourceID: "service1",
			TargetID: "pod1",
			Type:     models.RelationTargets,
		},
	}

	// Build dependency graph from pod1
	graph := builder.GetDependencyGraph("pod1", relations)

	if graph.Root != "pod1" {
		t.Errorf("Expected root to be pod1, got %s", graph.Root)
	}

	// Check dependencies (pod1 depends on node1)
	if deps, ok := graph.Dependencies["pod1"]; !ok || len(deps) == 0 {
		t.Error("Expected pod1 to have dependencies")
	}

	// Check dependents (pod1 is depended on by deployment1 and service1)
	// Note: The graph builder creates bidirectional relationships, so we check if at least 2 dependents exist
	if deps, ok := graph.Dependents["pod1"]; !ok || len(deps) < 2 {
		t.Errorf("Expected pod1 to have at least 2 dependents, got %d", len(deps))
	}
}

func TestExportGraph(t *testing.T) {
	graph := &DependencyGraph{
		Root: "pod1",
		Dependencies: map[string][]string{
			"pod1":        {"node1", "pvc1"},
			"deployment1": {"pod1"},
		},
		Dependents: map[string][]string{
			"pod1":  {"deployment1"},
			"node1": {"pod1"},
			"pvc1":  {"pod1"},
		},
	}

	export := graph.ExportGraph()

	if len(export.Nodes) == 0 {
		t.Error("Expected nodes to be exported")
	}

	if len(export.Edges) == 0 {
		t.Error("Expected edges to be exported")
	}

	// Verify edges match dependencies
	expectedEdges := 2 // pod1->node1, pod1->pvc1, deployment1->pod1
	if len(export.Edges) < expectedEdges {
		t.Errorf("Expected at least %d edges, got %d", expectedEdges, len(export.Edges))
	}
}

func TestExportRelationshipGraph(t *testing.T) {
	entities := make(map[string]models.Entity)
	
	// Add some test entities
	pod := &models.Pod{
		BaseEntity: models.BaseEntity{
			ID:   "pod1",
			Type: models.EntityTypePod,
		},
	}
	entities["pod1"] = pod

	node := &models.Node{
		BaseEntity: models.BaseEntity{
			ID:   "node1",
			Type: models.EntityTypeNode,
		},
	}
	entities["node1"] = node

	relations := []models.Relation{
		{
			SourceID: "pod1",
			TargetID: "node1",
			Type:     models.RelationRunsOn,
		},
	}

	export := ExportRelationshipGraph(entities, relations)

	if len(export.Nodes) != 2 {
		t.Errorf("Expected 2 nodes, got %d", len(export.Nodes))
	}

	if len(export.Edges) != 1 {
		t.Errorf("Expected 1 edge, got %d", len(export.Edges))
	}

	if export.Edges[0].Source != "pod1" || export.Edges[0].Target != "node1" {
		t.Error("Edge source/target mismatch")
	}
}
