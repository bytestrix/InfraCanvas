package host

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"infracanvas/internal/models"
)

// DiscoverAll performs a complete host discovery
func (d *Discovery) DiscoverAll() (*models.Host, []models.Process, []models.Service, error) {
	// Get host info (required)
	host, err := d.GetHostInfo()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get host info: %w", err)
	}

	// Use goroutines for parallel collection of optional data
	var wg sync.WaitGroup

	// Get cloud metadata (optional)
	wg.Add(1)
	go func() {
		defer wg.Done()
		cloudMetadata, err := d.GetCloudMetadata()
		if err == nil {
			host.CloudProvider = cloudMetadata.Provider
			host.InstanceID = cloudMetadata.InstanceID
			host.InstanceType = cloudMetadata.InstanceType
			host.Region = cloudMetadata.Region
			host.AvailabilityZone = cloudMetadata.AvailabilityZone
			host.CloudTags = cloudMetadata.Tags
		}
	}()

	// Get resource usage
	wg.Add(1)
	go func() {
		defer wg.Done()
		resourceHost, err := d.GetResourceUsage()
		if err == nil {
			host.CPUUsagePercent = resourceHost.CPUUsagePercent
			host.MemoryTotalBytes = resourceHost.MemoryTotalBytes
			host.MemoryUsedBytes = resourceHost.MemoryUsedBytes
			host.MemoryUsagePercent = resourceHost.MemoryUsagePercent
			host.Filesystems = resourceHost.Filesystems
		}
	}()

	// Get network interfaces
	wg.Add(1)
	go func() {
		defer wg.Done()
		interfaces, err := d.GetNetworkInterfaces()
		if err == nil {
			host.NetworkInterfaces = interfaces
		}
	}()

	// Get listening ports
	wg.Add(1)
	go func() {
		defer wg.Done()
		ports, err := d.GetListeningPorts()
		if err == nil {
			host.ListeningPorts = ports
		}
	}()

	var processes []models.Process
	var services []models.Service

	// Get processes
	wg.Add(1)
	go func() {
		defer wg.Done()
		p, err := d.GetProcesses()
		if err != nil {
			processes = []models.Process{}
		} else {
			processes = p
		}
	}()

	// Get services
	wg.Add(1)
	go func() {
		defer wg.Done()
		s, err := d.GetServices()
		if err != nil {
			services = []models.Service{}
		} else {
			services = s
		}
	}()

	// Wait for all parallel operations to complete
	wg.Wait()

	// Cross-link sockets, processes, and services: who listens where, which
	// process belongs to which systemd unit, and who talks to whom locally.
	enrichWithSockets(processes, services)

	// Calculate host health
	host.Health = calculateHostHealth(host)
	host.Timestamp = time.Now()

	return host, processes, services, nil
}

// enrichWithSockets populates listening/outbound ports on processes, assigns
// processes to their owning systemd service (via MainPID + PPID descent), and
// aggregates the service's ports from its process tree.
func enrichWithSockets(processes []models.Process, services []models.Service) {
	socks, err := buildSocketMaps()
	if err != nil {
		return
	}

	byPid := make(map[int]*models.Process, len(processes))
	children := map[int][]int{}
	for i := range processes {
		p := &processes[i]
		byPid[p.PID] = p
		children[p.PPID] = append(children[p.PPID], p.PID)
		p.ListeningPorts = socks.PidListenPorts[p.PID]
		p.OutboundPorts = socks.PidOutboundPorts[p.PID]
	}

	// Walk each service's process tree from MainPID.
	for i := range services {
		svc := &services[i]
		if svc.MainPID <= 0 {
			continue
		}
		// user@N.service is the per-user session manager — descending its tree
		// would swallow every user process (PM2 apps, dev servers) that should
		// stand on their own.
		if strings.HasPrefix(svc.Name, "user@") {
			continue
		}
		stack := []int{svc.MainPID}
		for len(stack) > 0 {
			pid := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if p, ok := byPid[pid]; ok && p.ServiceUnit == "" {
				p.ServiceUnit = svc.Name
				for _, port := range p.ListeningPorts {
					svc.Ports = appendUnique(svc.Ports, port)
				}
				for _, port := range p.OutboundPorts {
					svc.OutboundPorts = appendUnique(svc.OutboundPorts, port)
				}
			}
			stack = append(stack, children[pid]...)
		}
	}
}

// calculateHostHealth calculates the health status of a host
func calculateHostHealth(host *models.Host) models.HealthStatus {
	// Check CPU usage
	if host.CPUUsagePercent > 90 {
		return models.HealthDegraded
	}

	// Check memory usage
	if host.MemoryUsagePercent > 90 {
		return models.HealthDegraded
	}

	// Check disk usage
	for _, fs := range host.Filesystems {
		if fs.UsagePercent > 85 {
			return models.HealthDegraded
		}
	}

	return models.HealthHealthy
}
