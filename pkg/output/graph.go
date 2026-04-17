package output

import (
	"encoding/json"
	"infracanvas/internal/models"
	"strings"
)

// GraphFormatter formats InfraSnapshot as a graph with nodes and edges for web UI
type GraphFormatter struct {
	FilterNoise bool // Filter out noisy processes and services
}

// GraphOutput represents the graph structure for web UI
type GraphOutput struct {
	Snapshot GraphSnapshot `json:"snapshot"`
	Nodes    []GraphNode   `json:"nodes"`
	Edges    []GraphEdge   `json:"edges"`
	Stats    GraphStats    `json:"stats"`
}

type GraphSnapshot struct {
	HostID             string  `json:"hostId"`
	Timestamp          string  `json:"timestamp"`
	CollectionDuration float64 `json:"collectionDuration"` // in seconds
}

type GraphNode struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Label    string                 `json:"label"`
	Health   string                 `json:"health"`
	Metadata map[string]interface{} `json:"metadata"`
}

type GraphEdge struct {
	ID         string            `json:"id"`
	Source     string            `json:"source"`
	Target     string            `json:"target"`
	Type       string            `json:"type"`
	Properties map[string]string `json:"properties,omitempty"`
}

type GraphStats struct {
	TotalNodes      int            `json:"totalNodes"`
	TotalEdges      int            `json:"totalEdges"`
	NodesByType     map[string]int `json:"nodesByType"`
	FilteredOut     FilteredStats  `json:"filteredOut"`
	CollectionScope []string       `json:"collectionScope"`
}

type FilteredStats struct {
	DanglingImages  int `json:"danglingImages"`
	Processes       int `json:"processes"`
	SystemdServices int `json:"systemdServices"`
	OrphanedNodes   int `json:"orphanedNodes"`
}

// Format converts InfraSnapshot to graph format
func (f *GraphFormatter) Format(snapshot *models.InfraSnapshot) ([]byte, error) {
	graph := GraphOutput{
		Snapshot: GraphSnapshot{
			HostID:             snapshot.HostID,
			Timestamp:          snapshot.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
			CollectionDuration: snapshot.Metadata.CollectionDuration.Seconds(),
		},
		Nodes: []GraphNode{},
		Edges: []GraphEdge{},
		Stats: GraphStats{
			NodesByType: make(map[string]int),
			CollectionScope: snapshot.Metadata.Scope,
		},
	}

	// Track filtered entities
	filteredStats := FilteredStats{}

	// Process entities and convert to nodes
	for id, entity := range snapshot.Entities {
		entityType := entity.GetType()

		// Apply filtering if enabled
		if f.FilterNoise {
			if f.shouldFilterEntity(entity, &filteredStats) {
				continue
			}
		}

		node := f.entityToNode(id, entity)
		graph.Nodes = append(graph.Nodes, node)
		graph.Stats.NodesByType[string(entityType)]++
	}

	// Add grouped dangling images node if we filtered any
	if filteredStats.DanglingImages > 0 {
		graph.Nodes = append(graph.Nodes, GraphNode{
			ID:     "group:dangling-images",
			Type:   "image_group",
			Label:  "Build Cache",
			Health: "unknown",
			Metadata: map[string]interface{}{
				"count":       filteredStats.DanglingImages,
				"description": "Dangling Docker images (intermediate build layers)",
				"action":      "docker image prune -a",
			},
		})
		graph.Stats.NodesByType["image_group"]++
	}

	// Process relationships and convert to edges
	for _, relation := range snapshot.Relations {
		// Skip edges if source or target was filtered out
		if f.FilterNoise && (f.isFiltered(relation.SourceID, snapshot.Entities) || f.isFiltered(relation.TargetID, snapshot.Entities)) {
			continue
		}

		edge := GraphEdge{
			ID:         "edge:" + relation.SourceID + "→" + relation.TargetID,
			Source:     relation.SourceID,
			Target:     relation.TargetID,
			Type:       string(relation.Type),
			Properties: relation.Properties,
		}
		graph.Edges = append(graph.Edges, edge)
	}

	// Remove orphaned nodes — nodes with no edges in either direction.
	// Floating disconnected nodes make the canvas unusable.
	if f.FilterNoise {
		connected := make(map[string]bool, len(graph.Edges)*2)
		for _, e := range graph.Edges {
			connected[e.Source] = true
			connected[e.Target] = true
		}
		kept := graph.Nodes[:0]
		for _, n := range graph.Nodes {
			if connected[n.ID] {
				kept = append(kept, n)
			} else {
				filteredStats.OrphanedNodes++
				delete(graph.Stats.NodesByType, n.Type)
			}
		}
		// Recount nodesByType after orphan removal
		graph.Stats.NodesByType = make(map[string]int)
		for _, n := range kept {
			graph.Stats.NodesByType[n.Type]++
		}
		graph.Nodes = kept
	}

	graph.Stats.TotalNodes = len(graph.Nodes)
	graph.Stats.TotalEdges = len(graph.Edges)
	graph.Stats.FilteredOut = filteredStats

	return json.MarshalIndent(graph, "", "  ")
}

