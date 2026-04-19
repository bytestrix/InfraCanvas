package host

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"infracanvas/internal/models"
	"infracanvas/pkg/validation"
)

// parseProcess parses process information from /proc/<pid>/
func parseProcess(pid int) (*models.Process, error) {
	process := &models.Process{
		BaseEntity: models.BaseEntity{
			Type:      models.EntityTypeProcess,
			Labels:    make(map[string]string),
			Timestamp: time.Now(),
		},
		PID: pid,
	}

	// Parse /proc/<pid>/stat for CPU and memory
	stat, err := parseProcStat(pid)
	if err != nil {
		return nil, err
	}

	process.Name = stat.Name
	process.PPID = stat.PPID
	process.CPUPercent = stat.CPUPercent
	process.MemoryBytes = stat.MemoryBytes

	// Parse /proc/<pid>/cmdline for command line
	cmdline, err := parseProcCmdline(pid)
	if err == nil {
		process.CommandLine = cmdline
	}

	// Parse /proc/<pid>/status for user and state
	status, err := parseProcStatus(pid)
	if err == nil {
		process.User = status.User
	}

	// Identify process type
	process.ProcessType = identifyProcessType(process.Name, process.CommandLine)

	// Generate entity ID
	process.ID = fmt.Sprintf("process:%d", pid)

	return process, nil
}

// ProcStat represents parsed data from /proc/<pid>/stat
type ProcStat struct {
	Name        string
	PPID        int
	CPUPercent  float64
	MemoryBytes int64
}

// parseProcStat parses /proc/<pid>/stat with validation
func parseProcStat(pid int) (*ProcStat, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return nil, err
	}

	line := string(data)

	// Validate output is not empty
	if err := validation.ValidateCommandOutput(line, "", fmt.Sprintf("/proc/%d/stat", pid)); err != nil {
		validation.LogParseError(err, "proc stat validation")
		return nil, err
	}

	// Parse process name (between parentheses)
	startIdx := strings.Index(line, "(")
	endIdx := strings.LastIndex(line, ")")
	if startIdx == -1 || endIdx == -1 {
		err := fmt.Errorf("invalid stat format: missing parentheses")
		validation.LogParseError(err, fmt.Sprintf("/proc/%d/stat", pid))
		return nil, err
	}

	name := line[startIdx+1 : endIdx]
	
	// Parse remaining fields after the name
	fields := strings.Fields(line[endIdx+2:])
	if len(fields) < 22 {
		err := fmt.Errorf("insufficient fields in stat: expected at least 22, got %d", len(fields))
		validation.LogParseError(err, fmt.Sprintf("/proc/%d/stat", pid))
		return nil, err
	}

	stat := &ProcStat{
		Name: name,
	}

	// Parse PPID (field 1 after name) with validation
	ppid, err := validation.SafeParseInt(fields[1], "ppid", fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		// Log but continue with default value
		stat.PPID = 0
	} else {
		stat.PPID = ppid
	}

	// Parse memory (RSS in pages, field 21 after name) with validation
	rss, err := validation.SafeParseInt64(fields[21], "rss", fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		// Log but continue with default value
		stat.MemoryBytes = 0
	} else {
		pageSize := int64(4096) // Typical page size
		stat.MemoryBytes = rss * pageSize
	}

	// CPU usage calculation would require two samples
	// For now, we'll set it to 0 and calculate it properly in a future enhancement
	stat.CPUPercent = 0

	return stat, nil
}

// parseProcCmdline parses /proc/<pid>/cmdline
func parseProcCmdline(pid int) (string, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return "", err
	}

	// Command line arguments are null-separated
	cmdline := strings.ReplaceAll(string(data), "\x00", " ")
	cmdline = strings.TrimSpace(cmdline)

	return cmdline, nil
}

// ProcStatus represents parsed data from /proc/<pid>/status
type ProcStatus struct {
	User string
}

// parseProcStatus parses /proc/<pid>/status
func parseProcStatus(pid int) (*ProcStatus, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return nil, err
	}

	status := &ProcStatus{}
	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		if strings.HasPrefix(line, "Uid:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				uid := fields[1]
				// Try to resolve UID to username
				username, err := resolveUID(uid)
				if err == nil {
					status.User = username
				} else {
					status.User = uid
				}
			}
			break
		}
	}

	return status, nil
}

