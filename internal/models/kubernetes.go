package models

import "time"

// Cluster represents a Kubernetes cluster
type Cluster struct {
	BaseEntity

	Name      string `json:"name"`
	Version   string `json:"version"`
	APIServer string `json:"api_server"`
	Platform  string `json:"platform,omitempty"` // EKS, GKE, AKS, OpenShift, etc.
}

// Node represents a Kubernetes node
type Node struct {
	BaseEntity

	Name              string `json:"name"`
	Status            string `json:"status"` // Ready, NotReady
	Roles             []string `json:"roles"`
	KubernetesVersion string `json:"kubernetes_version"`
	ContainerRuntime  string `json:"container_runtime"`
	OSImage           string `json:"os_image"`
	KernelVersion     string `json:"kernel_version"`

	// Capacity
	CPUCapacity    string `json:"cpu_capacity"`
	MemoryCapacity string `json:"memory_capacity"`
	PodsCapacity   int    `json:"pods_capacity"`

	// Allocatable
	CPUAllocatable    string `json:"cpu_allocatable"`
	MemoryAllocatable string `json:"memory_allocatable"`
	PodsAllocatable   int    `json:"pods_allocatable"`

	// Conditions
	Conditions []NodeCondition `json:"conditions"`
}

// NodeCondition represents a node condition
type NodeCondition struct {
	Type           string    `json:"type"`
	Status         string    `json:"status"`
	Reason         string    `json:"reason,omitempty"`
	Message        string    `json:"message,omitempty"`
	LastTransition time.Time `json:"last_transition"`
}

// Namespace represents a Kubernetes namespace
type Namespace struct {
	BaseEntity

	Name   string `json:"name"`
	Status string `json:"status"`
	Phase  string `json:"phase"`
}

// Deployment represents a Kubernetes Deployment
type Deployment struct {
	BaseEntity

	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	Replicas          int32             `json:"replicas"`
	AvailableReplicas int32             `json:"available_replicas"`
	ReadyReplicas     int32             `json:"ready_replicas"`
	UpdatedReplicas   int32             `json:"updated_replicas"`
	Selector          map[string]string `json:"selector"`
	Containers        []ContainerSpec   `json:"containers"`
	Strategy          string            `json:"strategy"`
}

// StatefulSet represents a Kubernetes StatefulSet
type StatefulSet struct {
	BaseEntity

	Name                 string            `json:"name"`
	Namespace            string            `json:"namespace"`
	Replicas             int32             `json:"replicas"`
	ReadyReplicas        int32             `json:"ready_replicas"`
	CurrentReplicas      int32             `json:"current_replicas"`
	UpdatedReplicas      int32             `json:"updated_replicas"`
	ServiceName          string            `json:"service_name"`
	Selector             map[string]string `json:"selector"`
	Containers           []ContainerSpec   `json:"containers"`
	VolumeClaimTemplates []string          `json:"volume_claim_templates"`
}

// DaemonSet represents a Kubernetes DaemonSet
type DaemonSet struct {
	BaseEntity

	Name                   string            `json:"name"`
	Namespace              string            `json:"namespace"`
	DesiredNumberScheduled int32             `json:"desired_number_scheduled"`
	CurrentNumberScheduled int32             `json:"current_number_scheduled"`
	NumberReady            int32             `json:"number_ready"`
	NumberAvailable        int32             `json:"number_available"`
	Selector               map[string]string `json:"selector"`
	Containers             []ContainerSpec   `json:"containers"`
}

// Job represents a Kubernetes Job
type Job struct {
	BaseEntity

	Name           string    `json:"name"`
	Namespace      string    `json:"namespace"`
	Completions    int32     `json:"completions"`
	Parallelism    int32     `json:"parallelism"`
	Active         int32     `json:"active"`
	Succeeded      int32     `json:"succeeded"`
	Failed         int32     `json:"failed"`
	StartTime      time.Time `json:"start_time,omitempty"`
	CompletionTime time.Time `json:"completion_time,omitempty"`
	OwnerKind      string    `json:"owner_kind,omitempty"`
	OwnerName      string    `json:"owner_name,omitempty"`
}

// CronJob represents a Kubernetes CronJob
type CronJob struct {
	BaseEntity

	Name             string    `json:"name"`
	Namespace        string    `json:"namespace"`
	Schedule         string    `json:"schedule"`
	Suspend          bool      `json:"suspend"`
	LastScheduleTime time.Time `json:"last_schedule_time,omitempty"`
	Active           int       `json:"active"`
}

// ContainerSpec represents a container specification in a workload
type ContainerSpec struct {
	Name      string               `json:"name"`
	Image     string               `json:"image"`
	Resources ResourceRequirements `json:"resources"`
}

// ResourceRequirements represents resource requests and limits
type ResourceRequirements struct {
	Requests map[string]string `json:"requests"`
	Limits   map[string]string `json:"limits"`
}

// Pod represents a Kubernetes Pod
type Pod struct {
	BaseEntity

	Name      string    `json:"name"`
	Namespace string    `json:"namespace"`
	Status    string    `json:"status"`
	Phase     string    `json:"phase"` // Pending, Running, Succeeded, Failed, Unknown
	NodeName  string    `json:"node_name"`
	PodIP     string    `json:"pod_ip"`
	HostIP    string    `json:"host_ip"`
	StartTime time.Time `json:"start_time,omitempty"`

	// Owner
	OwnerKind string `json:"owner_kind,omitempty"`
	OwnerName string `json:"owner_name,omitempty"`

	// Containers
	Containers []PodContainer `json:"containers"`

	// Conditions
	Conditions []PodCondition `json:"conditions"`

	// Volume references
	VolumeRefs PodVolumeRefs `json:"volume_refs,omitempty"`
}

