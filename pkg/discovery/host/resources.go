package host

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"infracanvas/internal/models"
	"infracanvas/pkg/validation"
)

// getResourceUsage collects current CPU, memory, and disk usage (internal implementation)
func getResourceUsage() (*ResourceUsage, error) {
	usage := &ResourceUsage{}

	// Collect CPU usage
	cpuUsage, err := getCPUUsage()
	if err == nil {
		usage.CPUUsagePercent = cpuUsage
	}

	// Collect memory usage
	memTotal, memUsed, memPercent, err := getMemoryUsage()
	if err == nil {
		usage.MemoryTotalBytes = memTotal
		usage.MemoryUsedBytes = memUsed
		usage.MemoryUsagePercent = memPercent
	}

	// Collect disk usage
	filesystems, err := getFilesystems()
	if err == nil {
		usage.Filesystems = filesystems
	}

	return usage, nil
}

// getCPUUsage calculates CPU usage percentage from /proc/stat with validation
func getCPUUsage() (float64, error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, fmt.Errorf("failed to read /proc/stat: %w", err)
	}

	lines, err := validation.SafeSplitLines(string(data), "/proc/stat")
	if err != nil || len(lines) == 0 {
		validation.LogParseError(err, "/proc/stat parsing")
		return 0, fmt.Errorf("empty /proc/stat")
	}

	// Parse first line (aggregate CPU stats)
	cpuLine := lines[0]
	if !strings.HasPrefix(cpuLine, "cpu ") {
		err := fmt.Errorf("invalid /proc/stat format: expected 'cpu ' prefix")
		validation.LogParseError(err, "/proc/stat")
		return 0, err
	}

	fields, err := validation.SafeSplitFields(cpuLine, 8, "/proc/stat cpu line")
	if err != nil {
		validation.LogParseError(err, "/proc/stat cpu line parsing")
		return 0, err
	}

	// Parse CPU times with validation
	user, err := validation.SafeParseInt64(fields[1], "user", "/proc/stat")
	if err != nil {
		user = 0
	}
	
	nice, err := validation.SafeParseInt64(fields[2], "nice", "/proc/stat")
	if err != nil {
		nice = 0
	}
	
	system, err := validation.SafeParseInt64(fields[3], "system", "/proc/stat")
	if err != nil {
		system = 0
	}
	
	idle, err := validation.SafeParseInt64(fields[4], "idle", "/proc/stat")
	if err != nil {
		idle = 0
	}
	
	iowait, err := validation.SafeParseInt64(fields[5], "iowait", "/proc/stat")
	if err != nil {
		iowait = 0
	}
	
	irq, err := validation.SafeParseInt64(fields[6], "irq", "/proc/stat")
	if err != nil {
		irq = 0
	}
	
	softirq, err := validation.SafeParseInt64(fields[7], "softirq", "/proc/stat")
	if err != nil {
		softirq = 0
	}

	// Calculate total and idle time
	totalTime := user + nice + system + idle + iowait + irq + softirq
	idleTime := idle + iowait

	// Calculate usage percentage
	if totalTime == 0 {
		return 0, nil
	}

	usagePercent := float64(totalTime-idleTime) / float64(totalTime) * 100.0

	// Validate range
	if err := validation.ValidateRange(usagePercent, 0.0, 100.0, "cpu_usage_percent", "/proc/stat"); err != nil {
		validation.LogParseError(err, "CPU usage validation")
		return 0, err
	}

	return usagePercent, nil
}

// getMemoryUsage collects memory usage from /proc/meminfo with validation
func getMemoryUsage() (total int64, used int64, percent float64, err error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to read /proc/meminfo: %w", err)
	}

	var memTotal, memFree, memAvailable, buffers, cached int64

	lines, err := validation.SafeSplitLines(string(data), "/proc/meminfo")
	if err != nil {
		validation.LogParseError(err, "/proc/meminfo parsing")
		return 0, 0, 0, err
	}

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		key := strings.TrimSuffix(fields[0], ":")
		value, err := validation.SafeParseInt64(fields[1], key, "/proc/meminfo")
		if err != nil {
			// Log but continue with other fields
			continue
		}

		// Convert from KB to bytes
		value *= 1024

		switch key {
		case "MemTotal":
			memTotal = value
		case "MemFree":
			memFree = value
		case "MemAvailable":
			memAvailable = value
		case "Buffers":
			buffers = value
		case "Cached":
			cached = value
		}
	}

	// Calculate used memory
	// If MemAvailable is present, use it for more accurate calculation
	if memAvailable > 0 {
		used = memTotal - memAvailable
	} else {
		// Fallback: MemTotal - MemFree - Buffers - Cached
		used = memTotal - memFree - buffers - cached
	}

	// Calculate percentage
	if memTotal > 0 {
		percent = float64(used) / float64(memTotal) * 100.0
		
		// Validate range
		if err := validation.ValidateRange(percent, 0.0, 100.0, "memory_usage_percent", "/proc/meminfo"); err != nil {
			validation.LogParseError(err, "memory usage validation")
			return memTotal, used, 0, err
		}
	}

	return memTotal, used, percent, nil
}

