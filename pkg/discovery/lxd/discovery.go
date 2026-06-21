// Package lxd discovers LXC/LXD/Incus instances via the local REST API
// exposed over a unix socket. No external dependencies — plain net/http
// with a unix-socket dialer.
package lxd

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"infracanvas/internal/models"
)

// Candidate unix sockets, in priority order. Covers snap LXD, deb LXD, and Incus.
var socketCandidates = []struct {
	path    string
	runtime string
}{
	{"/var/snap/lxd/common/lxd/unix.socket", "lxd"},
	{"/var/lib/lxd/unix.socket", "lxd"},
	{"/run/incus/unix.socket", "incus"},
	{"/var/lib/incus/unix.socket", "incus"},
}

// Discovery implements LXC/LXD/Incus discovery.
type Discovery struct {
	client     *http.Client
	socketPath string
	runtime    string // "lxd" | "incus"
}

// NewDiscovery finds the first available LXD/Incus socket. Returns an unavailable
// (but non-nil) Discovery if none is found, so callers can call IsAvailable().
func NewDiscovery() *Discovery {
	for _, c := range socketCandidates {
		if fi, err := os.Stat(c.path); err == nil && fi.Mode()&os.ModeSocket != 0 {
			sock := c.path
			return &Discovery{
				socketPath: sock,
				runtime:    c.runtime,
				client: &http.Client{
					Timeout: 4 * time.Second,
					Transport: &http.Transport{
						DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
							var d net.Dialer
							return d.DialContext(ctx, "unix", sock)
						},
					},
				},
			}
		}
	}
	return &Discovery{}
}

// IsAvailable reports whether an LXD/Incus daemon answers on the socket.
func (d *Discovery) IsAvailable() bool {
	if d.client == nil || d.socketPath == "" {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://lxd/1.0", nil)
	if err != nil {
		return false
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// instance mirrors the subset of the LXD/Incus instance object we use.
type instance struct {
	Name   string            `json:"name"`
	Status string            `json:"status"` // Running, Stopped, Frozen, ...
	Type   string            `json:"type"`   // container | virtual-machine
	Config map[string]string `json:"config"`
	State  *struct {
		CPU struct {
			Usage int64 `json:"usage"`
		} `json:"cpu"`
		Memory struct {
			Usage int64 `json:"usage"`
		} `json:"memory"`
		Network map[string]struct {
			Counters struct {
				BytesReceived int64 `json:"bytes_received"`
				BytesSent     int64 `json:"bytes_sent"`
			} `json:"counters"`
		} `json:"network"`
	} `json:"state"`
}

type instancesResponse struct {
	Metadata []instance `json:"metadata"`
}

// DiscoverAll returns the runtime entity and all instances as containers.
func (d *Discovery) DiscoverAll() (*models.ContainerRuntime, []models.Container, error) {
	if !d.IsAvailable() {
		return nil, nil, fmt.Errorf("LXD/Incus is not available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	// recursion=2 returns full instance objects including live state.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://lxd/1.0/instances?recursion=2", nil)
	if err != nil {
		return nil, nil, err
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("list instances: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("list instances: status %d", resp.StatusCode)
	}

	var out instancesResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, nil, fmt.Errorf("decode instances: %w", err)
	}

	now := time.Now()
	runtime := &models.ContainerRuntime{
		BaseEntity: models.BaseEntity{
			ID:        "lxd-runtime",
			Type:      models.EntityTypeContainerRuntime,
			Labels:    map[string]string{"runtime": d.runtime},
			Timestamp: now,
		},
		RuntimeType: d.runtime,
		SocketPath:  d.socketPath,
	}

	containers := make([]models.Container, 0, len(out.Metadata))
	for _, in := range out.Metadata {
		state := "running"
		if !strings.EqualFold(in.Status, "Running") {
			state = strings.ToLower(in.Status)
		}

		var memUsage, rx, tx int64
		if in.State != nil {
			memUsage = in.State.Memory.Usage
			for iface, n := range in.State.Network {
				if iface == "lo" {
					continue
				}
				rx += n.Counters.BytesReceived
				tx += n.Counters.BytesSent
			}
		}

		image := in.Config["image.description"]
		if image == "" {
			os := in.Config["image.os"]
			rel := in.Config["image.release"]
			if os != "" {
				image = strings.TrimSpace(os + " " + rel)
			}
		}

		kind := in.Type
		if kind == "" {
			kind = "container"
		}

		containers = append(containers, models.Container{
			BaseEntity: models.BaseEntity{
				ID:        d.runtime + ":" + in.Name,
				Type:      models.EntityTypeContainer,
				Labels:    map[string]string{"runtime": d.runtime, "lxdType": kind},
				Timestamp: now,
			},
			ContainerID: in.Name,
			Name:        in.Name,
			Image:       image,
			State:       state,
			Status:      in.Status,
			MemoryUsage: memUsage,
			NetworkRxBytes: rx,
			NetworkTxBytes: tx,
			Environment:    map[string]string{},
		})
	}

	return runtime, containers, nil
}
