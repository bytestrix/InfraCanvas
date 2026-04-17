package host

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"infracanvas/internal/models"
	"infracanvas/pkg/validation"
)

// Discovery implements host-level infrastructure discovery
type Discovery struct {
	hostname string
}

// NewDiscovery creates a new host discovery instance
func NewDiscovery() *Discovery {
	return &Discovery{}
}

// IsAvailable checks if host discovery is available
func (d *Discovery) IsAvailable() bool {
	return true // Host discovery is always available
}

// GetHostInfo collects static host information
func (d *Discovery) GetHostInfo() (*models.Host, error) {
	host := &models.Host{
		BaseEntity: models.BaseEntity{
			Type:      models.EntityTypeHost,
			Labels:    make(map[string]string),
			Timestamp: time.Now(),
		},
	}

	// Collect hostname and FQDN
	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("failed to get hostname: %w", err)
	}
	host.Hostname = hostname
	d.hostname = hostname

	// Try to get FQDN
	fqdn, err := getFQDN()
	if err == nil {
		host.FQDN = fqdn
	} else {
		host.FQDN = hostname
	}

	// Get machine ID
	machineID, err := getMachineID()
	if err == nil {
		host.MachineID = machineID
	}

	// Collect OS information from /etc/os-release
	osInfo, err := getOSInfo()
	if err == nil {
		host.OS = osInfo.Name
		host.OSVersion = osInfo.Version
	}

	// Collect kernel version and architecture using uname
	kernelVersion, arch, err := getUnameInfo()
	if err == nil {
		host.KernelVersion = kernelVersion
		host.Architecture = arch
	}

	// Detect virtualization type
	virtType, hypervisor, err := detectVirtualization()
	if err == nil {
		host.VirtualizationType = virtType
		host.Hypervisor = hypervisor
	}

	// Collect CPU information
	cpuModel, cpuCores, err := getCPUInfo()
	if err == nil {
		host.CPUModel = cpuModel
		host.CPUCores = cpuCores
	}

	// Generate entity ID
	host.ID = fmt.Sprintf("host:%s", hostname)

	return host, nil
}

// OSInfo represents OS information from /etc/os-release
type OSInfo struct {
	Name    string
	Version string
	ID      string
}

// getOSInfo reads OS information from /etc/os-release
func getOSInfo() (*OSInfo, error) {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return nil, fmt.Errorf("failed to read /etc/os-release: %w", err)
	}

	info := &OSInfo{}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "NAME=") {
			info.Name = strings.Trim(strings.TrimPrefix(line, "NAME="), "\"")
		} else if strings.HasPrefix(line, "VERSION=") {
			info.Version = strings.Trim(strings.TrimPrefix(line, "VERSION="), "\"")
		} else if strings.HasPrefix(line, "ID=") {
			info.ID = strings.Trim(strings.TrimPrefix(line, "ID="), "\"")
		}
	}

	return info, nil
}

// getUnameInfo collects kernel version and architecture using uname
func getUnameInfo() (kernelVersion string, arch string, err error) {
	// Get kernel version
	cmd := exec.Command("uname", "-r")
	output, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("failed to execute uname -r: %w", err)
	}
	
	kernelVersion = strings.TrimSpace(string(output))
	if err := validation.ValidateNotEmpty(kernelVersion, "kernel_version", "uname -r"); err != nil {
		validation.LogParseError(err, "uname -r output validation")
		return "", "", err
	}

	// Get architecture
	cmd = exec.Command("uname", "-m")
	output, err = cmd.Output()
	if err != nil {
		return kernelVersion, "", fmt.Errorf("failed to execute uname -m: %w", err)
	}
	
	arch = strings.TrimSpace(string(output))
	if err := validation.ValidateNotEmpty(arch, "architecture", "uname -m"); err != nil {
		validation.LogParseError(err, "uname -m output validation")
		return kernelVersion, "", err
	}

	return kernelVersion, arch, nil
}

