package validation

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
)

// ParseError represents a parsing error with context
type ParseError struct {
	Field   string
	Value   string
	Reason  string
	Context string
}

func (e *ParseError) Error() string {
	if e.Context != "" {
		return fmt.Sprintf("parse error in %s: field=%s, value=%q, reason=%s, context=%s",
			e.Context, e.Field, e.Value, e.Reason, e.Context)
	}
	return fmt.Sprintf("parse error: field=%s, value=%q, reason=%s", e.Field, e.Value, e.Reason)
}

// SafeParseInt parses an integer with validation and error logging
func SafeParseInt(value string, field string, context string) (int, error) {
	if value == "" {
		return 0, &ParseError{
			Field:   field,
			Value:   value,
			Reason:  "empty value",
			Context: context,
		}
	}

	result, err := strconv.Atoi(value)
	if err != nil {
		parseErr := &ParseError{
			Field:   field,
			Value:   value,
			Reason:  "invalid integer format",
			Context: context,
		}
		log.Printf("parsing error: %v", parseErr)
		return 0, parseErr
	}

	return result, nil
}

// SafeParseInt64 parses an int64 with validation and error logging
func SafeParseInt64(value string, field string, context string) (int64, error) {
	if value == "" {
		return 0, &ParseError{
			Field:   field,
			Value:   value,
			Reason:  "empty value",
			Context: context,
		}
	}

	result, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		parseErr := &ParseError{
			Field:   field,
			Value:   value,
			Reason:  "invalid int64 format",
			Context: context,
		}
		log.Printf("parsing error: %v", parseErr)
		return 0, parseErr
	}

	return result, nil
}

// SafeParseFloat parses a float64 with validation and error logging
func SafeParseFloat(value string, field string, context string) (float64, error) {
	if value == "" {
		return 0, &ParseError{
			Field:   field,
			Value:   value,
			Reason:  "empty value",
			Context: context,
		}
	}

	result, err := strconv.ParseFloat(value, 64)
	if err != nil {
		parseErr := &ParseError{
			Field:   field,
			Value:   value,
			Reason:  "invalid float format",
			Context: context,
		}
		log.Printf("parsing error: %v", parseErr)
		return 0, parseErr
	}

	return result, nil
}

// SafeParseBool parses a boolean with validation and error logging
func SafeParseBool(value string, field string, context string) (bool, error) {
	if value == "" {
		return false, &ParseError{
			Field:   field,
			Value:   value,
			Reason:  "empty value",
			Context: context,
		}
	}

	result, err := strconv.ParseBool(value)
	if err != nil {
		parseErr := &ParseError{
			Field:   field,
			Value:   value,
			Reason:  "invalid boolean format",
			Context: context,
		}
		log.Printf("parsing error: %v", parseErr)
		return false, parseErr
	}

	return result, nil
}

// ValidateJSON validates that a string is valid JSON
func ValidateJSON(data []byte, context string) error {
	var js json.RawMessage
	if err := json.Unmarshal(data, &js); err != nil {
		parseErr := &ParseError{
			Field:   "json",
			Value:   string(data),
			Reason:  "invalid JSON format",
			Context: context,
		}
		log.Printf("parsing error: %v", parseErr)
		return parseErr
	}
	return nil
}

// SafeUnmarshalJSON unmarshals JSON with validation and error logging
func SafeUnmarshalJSON(data []byte, v interface{}, context string) error {
	if len(data) == 0 {
		return &ParseError{
			Field:   "json",
			Value:   "",
			Reason:  "empty data",
			Context: context,
		}
	}

	if err := json.Unmarshal(data, v); err != nil {
		parseErr := &ParseError{
			Field:   "json",
			Value:   string(data),
			Reason:  fmt.Sprintf("unmarshal failed: %v", err),
			Context: context,
		}
		log.Printf("parsing error: %v", parseErr)
		return parseErr
	}

	return nil
}

// ValidateCommandOutput validates that command output is not empty and contains expected content
func ValidateCommandOutput(output string, expectedSubstring string, context string) error {
	if output == "" {
		return &ParseError{
			Field:   "command_output",
			Value:   output,
			Reason:  "empty output",
			Context: context,
		}
	}

	if expectedSubstring != "" && !strings.Contains(output, expectedSubstring) {
		return &ParseError{
			Field:   "command_output",
			Value:   output,
			Reason:  fmt.Sprintf("expected substring %q not found", expectedSubstring),
			Context: context,
		}
	}

	return nil
}

// SafeSplitLines splits output into lines and validates
func SafeSplitLines(output string, context string) ([]string, error) {
	if output == "" {
		return []string{}, nil
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	
	// Filter out empty lines
	validLines := make([]string, 0, len(lines))
	for _, line := range lines {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			validLines = append(validLines, trimmed)
		}
	}

	return validLines, nil
}

// SafeSplitFields splits a line into fields and validates minimum field count
func SafeSplitFields(line string, minFields int, context string) ([]string, error) {
	fields := strings.Fields(line)
	
	if len(fields) < minFields {
		return nil, &ParseError{
			Field:   "fields",
			Value:   line,
			Reason:  fmt.Sprintf("expected at least %d fields, got %d", minFields, len(fields)),
			Context: context,
		}
	}

	return fields, nil
}

// ValidateNotEmpty validates that a string is not empty
func ValidateNotEmpty(value string, field string, context string) error {
	if strings.TrimSpace(value) == "" {
		return &ParseError{
			Field:   field,
			Value:   value,
			Reason:  "value is empty",
			Context: context,
		}
	}
	return nil
}

// ValidateRange validates that a numeric value is within a specified range
func ValidateRange(value float64, min float64, max float64, field string, context string) error {
	if value < min || value > max {
		return &ParseError{
			Field:   field,
			Value:   fmt.Sprintf("%f", value),
			Reason:  fmt.Sprintf("value out of range [%f, %f]", min, max),
			Context: context,
		}
	}
	return nil
}

// LogParseError logs a parse error with full context
func LogParseError(err error, additionalContext string) {
	if parseErr, ok := err.(*ParseError); ok {
		if additionalContext != "" {
			log.Printf("parse error [%s]: %v", additionalContext, parseErr)
		} else {
			log.Printf("parse error: %v", parseErr)
		}
	} else {
		log.Printf("error: %v (context: %s)", err, additionalContext)
	}
}