// getFilesystems collects disk usage for all mounted filesystems with validation
func getFilesystems() ([]models.Filesystem, error) {
	// Read /proc/mounts to get all mounted filesystems
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return nil, fmt.Errorf("failed to read /proc/mounts: %w", err)
	}

	filesystems := []models.Filesystem{}
	lines, err := validation.SafeSplitLines(string(data), "/proc/mounts")
	if err != nil {
		validation.LogParseError(err, "/proc/mounts parsing")
		return nil, err
	}

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		device := fields[0]
		mountPoint := fields[1]
		fsType := fields[2]

		// Skip pseudo filesystems
		if isPseudoFilesystem(fsType) {
			continue
		}

		// Skip special mount points
		if strings.HasPrefix(mountPoint, "/proc") ||
			strings.HasPrefix(mountPoint, "/sys") ||
			strings.HasPrefix(mountPoint, "/dev") ||
			strings.HasPrefix(mountPoint, "/run") {
			continue
		}

		// Get disk usage using syscall.Statfs
		var stat syscall.Statfs_t
		err := syscall.Statfs(mountPoint, &stat)
		if err != nil {
			// Skip filesystems we can't stat
			continue
		}

		// Calculate sizes
		totalBytes := int64(stat.Blocks) * int64(stat.Bsize)
		availBytes := int64(stat.Bavail) * int64(stat.Bsize)
		usedBytes := totalBytes - (int64(stat.Bfree) * int64(stat.Bsize))

		// Calculate usage percentage
		var usagePercent float64
		if totalBytes > 0 {
			usagePercent = float64(usedBytes) / float64(totalBytes) * 100.0
			
			// Validate range
			if err := validation.ValidateRange(usagePercent, 0.0, 100.0, "disk_usage_percent", mountPoint); err != nil {
				validation.LogParseError(err, fmt.Sprintf("disk usage validation for %s", mountPoint))
				usagePercent = 0
			}
		}

		filesystems = append(filesystems, models.Filesystem{
			MountPoint:   mountPoint,
			Device:       device,
			FSType:       fsType,
			TotalBytes:   totalBytes,
			UsedBytes:    usedBytes,
			AvailBytes:   availBytes,
			UsagePercent: usagePercent,
		})
	}

	return filesystems, nil
}

// isPseudoFilesystem checks if a filesystem type is a pseudo filesystem
func isPseudoFilesystem(fsType string) bool {
	pseudoFS := []string{
		"proc", "sysfs", "devpts", "devtmpfs", "tmpfs",
		"cgroup", "cgroup2", "pstore", "bpf", "tracefs",
		"debugfs", "securityfs", "sockfs", "pipefs",
		"configfs", "selinuxfs", "autofs", "mqueue",
		"hugetlbfs", "rpc_pipefs", "binfmt_misc",
	}

	for _, pseudo := range pseudoFS {
		if fsType == pseudo {
			return true
		}
	}

	return false
}

// GetNetworkIOStats collects network I/O statistics from /proc/net/dev with validation
func (d *Discovery) GetNetworkIOStats() (map[string]*NetworkIOStats, error) {
	data, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return nil, fmt.Errorf("failed to read /proc/net/dev: %w", err)
	}

	stats := make(map[string]*NetworkIOStats)
	lines, err := validation.SafeSplitLines(string(data), "/proc/net/dev")
	if err != nil {
		validation.LogParseError(err, "/proc/net/dev parsing")
		return nil, err
	}

	// Skip first two header lines
	for i := 2; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		// Parse interface name
		parts := strings.Split(line, ":")
		if len(parts) != 2 {
			continue
		}

		ifaceName := strings.TrimSpace(parts[0])
		fields := strings.Fields(parts[1])

		if len(fields) < 16 {
			validation.LogParseError(
				fmt.Errorf("insufficient fields: expected 16, got %d", len(fields)),
				fmt.Sprintf("/proc/net/dev interface %s", ifaceName),
			)
			continue
		}

		// Parse statistics with validation
		rxBytes, err := validation.SafeParseInt64(fields[0], "rx_bytes", fmt.Sprintf("/proc/net/dev:%s", ifaceName))
		if err != nil {
			rxBytes = 0
		}
		
		rxPackets, err := validation.SafeParseInt64(fields[1], "rx_packets", fmt.Sprintf("/proc/net/dev:%s", ifaceName))
		if err != nil {
			rxPackets = 0
		}
		
		txBytes, err := validation.SafeParseInt64(fields[8], "tx_bytes", fmt.Sprintf("/proc/net/dev:%s", ifaceName))
		if err != nil {
			txBytes = 0
		}
		
		txPackets, err := validation.SafeParseInt64(fields[9], "tx_packets", fmt.Sprintf("/proc/net/dev:%s", ifaceName))
		if err != nil {
			txPackets = 0
		}

		stats[ifaceName] = &NetworkIOStats{
			BytesReceived:   rxBytes,
			BytesSent:       txBytes,
			PacketsReceived: rxPackets,
			PacketsSent:     txPackets,
		}
	}

	return stats, nil
}

// NetworkIOStats represents network I/O statistics for an interface
type NetworkIOStats struct {
	BytesReceived   int64
	BytesSent       int64
	PacketsReceived int64
	PacketsSent     int64
}
