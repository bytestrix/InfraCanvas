package models

// Host represents a physical or virtual machine
type Host struct {
	BaseEntity

	// Identity
	Hostname  string `json:"hostname"`
	FQDN      string `json:"fqdn"`
	MachineID string `json:"machine_id"`

	// OS Information
	OS            string `json:"os"`
	OSVersion     string `json:"os_version"`
	KernelVersion string `json:"kernel_version"`
	Architecture  string `json:"architecture"`

	// Virtualization
	VirtualizationType string `json:"virtualization_type"`
	Hypervisor         string `json:"hypervisor,omitempty"`

	// Cloud Metadata
	CloudProvider    string            `json:"cloud_provider,omitempty"`
	InstanceID       string            `json:"instance_id,omitempty"`
	InstanceType     string            `json:"instance_type,omitempty"`
	Region           string            `json:"region,omitempty"`
	AvailabilityZone string            `json:"availability_zone,omitempty"`
	CloudTags        map[string]string `json:"cloud_tags,omitempty"`

	// Resources
	CPUModel           string  `json:"cpu_model"`
	CPUCores           int     `json:"cpu_cores"`
	CPUUsagePercent    float64 `json:"cpu_usage_percent"`
	MemoryTotalBytes   int64   `json:"memory_total_bytes"`
	MemoryUsedBytes    int64   `json:"memory_used_bytes"`
	MemoryUsagePercent float64 `json:"memory_usage_percent"`

	// Network
	NetworkInterfaces []NetworkInterface `json:"network_interfaces"`
	ListeningPorts    []ListeningPort    `json:"listening_ports"`

	// Storage
	Filesystems []Filesystem `json:"filesystems"`
}

// NetworkInterface represents a network interface on the host
type NetworkInterface struct {
	Name        string   `json:"name"`
	IPAddresses []string `json:"ip_addresses"`
	MACAddress  string   `json:"mac_address"`
	Status      string   `json:"status"`
}

// ListeningPort represents a port listening on the host
type ListeningPort struct {
	Port      int    `json:"port"`
	Protocol  string `json:"protocol"`
	ProcessID int    `json:"process_id,omitempty"`
	Process   string `json:"process,omitempty"`
}

// Filesystem represents a mounted filesystem
type Filesystem struct {
	MountPoint   string  `json:"mount_point"`
	Device       string  `json:"device"`
	FSType       string  `json:"fs_type"`
	TotalBytes   int64   `json:"total_bytes"`
	UsedBytes    int64   `json:"used_bytes"`
	AvailBytes   int64   `json:"avail_bytes"`
	UsagePercent float64 `json:"usage_percent"`
}

// Process represents a running process
type Process struct {
	BaseEntity

	PID            int     `json:"pid"`
	PPID           int     `json:"ppid"`
	Name           string  `json:"name"`
	CommandLine    string  `json:"command_line"`
	User           string  `json:"user"`
	CPUPercent     float64 `json:"cpu_percent"`
	MemoryPercent  float64 `json:"memory_percent"`
	MemoryBytes    int64   `json:"memory_bytes"`
	ListeningPorts []int   `json:"listening_ports"`
	ProcessType    string  `json:"process_type,omitempty"` // docker, kubelet, database, webserver, etc.
}

// Service represents a systemd service
type Service struct {
	BaseEntity

	Name         string   `json:"name"`
	Status       string   `json:"status"` // active, inactive, failed
	Enabled      bool     `json:"enabled"`
	Description  string   `json:"description"`
	Dependencies []string `json:"dependencies"` // Service names
	RestartCount int      `json:"restart_count"`
	IsCritical   bool     `json:"is_critical"`
}
