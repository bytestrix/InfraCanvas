package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
)

// ContainerStats represents container resource usage statistics
type ContainerStats struct {
	CPUPercent      float64
	MemoryUsage     int64
	MemoryLimit     int64
	NetworkRxBytes  int64
	NetworkTxBytes  int64
	BlockReadBytes  int64
	BlockWriteBytes int64
}

// GetContainerStats collects resource usage statistics for a container
func (d *Discovery) GetContainerStats(ctx context.Context, containerID string) (*ContainerStats, error) {
	// Get stats with stream=false for a single snapshot
	stats, err := d.client.ContainerStats(ctx, containerID, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get container stats: %w", err)
	}
	defer stats.Body.Close()
	
	// Read and parse stats
	data, err := io.ReadAll(stats.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read stats: %w", err)
	}
	
	var dockerStats struct {
		CPUStats struct {
			CPUUsage struct {
				TotalUsage uint64 `json:"total_usage"`
			} `json:"cpu_usage"`
			SystemCPUUsage uint64 `json:"system_cpu_usage"`
			OnlineCPUs     uint64 `json:"online_cpus"`
		} `json:"cpu_stats"`
		PreCPUStats struct {
			CPUUsage struct {
				TotalUsage uint64 `json:"total_usage"`
			} `json:"cpu_usage"`
			SystemCPUUsage uint64 `json:"system_cpu_usage"`
		} `json:"precpu_stats"`
		MemoryStats struct {
			Usage int64 `json:"usage"`
			Limit int64 `json:"limit"`
		} `json:"memory_stats"`
		Networks map[string]struct {
			RxBytes uint64 `json:"rx_bytes"`
			TxBytes uint64 `json:"tx_bytes"`
		} `json:"networks"`
		BlkioStats struct {
			IoServiceBytesRecursive []struct {
				Op    string `json:"op"`
				Value uint64 `json:"value"`
			} `json:"io_service_bytes_recursive"`
		} `json:"blkio_stats"`
	}
	
	if err := json.Unmarshal(data, &dockerStats); err != nil {
		return nil, fmt.Errorf("failed to parse stats: %w", err)
	}
	
	// Calculate CPU percentage
	cpuPercent := calculateCPUPercent(
		dockerStats.CPUStats.CPUUsage.TotalUsage,
		dockerStats.PreCPUStats.CPUUsage.TotalUsage,
		dockerStats.CPUStats.SystemCPUUsage,
		dockerStats.PreCPUStats.SystemCPUUsage,
		dockerStats.CPUStats.OnlineCPUs,
	)
	
	// Calculate network I/O
	var networkRx, networkTx uint64
	for _, netStats := range dockerStats.Networks {
		networkRx += netStats.RxBytes
		networkTx += netStats.TxBytes
	}
	
	// Calculate block I/O
	var blockRead, blockWrite uint64
	for _, ioStat := range dockerStats.BlkioStats.IoServiceBytesRecursive {
		switch ioStat.Op {
		case "Read":
			blockRead += ioStat.Value
		case "Write":
			blockWrite += ioStat.Value
		}
	}
	
	return &ContainerStats{
		CPUPercent:      cpuPercent,
		MemoryUsage:     dockerStats.MemoryStats.Usage,
		MemoryLimit:     dockerStats.MemoryStats.Limit,
		NetworkRxBytes:  int64(networkRx),
		NetworkTxBytes:  int64(networkTx),
		BlockReadBytes:  int64(blockRead),
		BlockWriteBytes: int64(blockWrite),
	}, nil
}

// calculateCPUPercent calculates CPU usage percentage
func calculateCPUPercent(cpuUsage, prevCPUUsage, systemUsage, prevSystemUsage, numCPUs uint64) float64 {
	cpuDelta := float64(cpuUsage - prevCPUUsage)
	systemDelta := float64(systemUsage - prevSystemUsage)
	
	if systemDelta > 0.0 && cpuDelta > 0.0 {
		return (cpuDelta / systemDelta) * float64(numCPUs) * 100.0
	}
	
	return 0.0
}
