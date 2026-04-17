package permissions

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/docker/docker/client"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PermissionLevel represents the level of access available
type PermissionLevel string

const (
	PermissionFull    PermissionLevel = "full"
	PermissionPartial PermissionLevel = "partial"
	PermissionNone    PermissionLevel = "none"
)

// PermissionCheck represents the result of a permission check
type PermissionCheck struct {
	Layer      string          `json:"layer"`
	Operation  string          `json:"operation"`
	Required   bool            `json:"required"`
	Available  bool            `json:"available"`
	Level      PermissionLevel `json:"level"`
	Message    string          `json:"message"`
	Suggestion string          `json:"suggestion,omitempty"`
}

// Checker performs permission validation for infrastructure discovery
type Checker struct {
	checks []PermissionCheck
}

// NewChecker creates a new permission checker
func NewChecker() *Checker {
	return &Checker{
		checks: []PermissionCheck{},
	}
}

// ValidatePermissions validates permissions for the specified scopes
func (c *Checker) ValidatePermissions(scopes []string) []PermissionCheck {
	c.checks = []PermissionCheck{}

	for _, scope := range scopes {
		switch scope {
		case "host":
			c.checkHostPermissions()
		case "docker":
			c.checkDockerPermissions()
		case "kubernetes", "k8s":
			c.checkKubernetesPermissions()
		}
	}

	return c.checks
}

// GetSummary returns a summary of permission checks
func (c *Checker) GetSummary() (available, unavailable, partial int) {
	for _, check := range c.checks {
		if check.Available {
			if check.Level == PermissionFull {
				available++
			} else if check.Level == PermissionPartial {
				partial++
			}
		} else {
			unavailable++
		}
	}
	return
}

// HasCriticalIssues returns true if any required permissions are unavailable
func (c *Checker) HasCriticalIssues() bool {
	for _, check := range c.checks {
		if check.Required && !check.Available {
			return true
		}
	}
	return false
}

// checkHostPermissions validates host-level permissions
func (c *Checker) checkHostPermissions() {
	// Check basic file system access
	c.addCheck(PermissionCheck{
		Layer:     "host",
		Operation: "read_os_info",
		Required:  true,
		Available: c.canReadFile("/etc/os-release"),
		Level:     c.getLevel(c.canReadFile("/etc/os-release")),
		Message:   "Read OS information from /etc/os-release",
		Suggestion: "Ensure the file /etc/os-release exists and is readable",
	})

	// Check /proc access
	c.addCheck(PermissionCheck{
		Layer:     "host",
		Operation: "read_proc",
		Required:  true,
		Available: c.canReadDir("/proc"),
		Level:     c.getLevel(c.canReadDir("/proc")),
		Message:   "Read process information from /proc",
		Suggestion: "Ensure /proc filesystem is mounted and accessible",
	})

	// Check systemd availability
	systemdAvailable := c.isCommandAvailable("systemctl")
	c.addCheck(PermissionCheck{
		Layer:     "host",
		Operation: "systemd_access",
		Required:  false,
		Available: systemdAvailable,
		Level:     c.getLevel(systemdAvailable),
		Message:   "Access systemd services via systemctl",
		Suggestion: "Install systemd or run on a system with systemd support",
	})

	// Check journalctl availability
	journalAvailable := c.isCommandAvailable("journalctl")
	c.addCheck(PermissionCheck{
		Layer:     "host",
		Operation: "journal_access",
		Required:  false,
		Available: journalAvailable,
		Level:     c.getLevel(journalAvailable),
		Message:   "Access system logs via journalctl",
		Suggestion: "Install systemd or add user to 'systemd-journal' group",
	})

	// Check if running as root or with elevated permissions
	isRoot := os.Geteuid() == 0
	c.addCheck(PermissionCheck{
		Layer:     "host",
		Operation: "elevated_access",
		Required:  false,
		Available: isRoot,
		Level:     c.getPartialLevel(isRoot),
		Message:   "Elevated permissions for full process and port information",
		Suggestion: "Run with sudo or as root for complete host discovery",
	})
}

