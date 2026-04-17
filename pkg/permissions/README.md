# Permission Checker

The permission checker validates access permissions for infrastructure discovery across host, Docker, and Kubernetes layers.

## Overview

The permission checker performs pre-flight validation to:
- Identify which discovery operations are available
- Detect permission issues before attempting discovery
- Provide actionable suggestions for resolving permission problems
- Support graceful degradation when some permissions are unavailable

## Usage

```go
import "rix/pkg/permissions"

// Create a new checker
checker := permissions.NewChecker()

// Validate permissions for specific scopes
checks := checker.ValidatePermissions([]string{"host", "docker", "kubernetes"})

// Check for critical issues
if checker.HasCriticalIssues() {
    fmt.Println("Critical permissions are missing!")
}

// Get summary
available, unavailable, partial := checker.GetSummary()
fmt.Printf("Available: %d, Unavailable: %d, Partial: %d\n", available, unavailable, partial)

// Print detailed results
for _, check := range checks {
    fmt.Printf("[%s] %s: %v\n", check.Layer, check.Operation, check.Available)
    if !check.Available && check.Suggestion != "" {
        fmt.Printf("  Suggestion: %s\n", check.Suggestion)
    }
}
```

## Permission Levels

- **Full**: Complete access to the operation
- **Partial**: Limited access (e.g., non-root user can see own processes but not all)
- **None**: No access to the operation

## Checked Permissions

### Host Layer

| Operation | Required | Description |
|-----------|----------|-------------|
| read_os_info | Yes | Read OS information from /etc/os-release |
| read_proc | Yes | Read process information from /proc |
| systemd_access | No | Access systemd services via systemctl |
| journal_access | No | Access system logs via journalctl |
| elevated_access | No | Root/elevated permissions for full discovery |

### Docker Layer

| Operation | Required | Description |
|-----------|----------|-------------|
| docker_socket | Yes | Access Docker socket at /var/run/docker.sock |
| docker_api | Yes | Connect to Docker API |

### Kubernetes Layer

| Operation | Required | Description |
|-----------|----------|-------------|
| kubeconfig | Yes | Kubeconfig file found |
| k8s_api | Yes | Connect to Kubernetes API server |
| list_nodes | No | List cluster nodes |
| list_pods | Yes | List pods across all namespaces |
| list_deployments | No | List deployments |
| list_services | No | List services |
| list_events | No | List cluster events |

## Permission Check Structure

```go
type PermissionCheck struct {
    Layer      string          // "host", "docker", "kubernetes"
    Operation  string          // Specific operation being checked
    Required   bool            // Is this required for basic functionality?
    Available  bool            // Is permission available?
    Level      PermissionLevel // "full", "partial", "none"
    Message    string          // User-facing message
    Suggestion string          // Actionable suggestion if unavailable
}
```

## Common Permission Issues

### Docker Socket Access

**Problem**: Permission denied when accessing /var/run/docker.sock

**Solution**: Add user to docker group
```bash
sudo usermod -aG docker $USER
# Log out and back in for changes to take effect
```

### Kubernetes API Access

**Problem**: Unable to connect to Kubernetes API

**Solutions**:
1. Configure kubectl: `kubectl config view`
2. Set KUBECONFIG: `export KUBECONFIG=/path/to/kubeconfig`
3. Check cluster connectivity: `kubectl cluster-info`

### Systemd Access

**Problem**: Cannot access systemd services or logs

**Solutions**:
1. For systemctl: Run with sudo or as root
2. For journalctl: Add user to systemd-journal group
```bash
sudo usermod -aG systemd-journal $USER
```

## Integration with Discovery

The permission checker should be called before discovery operations:

```go
// Validate permissions first
checker := permissions.NewChecker()
checks := checker.ValidatePermissions([]string{"host", "docker", "kubernetes"})

// Report issues
for _, check := range checks {
    if check.Required && !check.Available {
        log.Errorf("Required permission unavailable: %s - %s", check.Operation, check.Message)
        if check.Suggestion != "" {
            log.Infof("Suggestion: %s", check.Suggestion)
        }
    }
}

// Proceed with discovery, handling unavailable layers gracefully
if !checker.HasCriticalIssues() {
    // Continue with available layers
}
```

## Graceful Degradation

When permissions are unavailable:
1. Skip the affected discovery layer
2. Log a warning with the permission issue
3. Continue with other available layers
4. Include permission status in output metadata

Example:
```json
{
  "metadata": {
    "permissions": {
      "host": "full",
      "docker": "none",
      "kubernetes": "partial"
    },
    "warnings": [
      "Docker discovery skipped: socket not accessible"
    ]
  }
}
```
