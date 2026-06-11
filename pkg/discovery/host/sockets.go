package host

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// tcpStateListen and tcpStateEstablished are socket states in /proc/net/tcp.
const (
	tcpStateEstablished = "01"
	tcpStateListen      = "0A"
)

// socketMaps holds the result of one pass over /proc/net/tcp* and /proc/*/fd.
type socketMaps struct {
	// PidListenPorts maps pid → TCP ports it is listening on.
	PidListenPorts map[int][]int
	// PidOutboundPorts maps pid → local destination ports of its established
	// connections (only destinations that some local process is listening on).
	PidOutboundPorts map[int][]int
}

// buildSocketMaps scans /proc/net/tcp{,6} once and walks /proc/<pid>/fd once,
// producing pid→listening-ports and pid→outbound-local-ports maps. This is the
// bulk equivalent of GetProcessListeningPorts without the per-pid /proc walks.
func buildSocketMaps() (*socketMaps, error) {
	listenInodes := map[string]int{} // socket inode → listening port
	estabInodes := map[string]int{}  // socket inode → remote port
	listenPorts := map[int]bool{}

	for _, path := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")
		for i := 1; i < len(lines); i++ {
			fields := strings.Fields(strings.TrimSpace(lines[i]))
			if len(fields) < 10 {
				continue
			}
			state := fields[3]
			inode := fields[9]
			switch state {
			case tcpStateListen:
				if port, ok := hexPort(fields[1]); ok {
					listenInodes[inode] = port
					listenPorts[port] = true
				}
			case tcpStateEstablished:
				if port, ok := hexPort(fields[2]); ok {
					estabInodes[inode] = port
				}
			}
		}
	}

	if len(listenInodes) == 0 && len(estabInodes) == 0 {
		return nil, fmt.Errorf("no socket data readable under /proc/net")
	}

	maps := &socketMaps{
		PidListenPorts:   map[int][]int{},
		PidOutboundPorts: map[int][]int{},
	}

	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() || !isNumeric(entry.Name()) {
			continue
		}
		var pid int
		_, _ = fmt.Sscanf(entry.Name(), "%d", &pid)
		fdDir := filepath.Join("/proc", entry.Name(), "fd")
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			continue // permission denied or process gone
		}
		for _, fd := range fds {
			target, err := os.Readlink(filepath.Join(fdDir, fd.Name()))
			if err != nil || !strings.HasPrefix(target, "socket:[") {
				continue
			}
			inode := strings.TrimSuffix(strings.TrimPrefix(target, "socket:["), "]")
			if port, ok := listenInodes[inode]; ok {
				maps.PidListenPorts[pid] = appendUnique(maps.PidListenPorts[pid], port)
			}
			if port, ok := estabInodes[inode]; ok && listenPorts[port] {
				// Only keep destinations served by this machine — those become
				// topology edges; external destinations are noise.
				maps.PidOutboundPorts[pid] = appendUnique(maps.PidOutboundPorts[pid], port)
			}
		}
	}
	return maps, nil
}

// hexPort extracts the port from a /proc/net/tcp address ("0100007F:1F90").
func hexPort(addr string) (int, bool) {
	idx := strings.LastIndex(addr, ":")
	if idx < 0 {
		return 0, false
	}
	var port int
	if _, err := fmt.Sscanf(addr[idx+1:], "%X", &port); err != nil {
		return 0, false
	}
	if port <= 0 || port > 65535 {
		return 0, false
	}
	return port, true
}

func appendUnique(s []int, v int) []int {
	for _, x := range s {
		if x == v {
			return s
		}
	}
	return append(s, v)
}
