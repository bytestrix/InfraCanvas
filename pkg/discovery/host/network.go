package host

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"infracanvas/internal/models"
	"infracanvas/pkg/validation"
)

// GetNetworkInterfaces collects all network interfaces
func (d *Discovery) GetNetworkInterfaces() ([]models.NetworkInterface, error) {
	interfaces := []models.NetworkInterface{}

	// Read network interfaces from /sys/class/net/
	entries, err := os.ReadDir("/sys/class/net")
	if err != nil {
		return nil, fmt.Errorf("failed to read /sys/class/net: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		ifaceName := entry.Name()
		
		// Skip loopback
		if ifaceName == "lo" {
			continue
		}

		iface := models.NetworkInterface{
			Name: ifaceName,
		}

		// Get MAC address
		macAddr, err := readSysFile(filepath.Join("/sys/class/net", ifaceName, "address"))
		if err == nil {
			iface.MACAddress = macAddr
		}

		// Get interface status
		operstate, err := readSysFile(filepath.Join("/sys/class/net", ifaceName, "operstate"))
		if err == nil {
			iface.Status = operstate
		}

		// Get IP addresses using ip addr
		ipAddrs, err := getIPAddresses(ifaceName)
		if err == nil {
			iface.IPAddresses = ipAddrs
		}

		interfaces = append(interfaces, iface)
	}

	return interfaces, nil
}

// readSysFile reads a single-line file from /sys
func readSysFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// getIPAddresses gets IP addresses for an interface using ip addr with validation
func getIPAddresses(ifaceName string) ([]string, error) {
	cmd := exec.Command("ip", "addr", "show", ifaceName)
	output, err := cmd.Output()
	if err != nil {
		// Fallback to parsing /proc/net/fib_trie if ip command fails
		return getIPAddressesFromProc(ifaceName)
	}

	// Validate command output
	if err := validation.ValidateCommandOutput(string(output), "", fmt.Sprintf("ip addr show %s", ifaceName)); err != nil {
		validation.LogParseError(err, "ip addr output validation")
		return getIPAddressesFromProc(ifaceName)
	}

	ipAddrs := []string{}
	lines, err := validation.SafeSplitLines(string(output), fmt.Sprintf("ip addr show %s", ifaceName))
	if err != nil {
		validation.LogParseError(err, "ip addr output parsing")
		return getIPAddressesFromProc(ifaceName)
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "inet ") || strings.HasPrefix(line, "inet6 ") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				// Extract IP address (remove CIDR notation if present)
				addr := fields[1]
				ipAddrs = append(ipAddrs, addr)
			}
		}
	}

	return ipAddrs, nil
}

// getIPAddressesFromProc parses IP addresses from /proc/net/if_inet6 with validation
func getIPAddressesFromProc(ifaceName string) ([]string, error) {
	// This is a simplified fallback - /proc/net/fib_trie doesn't directly
	// map interfaces to IPs, so we'll try to read from /proc/net/if_inet6
	// for IPv6 and construct IPv4 from other sources
	
	ipAddrs := []string{}

	// Try IPv6 from /proc/net/if_inet6
	data, err := os.ReadFile("/proc/net/if_inet6")
	if err == nil {
		lines, err := validation.SafeSplitLines(string(data), "/proc/net/if_inet6")
		if err != nil {
			validation.LogParseError(err, "/proc/net/if_inet6 parsing")
			return ipAddrs, nil
		}

		for _, line := range lines {
			fields := strings.Fields(line)
			if len(fields) >= 6 && fields[5] == ifaceName {
				// Parse IPv6 address from hex format
				ipv6Hex := fields[0]
				ipv6 := formatIPv6(ipv6Hex)
				ipAddrs = append(ipAddrs, ipv6)
			}
		}
	}

	return ipAddrs, nil
}

// formatIPv6 formats a hex IPv6 address into standard notation
func formatIPv6(hex string) string {
	if len(hex) != 32 {
		return hex
	}

	parts := []string{}
	for i := 0; i < 32; i += 4 {
		parts = append(parts, hex[i:i+4])
	}

	return strings.Join(parts, ":")
}

// GetListeningPorts collects listening ports
func (d *Discovery) GetListeningPorts() ([]models.ListeningPort, error) {
	ports := []models.ListeningPort{}

	// Parse TCP ports
	tcpPorts, err := parseListeningPorts("/proc/net/tcp", "tcp")
	if err == nil {
		ports = append(ports, tcpPorts...)
	}

	// Parse TCP6 ports
	tcp6Ports, err := parseListeningPorts("/proc/net/tcp6", "tcp")
	if err == nil {
		ports = append(ports, tcp6Ports...)
	}

	// Parse UDP ports
	udpPorts, err := parseListeningPorts("/proc/net/udp", "udp")
	if err == nil {
		ports = append(ports, udpPorts...)
	}

	// Parse UDP6 ports
	udp6Ports, err := parseListeningPorts("/proc/net/udp6", "udp")
	if err == nil {
		ports = append(ports, udp6Ports...)
	}

	// Map ports to processes (requires elevated permissions)
	mapPortsToProcesses(ports)

	return ports, nil
}

