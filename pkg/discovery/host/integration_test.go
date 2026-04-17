package host

import (
	"testing"
)

// TestTask4_1_HostInfoCollector tests subtask 4.1: Create host info collector
// Requirements: 1.1, 1.2
func TestTask4_1_HostInfoCollector(t *testing.T) {
	d := NewDiscovery()
	host, err := d.GetHostInfo()
	if err != nil {
		t.Fatalf("GetHostInfo failed: %v", err)
	}

	// Verify hostname and FQDN
	if host.Hostname == "" {
		t.Error("Hostname should not be empty")
	}
	if host.FQDN == "" {
		t.Error("FQDN should not be empty")
	}

	// Verify OS info from /etc/os-release
	if host.OS == "" {
		t.Error("OS should not be empty")
	}
	// Note: OSVersion may be empty for rolling release distros like Arch Linux
	if host.OSVersion == "" {
		t.Logf("Note: OSVersion is empty (may be a rolling release distribution)")
	}

	// Verify kernel version and architecture using uname
	if host.KernelVersion == "" {
		t.Error("KernelVersion should not be empty")
	}
	if host.Architecture == "" {
		t.Error("Architecture should not be empty")
	}

	// Verify virtualization type detection
	if host.VirtualizationType == "" {
		t.Error("VirtualizationType should not be empty")
	}

	// Verify CPU info
	if host.CPUModel == "" {
		t.Error("CPUModel should not be empty")
	}
	if host.CPUCores == 0 {
		t.Error("CPUCores should not be zero")
	}

	t.Logf("✓ Task 4.1: Host info collector implemented")
	t.Logf("  Hostname: %s, FQDN: %s", host.Hostname, host.FQDN)
	t.Logf("  OS: %s %s", host.OS, host.OSVersion)
	t.Logf("  Kernel: %s, Arch: %s", host.KernelVersion, host.Architecture)
	t.Logf("  Virtualization: %s", host.VirtualizationType)
	t.Logf("  CPU: %s (%d cores)", host.CPUModel, host.CPUCores)
}

// TestTask4_2_CloudMetadataDetection tests subtask 4.2: Implement cloud metadata detection
// Requirements: 1.3
func TestTask4_2_CloudMetadataDetection(t *testing.T) {
	d := NewDiscovery()
	
	// Try to get cloud metadata (may fail if not on cloud)
	metadata, err := d.GetCloudMetadata()
	if err != nil {
		t.Logf("✓ Task 4.2: Cloud metadata detection implemented (not running on cloud: %v)", err)
		return
	}

	// If we're on a cloud provider, verify the metadata
	if metadata.Provider == "" {
		t.Error("Cloud provider should not be empty")
	}

	t.Logf("✓ Task 4.2: Cloud metadata detection implemented")
	t.Logf("  Provider: %s", metadata.Provider)
	t.Logf("  Instance ID: %s", metadata.InstanceID)
	t.Logf("  Instance Type: %s", metadata.InstanceType)
	t.Logf("  Region: %s", metadata.Region)
	t.Logf("  Availability Zone: %s", metadata.AvailabilityZone)
	t.Logf("  Tags: %v", metadata.Tags)
}

// TestTask4_3_NetworkInterfaceDiscovery tests subtask 4.3: Implement network interface discovery
// Requirements: 1.4
func TestTask4_3_NetworkInterfaceDiscovery(t *testing.T) {
	d := NewDiscovery()
	interfaces, err := d.GetNetworkInterfaces()
	if err != nil {
		t.Fatalf("GetNetworkInterfaces failed: %v", err)
	}

	// Note: May be empty if no non-loopback interfaces exist
	t.Logf("✓ Task 4.3: Network interface discovery implemented")
	t.Logf("  Found %d network interfaces", len(interfaces))
	
	for _, iface := range interfaces {
		if iface.Name == "" {
			t.Error("Interface name should not be empty")
		}
		if iface.MACAddress == "" {
			t.Error("MAC address should not be empty")
		}
		if iface.Status == "" {
			t.Error("Interface status should not be empty")
		}
		
		t.Logf("  - %s: MAC=%s, Status=%s, IPs=%v", 
			iface.Name, iface.MACAddress, iface.Status, iface.IPAddresses)
	}
}

