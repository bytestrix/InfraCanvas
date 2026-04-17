# Infrastructure Discovery Agent

The Infrastructure Discovery Agent provides continuous background collection with real-time updates to a backend platform.

## Features

- **Continuous Collection**: Periodic discovery of infrastructure state
- **Incremental Updates**: Only sends changes (deltas) to reduce bandwidth
- **Event Watching**: Real-time monitoring of Docker and Kubernetes events
- **Heartbeat Monitoring**: Regular health checks to backend
- **Graceful Shutdown**: Handles SIGTERM/SIGINT for clean shutdown
- **Configurable Intervals**: Separate collection intervals for each layer

## Architecture

The agent consists of several key components:

1. **Agent Main Loop**: Orchestrates all collection and communication activities
2. **Backend Client**: Handles HTTP communication with the backend platform
3. **Strategy Manager**: Manages collection strategies and delta calculation
4. **Watcher Manager**: Manages event watchers for Docker and Kubernetes
5. **Orchestrator**: Performs actual infrastructure discovery

## Configuration

The agent can be configured via:
- Configuration file (YAML)
- Environment variables
- Command-line flags

### Configuration File

```yaml
backend_url: "http://localhost:8080"
auth_token: "your-auth-token-here"
tls_insecure: false

host_interval: 10
docker_interval: 15
kubernetes_interval: 20
heartbeat_interval: 30

scope:
  - host
  - docker
  - kubernetes

agent_id: ""
agent_name: "my-agent"

enable_redaction: true
enable_watchers: true
```

### Environment Variables

- `RIX_BACKEND_URL`: Backend platform URL
- `RIX_AUTH_TOKEN`: Authentication token
- `RIX_AGENT_ID`: Agent identifier
- `RIX_AGENT_NAME`: Agent friendly name

## Usage

### Running the Agent

```bash
# With configuration file
rix agent --config /etc/rix/agent.yaml

# Using environment variables
export RIX_BACKEND_URL="http://localhost:8080"
export RIX_AUTH_TOKEN="your-token"
rix agent

# With custom intervals (via config file)
rix agent --config custom-config.yaml
```

### Agent Lifecycle

1. **Registration**: Agent registers with backend, receives agent ID
2. **Initial Discovery**: Performs full infrastructure discovery
3. **Initial Snapshot**: Sends complete snapshot to backend
4. **Start Watchers**: Begins watching for events (if enabled)
5. **Main Loop**: 
   - Periodic collection at configured intervals
   - Delta calculation and transmission
   - Heartbeat sending
   - Command processing
6. **Shutdown**: Graceful cleanup on SIGTERM/SIGINT

## Collection Strategies

### Static Data (Collected Once)
- OS information, kernel version
- CPU model, architecture
- Docker engine version
- Kubernetes cluster version
- Node capacity

### Periodic Data (Polled)
- Host: CPU/memory/disk usage, processes
- Docker: Container states, resource usage
- Kubernetes: Pod states, replica counts

### Event-Based Data (Watched)
- Docker: Container lifecycle events
- Kubernetes: Pod events, cluster events

## Backend API Endpoints

The agent communicates with the following backend endpoints:

- `POST /api/v1/agents/register` - Agent registration
- `POST /api/v1/snapshots` - Send full snapshot
- `POST /api/v1/deltas` - Send incremental updates
- `POST /api/v1/events` - Send real-time events
- `POST /api/v1/heartbeat` - Send health status
- `GET /api/v1/commands` - Receive commands (long-polling)

## Delta Calculation

The agent calculates deltas between snapshots to minimize data transfer:

```go
type Delta struct {
    Added    map[string]Entity  // New entities
    Modified map[string]Entity  // Changed entities
    Removed  []string           // Deleted entity IDs
}
```

Only non-empty deltas are sent to the backend.

## Event Watching

### Docker Events
- Container lifecycle: create, start, stop, die, destroy
- Image events: pull, push, delete
- Volume events: create, destroy
- Network events: create, destroy

### Kubernetes Events
- Pod lifecycle: created, deleted, scheduled
- Deployment rollouts
- Node join/leave
- Warning and error events

## Error Handling

The agent implements robust error handling:

- **Network Errors**: Automatic retry with exponential backoff
- **Collection Errors**: Tracked and reported in heartbeats
- **Watcher Failures**: Automatic reconnection after delay
- **Graceful Degradation**: Continues with available layers if one fails

## Monitoring

The agent sends heartbeats every 30 seconds (configurable) containing:

```go
type AgentHealth struct {
    Status           string    // "running" or "stopped"
    Uptime           int64     // Seconds since start
    LastCollection   time.Time // Last successful collection
    CollectionErrors int       // Total collection errors
    MemoryUsage      int64     // Current memory usage
}
```

## Security

- **TLS Support**: Secure communication with backend
- **Token Authentication**: Bearer token authentication
- **Data Redaction**: Automatic redaction of sensitive values
- **Permission Handling**: Graceful degradation on permission errors

## Performance

- **Parallel Collection**: Discovery layers run in parallel
- **Incremental Updates**: Only changes are transmitted
- **Efficient Watching**: Event-based updates for real-time data
- **Memory Management**: Bounded cache sizes, periodic cleanup

## Troubleshooting

### Agent Won't Start
- Check backend URL is accessible
- Verify authentication token is valid
- Ensure required permissions (Docker socket, kubeconfig)

### No Data Being Sent
- Check network connectivity to backend
- Verify scope configuration includes desired layers
- Review agent logs for collection errors

### High Memory Usage
- Reduce collection intervals
- Limit scope to required layers only
- Check for large numbers of entities

### Events Not Being Received
- Ensure `enable_watchers: true` in configuration
- Verify Docker socket access
- Check Kubernetes API permissions

## Development

### Adding New Collection Strategies

1. Define data classification in `strategy.go`
2. Implement collection method in orchestrator
3. Add scheduling logic in agent main loop

### Adding New Event Watchers

1. Implement watcher in `watchers.go`
2. Register watcher in `WatcherManager.StartAll()`
3. Define event handling logic

### Testing

```bash
# Run with verbose logging
rix agent --config test-config.yaml -v

# Test with mock backend
# (Start mock backend on localhost:8080)
rix agent --config examples/agent-config.yaml
```
