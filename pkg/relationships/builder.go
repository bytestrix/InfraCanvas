package relationships

import (
	"strings"

	"infracanvas/internal/models"
)

// Builder implements relationship mapping between infrastructure entities
type Builder struct{}

// NewBuilder creates a new relationship builder
func NewBuilder() *Builder {
	return &Builder{}
}

// BuildRelationships processes all entities and creates relationships between them
func (b *Builder) BuildRelationships(entities map[string]models.Entity) []models.Relation {
	raw := []models.Relation{}

	// Build Docker relationships
	raw = append(raw, b.buildDockerRelationships(entities)...)

	// Build Kubernetes relationships
	raw = append(raw, b.buildKubernetesRelationships(entities)...)

	// Deduplicate: same source+target+type is only emitted once
	seen := make(map[string]bool, len(raw))
	relations := make([]models.Relation, 0, len(raw))
	for _, r := range raw {
		key := r.SourceID + "→" + r.TargetID + "→" + string(r.Type)
		if !seen[key] {
			seen[key] = true
			relations = append(relations, r)
		}
	}

	return relations
}

// buildDockerRelationships creates relationships for Docker entities
func (b *Builder) buildDockerRelationships(entities map[string]models.Entity) []models.Relation {
	relations := []models.Relation{}

	// Map containers to images, volumes, and networks
	for _, entity := range entities {
		if entity.GetType() == models.EntityTypeContainer {
			container, ok := entity.(*models.Container)
			if !ok {
				continue
			}

			// Container -> Image (USES relation)
			if container.ImageID != "" {
				imageID := b.findImageByID(entities, container.ImageID)
				if imageID != "" {
					relations = append(relations, models.Relation{
						SourceID: container.ID,
						TargetID: imageID,
						Type:     models.RelationUses,
						Properties: map[string]string{
							"image": container.Image,
						},
					})
				}
			}

			// Container -> Volume (MOUNTS relation)
			for _, mount := range container.Mounts {
				if mount.Type == "volume" {
					volumeID := b.findVolumeByName(entities, mount.Source)
					if volumeID != "" {
						relations = append(relations, models.Relation{
							SourceID: container.ID,
							TargetID: volumeID,
							Type:     models.RelationMounts,
							Properties: map[string]string{
								"destination": mount.Destination,
								"mode":        mount.Mode,
							},
						})
					}
				}
			}

			// Container -> Network (CONNECTS_TO relation)
			// Prefer the full Networks list; fall back to NetworkMode when it's absent.
			networks := container.Networks
			if len(networks) == 0 && container.NetworkMode != "" {
				networks = []string{container.NetworkMode}
			}
			for _, networkName := range networks {
				networkID := b.findNetworkByName(entities, networkName)
				if networkID != "" {
					relations = append(relations, models.Relation{
						SourceID: container.ID,
						TargetID: networkID,
						Type:     models.RelationConnectsTo,
					})
				}
			}
		}
	}

	// Container -> Host (RUNS_ON relation)
	hostID := b.findHostEntity(entities)
	if hostID != "" {
		for _, entity := range entities {
			if entity.GetType() == models.EntityTypeContainer {
				relations = append(relations, models.Relation{
					SourceID: entity.GetID(),
					TargetID: hostID,
					Type:     models.RelationRunsOn,
				})
			}
		}

		// DockerRuntime -> Host (RUNS_ON relation)
		for _, entity := range entities {
			if entity.GetType() == models.EntityTypeContainerRuntime {
				relations = append(relations, models.Relation{
					SourceID: entity.GetID(),
					TargetID: hostID,
					Type:     models.RelationRunsOn,
				})
			}
		}
	}

	// ContainerRuntime -> named Image (CONTAINS relation)
	// Ensures images are visible on the canvas even when no containers are running.
	runtimeID := ""
	for id, entity := range entities {
		if entity.GetType() == models.EntityTypeContainerRuntime {
			runtimeID = id
			break
		}
	}
	if runtimeID != "" {
		for _, entity := range entities {
			if entity.GetType() == models.EntityTypeImage {
				if img, ok := entity.(*models.Image); ok {
					if img.Repository != "<none>" && img.Tag != "<none>" {
						relations = append(relations, models.Relation{
							SourceID: runtimeID,
							TargetID: entity.GetID(),
							Type:     models.RelationContains,
						})
					}
				}
			}
		}
		for _, entity := range entities {
			switch entity.GetType() {
			case models.EntityTypeVolume, models.EntityTypeNetwork:
				relations = append(relations, models.Relation{
					SourceID: runtimeID,
					TargetID: entity.GetID(),
					Type:     models.RelationContains,
				})
			}
		}
	}

	return relations
}

