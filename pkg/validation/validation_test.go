package validation

import (
	"strings"
	"testing"
)

func TestSafeParseInt_Success(t *testing.T) {
	result, err := SafeParseInt("42", "test_field", "test_context")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result != 42 {
		t.Errorf("expected 42, got %d", result)
	}
}

func TestSafeParseInt_EmptyValue(t *testing.T) {
	_, err := SafeParseInt("", "test_field", "test_context")
	if err == nil {
		t.Error("expected error for empty value")
	}
	
	parseErr, ok := err.(*ParseError)
	if !ok {
		t.Error("expected ParseError type")
	}
	if parseErr.Field != "test_field" {
		t.Errorf("expected field 'test_field', got %s", parseErr.Field)
	}
}

func TestSafeParseInt_InvalidFormat(t *testing.T) {
	_, err := SafeParseInt("not_a_number", "test_field", "test_context")
	if err == nil {
		t.Error("expected error for invalid format")
	}
	
	parseErr, ok := err.(*ParseError)
	if !ok {
		t.Error("expected ParseError type")
	}
	if !strings.Contains(parseErr.Reason, "invalid integer format") {
		t.Errorf("expected 'invalid integer format' in reason, got %s", parseErr.Reason)
	}
}

func TestSafeParseInt64_Success(t *testing.T) {
	result, err := SafeParseInt64("9223372036854775807", "test_field", "test_context")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result != 9223372036854775807 {
		t.Errorf("expected max int64, got %d", result)
	}
}

func TestSafeParseFloat_Success(t *testing.T) {
	result, err := SafeParseFloat("3.14159", "test_field", "test_context")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result < 3.14 || result > 3.15 {
		t.Errorf("expected ~3.14159, got %f", result)
	}
}

func TestSafeParseBool_Success(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"true", true},
		{"false", false},
		{"1", true},
		{"0", false},
		{"t", true},
		{"f", false},
	}

	for _, tt := range tests {
		result, err := SafeParseBool(tt.input, "test_field", "test_context")
		if err != nil {
			t.Errorf("input %s: expected no error, got %v", tt.input, err)
		}
		if result != tt.expected {
			t.Errorf("input %s: expected %v, got %v", tt.input, tt.expected, result)
		}
	}
}

func TestValidateJSON_Valid(t *testing.T) {
	validJSON := []byte(`{"key": "value", "number": 42}`)
	err := ValidateJSON(validJSON, "test_context")
	if err != nil {
		t.Errorf("expected no error for valid JSON, got %v", err)
	}
}

func TestValidateJSON_Invalid(t *testing.T) {
	invalidJSON := []byte(`{"key": "value", "number": }`)
	err := ValidateJSON(invalidJSON, "test_context")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestSafeUnmarshalJSON_Success(t *testing.T) {
	data := []byte(`{"name": "test", "value": 42}`)
	var result map[string]interface{}
	
	err := SafeUnmarshalJSON(data, &result, "test_context")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	
	if result["name"] != "test" {
		t.Errorf("expected name='test', got %v", result["name"])
	}
}

func TestSafeUnmarshalJSON_EmptyData(t *testing.T) {
	var result map[string]interface{}
	err := SafeUnmarshalJSON([]byte{}, &result, "test_context")
	if err == nil {
		t.Error("expected error for empty data")
	}
}

func TestValidateCommandOutput_Success(t *testing.T) {
	output := "some command output with expected content"
	err := ValidateCommandOutput(output, "expected", "test_context")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestValidateCommandOutput_Empty(t *testing.T) {
	err := ValidateCommandOutput("", "", "test_context")
	if err == nil {
		t.Error("expected error for empty output")
	}
}

func TestValidateCommandOutput_MissingSubstring(t *testing.T) {
	output := "some command output"
	err := ValidateCommandOutput(output, "missing", "test_context")
	if err == nil {
		t.Error("expected error for missing substring")
	}
}

func TestSafeSplitLines_Success(t *testing.T) {
	output := "line1\nline2\n\nline3\n"
	lines, err := SafeSplitLines(output, "test_context")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	
	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(lines))
	}
	
	if lines[0] != "line1" || lines[1] != "line2" || lines[2] != "line3" {
		t.Errorf("unexpected line values: %v", lines)
	}
}

func TestSafeSplitLines_Empty(t *testing.T) {
	lines, err := SafeSplitLines("", "test_context")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	
	if len(lines) != 0 {
		t.Errorf("expected 0 lines, got %d", len(lines))
	}
}

func TestSafeSplitFields_Success(t *testing.T) {
	line := "field1 field2 field3 field4"
	fields, err := SafeSplitFields(line, 3, "test_context")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	
	if len(fields) != 4 {
		t.Errorf("expected 4 fields, got %d", len(fields))
	}
}

func TestSafeSplitFields_InsufficientFields(t *testing.T) {
	line := "field1 field2"
	_, err := SafeSplitFields(line, 5, "test_context")
	if err == nil {
		t.Error("expected error for insufficient fields")
	}
	
	parseErr, ok := err.(*ParseError)
	if !ok {
		t.Error("expected ParseError type")
	}
	if !strings.Contains(parseErr.Reason, "expected at least 5 fields") {
		t.Errorf("unexpected reason: %s", parseErr.Reason)
	}
}

func TestValidateNotEmpty_Success(t *testing.T) {
	err := ValidateNotEmpty("value", "test_field", "test_context")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestValidateNotEmpty_Empty(t *testing.T) {
	err := ValidateNotEmpty("", "test_field", "test_context")
	if err == nil {
		t.Error("expected error for empty value")
	}
}

func TestValidateNotEmpty_Whitespace(t *testing.T) {
	err := ValidateNotEmpty("   ", "test_field", "test_context")
	if err == nil {
		t.Error("expected error for whitespace-only value")
	}
}

func TestValidateRange_Success(t *testing.T) {
	err := ValidateRange(50.0, 0.0, 100.0, "test_field", "test_context")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestValidateRange_BelowMin(t *testing.T) {
	err := ValidateRange(-5.0, 0.0, 100.0, "test_field", "test_context")
	if err == nil {
		t.Error("expected error for value below minimum")
	}
}

func TestValidateRange_AboveMax(t *testing.T) {
	err := ValidateRange(150.0, 0.0, 100.0, "test_field", "test_context")
	if err == nil {
		t.Error("expected error for value above maximum")
	}
}
