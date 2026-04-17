# Validation Package

The validation package provides input validation and parsing safety utilities for external command outputs and data parsing.

## Features

- Safe parsing functions with automatic error logging
- Command output validation
- JSON validation and unmarshaling
- Range validation for numeric values
- Structured error types with context

## Usage

### Safe Integer Parsing

```go
import "rix/pkg/validation"

value, err := validation.SafeParseInt("42", "port", "/proc/net/tcp")
if err != nil {
    // Error is logged automatically
    // Handle error or use default value
}
```

### Safe Float Parsing

```go
cpuUsage, err := validation.SafeParseFloat("85.5", "cpu_usage", "/proc/stat")
if err != nil {
    // Error is logged automatically
}
```

### Command Output Validation

```go
output := string(cmdOutput)
err := validation.ValidateCommandOutput(output, "expected_substring", "systemctl status")
if err != nil {
    validation.LogParseError(err, "additional context")
    return err
}
```

### Safe Line Splitting

```go
lines, err := validation.SafeSplitLines(output, "/proc/meminfo")
if err != nil {
    return err
}

for _, line := range lines {
    // Process each line
}
```

### Safe Field Splitting

```go
fields, err := validation.SafeSplitFields(line, 5, "systemctl list-units")
if err != nil {
    // Line doesn't have minimum required fields
    continue
}
```

### Range Validation

```go
err := validation.ValidateRange(cpuPercent, 0.0, 100.0, "cpu_percent", "/proc/stat")
if err != nil {
    // Value is out of valid range
}
```

### JSON Validation

```go
err := validation.ValidateJSON(jsonData, "API response")
if err != nil {
    return err
}

var result MyStruct
err = validation.SafeUnmarshalJSON(jsonData, &result, "API response")
if err != nil {
    return err
}
```

## Error Types

### ParseError

All validation functions return a `ParseError` type that includes:

- Field: The field being parsed
- Value: The value that failed parsing
- Reason: Why parsing failed
- Context: Where the parsing occurred

```go
type ParseError struct {
    Field   string
    Value   string
    Reason  string
    Context string
}
```

## Automatic Error Logging

All safe parsing functions automatically log errors with full context using `log.Printf()`. This ensures that parsing failures are visible in logs without requiring explicit error handling at every call site.

## Graceful Degradation

The validation package is designed to support graceful degradation:

1. Parse errors are logged automatically
2. Callers can choose to use default values on error
3. Parsing failures don't crash the application
4. Context information helps debugging

## Requirements Satisfied

- Requirement 18.6: Validate all external command outputs before parsing
- Requirement 18.7: Handle malformed data gracefully and log parsing errors with context