// parseListeningPorts parses listening ports from /proc/net/tcp or /proc/net/udp with validation
func parseListeningPorts(path string, protocol string) ([]models.ListeningPort, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	ports := []models.ListeningPort{}
	lines, err := validation.SafeSplitLines(string(data), path)
	if err != nil {
		validation.LogParseError(err, fmt.Sprintf("%s parsing", path))
		return nil, err
	}

	// Skip header line
	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		fields, err := validation.SafeSplitFields(line, 10, path)
		if err != nil {
			// Log but continue with other lines
			validation.LogParseError(err, fmt.Sprintf("%s line parsing", path))
			continue
		}

		// Parse local address (format: "0100007F:0050" = 127.0.0.1:80)
		localAddr := fields[1]
		parts := strings.Split(localAddr, ":")
		if len(parts) != 2 {
			continue
		}

		// Parse port from hex
		var port int
		fmt.Sscanf(parts[1], "%X", &port)

		// Validate port range
		if err := validation.ValidateRange(float64(port), 0, 65535, "port", path); err != nil {
			validation.LogParseError(err, fmt.Sprintf("%s port validation", path))
			continue
		}

		// Check if socket is in LISTEN state (0A for TCP)
		state := fields[3]
		if protocol == "tcp" && state != "0A" {
			continue
		}

		// Parse inode for process mapping
		inode := fields[9]

		ports = append(ports, models.ListeningPort{
			Port:     port,
			Protocol: protocol,
		})

		// Store inode for later process mapping
		if len(ports) > 0 {
			// We'll use this inode to map to process later
			_ = inode
		}
	}

	return ports, nil
}

// mapPortsToProcesses maps listening ports to processes using /proc/<pid>/fd/
func mapPortsToProcesses(ports []models.ListeningPort) {
	// Build inode to port index mapping
	inodeToPortIdx := make(map[string]int)
	
	// First pass: collect inodes from /proc/net files
	for protocol, path := range map[string]string{
		"tcp": "/proc/net/tcp",
		"tcp6": "/proc/net/tcp6",
		"udp": "/proc/net/udp",
		"udp6": "/proc/net/udp6",
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		lines := strings.Split(string(data), "\n")
		portIdx := 0

		for i := 1; i < len(lines); i++ {
			line := strings.TrimSpace(lines[i])
			if line == "" {
				continue
			}

			fields := strings.Fields(line)
			if len(fields) < 10 {
				continue
			}

			// Parse local address
			localAddr := fields[1]
			parts := strings.Split(localAddr, ":")
			if len(parts) != 2 {
				continue
			}

			var port int
			fmt.Sscanf(parts[1], "%X", &port)

			// Check if listening
			state := fields[3]
			if strings.HasPrefix(protocol, "tcp") && state != "0A" {
				continue
			}

			// Get inode
			inode := fields[9]
			
			// Find matching port in our list
			for idx, p := range ports {
				if p.Port == port && p.Protocol == strings.TrimSuffix(protocol, "6") {
					inodeToPortIdx[inode] = idx
					break
				}
			}
			portIdx++
		}
	}

	// Second pass: scan /proc/<pid>/fd/ to map inodes to PIDs
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pid := entry.Name()
		if !isNumeric(pid) {
			continue
		}

		// Try to read fd directory (may fail without permissions)
		fdDir := filepath.Join("/proc", pid, "fd")
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			// Graceful degradation: skip processes we can't access
			continue
		}

		// Read process name
		processName := ""
		if cmdline, err := os.ReadFile(filepath.Join("/proc", pid, "cmdline")); err == nil {
			// Parse command line (null-separated)
			parts := strings.Split(string(cmdline), "\x00")
			if len(parts) > 0 && parts[0] != "" {
				processName = filepath.Base(parts[0])
			}
		}

		for _, fd := range fds {
			linkPath := filepath.Join(fdDir, fd.Name())
			target, err := os.Readlink(linkPath)
			if err != nil {
				continue
			}

			// Check if it's a socket
			if strings.HasPrefix(target, "socket:[") {
				var inode string
				fmt.Sscanf(target, "socket:[%s]", &inode)
				inode = strings.TrimSuffix(inode, "]")

				// Map to port
				if portIdx, ok := inodeToPortIdx[inode]; ok {
					var pidInt int
					fmt.Sscanf(pid, "%d", &pidInt)
					ports[portIdx].ProcessID = pidInt
					ports[portIdx].Process = processName
				}
			}
		}
	}
}

// isNumeric checks if a string contains only digits
func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}
