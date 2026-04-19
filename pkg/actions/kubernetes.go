package actions

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// KubernetesExecutor handles actions on Kubernetes resources
type KubernetesExecutor struct {
	clientset *kubernetes.Clientset
}

// NewKubernetesExecutor creates a new Kubernetes executor
func NewKubernetesExecutor() (*KubernetesExecutor, error) {
	config, err := getKubeConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get kubeconfig: %w", err)
	}

	config.WarningHandler = rest.NoWarnings{}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	return &KubernetesExecutor{
		clientset: clientset,
	}, nil
}

// getKubeConfig attempts to load kubeconfig from multiple sources
func getKubeConfig() (*rest.Config, error) {
	// 1. Try in-cluster config first
	config, err := rest.InClusterConfig()
	if err == nil {
		return config, nil
	}

	// 2. Try KUBECONFIG environment variable
	kubeconfigPath := os.Getenv("KUBECONFIG")
	if kubeconfigPath != "" {
		config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err == nil {
			return config, nil
		}
	}

	// 3. Try default kubeconfig location
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

	return nil, fmt.Errorf("unable to load kubeconfig from any source")
}

// ValidateAction validates a Kubernetes action
func (k *KubernetesExecutor) ValidateAction(action *Action) error {
	if action.Target.Namespace == "" {
		return fmt.Errorf("namespace is required for Kubernetes actions")
	}

	switch action.Type {
	case ActionScaleDeployment:
		if action.Target.EntityID == "" {
			return fmt.Errorf("deployment name is required")
		}
		if _, ok := action.Parameters["replicas"]; !ok {
			return fmt.Errorf("replicas parameter is required for scaling")
		}
		return k.validateDeploymentExists(action.Target.Namespace, action.Target.EntityID)

	case ActionScaleStatefulSet:
		if action.Target.EntityID == "" {
			return fmt.Errorf("statefulset name is required")
		}
		if _, ok := action.Parameters["replicas"]; !ok {
			return fmt.Errorf("replicas parameter is required for scaling")
		}
		return k.validateStatefulSetExists(action.Target.Namespace, action.Target.EntityID)

	case ActionRestartPod:
		if action.Target.EntityID == "" {
			return fmt.Errorf("pod name is required")
		}
		return k.validatePodExists(action.Target.Namespace, action.Target.EntityID)

	default:
		return fmt.Errorf("unsupported kubernetes action type: %s", action.Type)
	}
}

// validateDeploymentExists checks if a deployment exists
func (k *KubernetesExecutor) validateDeploymentExists(namespace, name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := k.clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("deployment %s not found in namespace %s: %w", name, namespace, err)
	}

	return nil
}

// validateStatefulSetExists checks if a statefulset exists
func (k *KubernetesExecutor) validateStatefulSetExists(namespace, name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := k.clientset.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("statefulset %s not found in namespace %s: %w", name, namespace, err)
	}

	return nil
}

// validatePodExists checks if a pod exists
func (k *KubernetesExecutor) validatePodExists(namespace, name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := k.clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("pod %s not found in namespace %s: %w", name, namespace, err)
	}

	return nil
}

// ExecuteAction executes a Kubernetes action
func (k *KubernetesExecutor) ExecuteAction(ctx context.Context, action *Action) (*ActionResult, error) {
	startTime := time.Now()

	switch action.Type {
	case ActionScaleDeployment:
		return k.scaleDeployment(ctx, action.Target.Namespace, action.Target.EntityID, action.Parameters["replicas"], startTime)

	case ActionScaleStatefulSet:
		return k.scaleStatefulSet(ctx, action.Target.Namespace, action.Target.EntityID, action.Parameters["replicas"], startTime)

	case ActionRestartPod:
		return k.restartPod(ctx, action.Target.Namespace, action.Target.EntityID, startTime)

	default:
		return &ActionResult{
			Success:   false,
			Message:   "Unsupported action type",
			Error:     fmt.Sprintf("unsupported kubernetes action type: %s", action.Type),
			StartTime: startTime,
			EndTime:   time.Now(),
		}, fmt.Errorf("unsupported kubernetes action type: %s", action.Type)
	}
}

