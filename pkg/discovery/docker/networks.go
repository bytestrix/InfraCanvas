package docker

import (
	"context"
	"fmt"
	"time"

	"github.com/docker/docker/api/types/network"
	"infracanvas/internal/models"
)

// GetNetworks collects all Docker networks
func (d *Discovery) GetNetworks(ctx context.Context) ([]models.Network, error) {
	networkList, err := d.client.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list networks: %w", err)
	}
	
	networks := make([]models.Network, 0, len(networkList))
	
	for _, net := range networkList {
		network := d.parseNetwork(net)
		networks = append(networks, network)
	}
	
	return networks, nil
}

// parseNetwork parses network information from Docker API
func (d *Discovery) parseNetwork(net network.Inspect) models.Network {
	// Extract subnet and gateway from IPAM config
	var subnet, gateway string
	if len(net.IPAM.Config) > 0 {
		subnet = net.IPAM.Config[0].Subnet
		gateway = net.IPAM.Config[0].Gateway
	}
	
	return models.Network{
		BaseEntity: models.BaseEntity{
			ID:          fmt.Sprintf("network:%s", net.ID[:12]),
			Type:        models.EntityTypeNetwork,
			Labels:      net.Labels,
			Annotations: make(map[string]string),
			Health:      models.HealthHealthy,
			Timestamp:   time.Now(),
		},
		NetworkID:           net.ID[:12],
		Name:                net.Name,
		Driver:              net.Driver,
		Scope:               net.Scope,
		Subnet:              subnet,
		Gateway:             gateway,
		ConnectedContainers: []string{},
	}
}

// TrackNetworkUsage updates network usage tracking based on containers
func TrackNetworkUsage(networks []models.Network, containers []models.Container) []models.Network {
	// Create a map for quick lookup by network name
	networkMap := make(map[string]*models.Network)
	for i := range networks {
		networkMap[networks[i].Name] = &networks[i]
	}
	
	// Track which containers are connected to each network
	for _, container := range containers {
		// Parse network mode to determine connected networks
		// Network mode can be: "bridge", "host", "none", "container:<id>", or custom network name
		networkMode := container.NetworkMode
		
		// For standard modes, use the mode name as network name
		if networkMode == "bridge" || networkMode == "host" || networkMode == "none" {
			if net, exists := networkMap[networkMode]; exists {
				net.ConnectedContainers = append(net.ConnectedContainers, container.ContainerID)
			}
		} else if networkMode != "" && networkMode != "default" {
			// For custom networks, use the network mode as network name
			if net, exists := networkMap[networkMode]; exists {
				net.ConnectedContainers = append(net.ConnectedContainers, container.ContainerID)
			}
		}
	}
	
	// Convert map back to slice
	result := make([]models.Network, 0, len(networkMap))
	for _, net := range networkMap {
		result = append(result, *net)
	}
	
	return result
}