// checkDockerPermissions validates Docker-level permissions
func (c *Checker) checkDockerPermissions() {
	// Check Docker socket accessibility
	dockerSocket := "/var/run/docker.sock"
	if dockerHost := os.Getenv("DOCKER_HOST"); dockerHost != "" {
		// If DOCKER_HOST is set, we'll check connectivity instead
		socketAccessible := c.canAccessDockerSocket()
		c.addCheck(PermissionCheck{
			Layer:     "docker",
			Operation: "docker_socket",
			Required:  true,
			Available: socketAccessible,
			Level:     c.getLevel(socketAccessible),
			Message:   fmt.Sprintf("Access Docker via DOCKER_HOST=%s", dockerHost),
			Suggestion: "Ensure DOCKER_HOST is correctly configured and accessible",
		})
	} else {
		socketAccessible := c.canAccessFile(dockerSocket)
		c.addCheck(PermissionCheck{
			Layer:     "docker",
			Operation: "docker_socket",
			Required:  true,
			Available: socketAccessible,
			Level:     c.getLevel(socketAccessible),
			Message:   fmt.Sprintf("Access Docker socket at %s", dockerSocket),
			Suggestion: "Add user to 'docker' group: sudo usermod -aG docker $USER",
		})
	}

	// Check Docker client connectivity
	dockerAvailable := c.canConnectToDocker()
	c.addCheck(PermissionCheck{
		Layer:     "docker",
		Operation: "docker_api",
		Required:  true,
		Available: dockerAvailable,
		Level:     c.getLevel(dockerAvailable),
		Message:   "Connect to Docker API",
		Suggestion: "Ensure Docker daemon is running and accessible",
	})
}

// checkKubernetesPermissions validates Kubernetes-level permissions
func (c *Checker) checkKubernetesPermissions() {
	// Check kubeconfig availability
	kubeconfigPath := c.getKubeconfigPath()
	kubeconfigExists := kubeconfigPath != ""
	
	c.addCheck(PermissionCheck{
		Layer:     "kubernetes",
		Operation: "kubeconfig",
		Required:  true,
		Available: kubeconfigExists,
		Level:     c.getLevel(kubeconfigExists),
		Message:   "Kubeconfig file found",
		Suggestion: "Configure kubectl or set KUBECONFIG environment variable",
	})

	if !kubeconfigExists {
		return
	}

	// Check Kubernetes API connectivity
	k8sAvailable, config := c.canConnectToKubernetes()
	c.addCheck(PermissionCheck{
		Layer:     "kubernetes",
		Operation: "k8s_api",
		Required:  true,
		Available: k8sAvailable,
		Level:     c.getLevel(k8sAvailable),
		Message:   "Connect to Kubernetes API server",
		Suggestion: "Ensure cluster is running and kubeconfig is valid",
	})

	if !k8sAvailable || config == nil {
		return
	}

	// Check specific resource permissions
	c.checkKubernetesResourcePermissions(config)
}

