package models

import "time"

// ContainerRuntime represents a container runtime (Docker, containerd, etc.)
type ContainerRuntime struct {
	BaseEntity

	RuntimeType   string `json:"runtime_type"` // docker, containerd, cri-o
	Version       string `json:"version"`
	StorageDriver string `json:"storage_driver"`
	CgroupDriver  string `json:"cgroup_driver"`
	SocketPath    string `json:"socket_path"`
}

// Container represents a Docker container
type Container struct {
	BaseEntity

	ContainerID  string    `json:"container_id"`
	Name         string    `json:"name"`
	Image        string    `json:"image"`
	ImageID      string    `json:"image_id"`
	State        string    `json:"state"`  // running, exited, paused, dead
	Status       string    `json:"status"` // Status message
	Created      time.Time `json:"created"`
	Started      time.Time `json:"started,omitempty"`
	Finished     time.Time `json:"finished,omitempty"`
	RestartCount int       `json:"restart_count"`

	// Resources
	CPUPercent      float64 `json:"cpu_percent"`
	MemoryUsage     int64   `json:"memory_usage"`
	MemoryLimit     int64   `json:"memory_limit"`
	NetworkRxBytes  int64   `json:"network_rx_bytes"`
	NetworkTxBytes  int64   `json:"network_tx_bytes"`
	BlockReadBytes  int64   `json:"block_read_bytes"`
	BlockWriteBytes int64   `json:"block_write_bytes"`

	// Configuration
	Environment  map[string]string `json:"environment"` // Redacted
	PortMappings []PortMapping     `json:"port_mappings"`
	Mounts       []Mount           `json:"mounts"`
	NetworkMode  string            `json:"network_mode"`
	Networks     []string          `json:"networks"` // All connected network names

	// Docker Compose
	ComposeProject string `json:"compose_project,omitempty"`
	ComposeService string `json:"compose_service,omitempty"`
}

// PortMapping represents a container port mapping
type PortMapping struct {
	HostIP        string `json:"host_ip"`
	HostPort      int    `json:"host_port"`
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol"`
}

// Mount represents a container mount
type Mount struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Mode        string `json:"mode"`
	Type        string `json:"type"` // bind, volume, tmpfs
}

// Image represents a container image
type Image struct {
	BaseEntity

	ImageID    string    `json:"image_id"`
	Registry   string    `json:"registry"`
	Repository string    `json:"repository"`
	Tag        string    `json:"tag"`
	Digest     string    `json:"digest,omitempty"`
	Size       int64     `json:"size"`
	Created    time.Time `json:"created"`

	// Usage tracking
	UsedByContainers []string `json:"used_by_containers"` // Container IDs
	UsedByPods       []string `json:"used_by_pods"`       // Pod IDs
}

// Volume represents a Docker volume
type Volume struct {
	BaseEntity

	Name       string    `json:"name"`
	Driver     string    `json:"driver"`
	MountPoint string    `json:"mount_point"`
	Size       int64     `json:"size,omitempty"`
	Created    time.Time `json:"created"`

	// Usage tracking
	UsedByContainers []string `json:"used_by_containers"` // Container IDs
}

// Network represents a Docker network
type Network struct {
	BaseEntity

	NetworkID string `json:"network_id"`
	Name      string `json:"name"`
	Driver    string `json:"driver"`
	Scope     string `json:"scope"`
	Subnet    string `json:"subnet,omitempty"`
	Gateway   string `json:"gateway,omitempty"`

	// Usage tracking
	ConnectedContainers []string `json:"connected_containers"` // Container IDs
}