// entityToNode converts an entity to a graph node
func (f *GraphFormatter) entityToNode(id string, entity models.Entity) GraphNode {
	node := GraphNode{
		ID:       id,
		Type:     string(entity.GetType()),
		Health:   string(entity.GetHealth()),
		Metadata: make(map[string]interface{}),
	}

	// Type-specific node creation
	switch entity.GetType() {
	case models.EntityTypeHost:
		host := entity.(*models.Host)
		node.Label = host.Hostname
		node.Metadata["os"] = host.OS
		node.Metadata["osVersion"] = host.OSVersion
		node.Metadata["cpu"] = host.CPUUsagePercent
		node.Metadata["memory"] = host.MemoryUsagePercent
		node.Metadata["architecture"] = host.Architecture
		node.Metadata["cloudProvider"] = host.CloudProvider
		node.Metadata["instanceType"] = host.InstanceType

	case models.EntityTypeContainer:
		container := entity.(*models.Container)
		node.Label = container.Name
		node.Metadata["image"] = container.Image
		node.Metadata["state"] = container.State
		node.Metadata["status"] = container.Status
		node.Metadata["cpu"] = container.CPUPercent
		node.Metadata["memory"] = container.MemoryUsage
		node.Metadata["memoryLimit"] = container.MemoryLimit
		node.Metadata["networkRx"] = container.NetworkRxBytes
		node.Metadata["networkTx"] = container.NetworkTxBytes
		if container.ComposeProject != "" {
			node.Metadata["composeProject"] = container.ComposeProject
			node.Metadata["composeService"] = container.ComposeService
		}
		if len(container.PortMappings) > 0 {
			ports := make([]map[string]interface{}, 0, len(container.PortMappings))
			for _, pm := range container.PortMappings {
				ports = append(ports, map[string]interface{}{
					"hostPort":      pm.HostPort,
					"containerPort": pm.ContainerPort,
					"protocol":      pm.Protocol,
					"hostIP":        pm.HostIP,
				})
			}
			node.Metadata["portMappings"] = ports
		}

	case models.EntityTypeImage:
		image := entity.(*models.Image)
		// Extract meaningful label from repository:tag
		if image.Repository != "<none>" && image.Tag != "<none>" {
			// For named images, use short name
			parts := strings.Split(image.Repository, "/")
			shortName := parts[len(parts)-1]
			node.Label = shortName
			node.Metadata["fullName"] = image.Repository + ":" + image.Tag
		} else {
			node.Label = "unnamed-image"
		}
		node.Metadata["repository"] = image.Repository
		node.Metadata["tag"] = image.Tag
		node.Metadata["size"] = image.Size
		node.Metadata["created"] = image.Created

	case models.EntityTypeProcess:
		process := entity.(*models.Process)
		node.Label = process.Name
		node.Metadata["pid"] = process.PID
		node.Metadata["user"] = process.User
		node.Metadata["cpu"] = process.CPUPercent
		node.Metadata["memory"] = process.MemoryPercent
		node.Metadata["processType"] = process.ProcessType

	case models.EntityTypeService:
		service := entity.(*models.Service)
		node.Label = service.Name
		node.Metadata["status"] = service.Status
		node.Metadata["enabled"] = service.Enabled
		node.Metadata["critical"] = service.IsCritical

	case models.EntityTypeVolume:
		volume := entity.(*models.Volume)
		node.Label = volume.Name
		node.Metadata["driver"] = volume.Driver
		node.Metadata["mountpoint"] = volume.MountPoint

	case models.EntityTypeNetwork:
		network := entity.(*models.Network)
		node.Label = network.Name
		node.Metadata["driver"] = network.Driver
		node.Metadata["scope"] = network.Scope

	case models.EntityTypePod:
		pod := entity.(*models.Pod)
		node.Label = pod.Name
		node.Metadata["namespace"] = pod.Namespace
		node.Metadata["phase"] = pod.Phase
		node.Metadata["nodeName"] = pod.NodeName
		node.Metadata["podIP"] = pod.PodIP
		if len(pod.Containers) > 0 {
			containers := make([]map[string]interface{}, 0, len(pod.Containers))
			for _, c := range pod.Containers {
				containers = append(containers, map[string]interface{}{
					"name":          c.Name,
					"image":         c.Image,
					"ready":         c.Ready,
					"state":         c.State,
					"restartCount":  c.RestartCount,
					"cpuRequest":    c.CPURequest,
					"memoryRequest": c.MemoryRequest,
					"cpuLimit":      c.CPULimit,
					"memoryLimit":   c.MemoryLimit,
				})
			}
			node.Metadata["containers"] = containers
		}

	case models.EntityTypeDeployment:
		deployment := entity.(*models.Deployment)
		node.Label = deployment.Name
		node.Metadata["namespace"] = deployment.Namespace
		node.Metadata["replicas"] = deployment.Replicas
		node.Metadata["ready"] = deployment.ReadyReplicas
		node.Metadata["available"] = deployment.AvailableReplicas

	case models.EntityTypeStatefulSet:
		ss := entity.(*models.StatefulSet)
		node.Label = ss.Name
		node.Metadata["namespace"] = ss.Namespace
		node.Metadata["replicas"] = ss.Replicas
		node.Metadata["ready"] = ss.ReadyReplicas

	case models.EntityTypeDaemonSet:
		ds := entity.(*models.DaemonSet)
		node.Label = ds.Name
		node.Metadata["namespace"] = ds.Namespace
		node.Metadata["desired"] = ds.DesiredNumberScheduled
		node.Metadata["ready"] = ds.NumberReady

	case models.EntityTypeJob:
		job := entity.(*models.Job)
		node.Label = job.Name
		node.Metadata["namespace"] = job.Namespace
		node.Metadata["succeeded"] = job.Succeeded
		node.Metadata["active"] = job.Active
		node.Metadata["failed"] = job.Failed

	case models.EntityTypeCronJob:
		cj := entity.(*models.CronJob)
		node.Label = cj.Name
		node.Metadata["namespace"] = cj.Namespace
		node.Metadata["schedule"] = cj.Schedule
		node.Metadata["active"] = cj.Active

	case models.EntityTypeK8sService:
		svc := entity.(*models.K8sService)
		node.Label = svc.Name
		node.Metadata["namespace"] = svc.Namespace
		node.Metadata["serviceType"] = svc.ServiceType
		node.Metadata["clusterIP"] = svc.ClusterIP
		node.Metadata["hasEndpoints"] = svc.HasEndpoints

	case models.EntityTypeIngress:
		ing := entity.(*models.Ingress)
		node.Label = ing.Name
		node.Metadata["namespace"] = ing.Namespace
		node.Metadata["ingressClass"] = ing.IngressClass
		hosts := make([]string, 0, len(ing.Rules))
		for _, r := range ing.Rules {
			if r.Host != "" {
				hosts = append(hosts, r.Host)
			}
		}
		node.Metadata["hosts"] = hosts

	case models.EntityTypeNamespace:
		ns := entity.(*models.Namespace)
		node.Label = ns.Name
		node.Metadata["status"] = ns.Status

	case models.EntityTypeNode:
		k8sNode := entity.(*models.Node)
		node.Label = k8sNode.Name
		node.Metadata["status"] = k8sNode.Status
		node.Metadata["version"] = k8sNode.KubernetesVersion
		node.Metadata["roles"] = k8sNode.Roles
		node.Metadata["cpuCapacity"] = k8sNode.CPUCapacity
		node.Metadata["memoryCapacity"] = k8sNode.MemoryCapacity
		node.Metadata["podsCapacity"] = k8sNode.PodsCapacity
		node.Metadata["cpuAllocatable"] = k8sNode.CPUAllocatable
		node.Metadata["memoryAllocatable"] = k8sNode.MemoryAllocatable
		node.Metadata["containerRuntime"] = k8sNode.ContainerRuntime
		node.Metadata["osImage"] = k8sNode.OSImage

	case models.EntityTypePVC:
		pvc := entity.(*models.PersistentVolumeClaim)
		node.Label = pvc.Name
		node.Metadata["namespace"] = pvc.Namespace
		node.Metadata["storageClass"] = pvc.StorageClass
		node.Metadata["requestedStorage"] = pvc.RequestedStorage
		node.Metadata["accessModes"] = pvc.AccessModes

	case models.EntityTypePV:
		pv := entity.(*models.PersistentVolume)
		node.Label = pv.Name
		node.Metadata["storageClass"] = pv.StorageClass
		node.Metadata["capacity"] = pv.Capacity
		node.Metadata["reclaimPolicy"] = pv.ReclaimPolicy

	case models.EntityTypeStorageClass:
		sc := entity.(*models.StorageClass)
		node.Label = sc.Name
		node.Metadata["provisioner"] = sc.Provisioner
		node.Metadata["reclaimPolicy"] = sc.ReclaimPolicy

	case models.EntityTypeConfigMap:
		cm := entity.(*models.ConfigMap)
		node.Label = cm.Name
		node.Metadata["namespace"] = cm.Namespace

	case models.EntityTypeSecret:
		secret := entity.(*models.Secret)
		node.Label = secret.Name
		node.Metadata["namespace"] = secret.Namespace
		node.Metadata["secretType"] = secret.Type

	case models.EntityTypeEvent:
		event := entity.(*models.Event)
		node.Label = event.Reason
		node.Metadata["namespace"] = event.ObjectNamespace
		node.Metadata["objectKind"] = event.ObjectKind
		node.Metadata["objectName"] = event.ObjectName
		node.Metadata["message"] = event.Message
		node.Metadata["eventType"] = event.EventType
		node.Metadata["category"] = event.Category
		node.Metadata["isCritical"] = event.IsCritical

	case models.EntityTypeCluster:
		cluster := entity.(*models.Cluster)
		node.Label = cluster.Name
		node.Metadata["platform"] = cluster.Platform
		node.Metadata["version"] = cluster.Version
		node.Metadata["apiServer"] = cluster.APIServer

	case models.EntityTypeContainerRuntime:
		rt := entity.(*models.ContainerRuntime)
		node.Label = rt.RuntimeType
		node.Metadata["version"] = rt.Version
		node.Metadata["storageDriver"] = rt.StorageDriver
		node.Metadata["cgroupDriver"] = rt.CgroupDriver

	default:
		node.Label = id
	}

	return node
}

