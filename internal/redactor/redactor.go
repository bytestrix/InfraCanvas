package redactor

import (
	"regexp"
	"strings"
)

// SensitiveDataRedactor defines the interface for redacting sensitive information
type SensitiveDataRedactor interface {
	// RedactEnvVars redacts sensitive values from environment variables
	RedactEnvVars(envVars map[string]string) map[string]string
	
	// RedactCommandLine redacts sensitive values from command-line arguments
	RedactCommandLine(cmdLine string) string
	
	// RedactValue redacts a single value if it matches sensitive patterns
	RedactValue(key, value string) string
	
	// IsEnabled returns whether redaction is enabled
	IsEnabled() bool
}

// Redactor implements the SensitiveDataRedactor interface
type Redactor struct {
	enabled            bool
	sensitiveKeyRegex  *regexp.Regexp
	awsKeyRegex        *regexp.Regexp
	privateKeyRegex    *regexp.Regexp
	jwtTokenRegex      *regexp.Regexp
	base64LongRegex    *regexp.Regexp
	genericSecretRegex *regexp.Regexp
}

const (
	redactedPlaceholder = "[REDACTED]"
)

// NewRedactor creates a new Redactor instance
func NewRedactor(enabled bool) *Redactor {
	return &Redactor{
		enabled: enabled,
		// Matches environment variable keys containing sensitive keywords
		sensitiveKeyRegex: regexp.MustCompile(`(?i)(password|passwd|pwd|secret|token|key|credential|auth|api[_-]?key|access[_-]?key|private[_-]?key|jwt|bearer)`),
		
		// AWS Access Key ID pattern (AKIA followed by 16 alphanumeric characters)
		awsKeyRegex: regexp.MustCompile(`(AKIA[0-9A-Z]{16})`),
		
		// AWS Secret Access Key pattern (40 base64 characters)
		// Private key patterns
		privateKeyRegex: regexp.MustCompile(`-----BEGIN\s+(?:RSA\s+)?PRIVATE\s+KEY-----[\s\S]*?-----END\s+(?:RSA\s+)?PRIVATE\s+KEY-----`),
		
		// JWT token pattern (three base64 segments separated by dots)
		jwtTokenRegex: regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`),
		
		// Long base64-encoded strings (likely secrets)
		base64LongRegex: regexp.MustCompile(`\b[A-Za-z0-9+/]{40,}={0,2}\b`),
		
		// Generic secret patterns in command lines
		genericSecretRegex: regexp.MustCompile(`(?i)(-{1,2}(?:password|passwd|pwd|secret|token|key|credential|auth|api[_-]?key)[\s=]+)([^\s]+)`),
	}
}

// IsEnabled returns whether redaction is enabled
func (r *Redactor) IsEnabled() bool {
	return r.enabled
}

// RedactEnvVars redacts sensitive values from environment variables
func (r *Redactor) RedactEnvVars(envVars map[string]string) map[string]string {
	if !r.enabled {
		return envVars
	}
	
	redacted := make(map[string]string, len(envVars))
	for key, value := range envVars {
		redacted[key] = r.RedactValue(key, value)
	}
	
	return redacted
}

// RedactCommandLine redacts sensitive values from command-line arguments
func (r *Redactor) RedactCommandLine(cmdLine string) string {
	if !r.enabled {
		return cmdLine
	}
	
	// Redact command-line flags with sensitive values
	// Pattern: --password=secret or --password secret
	result := r.genericSecretRegex.ReplaceAllString(cmdLine, "${1}"+redactedPlaceholder)
	
	// Redact AWS keys
	result = r.awsKeyRegex.ReplaceAllString(result, redactedPlaceholder)
	
	// Redact JWT tokens
	result = r.jwtTokenRegex.ReplaceAllString(result, redactedPlaceholder)
	
	// Redact private keys
	result = r.privateKeyRegex.ReplaceAllString(result, redactedPlaceholder)
	
	return result
}

// RedactValue redacts a single value if it matches sensitive patterns
func (r *Redactor) RedactValue(key, value string) string {
	if !r.enabled {
		return value
	}
	
	// Check if the key name indicates sensitive data
	if r.sensitiveKeyRegex.MatchString(key) {
		return redactedPlaceholder
	}
	
	// Check if the value matches known secret patterns
	if r.awsKeyRegex.MatchString(value) {
		return redactedPlaceholder
	}
	
	if r.jwtTokenRegex.MatchString(value) {
		return redactedPlaceholder
	}
	
	if r.privateKeyRegex.MatchString(value) {
		return redactedPlaceholder
	}
	
	// Check for long base64 strings (likely secrets)
	if len(value) > 20 && r.base64LongRegex.MatchString(value) {
		return redactedPlaceholder
	}
	
	return value
}

// RedactSlice redacts sensitive values from a slice of strings
func (r *Redactor) RedactSlice(items []string) []string {
	if !r.enabled {
		return items
	}
	
	redacted := make([]string, len(items))
	for i, item := range items {
		// Check if this looks like a key=value pair
		if strings.Contains(item, "=") {
			parts := strings.SplitN(item, "=", 2)
			if len(parts) == 2 {
				redacted[i] = parts[0] + "=" + r.RedactValue(parts[0], parts[1])
				continue
			}
		}
		
		// Otherwise, check the value itself
		redacted[i] = r.RedactValue("", item)
	}
	
	return redacted
}
