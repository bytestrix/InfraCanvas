# Output Package

The `output` package provides formatters and progress indicators for displaying infrastructure discovery results.

## Features

- **Multiple Output Formats**: JSON, YAML, and human-readable tables
- **Progress Indicators**: Spinners and multi-stage progress tracking
- **Color Coding**: Health status visualization with ANSI colors
- **Smart Truncation**: Automatic truncation of long values for readability

## Formatters

### JSON Formatter

Formats `InfraSnapshot` as JSON with optional pretty-printing.

```go
formatter := &output.JSONFormatter{PrettyPrint: true}
result, err := formatter.Format(snapshot)
```

### YAML Formatter

Formats `InfraSnapshot` as YAML.

```go
formatter := &output.YAMLFormatter{}
result, err := formatter.Format(snapshot)
```

### Table Formatter

Formats `InfraSnapshot` as human-readable tables with:
- Entity-specific columns showing key fields
- Color-coded health status (green=healthy, yellow=degraded, red=unhealthy)
- Automatic truncation of long values
- Relationship summaries
- Error reporting

```go
formatter := &output.TableFormatter{}
result, err := formatter.Format(snapshot)
```

### Factory Function

Use `NewFormatter` to create formatters by type:

```go
formatter := output.NewFormatter(output.FormatJSON)
// or
formatter := output.NewFormatter(output.FormatYAML)
// or
formatter := output.NewFormatter(output.FormatTable)
```

## Progress Indicators

### Progress Indicator

Displays a spinner with a message during long-running operations.

```go
indicator := output.NewProgressIndicator("Discovering infrastructure...")
indicator.Start()

// Do work...

indicator.UpdateMessage("Collecting containers...")
// Do more work...

indicator.Stop()
```

### Progress Tracker

Tracks progress across multiple stages with automatic stage numbering.

```go
stages := []string{
    "Discovering host information",
    "Discovering Docker containers",
    "Discovering Kubernetes resources",
}

tracker := output.NewProgressTracker(stages)
tracker.Start()

// Complete stage 1
tracker.NextStage()

// Complete stage 2
tracker.NextStage()

// All done
tracker.Complete()
```

### Simple Progress

Displays simple progress messages without spinners.

```go
progress := output.NewSimpleProgress()
progress.Show("Starting discovery...")
progress.Success("Host information collected")
progress.Warning("Docker socket not accessible")
progress.Error("Kubernetes API unreachable")
```

## Table Formatter Details

The table formatter renders different entity types with appropriate columns:

### Host Table
- Hostname, OS, CPU%, Memory%, Health

### Process Table
- PID, Name, User, CPU%, Memory%, Type

### Service Table
- Name, Status, Enabled, Critical, Health

### Container Table
- Name, Image, State, CPU%, Memory, Health

### Image Table
- Repository, Tag, Size, Created

### Pod Table
- Namespace, Name, Phase, Node, Restarts, Health

### Deployment Table
- Namespace, Name, Ready, Up-to-Date, Available, Health

### Node Table
- Name, Status, Roles, Version, Health

### Kubernetes Service Table
- Namespace, Name, Type, Cluster-IP, Endpoints

## Color Coding

The table formatter uses ANSI color codes for health status:

- **Green** (`\033[32m`): Healthy, Running, Active, Ready
- **Yellow** (`\033[33m`): Degraded, Pending, Inactive
- **Red** (`\033[31m`): Unhealthy, Failed, Exited, NotReady

## Usage in CLI Commands

```go
// In a CLI command
snapshot, err := orchestrator.Discover(ctx, scope)
if err != nil {
    return err
}

// Create formatter based on output flag
formatter := output.NewFormatter(outputFormat)
result, err := formatter.Format(snapshot)
if err != nil {
    return err
}

fmt.Println(string(result))
```

## Usage with Progress Tracking

```go
// Create progress tracker
tracker := output.NewProgressTracker([]string{
    "Discovering host layer",
    "Discovering Docker layer",
    "Discovering Kubernetes layer",
})

tracker.Start()

// Discover host
hostData, err := discoverHost()
if err != nil {
    tracker.Fail(err)
    return err
}
tracker.NextStage()

// Discover Docker
dockerData, err := discoverDocker()
if err != nil {
    tracker.Fail(err)
    return err
}
tracker.NextStage()

// Discover Kubernetes
k8sData, err := discoverKubernetes()
if err != nil {
    tracker.Fail(err)
    return err
}

tracker.Complete()
```

## Testing

Run tests with:

```bash
go test ./pkg/output/...
```

Run tests with coverage:

```bash
go test ./pkg/output/... -cover
```

## Requirements Covered

This package implements the following requirements from the spec:

- **16.1**: JSON output format with pretty-printing
- **16.2**: YAML output format
- **16.3**: Table output format with color coding and truncation
- **16.9**: Progress indicators for long-running operations