// scaleDeployment scales a Kubernetes deployment
func (k *KubernetesExecutor) scaleDeployment(ctx context.Context, namespace, name, replicasStr string, startTime time.Time) (*ActionResult, error) {
	var replicas int32
	_, err := fmt.Sscanf(replicasStr, "%d", &replicas)
	if err != nil {
		return &ActionResult{
			Success:   false,
			Message:   "Invalid replicas value",
			Error:     err.Error(),
			StartTime: startTime,
			EndTime:   time.Now(),
		}, err
	}

	// Get the deployment
	deployment, err := k.clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return &ActionResult{
			Success:   false,
			Message:   fmt.Sprintf("Failed to get deployment %s", name),
			Error:     err.Error(),
			StartTime: startTime,
			EndTime:   time.Now(),
		}, err
	}

	// Update replicas
	deployment.Spec.Replicas = &replicas
	_, err = k.clientset.AppsV1().Deployments(namespace).Update(ctx, deployment, metav1.UpdateOptions{})
	if err != nil {
		return &ActionResult{
			Success:   false,
			Message:   fmt.Sprintf("Failed to scale deployment %s", name),
			Error:     err.Error(),
			StartTime: startTime,
			EndTime:   time.Now(),
		}, err
	}

	return &ActionResult{
		Success:   true,
		Message:   fmt.Sprintf("Successfully scaled deployment %s to %d replicas", name, replicas),
		StartTime: startTime,
		EndTime:   time.Now(),
	}, nil
}

// scaleStatefulSet scales a Kubernetes statefulset
func (k *KubernetesExecutor) scaleStatefulSet(ctx context.Context, namespace, name, replicasStr string, startTime time.Time) (*ActionResult, error) {
	var replicas int32
	_, err := fmt.Sscanf(replicasStr, "%d", &replicas)
	if err != nil {
		return &ActionResult{
			Success:   false,
			Message:   "Invalid replicas value",
			Error:     err.Error(),
			StartTime: startTime,
			EndTime:   time.Now(),
		}, err
	}

	// Get the statefulset
	statefulset, err := k.clientset.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return &ActionResult{
			Success:   false,
			Message:   fmt.Sprintf("Failed to get statefulset %s", name),
			Error:     err.Error(),
			StartTime: startTime,
			EndTime:   time.Now(),
		}, err
	}

	// Update replicas
	statefulset.Spec.Replicas = &replicas
	_, err = k.clientset.AppsV1().StatefulSets(namespace).Update(ctx, statefulset, metav1.UpdateOptions{})
	if err != nil {
		return &ActionResult{
			Success:   false,
			Message:   fmt.Sprintf("Failed to scale statefulset %s", name),
			Error:     err.Error(),
			StartTime: startTime,
			EndTime:   time.Now(),
		}, err
	}

	return &ActionResult{
		Success:   true,
		Message:   fmt.Sprintf("Successfully scaled statefulset %s to %d replicas", name, replicas),
		StartTime: startTime,
		EndTime:   time.Now(),
	}, nil
}

// restartPod restarts a Kubernetes pod by deleting it
func (k *KubernetesExecutor) restartPod(ctx context.Context, namespace, name string, startTime time.Time) (*ActionResult, error) {
	// Delete the pod - it will be recreated by its controller
	err := k.clientset.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return &ActionResult{
			Success:   false,
			Message:   fmt.Sprintf("Failed to restart pod %s", name),
			Error:     err.Error(),
			StartTime: startTime,
			EndTime:   time.Now(),
		}, err
	}

	return &ActionResult{
		Success:   true,
		Message:   fmt.Sprintf("Successfully restarted pod %s (deleted for recreation)", name),
		StartTime: startTime,
		EndTime:   time.Now(),
	}, nil
}
