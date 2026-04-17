package kubernetes

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"infracanvas/internal/models"
)

// GetEvents collects all events in the specified namespace
func (d *Discovery) GetEvents(ctx context.Context, namespace string) ([]models.Event, error) {
	cacheKey := fmt.Sprintf("events:%s", namespace)
	
	// Check cache first
	if cached, found := d.cache.Get(cacheKey); found {
		if events, ok := cached.([]models.Event); ok {
			return events, nil
		}
	}
	
	eventList, err := d.clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list events: %w", err)
	}

	events := make([]models.Event, 0, len(eventList.Items))
	for _, e := range eventList.Items {
		event := d.parseEvent(&e)
		events = append(events, event)
	}

	// Cache the result
	d.cache.Set(cacheKey, events)

	return events, nil
}

func (d *Discovery) parseEvent(e *corev1.Event) models.Event {
	// Classify event
	category, isCritical := classifyEvent(e.Reason, e.Message, e.Type)

	// Determine health based on criticality
	health := models.HealthHealthy
	if isCritical {
		health = models.HealthUnhealthy
	} else if e.Type == "Warning" {
		health = models.HealthDegraded
	}

	timestamp := e.LastTimestamp.Time
	if timestamp.IsZero() {
		timestamp = e.FirstTimestamp.Time
	}
	if timestamp.IsZero() {
		timestamp = time.Now()
	}

	event := models.Event{
		BaseEntity: models.BaseEntity{
			ID:          fmt.Sprintf("event/%s/%s/%s", e.Namespace, e.InvolvedObject.Name, e.Name),
			Type:        models.EntityTypeEvent,
			Labels:      e.Labels,
			Annotations: e.Annotations,
			Health:      health,
			Timestamp:   time.Now(),
		},
		Timestamp:       timestamp,
		EventType:       e.Type,
		Reason:          e.Reason,
		Message:         e.Message,
		ObjectKind:      e.InvolvedObject.Kind,
		ObjectName:      e.InvolvedObject.Name,
		ObjectNamespace: e.InvolvedObject.Namespace,
		IsCritical:      isCritical,
		Category:        category,
	}

	return event
}

// classifyEvent categorizes events and determines criticality
func classifyEvent(reason, message, eventType string) (category string, isCritical bool) {
	reasonLower := strings.ToLower(reason)
	messageLower := strings.ToLower(message)

	// Image pull failures
	if strings.Contains(reasonLower, "imagepull") || 
	   strings.Contains(reasonLower, "errimagepull") ||
	   strings.Contains(messageLower, "failed to pull image") {
		return "image_pull", true
	}

	// CrashLoopBackOff
	if strings.Contains(reasonLower, "backoff") || 
	   strings.Contains(messageLower, "crashloopbackoff") {
		return "crash_loop", true
	}

	// Probe failures
	if strings.Contains(reasonLower, "unhealthy") || 
	   strings.Contains(reasonLower, "probe") ||
	   strings.Contains(messageLower, "liveness probe failed") ||
	   strings.Contains(messageLower, "readiness probe failed") {
		return "probe_failure", true
	}

	// Scheduling failures
	if strings.Contains(reasonLower, "failedscheduling") || 
	   strings.Contains(messageLower, "insufficient") ||
	   strings.Contains(messageLower, "unschedulable") {
		return "scheduling_failure", true
	}

	// Volume mount failures
	if strings.Contains(reasonLower, "failedmount") || 
	   strings.Contains(messageLower, "failed to mount") {
		return "volume_mount", true
	}

	// OOM kills
	if strings.Contains(reasonLower, "oomkilled") || 
	   strings.Contains(messageLower, "out of memory") {
		return "oom_killed", true
	}

	// Container creation failures
	if strings.Contains(reasonLower, "failed") && 
	   (strings.Contains(messageLower, "create container") || 
	    strings.Contains(messageLower, "start container")) {
		return "container_failure", true
	}

	// Network issues
	if strings.Contains(reasonLower, "networkunavailable") || 
	   strings.Contains(messageLower, "network") {
		return "network_issue", eventType == "Warning"
	}

	// Node issues
	if strings.Contains(reasonLower, "nodenotready") || 
	   strings.Contains(reasonLower, "nodepressure") {
		return "node_issue", true
	}

	// Default categorization
	if eventType == "Warning" {
		return "warning", false
	}

	return "normal", false
}