// buildKubernetesRelationships creates relationships for Kubernetes entities
func (b *Builder) buildKubernetesRelationships(entities map[string]models.Entity) []models.Relation {
	relations := []models.Relation{}

	// Process all pods to create relationships
	for _, entity := range entities {
		if entity.GetType() == models.EntityTypePod {
			pod, ok := entity.(*models.Pod)
			if !ok {
				continue
			}

			// Pod -> Node (RUNS_ON relation)
			if pod.NodeName != "" {
				nodeID := b.findNodeByName(entities, pod.NodeName)
				if nodeID != "" {
					relations = append(relations, models.Relation{
						SourceID: pod.ID,
						TargetID: nodeID,
						Type:     models.RelationRunsOn,
					})
				}
			}

			// Pod -> Workload (OWNS relation via owner references)
			if pod.OwnerKind != "" && pod.OwnerName != "" {
				ownerID := b.findWorkloadByNameAndKind(entities, pod.Namespace, pod.OwnerName, pod.OwnerKind)
				if ownerID != "" {
					relations = append(relations, models.Relation{
						SourceID: ownerID,
						TargetID: pod.ID,
						Type:     models.RelationOwns,
						Properties: map[string]string{
							"owner_kind": pod.OwnerKind,
						},
					})
				} else if strings.ToLower(pod.OwnerKind) == "replicaset" {
					// Pods owned by ReplicaSets belong to a Deployment.
					// ReplicaSet name format: {deployment-name}-{pod-template-hash}
					if hash, ok := pod.GetLabels()["pod-template-hash"]; ok && strings.HasSuffix(pod.OwnerName, "-"+hash) {
						deploymentName := strings.TrimSuffix(pod.OwnerName, "-"+hash)
						deploymentID := b.findWorkloadByNameAndKind(entities, pod.Namespace, deploymentName, "Deployment")
						if deploymentID != "" {
							relations = append(relations, models.Relation{
								SourceID: deploymentID,
								TargetID: pod.ID,
								Type:     models.RelationOwns,
								Properties: map[string]string{
									"owner_kind": "Deployment",
								},
							})
						}
					}
				}
			}

			// Pod -> ConfigMap (REFERENCES relation)
			for _, configMapName := range pod.VolumeRefs.ConfigMaps {
				configMapID := b.findConfigMapByName(entities, pod.Namespace, configMapName)
				if configMapID != "" {
					relations = append(relations, models.Relation{
						SourceID: pod.ID,
						TargetID: configMapID,
						Type:     models.RelationReferences,
					})
				}
			}

			// Pod -> Secret (REFERENCES relation)
			for _, secretName := range pod.VolumeRefs.Secrets {
				secretID := b.findSecretByName(entities, pod.Namespace, secretName)
				if secretID != "" {
					relations = append(relations, models.Relation{
						SourceID: pod.ID,
						TargetID: secretID,
						Type:     models.RelationReferences,
					})
				}
			}

			// Pod -> PVC (MOUNTS relation)
			for _, pvcName := range pod.VolumeRefs.PVCs {
				pvcID := b.findPVCByName(entities, pod.Namespace, pvcName)
				if pvcID != "" {
					relations = append(relations, models.Relation{
						SourceID: pod.ID,
						TargetID: pvcID,
						Type:     models.RelationMounts,
					})
				}
			}
		}
	}

	// Pod -> Docker Image (USES relation via container image name matching)
	// Builds two indexes: exact repo:tag and normalized (registry-stripped) repo:tag
	imageIndex := make(map[string]string)       // exact "repo:tag" -> entity ID
	imageIndexNorm := make(map[string]string)   // normalized (no registry prefix) -> entity ID
	for id, entity := range entities {
		if entity.GetType() == models.EntityTypeImage {
			if img, ok := entity.(*models.Image); ok && img.Repository != "<none>" && img.Tag != "<none>" {
				key := img.Repository + ":" + img.Tag
				imageIndex[key] = id
				imageIndexNorm[normalizeImageName(key)] = id
			}
		}
	}
	if len(imageIndex) > 0 {
		for _, entity := range entities {
			if entity.GetType() == models.EntityTypePod {
				pod, ok := entity.(*models.Pod)
				if !ok {
					continue
				}
				for _, c := range pod.Containers {
					if c.Image == "" {
						continue
					}
					imageID := imageIndex[c.Image]
					if imageID == "" {
						imageID = imageIndexNorm[normalizeImageName(c.Image)]
					}
					if imageID != "" {
						relations = append(relations, models.Relation{
							SourceID: pod.ID,
							TargetID: imageID,
							Type:     models.RelationUses,
							Properties: map[string]string{
								"container": c.Name,
							},
						})
					}
				}
			}
		}
	}

	// Service -> Pod (TARGETS relation via label selectors)
	for _, entity := range entities {
		if entity.GetType() == models.EntityTypeK8sService {
			service, ok := entity.(*models.K8sService)
			if !ok {
				continue
			}

			// Find pods that match the service selector
			matchingPods := b.findPodsByLabels(entities, service.Namespace, service.Selector)
			for _, podID := range matchingPods {
				relations = append(relations, models.Relation{
					SourceID: service.ID,
					TargetID: podID,
					Type:     models.RelationTargets,
				})
			}
		}
	}

	// Ingress -> Service (ROUTES_TO relation)
	for _, entity := range entities {
		if entity.GetType() == models.EntityTypeIngress {
			ingress, ok := entity.(*models.Ingress)
			if !ok {
				continue
			}

			// Extract service names from ingress rules
			for _, rule := range ingress.Rules {
				for _, path := range rule.Paths {
					serviceID := b.findServiceByName(entities, ingress.Namespace, path.ServiceName)
					if serviceID != "" {
						relations = append(relations, models.Relation{
							SourceID: ingress.ID,
							TargetID: serviceID,
							Type:     models.RelationRoutesTo,
							Properties: map[string]string{
								"host": rule.Host,
								"path": path.Path,
							},
						})
					}
				}
			}
		}
	}

	// PVC -> PV (BINDS_TO relation)
	for _, entity := range entities {
		if entity.GetType() == models.EntityTypePVC {
			pvc, ok := entity.(*models.PersistentVolumeClaim)
			if !ok {
				continue
			}

			if pvc.VolumeName != "" {
				pvID := b.findPVByName(entities, pvc.VolumeName)
				if pvID != "" {
					relations = append(relations, models.Relation{
						SourceID: pvc.ID,
						TargetID: pvID,
						Type:     models.RelationBindsTo,
					})
				}
			}
		}
	}

	// PV -> StorageClass (PROVISIONS relation)
	for _, entity := range entities {
		if entity.GetType() == models.EntityTypePV {
			pv, ok := entity.(*models.PersistentVolume)
			if !ok {
				continue
			}

			if pv.StorageClass != "" {
				scID := b.findStorageClassByName(entities, pv.StorageClass)
				if scID != "" {
					relations = append(relations, models.Relation{
						SourceID: scID,
						TargetID: pv.ID,
						Type:     models.RelationProvisions,
					})
				}
			}
		}
	}

	// Cluster -> Node (CONTAINS relation)
	clusterID := ""
	for id, entity := range entities {
		if entity.GetType() == models.EntityTypeCluster {
			clusterID = id
			break
		}
	}
	if clusterID != "" {
		for _, entity := range entities {
			if entity.GetType() == models.EntityTypeNode {
				relations = append(relations, models.Relation{
					SourceID: clusterID,
					TargetID: entity.GetID(),
					Type:     models.RelationContains,
				})
			}
		}
	}

	// CronJob -> Job (OWNS relation via job owner references)
	for _, entity := range entities {
		if entity.GetType() == models.EntityTypeJob {
			job, ok := entity.(*models.Job)
			if !ok {
				continue
			}
			if strings.ToLower(job.OwnerKind) == "cronjob" && job.OwnerName != "" {
				cronJobID := b.findWorkloadByNameAndKind(entities, job.Namespace, job.OwnerName, "CronJob")
				if cronJobID != "" {
					relations = append(relations, models.Relation{
						SourceID: cronJobID,
						TargetID: job.ID,
						Type:     models.RelationOwns,
						Properties: map[string]string{
							"owner_kind": "CronJob",
						},
					})
				}
			}
		}
	}

	// Namespace -> namespaced resources (CONTAINS relation)
	// Build a map of namespace name -> namespace entity ID for fast lookup
	nsIDByName := map[string]string{}
	for id, entity := range entities {
		if entity.GetType() == models.EntityTypeNamespace {
			if ns, ok := entity.(*models.Namespace); ok {
				nsIDByName[ns.Name] = id
			}
		}
	}
	if len(nsIDByName) > 0 {
		for _, entity := range entities {
			ns := b.getNamespace(entity)
			if ns == "" {
				continue
			}
			nsID, ok := nsIDByName[ns]
			if !ok {
				continue
			}
			// Only wire high-level workload types — skip pods (already owned by workloads),
			// secrets, configmaps, events (too noisy at the namespace level)
			switch entity.GetType() {
			case models.EntityTypeDeployment,
				models.EntityTypeStatefulSet,
				models.EntityTypeDaemonSet,
				models.EntityTypeCronJob,
				models.EntityTypeJob,
				models.EntityTypeK8sService,
				models.EntityTypeIngress,
				models.EntityTypePVC:
				relations = append(relations, models.Relation{
					SourceID: nsID,
					TargetID: entity.GetID(),
					Type:     models.RelationContains,
				})
			}
		}
	}

	// Event -> target resource (RELATES_TO relation)
	for _, entity := range entities {
		if entity.GetType() == models.EntityTypeEvent {
			event, ok := entity.(*models.Event)
			if !ok {
				continue
			}
			targetID := b.findEventTarget(entities, event)
			if targetID != "" {
				relations = append(relations, models.Relation{
					SourceID: entity.GetID(),
					TargetID: targetID,
					Type:     models.RelationRelatesTo,
					Properties: map[string]string{
						"reason": event.Reason,
					},
				})
			}
		}
	}

	return relations
}

