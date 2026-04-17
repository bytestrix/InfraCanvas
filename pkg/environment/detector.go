package environment

import (
	"regexp"
	"strings"

	"infracanvas/internal/models"
)

// Environment represents the detected environment
type Environment string

const (
	EnvProduction  Environment = "production"
	EnvProd        Environment = "prod"
	EnvStaging     Environment = "staging"
	EnvQA          Environment = "qa"
	EnvDev         Environment = "dev"
	EnvDevelopment Environment = "development"
	EnvTest        Environment = "test"
	EnvUnknown     Environment = "unknown"
)

// Detector detects the environment from various infrastructure sources
type Detector struct {
	// Patterns for hostname matching
	hostnamePatterns map[Environment]*regexp.Regexp
	
	// Patterns for namespace matching
	namespacePatterns map[Environment]*regexp.Regexp
	
	// Label keys to check for environment information
	envLabelKeys []string
}

// NewDetector creates a new environment detector with default patterns
func NewDetector() *Detector {
	return &Detector{
		hostnamePatterns: map[Environment]*regexp.Regexp{
			EnvProduction:  regexp.MustCompile(`(?i)(prod|production|prd)`),
			EnvStaging:     regexp.MustCompile(`(?i)(stag|staging|stage|stg)`),
			EnvQA:          regexp.MustCompile(`(?i)(qa|qas|quality)`),
			EnvDev:         regexp.MustCompile(`(?i)(dev|devel|development)`),
			EnvTest:        regexp.MustCompile(`(?i)(test|tst)`),
		},
		namespacePatterns: map[Environment]*regexp.Regexp{
			EnvProduction:  regexp.MustCompile(`(?i)(prod|production|prd)`),
			EnvStaging:     regexp.MustCompile(`(?i)(stag|staging|stage|stg)`),
			EnvQA:          regexp.MustCompile(`(?i)(qa|qas|quality)`),
			EnvDev:         regexp.MustCompile(`(?i)(dev|devel|development)`),
			EnvTest:        regexp.MustCompile(`(?i)(test|tst)`),
		},
		envLabelKeys: []string{
			"environment",
			"env",
			"tier",
			"stage",
		},
	}
}

// DetectFromHost infers environment from host information
func (d *Detector) DetectFromHost(host *models.Host) Environment {
	if host == nil {
		return EnvUnknown
	}
	
	// Check hostname
	if env := d.matchHostname(host.Hostname); env != EnvUnknown {
		return env
	}
	
	// Check FQDN
	if env := d.matchHostname(host.FQDN); env != EnvUnknown {
		return env
	}
	
	// Check cloud provider tags
	if host.CloudTags != nil {
		if env := d.detectFromLabels(host.CloudTags); env != EnvUnknown {
			return env
		}
	}
	
	return EnvUnknown
}

// DetectFromContainer infers environment from Docker container
func (d *Detector) DetectFromContainer(container *models.Container) Environment {
	if container == nil {
		return EnvUnknown
	}
	
	// Check container labels
	if env := d.detectFromLabels(container.Labels); env != EnvUnknown {
		return env
	}
	
	// Check container name
	if env := d.matchHostname(container.Name); env != EnvUnknown {
		return env
	}
	
	return EnvUnknown
}

// DetectFromNamespace infers environment from Kubernetes namespace
func (d *Detector) DetectFromNamespace(namespace *models.Namespace) Environment {
	if namespace == nil {
		return EnvUnknown
	}
	
	// Check namespace name
	if env := d.matchNamespace(namespace.Name); env != EnvUnknown {
		return env
	}
	
	// Check namespace labels
	if env := d.detectFromLabels(namespace.Labels); env != EnvUnknown {
		return env
	}
	
	return EnvUnknown
}

// DetectFromPod infers environment from Kubernetes pod
func (d *Detector) DetectFromPod(pod *models.Pod) Environment {
	if pod == nil {
		return EnvUnknown
	}
	
	// Check pod namespace
	if env := d.matchNamespace(pod.Namespace); env != EnvUnknown {
		return env
	}
	
	// Check pod labels
	if env := d.detectFromLabels(pod.Labels); env != EnvUnknown {
		return env
	}
	
	return EnvUnknown
}

// DetectFromDeployment infers environment from Kubernetes deployment
func (d *Detector) DetectFromDeployment(deployment *models.Deployment) Environment {
	if deployment == nil {
		return EnvUnknown
	}
	
	// Check deployment namespace
	if env := d.matchNamespace(deployment.Namespace); env != EnvUnknown {
		return env
	}
	
	// Check deployment labels
	if env := d.detectFromLabels(deployment.Labels); env != EnvUnknown {
		return env
	}
	
	return EnvUnknown
}

