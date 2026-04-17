# infracanvas - Infrastructure Discovery CLI

[![Go Version](https://img.shields.io/badge/go-1.21+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

**infracanvas** is a comprehensive infrastructure discovery tool that provides complete visibility into your system infrastructure across bare metal, virtual machines, containers, and Kubernetes environments. It operates in two modes: CLI mode for one-shot discovery operations and Agent mode for continuous monitoring with real-time updates.

## Features

- **Multi-Layer Discovery**: Discover infrastructure across host, Docker, and Kubernetes layers
- **Comprehensive Visibility**: Collect detailed information about processes, services, containers, pods, workloads, storage, and networking
- **Relationship Mapping**: Automatically build dependency graphs between infrastructure components
- **Health Monitoring**: Calculate health status for all discovered entities with actionable alerts
- **Dual Operating Modes**: CLI mode for ad-hoc queries and Agent mode for continuous monitoring
- **Multiple Output Formats**: JSON, YAML, and human-readable table formats
- **Sensitive Data Protection**: Automatic redaction of credentials, tokens, and secrets
- **Permission-Aware**: Graceful degradation when elevated permissions are unavailable
- **Action Execution**: Safely perform operations like restarting containers, scaling deployments, and viewing logs

## Quick Start

### Installation

#### Binary Download

```bash
# Linux (x86_64)
curl -fsSL https://github.com/example/infracanvas/releases/latest/download/infracanvas-linux-amd64 -o infracanvas
chmod +x infracanvas
sudo mv infracanvas /usr/local/bin/

# macOS (Intel)
curl -fsSL https://github.com/example/infracanvas/releases/latest/download/infracanvas-darwin-amd64 -o infracanvas
chmod +x infracanvas
sudo mv infracanvas /usr/local/bin/

# macOS (Apple Silicon)
curl -fsSL https://github.com/example/infracanvas/releases/latest/download/infracanvas-darwin-arm64 -o infracanvas
chmod +x infracanvas
sudo mv infracanvas /usr/local/bin/
```

#### Build from Source

```bash
git clone https://github.com/example/infracanvas.git
cd infracanvas
make build
sudo make install
```

### Basic Usage

```bash
# Discover all infrastructure
infracanvas discover

# Discover specific layers
infracanvas discover --scope host,docker
infracanvas discover --scope kubernetes --namespace production

# Get specific resources
infracanvas get pods
infracanvas get containers --filter "status=running"
infracanvas get services --output json

# Health diagnostics
infracanvas diagnose

# View logs
infracanvas logs pod/my-pod -n default --follow
infracanvas logs container/my-container --tail 100

# Export infrastructure data
infracanvas export --format json --output infra.json
```

## CLI Mode

CLI mode provides one-shot discovery operations for immediate infrastructure inspection, troubleshooting, and exports.

### Discovery Commands

#### Full Discovery

Discover all infrastructure across all available layers:

```bash
infracanvas discover
```

Output includes:
- Host information (OS, resources, network, processes)
- Docker containers, images, volumes, networks
- Kubernetes clusters, nodes, workloads, services, storage
- Relationships between all entities
- Health status for all components

#### Scoped Discovery

Limit discovery to specific layers:

```bash
# Host layer only
infracanvas discover --scope host

# Docker layer only
infracanvas discover --scope docker

# Kubernetes layer only
infracanvas discover --scope kubernetes

# Multiple layers
infracanvas discover --scope host,docker
```

#### Filtered Discovery

Filter Kubernetes resources by namespace or labels:

```bash
# Specific namespace
infracanvas discover --scope kubernetes --namespace production

# Multiple namespaces
infracanvas discover --scope kubernetes --namespace prod,staging

# Label selector
infracanvas discover --scope kubernetes --labels app=web,tier=frontend

# All namespaces
infracanvas discover --scope kubernetes --all-namespaces
```

### Get Commands

Query specific resource types:

```bash
# Pods
infracanvas get pods
infracanvas get pods --namespace default
infracanvas get pods --all-namespaces
infracanvas get pods --output json

# Containers
infracanvas get containers
infracanvas get containers --filter "status=running"
infracanvas get containers --filter "image=nginx"

# Services
infracanvas get services
infracanvas get services --namespace kube-system

# Deployments
infracanvas get deployments
infracanvas get deployments --namespace production

# Nodes
infracanvas get nodes

# Images
infracanvas get images
infracanvas get images --filter "tag=latest"

# Volumes
infracanvas get volumes

# Networks
infracanvas get networks
```

### Diagnose Command

Run health checks and diagnostics:

```bash
# Full diagnostics
infracanvas diagnose

# Specific checks
infracanvas diagnose --check health
infracanvas diagnose --check relationships
infracanvas diagnose --check permissions

# Output format
infracanvas diagnose --output json
```

Diagnostics include:
- Resource usage alerts (CPU, memory, disk)
- Unhealthy containers and pods
- Failed deployments and services
- Permission issues
- Relationship inconsistencies

### Logs Command

View logs from various sources:

```bash
# Pod logs
infracanvas logs pod/my-pod
infracanvas logs pod/my-pod --namespace default
infracanvas logs pod/my-pod --container app
infracanvas logs pod/my-pod --follow
infracanvas logs pod/my-pod --tail 100
infracanvas logs pod/my-pod --since 1h
infracanvas logs pod/my-pod --previous

# Container logs
infracanvas logs container/my-container
infracanvas logs container/my-container --follow
infracanvas logs container/my-container --tail 50
infracanvas logs container/my-container --since 30m

# Host logs (journald)
infracanvas logs host
infracanvas logs host --unit docker
infracanvas logs host --priority err
infracanvas logs host --since "2024-01-01 00:00:00"
```

### Export Command

Export infrastructure data for external analysis:

```bash
# JSON export
infracanvas export --format json --output infra.json

# YAML export
infracanvas export --format yaml --output infra.yaml

# Graph export (nodes and edges)
infracanvas export --format graph --output graph.json

# Scoped export
infracanvas export --scope kubernetes --format json --output k8s.json
```

### Output Formats

#### JSON Format

```bash
infracanvas discover --output json
```

Produces structured JSON with all entities and relationships.

#### YAML Format

```bash
infracanvas discover --output yaml
```

Produces human-readable YAML output.

#### Table Format (Default)

```bash
infracanvas discover --output table
```

Produces formatted tables with color-coded health status:
- 🟢 Green: Healthy
- 🟡 Yellow: Degraded
- 🔴 Red: Unhealthy
- ⚪ Gray: Unknown

### Global Flags

```bash
--output, -o      Output format: json, yaml, table (default: table)
--scope, -s       Discovery scope: host, docker, kubernetes (default: all)
--namespace, -n   Kubernetes namespace filter
--all-namespaces  Include all Kubernetes namespaces
--labels, -l      Label selector for Kubernetes resources
--verbose, -v     Verbose output with debug information
--quiet, -q       Quiet mode (suppress non-essential output)
--no-color        Disable color output
--no-redact       Disable sensitive data redaction (use with caution)
```

## Agent Mode

Agent mode provides continuous background collection with real-time updates to a backend platform.

### Installation

#### Using Installation Script

```bash
curl -fsSL https://platform.example.com/install.sh | sudo sh
```

The script will:
1. Download the infracanvas binary
2. Create systemd service file
3. Create agent configuration directory
4. Register with the backend platform

#### Manual Installation

1. Download and install the binary:

```bash
curl -fsSL https://github.com/example/infracanvas/releases/latest/download/infracanvas-linux-amd64 -o infracanvas
chmod +x infracanvas
sudo mv infracanvas /usr/local/bin/
```

2. Create configuration directory:

```bash
sudo mkdir -p /etc/infracanvas
```

3. Create agent configuration file `/etc/infracanvas/agent-config.yaml`:

```yaml
backend:
  url: https://platform.example.com
  token: YOUR_REGISTRATION_TOKEN

collection:
  host_interval: 10s
  docker_interval: 15s
  kubernetes_interval: 20s
  
scope:
  - host
  - docker
  - kubernetes

agent:
  name: my-server-01
  tags:
    environment: production
    region: us-east-1
```

4. Install systemd service:

```bash
sudo cp examples/infracanvas-agent.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable infracanvas-agent
sudo systemctl start infracanvas-agent
```

### Agent Configuration

Configuration file location: `/etc/infracanvas/agent-config.yaml`

```yaml
# Backend connection
backend:
  url: https://platform.example.com
  token: YOUR_REGISTRATION_TOKEN
  timeout: 30s
  retry_attempts: 3

# Collection intervals
collection:
  host_interval: 10s      # Host metrics collection
  docker_interval: 15s    # Docker stats collection
  kubernetes_interval: 20s # Kubernetes resource polling
  static_refresh: 24h     # Static data refresh interval

# Discovery scope
scope:
  - host
  - docker
  - kubernetes

# Agent identity
agent:
  name: my-server-01
  tags:
    environment: production
    region: us-east-1
    team: platform

# Kubernetes configuration
kubernetes:
  kubeconfig: /home/user/.kube/config
  namespaces:
    - default
    - production
    - staging
  # Leave empty to watch all namespaces

# Docker configuration
docker:
  socket: /var/run/docker.sock
  # Or use remote Docker
  # host: tcp://docker-host:2375

# Logging
logging:
  level: info  # debug, info, warn, error
  file: /var/log/infracanvas-agent.log
  max_size: 100  # MB
  max_backups: 3
```

### Agent Management

#### Start Agent

```bash
# Foreground (for testing)
infracanvas agent run

# Background (systemd)
sudo systemctl start infracanvas-agent
```

#### Stop Agent

```bash
sudo systemctl stop infracanvas-agent
```

#### Check Status

```bash
sudo systemctl status infracanvas-agent

# Or use infracanvas command
infracanvas agent status
```

#### View Logs

```bash
# Systemd logs
sudo journalctl -u infracanvas-agent -f

# Or use infracanvas command
infracanvas agent logs --follow
```

#### Restart Agent

```bash
sudo systemctl restart infracanvas-agent
```

### Agent Operations

The agent performs the following operations:

1. **Initial Registration**: Sends host identity and capabilities to backend
2. **Full Discovery**: Performs complete infrastructure discovery on startup
3. **Periodic Collection**: Polls for resource usage and state changes at configured intervals
4. **Event Watching**: Monitors Kubernetes and Docker events in real-time
5. **Incremental Updates**: Sends only changed data to reduce bandwidth
6. **Heartbeats**: Sends health status every 30 seconds
7. **Command Processing**: Receives and executes commands from backend

### Uninstallation

```bash
# Stop and disable service
sudo systemctl stop infracanvas-agent
sudo systemctl disable infracanvas-agent

# Remove files
sudo rm /usr/local/bin/infracanvas
sudo rm /etc/systemd/system/infracanvas-agent.service
sudo rm -rf /etc/infracanvas

# Reload systemd
sudo systemctl daemon-reload
```

## Permissions

infracanvas requires different permissions depending on the discovery scope. See [PERMISSIONS.md](PERMISSIONS.md) for detailed information.

### Quick Permission Setup

#### Docker Access

```bash
# Add user to docker group
sudo usermod -aG docker $USER

# Re-login or use newgrp
newgrp docker
```

#### Kubernetes Access

```bash
# Ensure kubeconfig is configured
kubectl cluster-info

# Apply RBAC for read-only discovery
kubectl apply -f examples/rbac-discovery.yaml
```

#### Host Discovery

Most host discovery works without elevated permissions. For full process and service discovery:

```bash
# Run with sudo
sudo infracanvas discover --scope host

# Or add user to systemd-journal group for log access
sudo usermod -aG systemd-journal $USER
```

## Examples

### Example 1: Quick Infrastructure Overview

```bash
infracanvas discover --output table
```

### Example 2: Export Kubernetes Infrastructure

```bash
infracanvas discover --scope kubernetes --output json > k8s-infra.json
```

### Example 3: Monitor Specific Namespace

```bash
infracanvas get pods --namespace production --output table
```

### Example 4: Find Unhealthy Resources

```bash
infracanvas diagnose --check health --output json | jq '.unhealthy'
```

### Example 5: View Container Logs

```bash
infracanvas logs container/nginx-proxy --follow
```

### Example 6: Export Dependency Graph

```bash
infracanvas export --format graph --output graph.json
```

### Example 7: CI/CD Integration

```bash
#!/bin/bash
# Check infrastructure health before deployment

infracanvas diagnose --check health --output json > health.json

unhealthy_count=$(jq '.summary.unhealthy_count' health.json)

if [ "$unhealthy_count" -gt 0 ]; then
  echo "Infrastructure is unhealthy. Aborting deployment."
  exit 1
fi

echo "Infrastructure is healthy. Proceeding with deployment."
```

## Architecture

infracanvas uses a layered architecture with clear separation between discovery layers:

```
┌─────────────────────────────────────┐
│         CLI / Agent Mode            │
├─────────────────────────────────────┤
│     Discovery Orchestrator          │
├─────────────────────────────────────┤
│  Host    │  Docker  │  Kubernetes  │
│ Discovery│ Discovery│  Discovery   │
├─────────────────────────────────────┤
│    Relationship Builder             │
│    Health Calculator                │
│    Sensitive Data Redactor          │
├─────────────────────────────────────┤
│    Output Formatters                │
└─────────────────────────────────────┘
```

## Security

### Sensitive Data Redaction

infracanvas automatically redacts sensitive data from output:

- Environment variables containing: PASSWORD, SECRET, TOKEN, KEY, CREDENTIAL, API
- Command-line arguments with sensitive patterns
- AWS access keys and secret keys
- Private keys (BEGIN PRIVATE KEY)
- JWT tokens
- Base64-encoded secrets (>20 characters)

Redacted values are replaced with `[REDACTED]`.

### Disable Redaction

For debugging purposes, redaction can be disabled:

```bash
infracanvas discover --no-redact
```

⚠️ **Warning**: Only use `--no-redact` in secure environments. Never share unredacted output.

### Permission Model

infracanvas follows the principle of least privilege:

- Attempts operations with current permissions
- Falls back gracefully when permissions are insufficient
- Reports permission issues with actionable guidance
- Never requires root unless explicitly needed

## Troubleshooting

### Docker Socket Permission Denied

```
Error: Cannot connect to Docker socket: permission denied
```

**Solution**: Add user to docker group:

```bash
sudo usermod -aG docker $USER
newgrp docker
```

### Kubernetes Connection Refused

```
Error: Cannot connect to Kubernetes API: connection refused
```

**Solution**: Ensure kubeconfig is configured:

```bash
kubectl cluster-info
export KUBECONFIG=~/.kube/config
```

### Insufficient Permissions

```
Warning: Some operations failed due to insufficient permissions
```

**Solution**: See [PERMISSIONS.md](PERMISSIONS.md) for detailed permission requirements.

### Agent Not Starting

```bash
# Check service status
sudo systemctl status infracanvas-agent

# View logs
sudo journalctl -u infracanvas-agent -n 50

# Verify configuration
infracanvas agent validate-config
```

## Performance

infracanvas is designed for efficiency:

- **Host Discovery**: < 2 seconds
- **Docker Discovery**: < 5 seconds (up to 100 containers)
- **Kubernetes Discovery**: < 10 seconds (up to 100 pods)
- **Memory Usage**: < 100 MB (CLI mode), < 200 MB (Agent mode)
- **Parallel Execution**: Discovery layers run concurrently
- **Incremental Updates**: Agent mode sends only changed data

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

MIT License - see [LICENSE](LICENSE) for details.

## Support

- Documentation: https://docs.example.com/infracanvas
- Issues: https://github.com/example/infracanvas/issues
- Discussions: https://github.com/example/infracanvas/discussions

## Version

```bash
infracanvas version
```

Shows version, commit hash, and build date.