// Helper functions to find entities by various criteria

func (b *Builder) findHostEntity(entities map[string]models.Entity) string {
	for id, entity := range entities {
		if entity.GetType() == models.EntityTypeHost {
			return id
		}
	}
	return ""
}

func (b *Builder) findImageByID(entities map[string]models.Entity, imageID string) string {
	// Clean the imageID - Docker returns it with sha256: prefix
	cleanImageID := strings.TrimPrefix(imageID, "sha256:")
	
	for id, entity := range entities {
		if entity.GetType() == models.EntityTypeImage {
			if image, ok := entity.(*models.Image); ok {
				// Try exact match first
				if image.ImageID == imageID || image.ImageID == cleanImageID {
					return id
				}
				// Also try matching without sha256: prefix
				cleanEntityImageID := strings.TrimPrefix(image.ImageID, "sha256:")
				if cleanEntityImageID == cleanImageID {
					return id
				}
			}
		}
	}
	return ""
}

func (b *Builder) findVolumeByName(entities map[string]models.Entity, volumeName string) string {
	for id, entity := range entities {
		if entity.GetType() == models.EntityTypeVolume {
			if volume, ok := entity.(*models.Volume); ok {
				if volume.Name == volumeName {
					return id
				}
			}
		}
	}
	return ""
}

