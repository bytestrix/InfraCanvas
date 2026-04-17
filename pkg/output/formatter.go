package output

import "infracanvas/internal/models"

// Formatter is the interface for all output formatters
type Formatter interface {
	Format(snapshot *models.InfraSnapshot) ([]byte, error)
}

// FormatType represents the output format type
type FormatType string

const (
	FormatJSON  FormatType = "json"
	FormatYAML  FormatType = "yaml"
	FormatTable FormatType = "table"
	FormatGraph FormatType = "graph"
)

// NewFormatter creates a new formatter based on the format type
func NewFormatter(format FormatType) Formatter {
	switch format {
	case FormatJSON:
		return &JSONFormatter{PrettyPrint: true}
	case FormatYAML:
		return &YAMLFormatter{}
	case FormatTable:
		return &TableFormatter{}
	case FormatGraph:
		return &GraphFormatter{FilterNoise: true}
	default:
		return &JSONFormatter{PrettyPrint: true}
	}
}
