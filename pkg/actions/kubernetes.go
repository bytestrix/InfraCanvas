package actions

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// KubernetesExecutor handles actions on Kubernetes resources
type KubernetesExecutor struct {
	clientset     *kubernetes.Clientset
	dynamicClient dynamic.Interface
	config        *rest.Config
}

// NewKubernetesExecutor creates a new Kubernetes executor, resolving the
// kubeconfig from the local host.
func NewKubernetesExecutor() (*KubernetesExecutor, error) {
	config, err := getKubeConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get kubeconfig: %w", err)
	}
	return NewKubernetesExecutorFromConfig(config)
}

// NewKubernetesExecutorFromConfig creates a KubernetesExecutor against an
// explicit *rest.Config — used for Clusters connections (an uploaded
// kubeconfig) rather than the local host's own cluster access.
func NewKubernetesExecutorFromConfig(config *rest.Config) (*KubernetesExecutor, error) {
	config.WarningHandler = rest.NoWarnings{}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	return &KubernetesExecutor{
		clientset:     clientset,
		dynamicClient: dynClient,
		config:        config,
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
	if action.Target.EntityID == "" {
		return fmt.Errorf("entity ID is required")
	}
	return nil
}

// ExecuteAction executes a Kubernetes action
func (k *KubernetesExecutor) ExecuteAction(ctx context.Context, action *Action) (*ActionResult, error) {
	startTime := time.Now()
	ns := action.Target.Namespace
	if ns == "" {
		ns = "default"
	}
	name := action.Target.EntityID

	switch action.Type {
	case ActionScaleDeployment, ActionK8sScaleDeployment:
		return k.scaleDeployment(ctx, ns, name, action.Parameters["replicas"], startTime)
	case ActionScaleStatefulSet, ActionK8sScaleStatefulSet:
		return k.scaleStatefulSet(ctx, ns, name, action.Parameters["replicas"], startTime)
	case ActionRestartPod:
		return k.deletePod(ctx, ns, name, startTime)
	case ActionK8sDeletePod:
		return k.deletePod(ctx, ns, name, startTime)
	case ActionK8sDeleteJob:
		return k.deleteJob(ctx, ns, name, startTime)
	case ActionK8sDeleteDeployment:
		return k.deleteDeployment(ctx, ns, name, startTime)
	case ActionK8sDeleteStatefulSet:
		return k.deleteStatefulSet(ctx, ns, name, startTime)
	case ActionK8sDeleteService:
		return k.deleteService(ctx, ns, name, startTime)
	case ActionK8sCordonNode:
		return k.cordonNode(ctx, name, true, startTime)
	case ActionK8sUnCordonNode:
		return k.cordonNode(ctx, name, false, startTime)
	case ActionK8sDrainNode:
		return k.drainNode(ctx, name, startTime)
	case ActionK8sGetConfigMap:
		return k.getConfigMap(ctx, ns, name, startTime)
	case ActionK8sUpdateConfigMap:
		return k.updateConfigMap(ctx, ns, name, action.Parameters, startTime)
	case ActionK8sGetSecret:
		return k.getSecret(ctx, ns, name, startTime)
	case ActionK8sUpdateSecret:
		return k.updateSecret(ctx, ns, name, action.Parameters, startTime)
	case ActionK8sApplyManifest:
		return k.applyManifest(ctx, action.Parameters["manifest"], startTime)
	case ActionK8sDeleteResource:
		return k.deleteResource(ctx, ns, action.Parameters["resource_type"], name, startTime)
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

// deletePod deletes a pod (it will be recreated by its controller).
func (k *KubernetesExecutor) deletePod(ctx context.Context, namespace, name string, startTime time.Time) (*ActionResult, error) {
	err := k.clientset.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return &ActionResult{Success: false, Message: fmt.Sprintf("Failed to delete pod %s", name), Error: err.Error(), StartTime: startTime, EndTime: time.Now()}, err
	}
	return &ActionResult{Success: true, Message: fmt.Sprintf("Pod %s deleted (will be recreated by controller)", name), StartTime: startTime, EndTime: time.Now()}, nil
}

// deleteJob deletes a Kubernetes Job and its pods.
func (k *KubernetesExecutor) deleteJob(ctx context.Context, namespace, name string, startTime time.Time) (*ActionResult, error) {
	propagation := metav1.DeletePropagationForeground
	err := k.clientset.BatchV1().Jobs(namespace).Delete(ctx, name, metav1.DeleteOptions{PropagationPolicy: &propagation})
	if err != nil {
		return &ActionResult{Success: false, Message: fmt.Sprintf("Failed to delete job %s", name), Error: err.Error(), StartTime: startTime, EndTime: time.Now()}, err
	}
	return &ActionResult{Success: true, Message: fmt.Sprintf("Job %s deleted", name), StartTime: startTime, EndTime: time.Now()}, nil
}

// deleteDeployment deletes a Kubernetes Deployment and its pods.
func (k *KubernetesExecutor) deleteDeployment(ctx context.Context, namespace, name string, startTime time.Time) (*ActionResult, error) {
	propagation := metav1.DeletePropagationForeground
	err := k.clientset.AppsV1().Deployments(namespace).Delete(ctx, name, metav1.DeleteOptions{PropagationPolicy: &propagation})
	if err != nil {
		return &ActionResult{Success: false, Message: fmt.Sprintf("Failed to delete deployment %s", name), Error: err.Error(), StartTime: startTime, EndTime: time.Now()}, err
	}
	return &ActionResult{Success: true, Message: fmt.Sprintf("Deployment %s deleted", name), StartTime: startTime, EndTime: time.Now()}, nil
}

// deleteStatefulSet deletes a Kubernetes StatefulSet and its pods.
func (k *KubernetesExecutor) deleteStatefulSet(ctx context.Context, namespace, name string, startTime time.Time) (*ActionResult, error) {
	propagation := metav1.DeletePropagationForeground
	err := k.clientset.AppsV1().StatefulSets(namespace).Delete(ctx, name, metav1.DeleteOptions{PropagationPolicy: &propagation})
	if err != nil {
		return &ActionResult{Success: false, Message: fmt.Sprintf("Failed to delete statefulset %s", name), Error: err.Error(), StartTime: startTime, EndTime: time.Now()}, err
	}
	return &ActionResult{Success: true, Message: fmt.Sprintf("StatefulSet %s deleted", name), StartTime: startTime, EndTime: time.Now()}, nil
}

// deleteService deletes a Kubernetes Service.
func (k *KubernetesExecutor) deleteService(ctx context.Context, namespace, name string, startTime time.Time) (*ActionResult, error) {
	err := k.clientset.CoreV1().Services(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return &ActionResult{Success: false, Message: fmt.Sprintf("Failed to delete service %s", name), Error: err.Error(), StartTime: startTime, EndTime: time.Now()}, err
	}
	return &ActionResult{Success: true, Message: fmt.Sprintf("Service %s deleted", name), StartTime: startTime, EndTime: time.Now()}, nil
}

// cordonNode marks a node schedulable (false) or unschedulable (true).
func (k *KubernetesExecutor) cordonNode(ctx context.Context, name string, unschedulable bool, startTime time.Time) (*ActionResult, error) {
	node, err := k.clientset.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return &ActionResult{Success: false, Message: fmt.Sprintf("Failed to get node %s", name), Error: err.Error(), StartTime: startTime, EndTime: time.Now()}, err
	}
	node.Spec.Unschedulable = unschedulable
	_, err = k.clientset.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
	if err != nil {
		return &ActionResult{Success: false, Message: fmt.Sprintf("Failed to update node %s", name), Error: err.Error(), StartTime: startTime, EndTime: time.Now()}, err
	}
	action := "cordoned"
	if !unschedulable {
		action = "uncordoned"
	}
	return &ActionResult{Success: true, Message: fmt.Sprintf("Node %s %s", name, action), StartTime: startTime, EndTime: time.Now()}, nil
}

// drainNode cordons a node then evicts all non-DaemonSet pods.
func (k *KubernetesExecutor) drainNode(ctx context.Context, name string, startTime time.Time) (*ActionResult, error) {
	// Cordon first
	if _, err := k.cordonNode(ctx, name, true, startTime); err != nil {
		return &ActionResult{Success: false, Message: "Drain failed: could not cordon node", Error: err.Error(), StartTime: startTime, EndTime: time.Now()}, err
	}

	// List pods on this node
	pods, err := k.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("spec.nodeName", name).String(),
	})
	if err != nil {
		return &ActionResult{Success: false, Message: "Drain failed: could not list pods", Error: err.Error(), StartTime: startTime, EndTime: time.Now()}, err
	}

	evicted := 0
	skipped := 0
	for _, pod := range pods.Items {
		// Skip DaemonSet pods (they can't be evicted meaningfully)
		isDaemonSet := false
		for _, ref := range pod.OwnerReferences {
			if ref.Kind == "DaemonSet" {
				isDaemonSet = true
				break
			}
		}
		if isDaemonSet {
			skipped++
			continue
		}
		eviction := &policyv1.Eviction{
			ObjectMeta: metav1.ObjectMeta{Name: pod.Name, Namespace: pod.Namespace},
		}
		if err := k.clientset.CoreV1().Pods(pod.Namespace).EvictV1(ctx, eviction); err != nil {
			// Best-effort: log but continue
			skipped++
			continue
		}
		evicted++
	}

	return &ActionResult{
		Success:   true,
		Message:   fmt.Sprintf("Node %s drained: %d pods evicted, %d skipped (DaemonSet or error)", name, evicted, skipped),
		StartTime: startTime, EndTime: time.Now(),
	}, nil
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