func (b *Builder) findNetworkByName(entities map[string]models.Entity, networkName string) string {
	for id, entity := range entities {
		if entity.GetType() == models.EntityTypeNetwork {
			if network, ok := entity.(*models.Network); ok {
				if network.Name == networkName {
					return id
				}
			}
		}
	}
	return ""
}

func (b *Builder) findNodeByName(entities map[string]models.Entity, nodeName string) string {
	for id, entity := range entities {
		if entity.GetType() == models.EntityTypeNode {
			if node, ok := entity.(*models.Node); ok {
				if node.Name == nodeName {
					return id
				}
			}
		}
	}
	return ""
}

func (b *Builder) findWorkloadByNameAndKind(entities map[string]models.Entity, namespace, name, kind string) string {
	var targetType models.EntityType

	switch strings.ToLower(kind) {
	case "deployment":
		targetType = models.EntityTypeDeployment
	case "statefulset":
		targetType = models.EntityTypeStatefulSet
	case "daemonset":
		targetType = models.EntityTypeDaemonSet
	case "job":
		targetType = models.EntityTypeJob
	case "cronjob":
		targetType = models.EntityTypeCronJob
	case "replicaset":
		// ReplicaSet is not in our entity types yet, but pods can be owned by them
		return ""
	default:
		return ""
	}

	for id, entity := range entities {
		if entity.GetType() == targetType {
			switch targetType {
			case models.EntityTypeDeployment:
				if deployment, ok := entity.(*models.Deployment); ok {
					if deployment.Name == name && deployment.Namespace == namespace {
						return id
					}
				}
			case models.EntityTypeStatefulSet:
				if statefulset, ok := entity.(*models.StatefulSet); ok {
					if statefulset.Name == name && statefulset.Namespace == namespace {
						return id
					}
				}
			case models.EntityTypeDaemonSet:
				if daemonset, ok := entity.(*models.DaemonSet); ok {
					if daemonset.Name == name && daemonset.Namespace == namespace {
						return id
					}
				}
			case models.EntityTypeJob:
				if job, ok := entity.(*models.Job); ok {
					if job.Name == name && job.Namespace == namespace {
						return id
					}
				}
			case models.EntityTypeCronJob:
				if cronjob, ok := entity.(*models.CronJob); ok {
					if cronjob.Name == name && cronjob.Namespace == namespace {
						return id
					}
				}
			}
		}
	}
	return ""
}