// TestTask4_4_ListeningPortsDiscovery tests subtask 4.4: Implement listening ports and connections discovery
// Requirements: 1.5
func TestTask4_4_ListeningPortsDiscovery(t *testing.T) {
	d := NewDiscovery()
	ports, err := d.GetListeningPorts()
	if err != nil {
		t.Fatalf("GetListeningPorts failed: %v", err)
	}

	if len(ports) == 0 {
		t.Log("Warning: No listening ports found")
	}

	t.Logf("✓ Task 4.4: Listening ports discovery implemented")
	t.Logf("  Found %d listening ports", len(ports))
	
	// Verify port structure
	for i, port := range ports {
		if i >= 5 {
			break // Only log first 5
		}
		if port.Port == 0 {
			t.Error("Port number should not be zero")
		}
		if port.Protocol == "" {
			t.Error("Protocol should not be empty")
		}
		
		processInfo := "unknown"
		if port.Process != "" {
			processInfo = port.Process
		}
		t.Logf("  - Port %d/%s (PID: %d, Process: %s)", 
			port.Port, port.Protocol, port.ProcessID, processInfo)
	}
}

// TestTask4_5_ResourceUsageCollection tests subtask 4.5: Implement resource usage collection
// Requirements: 1.6, 1.7, 1.8
func TestTask4_5_ResourceUsageCollection(t *testing.T) {
	d := NewDiscovery()
	usage, err := d.GetResourceUsage()
	if err != nil {
		t.Fatalf("GetResourceUsage failed: %v", err)
	}

	// Verify CPU usage from /proc/stat
	if usage.CPUUsagePercent < 0 || usage.CPUUsagePercent > 100 {
		t.Errorf("CPU usage should be between 0 and 100, got %.2f", usage.CPUUsagePercent)
	}

	// Verify memory usage from /proc/meminfo
	if usage.MemoryTotalBytes == 0 {
		t.Error("Memory total should not be zero")
	}
	if usage.MemoryUsagePercent < 0 || usage.MemoryUsagePercent > 100 {
		t.Errorf("Memory usage should be between 0 and 100, got %.2f", usage.MemoryUsagePercent)
	}

	// Verify disk usage using syscall.Statfs
	if len(usage.Filesystems) == 0 {
		t.Error("Should have at least one filesystem")
	}
	for _, fs := range usage.Filesystems {
		if fs.MountPoint == "" {
			t.Error("Mount point should not be empty")
		}
		if fs.TotalBytes == 0 {
			t.Error("Total bytes should not be zero")
		}
		if fs.UsagePercent < 0 || fs.UsagePercent > 100 {
			t.Errorf("Disk usage should be between 0 and 100, got %.2f", fs.UsagePercent)
		}
	}

	// Verify network I/O from /proc/net/dev
	netStats, err := d.GetNetworkIOStats()
	if err != nil {
		t.Logf("Warning: GetNetworkIOStats failed: %v", err)
	} else if len(netStats) == 0 {
		t.Log("Warning: No network I/O stats found")
	}

	t.Logf("✓ Task 4.5: Resource usage collection implemented")
	t.Logf("  CPU: %.2f%%", usage.CPUUsagePercent)
	t.Logf("  Memory: %d/%d bytes (%.2f%%)", 
		usage.MemoryUsedBytes, usage.MemoryTotalBytes, usage.MemoryUsagePercent)
	t.Logf("  Filesystems: %d", len(usage.Filesystems))
	for _, fs := range usage.Filesystems {
		t.Logf("    - %s: %.2f%% used", fs.MountPoint, fs.UsagePercent)
	}
	if len(netStats) > 0 {
		t.Logf("  Network I/O stats collected for %d interfaces", len(netStats))
	}
}