// PodVolumeRefs contains references to volumes used by a pod
type PodVolumeRefs struct {
	ConfigMaps []string `json:"configmaps,omitempty"` // ConfigMap names
	Secrets    []string `json:"secrets,omitempty"`    // Secret names
	PVCs       []string `json:"pvcs,omitempty"`       // PVC names
}

// PodContainer represents a container in a pod
type PodContainer struct {
	Name         string `json:"name"`
	Image        string `json:"image"`
	ImageID      string `json:"image_id"`
	State        string `json:"state"` // running, waiting, terminated
	Ready        bool   `json:"ready"`
	RestartCount int32  `json:"restart_count"`

	// Resources
	CPURequest    string `json:"cpu_request,omitempty"`
	CPULimit      string `json:"cpu_limit,omitempty"`
	MemoryRequest string `json:"memory_request,omitempty"`
	MemoryLimit   string `json:"memory_limit,omitempty"`
}

// PodCondition represents a pod condition
type PodCondition struct {
	Type           string    `json:"type"`
	Status         string    `json:"status"`
	Reason         string    `json:"reason,omitempty"`
	Message        string    `json:"message,omitempty"`
	LastTransition time.Time `json:"last_transition"`
}

// K8sService represents a Kubernetes Service
type K8sService struct {
	BaseEntity

	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	ServiceType string            `json:"service_type"` // ClusterIP, NodePort, LoadBalancer
	ClusterIP   string            `json:"cluster_ip"`
	ExternalIPs []string          `json:"external_ips,omitempty"`
	Ports       []ServicePort     `json:"ports"`
	Selector    map[string]string `json:"selector"`

	// Endpoints
	HasEndpoints bool `json:"has_endpoints"`
}

// ServicePort represents a service port
type ServicePort struct {
	Name       string `json:"name,omitempty"`
	Protocol   string `json:"protocol"`
	Port       int32  `json:"port"`
	TargetPort string `json:"target_port"`
	NodePort   int32  `json:"node_port,omitempty"`
}

// Ingress represents a Kubernetes Ingress
type Ingress struct {
	BaseEntity

	Name         string        `json:"name"`
	Namespace    string        `json:"namespace"`
	IngressClass string        `json:"ingress_class,omitempty"`
	Rules        []IngressRule `json:"rules"`
	TLS          []IngressTLS  `json:"tls,omitempty"`
}

// IngressRule represents an ingress rule
type IngressRule struct {
	Host  string        `json:"host"`
	Paths []IngressPath `json:"paths"`
}

// IngressPath represents an ingress path
type IngressPath struct {
	Path        string `json:"path"`
	PathType    string `json:"path_type"`
	ServiceName string `json:"service_name"`
	ServicePort int32  `json:"service_port"`
}

// IngressTLS represents ingress TLS configuration
type IngressTLS struct {
	Hosts      []string `json:"hosts"`
	SecretName string   `json:"secret_name"`
}

// ConfigMap represents a Kubernetes ConfigMap
type ConfigMap struct {
	BaseEntity

	Name      string   `json:"name"`
	Namespace string   `json:"namespace"`
	DataKeys  []string `json:"data_keys"` // Keys only, not values
}

// Secret represents a Kubernetes Secret
type Secret struct {
	BaseEntity

	Name      string   `json:"name"`
	Namespace string   `json:"namespace"`
	Type      string   `json:"type"`
	DataKeys  []string `json:"data_keys"` // Keys only, not values
}

// PersistentVolumeClaim represents a Kubernetes PVC
type PersistentVolumeClaim struct {
	BaseEntity

	Name             string   `json:"name"`
	Namespace        string   `json:"namespace"`
	Status           string   `json:"status"`
	StorageClass     string   `json:"storage_class,omitempty"`
	RequestedStorage string   `json:"requested_storage"`
	AccessModes      []string `json:"access_modes"`
	VolumeName       string   `json:"volume_name,omitempty"`
}

// PersistentVolume represents a Kubernetes PV
type PersistentVolume struct {
	BaseEntity

	Name          string   `json:"name"`
	Capacity      string   `json:"capacity"`
	AccessModes   []string `json:"access_modes"`
	ReclaimPolicy string   `json:"reclaim_policy"`
	Status        string   `json:"status"`
	StorageClass  string   `json:"storage_class,omitempty"`
	ClaimRef      string   `json:"claim_ref,omitempty"` // namespace/name
}

// StorageClass represents a Kubernetes StorageClass
type StorageClass struct {
	BaseEntity

	Name              string            `json:"name"`
	Provisioner       string            `json:"provisioner"`
	ReclaimPolicy     string            `json:"reclaim_policy"`
	VolumeBindingMode string            `json:"volume_binding_mode"`
	Parameters        map[string]string `json:"parameters"`
}

// Event represents a Kubernetes event
type Event struct {
	BaseEntity

	Timestamp time.Time `json:"timestamp"`
	EventType string    `json:"event_type"` // Normal, Warning
	Reason    string    `json:"reason"`
	Message   string    `json:"message"`

	// Involved Object
	ObjectKind      string `json:"object_kind"`
	ObjectName      string `json:"object_name"`
	ObjectNamespace string `json:"object_namespace,omitempty"`

	// Classification
	IsCritical bool   `json:"is_critical"`
	Category   string `json:"category"` // image_pull, crash_loop, probe_failure, etc.
}