func (b *Builder) findPodsByLabels(entities map[string]models.Entity, namespace string, selector map[string]string) []string {
	var matchingPods []string

	if len(selector) == 0 {
		return matchingPods
	}

	for id, entity := range entities {
		if entity.GetType() == models.EntityTypePod {
			if pod, ok := entity.(*models.Pod); ok {
				if pod.Namespace != namespace {
					continue
				}

				// Check if pod labels match all selector labels
				matches := true
				podLabels := entity.GetLabels()
				for key, value := range selector {
					if podLabels[key] != value {
						matches = false
						break
					}
				}

				if matches {
					matchingPods = append(matchingPods, id)
				}
			}
		}
	}

	return matchingPods
}

func (b *Builder) findServiceByName(entities map[string]models.Entity, namespace, name string) string {
	for id, entity := range entities {
		if entity.GetType() == models.EntityTypeK8sService {
			if service, ok := entity.(*models.K8sService); ok {
				if service.Name == name && service.Namespace == namespace {
					return id
				}
			}
		}
	}
	return ""
}

func (b *Builder) findPVByName(entities map[string]models.Entity, name string) string {
	for id, entity := range entities {
		if entity.GetType() == models.EntityTypePV {
			if pv, ok := entity.(*models.PersistentVolume); ok {
				if pv.Name == name {
					return id
				}
			}
		}
	}
	return ""
}