// checkKubernetesResourcePermissions checks permissions for specific Kubernetes resources
func (c *Checker) checkKubernetesResourcePermissions(config *rest.Config) {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check node access
	_, err = clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{Limit: 1})
	c.addCheck(PermissionCheck{
		Layer:     "kubernetes",
		Operation: "list_nodes",
		Required:  false,
		Available: err == nil,
		Level:     c.getLevel(err == nil),
		Message:   "List cluster nodes",
		Suggestion: "Grant 'get' and 'list' permissions for nodes resource",
	})

	// Check pod access
	_, err = clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{Limit: 1})
	c.addCheck(PermissionCheck{
		Layer:     "kubernetes",
		Operation: "list_pods",
		Required:  true,
		Available: err == nil,
		Level:     c.getLevel(err == nil),
		Message:   "List pods across all namespaces",
		Suggestion: "Grant 'get' and 'list' permissions for pods resource",
	})

	// Check deployment access
	_, err = clientset.AppsV1().Deployments("").List(ctx, metav1.ListOptions{Limit: 1})
	c.addCheck(PermissionCheck{
		Layer:     "kubernetes",
		Operation: "list_deployments",
		Required:  false,
		Available: err == nil,
		Level:     c.getLevel(err == nil),
		Message:   "List deployments",
		Suggestion: "Grant 'get' and 'list' permissions for deployments resource",
	})

	// Check service access
	_, err = clientset.CoreV1().Services("").List(ctx, metav1.ListOptions{Limit: 1})
	c.addCheck(PermissionCheck{
		Layer:     "kubernetes",
		Operation: "list_services",
		Required:  false,
		Available: err == nil,
		Level:     c.getLevel(err == nil),
		Message:   "List services",
		Suggestion: "Grant 'get' and 'list' permissions for services resource",
	})

	// Check event access
	_, err = clientset.CoreV1().Events("").List(ctx, metav1.ListOptions{Limit: 1})
	c.addCheck(PermissionCheck{
		Layer:     "kubernetes",
		Operation: "list_events",
		Required:  false,
		Available: err == nil,
		Level:     c.getLevel(err == nil),
		Message:   "List cluster events",
		Suggestion: "Grant 'get' and 'list' permissions for events resource",
	})
}

// Helper methods

func (c *Checker) addCheck(check PermissionCheck) {
	c.checks = append(c.checks, check)
}

func (c *Checker) canReadFile(path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		return false
	}
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	file.Close()
	return true
}

func (c *Checker) canReadDir(path string) bool {
	_, err := os.ReadDir(path)
	return err == nil
}

func (c *Checker) canAccessFile(path string) bool {
	return c.canReadFile(path)
}

func (c *Checker) isCommandAvailable(command string) bool {
	_, err := exec.LookPath(command)
	return err == nil
}

func (c *Checker) canAccessDockerSocket() bool {
	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return false
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err = cli.Ping(ctx)
	return err == nil
}

func (c *Checker) canConnectToDocker() bool {
	return c.canAccessDockerSocket()
}

func (c *Checker) getKubeconfigPath() string {
	// Check in-cluster config
	if _, err := rest.InClusterConfig(); err == nil {
		return "in-cluster"
	}

	// Check KUBECONFIG environment variable
	if kubeconfigPath := os.Getenv("KUBECONFIG"); kubeconfigPath != "" {
		if _, err := os.Stat(kubeconfigPath); err == nil {
			return kubeconfigPath
		}
	}

	// Check default location
	homeDir, err := os.UserHomeDir()
	if err == nil {
		defaultKubeconfig := filepath.Join(homeDir, ".kube", "config")
		if _, err := os.Stat(defaultKubeconfig); err == nil {
			return defaultKubeconfig
		}
	}

	return ""
}

func (c *Checker) canConnectToKubernetes() (bool, *rest.Config) {
	config, err := c.getKubeConfig()
	if err != nil {
		return false, nil
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return false, nil
	}

	_, err = clientset.Discovery().ServerVersion()
	return err == nil, config
}

func (c *Checker) getKubeConfig() (*rest.Config, error) {
	// Try in-cluster config
	config, err := rest.InClusterConfig()
	if err == nil {
		return config, nil
	}

	// Try KUBECONFIG environment variable
	if kubeconfigPath := os.Getenv("KUBECONFIG"); kubeconfigPath != "" {
		config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err == nil {
			return config, nil
		}
	}

	// Try default location
	homeDir, err := os.UserHomeDir()
	if err == nil {
		defaultKubeconfig := filepath.Join(homeDir, ".kube", "config")
		if _, err := os.Stat(defaultKubeconfig); err == nil {
			config, err := clientcmd.BuildConfigFromFlags("", defaultKubeconfig)
			if err == nil {
				return config, nil
			}
		}
	}

	return nil, fmt.Errorf("unable to load kubeconfig")
}

func (c *Checker) getLevel(available bool) PermissionLevel {
	if available {
		return PermissionFull
	}
	return PermissionNone
}

func (c *Checker) getPartialLevel(available bool) PermissionLevel {
	if available {
		return PermissionFull
	}
	return PermissionPartial
}