// shouldFilterEntity determines if an entity should be filtered out
func (f *GraphFormatter) shouldFilterEntity(entity models.Entity, stats *FilteredStats) bool {
	switch entity.GetType() {
	case models.EntityTypeImage:
		image := entity.(*models.Image)
		// Filter dangling images
		if image.Repository == "<none>" && image.Tag == "<none>" {
			stats.DanglingImages++
			return true
		}

	case models.EntityTypeProcess:
		// Filter all processes — too numerous for a canvas view
		stats.Processes++
		return true

	case models.EntityTypeService:
		// Filter all systemd host services from the canvas — the host entity and
		// docker-runtime entity already represent the host layer. 158 systemd services
		// would make the canvas unworkable.
		stats.SystemdServices++
		return true

	case models.EntityTypeEvent:
		// Filter non-critical events — they're informational noise; only critical events
		// belong on the canvas. Critical events are still shown via RELATES_TO edges.
		event := entity.(*models.Event)
		if !event.IsCritical {
			return true
		}

	case models.EntityTypeConfigMap:
		// ConfigMaps are pure config data — dozens exist in kube-system alone.
		// They add no topological signal to the canvas.
		return true

	case models.EntityTypeSecret:
		// Secrets (service account tokens, TLS certs, etc.) are numerous and
		// security-sensitive. Not useful on a topology canvas.
		return true

	case models.EntityTypePV:
		// PersistentVolumes are cluster-level infra — the user-facing side (PVCs)
		// is already more informative. PVs just duplicate that.
		return true

	case models.EntityTypeStorageClass:
		// StorageClasses are provisioner configuration, not runtime topology.
		return true
	}

	return false
}

// isKernelThread checks if a process is a kernel thread
func (f *GraphFormatter) isKernelThread(name string) bool {
	kernelPrefixes := []string{
		"kworker", "ksoftirqd", "migration", "idle_inject",
		"cpuhp", "kdevtmpfs", "kauditd", "khungtaskd",
		"oom_reaper", "kcompactd", "ksmd", "khugepaged",
		"kswapd", "ecryptfs-kthread", "kthreadd", "rcu_",
		"watchdogd", "kintegrityd", "kblockd", "ata_sff",
		"scsi_eh", "scsi_tmf", "irq/", "acpi", "psimon",
	}

	for _, prefix := range kernelPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}

	return false
}

// isFiltered checks if an entity ID was filtered out
func (f *GraphFormatter) isFiltered(entityID string, entities map[string]models.Entity) bool {
	entity, exists := entities[entityID]
	if !exists {
		return true
	}

	stats := &FilteredStats{}
	return f.shouldFilterEntity(entity, stats)
}
