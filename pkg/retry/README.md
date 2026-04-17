# Retry Package

The retry package provides exponential backoff retry logic for operations that may fail transiently, particularly network operations.

## Features

- Exponential backoff with configurable parameters
- Context-aware retry with cancellation support
- Support for operations with and without return values
- Predefined configurations for common use cases

## Usage

### Basic Retry

```go
import "rix/pkg/retry"

err := retry.Do(func() error {
    // Your operation here
    return someNetworkCall()
}, retry.DefaultConfig())
```

### Retry with Context

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

err := retry.DoWithContext(ctx, func() error {
    return someNetworkCall()
}, retry.NetworkConfig())
```

### Retry with Result

```go
result, err := retry.DoWithResult(func() (string, error) {
    return fetchData()
}, retry.NetworkConfig())
```

### Custom Configuration

```go
config := &retry.Config{
    MaxRetries:     5,
    InitialBackoff: 1 * time.Second,
    MaxBackoff:     30 * time.Second,
    Multiplier:     2.0,
}

err := retry.Do(operation, config)
```

## Predefined Configurations

### DefaultConfig
- MaxRetries: 3
- InitialBackoff: 100ms
- MaxBackoff: 10s
- Multiplier: 2.0

### NetworkConfig
- MaxRetries: 3
- InitialBackoff: 500ms
- MaxBackoff: 5s
- Multiplier: 2.0

## Backoff Calculation

The backoff duration is calculated using exponential backoff:

```
backoff = InitialBackoff * (Multiplier ^ attempt)
```

The backoff is capped at MaxBackoff to prevent excessive delays.

## Example Backoff Sequence (NetworkConfig)

- Attempt 0: 500ms
- Attempt 1: 1000ms
- Attempt 2: 2000ms
- Attempt 3: 4000ms

## Requirements Satisfied

- Requirement 18.4: Network timeout retry with exponential backoff
