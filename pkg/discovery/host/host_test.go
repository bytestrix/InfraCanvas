package host

import (
	"testing"
)

func TestNewDiscovery(t *testing.T) {
	d := NewDiscovery()
	if d == nil {
		t.Fatal("NewDiscovery returned nil")
	}
}

func TestIsAvailable(t *testing.T) {
	d := NewDiscovery()
	if !d.IsAvailable() {
		t.Error("IsAvailable should always return true for host discovery")
	}
}

func TestGetHostInfo(t *testing.T) {
	d := NewDiscovery()
	host, err := d.GetHostInfo()
	if err != nil {
		t.Fatalf("GetHostInfo failed: %v", err)
	}

	if host == nil {
		t.Fatal("GetHostInfo returned nil host")
	}

	if host.Hostname == "" {
		t.Error("Hostname should not be empty")
	}

	if host.Architecture == "" {
		t.Error("Architecture should not be empty")
	}

	if host.KernelVersion == "" {
		t.Error("KernelVersion should not be empty")
	}

	if host.CPUModel == "" {
		t.Error("CPUModel should not be empty")
	}

	if host.CPUCores == 0 {
		t.Error("CPUCores should not be zero")
	}

	t.Logf("Host: %s, Arch: %s, Kernel: %s, CPU: %s (%d cores)", 
		host.Hostname, host.Architecture, host.KernelVersion, host.CPUModel, host.CPUCores)
}

func TestGetNetworkInterfaces(t *testing.T) {
	d := NewDiscovery()
	interfaces, err := d.GetNetworkInterfaces()
	if err != nil {
		t.Fatalf("GetNetworkInterfaces failed: %v", err)
	}

	if len(interfaces) == 0 {
		t.Log("Warning: No network interfaces found (excluding loopback)")
	}

	for _, iface := range interfaces {
		t.Logf("Interface: %s, MAC: %s, Status: %s, IPs: %v", 
			iface.Name, iface.MACAddress, iface.Status, iface.IPAddresses)
	}
}

func TestGetResourceUsage(t *testing.T) {
	d := NewDiscovery()
	usage, err := d.GetResourceUsage()
	if err != nil {
		t.Fatalf("GetResourceUsage failed: %v", err)
	}

	if usage == nil {
		t.Fatal("GetResourceUsage returned nil")
	}

	t.Logf("CPU: %.2f%%, Memory: %d/%d bytes (%.2f%%)", 
		usage.CPUUsagePercent, 
		usage.MemoryUsedBytes, 
		usage.MemoryTotalBytes, 
		usage.MemoryUsagePercent)

	if len(usage.Filesystems) > 0 {
		t.Logf("Found %d filesystems", len(usage.Filesystems))
		for _, fs := range usage.Filesystems {
			t.Logf("  %s: %s, %.2f%% used", fs.MountPoint, fs.FSType, fs.UsagePercent)
		}
	}
}

func TestGetProcesses(t *testing.T) {
	d := NewDiscovery()
	processes, err := d.GetProcesses()
	if err != nil {
		t.Fatalf("GetProcesses failed: %v", err)
	}

	if len(processes) == 0 {
		t.Error("No processes found")
	}

	t.Logf("Found %d processes", len(processes))

	// Count processes by type
	typeCount := make(map[string]int)
	for _, proc := range processes {
		if proc.ProcessType != "" {
			typeCount[proc.ProcessType]++
		}
	}

	for ptype, count := range typeCount {
		t.Logf("  %s: %d", ptype, count)
	}
}

func TestDiscoverAll(t *testing.T) {
	d := NewDiscovery()
	host, processes, services, err := d.DiscoverAll()
	if err != nil {
		t.Fatalf("DiscoverAll failed: %v", err)
	}

	if host == nil {
		t.Fatal("DiscoverAll returned nil host")
	}

	t.Logf("Host: %s", host.Hostname)
	t.Logf("Processes: %d", len(processes))
	t.Logf("Services: %d", len(services))
	t.Logf("Network Interfaces: %d", len(host.NetworkInterfaces))
	t.Logf("Listening Ports: %d", len(host.ListeningPorts))
	t.Logf("Filesystems: %d", len(host.Filesystems))
	t.Logf("Health: %s", host.Health)
}