// detectVirtualization detects the virtualization type
func detectVirtualization() (virtType string, hypervisor string, err error) {
	// Try systemd-detect-virt first
	cmd := exec.Command("systemd-detect-virt")
	output, err := cmd.Output()
	if err == nil {
		virt := strings.TrimSpace(string(output))
		
		// Validate output is not empty
		if err := validation.ValidateNotEmpty(virt, "virtualization_type", "systemd-detect-virt"); err != nil {
			validation.LogParseError(err, "systemd-detect-virt output validation")
		} else if virt != "none" {
			return virt, virt, nil
		}
		return "bare-metal", "", nil
	}

	// Try /sys/hypervisor/type
	data, err := os.ReadFile("/sys/hypervisor/type")
	if err == nil {
		hypervisorType := strings.TrimSpace(string(data))
		if err := validation.ValidateNotEmpty(hypervisorType, "hypervisor_type", "/sys/hypervisor/type"); err != nil {
			validation.LogParseError(err, "/sys/hypervisor/type validation")
		} else {
			return hypervisorType, hypervisorType, nil
		}
	}

	// Check for common virtualization indicators
	if _, err := os.Stat("/proc/xen"); err == nil {
		return "xen", "xen", nil
	}

	// Default to bare-metal
	return "bare-metal", "", nil
}

// getFQDN attempts to get the fully qualified domain name
func getFQDN() (string, error) {
	cmd := exec.Command("hostname", "-f")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// getMachineID reads the machine ID
func getMachineID() (string, error) {
	// Try /etc/machine-id first
	data, err := os.ReadFile("/etc/machine-id")
	if err == nil {
		return strings.TrimSpace(string(data)), nil
	}

	// Try /var/lib/dbus/machine-id
	data, err = os.ReadFile("/var/lib/dbus/machine-id")
	if err == nil {
		return strings.TrimSpace(string(data)), nil
	}

	return "", fmt.Errorf("machine-id not found")
}

// getCPUInfo reads CPU model and core count from /proc/cpuinfo
func getCPUInfo() (model string, cores int, err error) {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return "", 0, fmt.Errorf("failed to read /proc/cpuinfo: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	coreCount := 0
	cpuModel := ""

	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// Parse model name (only capture first occurrence)
		if cpuModel == "" && strings.HasPrefix(line, "model name") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				cpuModel = strings.TrimSpace(parts[1])
			}
		}
		
		// Count processor entries to get core count
		if strings.HasPrefix(line, "processor") {
			coreCount++
		}
	}

	if cpuModel == "" {
		cpuModel = "Unknown"
	}

	if coreCount == 0 {
		coreCount = 1 // Default to 1 if we can't determine
	}

	return cpuModel, coreCount, nil
}

// GetProcesses collects all running processes
func (d *Discovery) GetProcesses() ([]models.Process, error) {
	// Implemented in processes.go
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, fmt.Errorf("failed to read /proc: %w", err)
	}

	processes := []models.Process{}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pid := entry.Name()
		if !isNumeric(pid) {
			continue
		}

		pidInt, _ := strconv.Atoi(pid)

		// Parse process information
		process, err := parseProcess(pidInt)
		if err != nil {
			// Skip processes we can't read (permission denied, process exited, etc.)
			continue
		}

		processes = append(processes, *process)
	}

	return processes, nil
}

// GetResourceUsage collects current resource usage
func (d *Discovery) GetResourceUsage() (*ResourceUsage, error) {
	return getResourceUsage()
}

// GetServices collects systemd services
func (d *Discovery) GetServices() ([]models.Service, error) {
	// Implemented in services.go
	return getSystemdServices()
}

// GetServiceDependencies gets dependencies for a service
func (d *Discovery) GetServiceDependencies(serviceName string) ([]string, error) {
	return getServiceDependencies(serviceName)
}

// ResourceUsage represents resource usage data
type ResourceUsage struct {
	CPUUsagePercent    float64
	MemoryTotalBytes   int64
	MemoryUsedBytes    int64
	MemoryUsagePercent float64
	Filesystems        []models.Filesystem
}

// NetworkStats represents network statistics
type NetworkStats struct {
	BytesSent       int64
	BytesReceived   int64
	PacketsSent     int64
	PacketsReceived int64
}

// LogOptions represents options for log retrieval
type LogOptions struct {
	Since      time.Time
	Until      time.Time
	Tail       int
	Follow     bool
	Priority   string
	Unit       string
}

// GetLogs retrieves system logs
func (d *Discovery) GetLogs(ctx context.Context, opts LogOptions) (io.ReadCloser, error) {
	// Will be implemented later
	return nil, fmt.Errorf("not implemented")
}
