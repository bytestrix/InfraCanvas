# Action Executor

The action executor framework provides a unified interface for performing operations on discovered infrastructure across host, Docker, and Kubernetes layers.

## Overview

The action executor supports the following operations:

### Host Actions
- **Restart Service**: Restart a systemd service using `systemctl restart`

### Docker Actions
- **Restart Container**: Restart a Docker container
- **Stop Container**: Stop a Docker container
- **Start Container**: Start a Docker container

### Kubernetes Actions
- **Scale Deployment**: Scale a Kubernetes Deployment to a specified number of replicas
- **Scale StatefulSet**: Scale a Kubernetes StatefulSet to a specified number of replicas
- **Restart Pod**: Restart a Kubernetes Pod by deletion (will be recreated by its controller)

## Usage

```go
import "rix/pkg/actions"

// Create an action executor
executor, err := actions.NewActionExecutor()
if err != nil {
    log.Fatal(err)
}

// Define an action
action := &actions.Action{
    ID:   "action-1",
    Type: actions.ActionRestartService,
    Target: actions.ActionTarget{
        Layer:    "host",
        EntityID: "nginx",
    },
    RequestedBy: "admin",
    RequestedAt: time.Now(),
}

// Validate the action
if err := executor.ValidateAction(action); err != nil {
    log.Fatal(err)
}

// Check if confirmation is required
if executor.RequiresConfirmation(action) {
    confirmed, err := actions.ConfirmAction(action)
    if err != nil || !confirmed {
        log.Fatal("Action not confirmed")
    }
}

// Execute the action
result, err := executor.ExecuteAction(context.Background(), action)
if err != nil {
    log.Fatal(err)
}

// Display the result
actions.DisplayActionResult(result)
```

## Action Structure

```go
type Action struct {
    ID          string            // Unique action identifier
    Type        ActionType        // Type of action to perform
    Target      ActionTarget      // Target entity
    Parameters  map[string]string // Action-specific parameters
    RequestedBy string            // User who requested the action
    RequestedAt time.Time         // When the action was requested
}

type ActionTarget struct {
    Layer      string // host, docker, kubernetes
    EntityType string // service, container, deployment, etc.
    EntityID   string // Name or ID of the entity
    Namespace  string // Kubernetes namespace (if applicable)
}
```

## Action Result

```go
type ActionResult struct {
    Success   bool      // Whether the action succeeded
    Message   string    // Human-readable message
    Output    string    // Command output (if any)
    Error     string    // Error message (if failed)
    StartTime time.Time // When execution started
    EndTime   time.Time // When execution completed
}
```

## Validation

All actions are validated before execution:
- Entity existence is checked
- Required parameters are verified
- Permissions are validated (implicitly through API calls)

## Confirmation

All actions are considered destructive and require user confirmation by default. The `ConfirmAction` function provides an interactive prompt for CLI usage.

## Error Handling

- If an entity doesn't exist, validation fails with a descriptive error
- If an action fails, the result includes both a message and detailed error
- Timeouts are applied to prevent hanging operations
