package kubernetes

import (
	corev1 "k8s.io/api/core/v1"
)

// parseResourceList converts Kubernetes ResourceList to map[string]string
func parseResourceList(resources corev1.ResourceList) map[string]string {
	result := make(map[string]string)
	for key, value := range resources {
		result[string(key)] = value.String()
	}
	return result
}
