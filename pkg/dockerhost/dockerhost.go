// Package dockerhost resolves which container-engine socket to talk to.
//
// Podman ships a Docker-Engine-API-compatible socket ("podman system
// service"), so the existing Docker SDK client works against it unchanged —
// the only thing missing was knowing where to look for it when the user
// hasn't set DOCKER_HOST and there's no Docker daemon running. Rootless is
// Podman's default mode, so its per-user socket is checked before the
// rootful one.
package dockerhost

import (
	"fmt"
	"os"
)

// Resolve returns a "unix://" DOCKER_HOST-style address to connect to, or ""
// if the caller should fall back to its own default behavior. It only ever
// returns non-empty for a Podman socket: DOCKER_HOST (if set) and the
// standard Docker socket (if present) are left for the caller's existing
// logic to handle, since neither needs Podman-specific detection.
func Resolve() string {
	if os.Getenv("DOCKER_HOST") != "" {
		return ""
	}
	if isSocket("/var/run/docker.sock") {
		return ""
	}
	for _, path := range podmanSocketPaths() {
		if isSocket(path) {
			return "unix://" + path
		}
	}
	return ""
}

func podmanSocketPaths() []string {
	paths := make([]string, 0, 3)
	if uid := os.Getuid(); uid >= 0 {
		paths = append(paths, fmt.Sprintf("/run/user/%d/podman/podman.sock", uid))
	}
	paths = append(paths, "/run/podman/podman.sock", "/var/run/podman/podman.sock")
	return paths
}

func isSocket(path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeSocket != 0
}