func (b *Builder) findStorageClassByName(entities map[string]models.Entity, name string) string {
	for id, entity := range entities {
		if entity.GetType() == models.EntityTypeStorageClass {
			if sc, ok := entity.(*models.StorageClass); ok {
				if sc.Name == name {
					return id
				}
			}
		}
	}
	return ""
}

func (b *Builder) findConfigMapByName(entities map[string]models.Entity, namespace, name string) string {
	for id, entity := range entities {
		if entity.GetType() == models.EntityTypeConfigMap {
			if cm, ok := entity.(*models.ConfigMap); ok {
				if cm.Name == name && cm.Namespace == namespace {
					return id
				}
			}
		}
	}
	return ""
}

func (b *Builder) findSecretByName(entities map[string]models.Entity, namespace, name string) string {
	for id, entity := range entities {
		if entity.GetType() == models.EntityTypeSecret {
			if secret, ok := entity.(*models.Secret); ok {
				if secret.Name == name && secret.Namespace == namespace {
					return id
				}
			}
		}
	}
	return ""
}

func (b *Builder) findPVCByName(entities map[string]models.Entity, namespace, name string) string {
	for id, entity := range entities {
		if entity.GetType() == models.EntityTypePVC {
			if pvc, ok := entity.(*models.PersistentVolumeClaim); ok {
				if pvc.Name == name && pvc.Namespace == namespace {
					return id
				}
			}
		}
	}
	return ""
}

// normalizeImageName strips registry prefixes and digest suffixes for fuzzy matching.
// "ghcr.io/cazelabs/foo:v1" -> "cazelabs/foo:v1"
// "docker.io/bitnami/foo:1.2" -> "bitnami/foo:1.2"
// "library/python:3.12-slim" -> "python:3.12-slim"
// "foo@sha256:abc123" -> "foo"
func normalizeImageName(image string) string {
	// Strip digest
	if idx := strings.Index(image, "@"); idx != -1 {
		image = image[:idx]
	}
	// Strip known registry prefixes
	for _, reg := range []string{"docker.io/", "ghcr.io/", "quay.io/", "registry.k8s.io/", "gcr.io/", "mcr.microsoft.com/"} {
		if strings.HasPrefix(image, reg) {
			image = image[len(reg):]
			break
		}
	}
	// Strip "library/" prefix for official Docker Hub images
	image = strings.TrimPrefix(image, "library/")
	return image
}

// getNamespace returns the namespace of a namespaced K8s entity, or "" if not applicable.
func (b *Builder) getNamespace(entity models.Entity) string {
	switch e := entity.(type) {
	case *models.Deployment:
		return e.Namespace
	case *models.StatefulSet:
		return e.Namespace
	case *models.DaemonSet:
		return e.Namespace
	case *models.Job:
		return e.Namespace
	case *models.CronJob:
		return e.Namespace
	case *models.K8sService:
		return e.Namespace
	case *models.Ingress:
		return e.Namespace
	case *models.PersistentVolumeClaim:
		return e.Namespace
	}
	return ""
}

// findEventTarget resolves an event's InvolvedObject to its entity ID.
func (b *Builder) findEventTarget(entities map[string]models.Entity, event *models.Event) string {
	kind := strings.ToLower(event.ObjectKind)
	ns := event.ObjectNamespace
	name := event.ObjectName

	switch kind {
	case "pod":
		id := "pod/" + ns + "/" + name
		if _, ok := entities[id]; ok {
			return id
		}
	case "deployment":
		id := "deployment/" + ns + "/" + name
		if _, ok := entities[id]; ok {
			return id
		}
	case "statefulset":
		id := "statefulset/" + ns + "/" + name
		if _, ok := entities[id]; ok {
			return id
		}
	case "daemonset":
		id := "daemonset/" + ns + "/" + name
		if _, ok := entities[id]; ok {
			return id
		}
	case "job":
		id := "job/" + ns + "/" + name
		if _, ok := entities[id]; ok {
			return id
		}
	case "cronjob":
		id := "cronjob/" + ns + "/" + name
		if _, ok := entities[id]; ok {
			return id
		}
	case "service":
		id := "service/" + ns + "/" + name
		if _, ok := entities[id]; ok {
			return id
		}
	case "persistentvolumeclaim":
		id := "pvc/" + ns + "/" + name
		if _, ok := entities[id]; ok {
			return id
		}
	case "node":
		id := "node/" + name
		if _, ok := entities[id]; ok {
			return id
		}
	}
	return ""
}

