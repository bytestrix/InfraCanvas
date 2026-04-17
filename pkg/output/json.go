package output

import (
	"encoding/json"
	"infracanvas/internal/models"
)

// JSONFormatter formats InfraSnapshot as JSON
type JSONFormatter struct {
	PrettyPrint bool
}

// Format marshals the InfraSnapshot to JSON
func (f *JSONFormatter) Format(snapshot *models.InfraSnapshot) ([]byte, error) {
	if f.PrettyPrint {
		return json.MarshalIndent(snapshot, "", "  ")
	}
	return json.Marshal(snapshot)
}
