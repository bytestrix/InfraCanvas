package output

import (
	"infracanvas/internal/models"

	"gopkg.in/yaml.v3"
)

// YAMLFormatter formats InfraSnapshot as YAML
type YAMLFormatter struct{}

// Format marshals the InfraSnapshot to YAML
func (f *YAMLFormatter) Format(snapshot *models.InfraSnapshot) ([]byte, error) {
	return yaml.Marshal(snapshot)
}