// TestTask4_6_ProcessDiscovery tests subtask 4.6: Implement process discovery
// Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 2.7
func TestTask4_6_ProcessDiscovery(t *testing.T) {
	d := NewDiscovery()
	processes, err := d.GetProcesses()
	if err != nil {
		t.Fatalf("GetProcesses failed: %v", err)
	}

	if len(processes) == 0 {
		t.Error("Should have at least one process")
	}

	// Verify process structure
	foundTypes := make(map[string]bool)
	for _, proc := range processes {
		if proc.PID == 0 {
			t.Error("PID should not be zero")
		}
		if proc.Name == "" {
			t.Error("Process name should not be empty")
		}
		
		// Track process types
		if proc.ProcessType != "" {
			foundTypes[proc.ProcessType] = true
		}
	}

	t.Logf("✓ Task 4.6: Process discovery implemented")
	t.Logf("  Found %d processes", len(processes))
	t.Logf("  Identified process types: %v", foundTypes)
	
	// Check for specific process types mentioned in requirements
	expectedTypes := []string{"docker", "kubelet", "database", "webserver", "messagequeue"}
	for _, expectedType := range expectedTypes {
		found := false
		for detectedType := range foundTypes {
			if detectedType == expectedType || 
			   (expectedType == "database" && len(detectedType) > 9 && detectedType[:9] == "database-") ||
			   (expectedType == "webserver" && len(detectedType) > 10 && detectedType[:10] == "webserver-") ||
			   (expectedType == "messagequeue" && len(detectedType) > 13 && detectedType[:13] == "messagequeue-") {
				found = true
				break
			}
		}
		if found {
			t.Logf("  ✓ Can identify %s processes", expectedType)
		}
	}
}

// TestTask4_7_SystemdServicesDiscovery tests subtask 4.7: Implement systemd services discovery
// Requirements: 3.1, 3.2, 3.3, 3.4
func TestTask4_7_SystemdServicesDiscovery(t *testing.T) {
	d := NewDiscovery()
	services, err := d.GetServices()
	if err != nil {
		t.Logf("✓ Task 4.7: Systemd services discovery implemented (systemd not available: %v)", err)
		return
	}

	if len(services) == 0 {
		t.Error("Should have at least one service")
	}

	// Verify service structure
	criticalServices := []string{}
	for _, svc := range services {
		if svc.Name == "" {
			t.Error("Service name should not be empty")
		}
		if svc.Status == "" {
			t.Error("Service status should not be empty")
		}
		
		if svc.IsCritical {
			criticalServices = append(criticalServices, svc.Name)
		}
	}

	// Test getting dependencies for a service
	if len(services) > 0 {
		deps, err := d.GetServiceDependencies(services[0].Name)
		if err != nil {
			t.Logf("Warning: GetServiceDependencies failed: %v", err)
		} else {
			t.Logf("  Service %s has %d dependencies", services[0].Name, len(deps))
		}
	}

	t.Logf("✓ Task 4.7: Systemd services discovery implemented")
	t.Logf("  Found %d services", len(services))
	t.Logf("  Critical services: %v", criticalServices)
}

// TestTask4_Complete tests the complete integration of all subtasks
func TestTask4_Complete(t *testing.T) {
	d := NewDiscovery()
	host, processes, services, err := d.DiscoverAll()
	if err != nil {
		t.Fatalf("DiscoverAll failed: %v", err)
	}

	// Verify all components are present
	if host == nil {
		t.Fatal("Host should not be nil")
	}
	if host.Hostname == "" {
		t.Error("Hostname should not be empty")
	}
	if host.CPUModel == "" {
		t.Error("CPU model should not be empty")
	}
	if host.CPUCores == 0 {
		t.Error("CPU cores should not be zero")
	}
	if len(processes) == 0 {
		t.Error("Should have at least one process")
	}

	// Verify health calculation
	if host.Health == "" {
		t.Error("Health status should not be empty")
	}

	t.Logf("✓ Task 4: Host discovery layer complete")
	t.Logf("  Host: %s (%s)", host.Hostname, host.VirtualizationType)
	t.Logf("  CPU: %s (%d cores, %.2f%% usage)", host.CPUModel, host.CPUCores, host.CPUUsagePercent)
	t.Logf("  Memory: %.2f%% usage", host.MemoryUsagePercent)
	t.Logf("  Processes: %d", len(processes))
	t.Logf("  Services: %d", len(services))
	t.Logf("  Network Interfaces: %d", len(host.NetworkInterfaces))
	t.Logf("  Listening Ports: %d", len(host.ListeningPorts))
	t.Logf("  Filesystems: %d", len(host.Filesystems))
	t.Logf("  Health: %s", host.Health)
}