// DetectFromStatefulSet infers environment from Kubernetes statefulset
func (d *Detector) DetectFromStatefulSet(statefulset *models.StatefulSet) Environment {
	if statefulset == nil {
		return EnvUnknown
	}
	
	// Check statefulset namespace
	if env := d.matchNamespace(statefulset.Namespace); env != EnvUnknown {
		return env
	}
	
	// Check statefulset labels
	if env := d.detectFromLabels(statefulset.Labels); env != EnvUnknown {
		return env
	}
	
	return EnvUnknown
}

// DetectFromDaemonSet infers environment from Kubernetes daemonset
func (d *Detector) DetectFromDaemonSet(daemonset *models.DaemonSet) Environment {
	if daemonset == nil {
		return EnvUnknown
	}
	
	// Check daemonset namespace
	if env := d.matchNamespace(daemonset.Namespace); env != EnvUnknown {
		return env
	}
	
	// Check daemonset labels
	if env := d.detectFromLabels(daemonset.Labels); env != EnvUnknown {
		return env
	}
	
	return EnvUnknown
}

// matchHostname matches hostname against environment patterns.
// Patterns are checked in a fixed priority order so that e.g. "qa-test-server"
// matches QA (via the qa prefix) rather than Test (via the test suffix).
func (d *Detector) matchHostname(hostname string) Environment {
	if hostname == "" {
		return EnvUnknown
	}

	priority := []Environment{EnvProduction, EnvStaging, EnvQA, EnvDev, EnvTest}
	for _, env := range priority {
		if pattern, ok := d.hostnamePatterns[env]; ok {
			if pattern.MatchString(hostname) {
				return env
			}
		}
	}

	return EnvUnknown
}

// matchNamespace matches namespace name against environment patterns.
// Patterns are checked in a fixed priority order so that e.g. "qas-testing"
// matches QA (via the qas prefix) rather than Test (via the test suffix).
func (d *Detector) matchNamespace(namespace string) Environment {
	if namespace == "" {
		return EnvUnknown
	}

	priority := []Environment{EnvProduction, EnvStaging, EnvQA, EnvDev, EnvTest}
	for _, env := range priority {
		if pattern, ok := d.namespacePatterns[env]; ok {
			if pattern.MatchString(namespace) {
				return env
			}
		}
	}

	return EnvUnknown
}

// detectFromLabels detects environment from labels or tags
func (d *Detector) detectFromLabels(labels map[string]string) Environment {
	if labels == nil {
		return EnvUnknown
	}
	
	// Check each known environment label key
	for _, key := range d.envLabelKeys {
		if value, exists := labels[key]; exists {
			if env := d.normalizeEnvironment(value); env != EnvUnknown {
				return env
			}
		}
	}
	
	return EnvUnknown
}

// normalizeEnvironment normalizes environment string to standard Environment type
func (d *Detector) normalizeEnvironment(value string) Environment {
	normalized := strings.ToLower(strings.TrimSpace(value))
	
	switch normalized {
	case "prod", "production", "prd":
		return EnvProduction
	case "staging", "stage", "stg", "stag":
		return EnvStaging
	case "qa", "qas", "quality":
		return EnvQA
	case "dev", "devel", "development":
		return EnvDev
	case "test", "tst", "testing":
		return EnvTest
	default:
		return EnvUnknown
	}
}

// DetectFromSnapshot analyzes an entire snapshot and returns environment information
// for each entity. This is useful for batch processing.
func (d *Detector) DetectFromSnapshot(snapshot *models.InfraSnapshot) map[string]Environment {
	envMap := make(map[string]Environment)
	
	if snapshot == nil {
		return envMap
	}
	
	for id, entity := range snapshot.Entities {
		var env Environment = EnvUnknown
		
		switch e := entity.(type) {
		case *models.Host:
			env = d.DetectFromHost(e)
		case *models.Container:
			env = d.DetectFromContainer(e)
		case *models.Namespace:
			env = d.DetectFromNamespace(e)
		case *models.Pod:
			env = d.DetectFromPod(e)
		case *models.Deployment:
			env = d.DetectFromDeployment(e)
		case *models.StatefulSet:
			env = d.DetectFromStatefulSet(e)
		case *models.DaemonSet:
			env = d.DetectFromDaemonSet(e)
		}
		
		if env != EnvUnknown {
			envMap[id] = env
		}
	}
	
	return envMap
}