// FindRelated finds all entities related to the given entity by the specified relation type
func (b *Builder) FindRelated(entityID string, relationType models.RelationType, relations []models.Relation) []string {
	var related []string

	for _, relation := range relations {
		if relation.SourceID == entityID && relation.Type == relationType {
			related = append(related, relation.TargetID)
		}
	}

	return related
}

// GetDependencyGraph builds a dependency graph starting from the given entity
func (b *Builder) GetDependencyGraph(entityID string, relations []models.Relation) *DependencyGraph {
	graph := &DependencyGraph{
		Root:         entityID,
		Dependencies: make(map[string][]string),
		Dependents:   make(map[string][]string),
	}

	// Build the graph by traversing relations
	visited := make(map[string]bool)
	b.buildGraphRecursive(entityID, relations, graph, visited)

	return graph
}

func (b *Builder) buildGraphRecursive(entityID string, relations []models.Relation, graph *DependencyGraph, visited map[string]bool) {
	if visited[entityID] {
		return
	}
	visited[entityID] = true

	// Find all outgoing relations (dependencies)
	for _, relation := range relations {
		if relation.SourceID == entityID {
			graph.Dependencies[entityID] = append(graph.Dependencies[entityID], relation.TargetID)
			graph.Dependents[relation.TargetID] = append(graph.Dependents[relation.TargetID], entityID)
			b.buildGraphRecursive(relation.TargetID, relations, graph, visited)
		}
	}

	// Find all incoming relations (dependents)
	for _, relation := range relations {
		if relation.TargetID == entityID {
			graph.Dependents[entityID] = append(graph.Dependents[entityID], relation.SourceID)
			graph.Dependencies[relation.SourceID] = append(graph.Dependencies[relation.SourceID], entityID)
			b.buildGraphRecursive(relation.SourceID, relations, graph, visited)
		}
	}
}

// DependencyGraph represents a graph of dependencies between entities
type DependencyGraph struct {
	Root         string              `json:"root"`
	Dependencies map[string][]string `json:"dependencies"` // Entity ID -> Entities this depends on
	Dependents   map[string][]string `json:"dependents"`   // Entity ID -> Entities that depend on this
}

// ExportGraph exports the dependency graph in nodes/edges format
func (g *DependencyGraph) ExportGraph() GraphExport {
	nodes := make(map[string]bool)
	var edges []GraphEdge

	// Collect all nodes
	nodes[g.Root] = true
	for source, targets := range g.Dependencies {
		nodes[source] = true
		for _, target := range targets {
			nodes[target] = true
			edges = append(edges, GraphEdge{
				Source: source,
				Target: target,
			})
		}
	}

	// Convert nodes map to slice
	nodeList := make([]string, 0, len(nodes))
	for node := range nodes {
		nodeList = append(nodeList, node)
	}

	return GraphExport{
		Nodes: nodeList,
		Edges: edges,
	}
}

// GraphExport represents a graph in nodes/edges format
type GraphExport struct {
	Nodes []string    `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

// GraphEdge represents an edge in the graph
type GraphEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// ExportRelationshipGraph exports all relationships as a graph
func ExportRelationshipGraph(entities map[string]models.Entity, relations []models.Relation) GraphExport {
	nodes := make([]string, 0, len(entities))
	for id := range entities {
		nodes = append(nodes, id)
	}

	edges := make([]GraphEdge, 0, len(relations))
	for _, relation := range relations {
		edges = append(edges, GraphEdge{
			Source: relation.SourceID,
			Target: relation.TargetID,
		})
	}

	return GraphExport{
		Nodes: nodes,
		Edges: edges,
	}
}