// resolveUID resolves a UID to a username
func resolveUID(uid string) (string, error) {
	// Read /etc/passwd to resolve UID
	data, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		fields := strings.Split(line, ":")
		if len(fields) >= 3 && fields[2] == uid {
			return fields[0], nil
		}
	}

	return "", fmt.Errorf("UID not found")
}

// identifyProcessType identifies the type of process based on name and command line
func identifyProcessType(name string, cmdline string) string {
	name = strings.ToLower(name)
	cmdline = strings.ToLower(cmdline)

	// Docker daemon
	if name == "dockerd" || strings.Contains(cmdline, "dockerd") {
		return "docker"
	}

	// Kubernetes kubelet
	if name == "kubelet" || strings.Contains(cmdline, "kubelet") {
		return "kubelet"
	}

	// Containerd
	if name == "containerd" || strings.Contains(cmdline, "containerd") {
		return "containerd"
	}

	// Databases
	if name == "postgres" || strings.Contains(cmdline, "postgres") {
		return "database-postgresql"
	}
	if name == "mysqld" || strings.Contains(cmdline, "mysqld") {
		return "database-mysql"
	}
	if name == "mongod" || strings.Contains(cmdline, "mongod") {
		return "database-mongodb"
	}
	if name == "redis-server" || strings.Contains(cmdline, "redis-server") {
		return "database-redis"
	}

	// Web servers
	if name == "nginx" || strings.Contains(cmdline, "nginx") {
		return "webserver-nginx"
	}
	if name == "apache2" || name == "httpd" || strings.Contains(cmdline, "apache") {
		return "webserver-apache"
	}
	if strings.Contains(cmdline, "tomcat") {
		return "webserver-tomcat"
	}

	// Message queues
	if strings.Contains(cmdline, "rabbitmq") {
		return "messagequeue-rabbitmq"
	}
	if strings.Contains(cmdline, "kafka") {
		return "messagequeue-kafka"
	}

	// SSH daemon
	if name == "sshd" || strings.Contains(cmdline, "sshd") {
		return "sshd"
	}

	// Systemd
	if name == "systemd" {
		return "systemd"
	}

	return ""
}

// GetProcessListeningPorts gets listening ports for a specific process
func (d *Discovery) GetProcessListeningPorts(pid int) ([]int, error) {
	ports := []int{}

	// Read /proc/<pid>/fd/ to find socket file descriptors
	fdDir := fmt.Sprintf("/proc/%d/fd", pid)
	fds, err := os.ReadDir(fdDir)
	if err != nil {
		return nil, err
	}

	// Build set of socket inodes for this process
	socketInodes := make(map[string]bool)
	for _, fd := range fds {
		linkPath := filepath.Join(fdDir, fd.Name())
		target, err := os.Readlink(linkPath)
		if err != nil {
			continue
		}

		if strings.HasPrefix(target, "socket:[") {
			var inode string
			_, _ = fmt.Sscanf(target, "socket:[%s]", &inode)
			inode = strings.TrimSuffix(inode, "]")
			socketInodes[inode] = true
		}
	}

	// Parse /proc/net/tcp and /proc/net/tcp6 to find listening ports
	for _, path := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		lines := strings.Split(string(data), "\n")
		for i := 1; i < len(lines); i++ {
			line := strings.TrimSpace(lines[i])
			if line == "" {
				continue
			}

			fields := strings.Fields(line)
			if len(fields) < 10 {
				continue
			}

			// Check if listening (state 0A)
			state := fields[3]
			if state != "0A" {
				continue
			}

			// Get inode
			inode := fields[9]
			if !socketInodes[inode] {
				continue
			}

			// Parse port
			localAddr := fields[1]
			parts := strings.Split(localAddr, ":")
			if len(parts) != 2 {
				continue
			}

			var port int
			_, _ = fmt.Sscanf(parts[1], "%X", &port)
			ports = append(ports, port)
		}
	}

	return ports, nil
}
