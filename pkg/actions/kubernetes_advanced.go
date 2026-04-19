package actions

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

// UpdateDeploymentImage updates the container image in a deployment
func (k *KubernetesExecutor) UpdateDeploymentImage(ctx context.Context, namespace, deploymentName, containerName, newImage string) (*ActionResult, error) {
	startTime := time.Now()

	// Get the deployment
	deployment, err := k.clientset.AppsV1().Deployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		return &ActionResult{
			Success:   false,
			Message:   fmt.Sprintf("Failed to get deployment %s", deploymentName),
			Error:     err.Error(),
			StartTime: startTime,
			EndTime:   time.Now(),
		}, err
	}

	// Store old image for rollback
	oldImage := ""
	containerFound := false

	// Update the image
	for i, container := range deployment.Spec.Template.Spec.Containers {
		if containerName == "" || container.Name == containerName {
			oldImage = container.Image
			deployment.Spec.Template.Spec.Containers[i].Image = newImage
			containerFound = true
			if containerName != "" {
				break
			}
		}
	}

	if !containerFound && containerName != "" {
		return &ActionResult{
			Success:   false,
			Message:   fmt.Sprintf("Container %s not found in deployment", containerName),
			StartTime: startTime,
			EndTime:   time.Now(),
		}, fmt.Errorf("container not found")
	}

	// Update the deployment
	_, err = k.clientset.AppsV1().Deployments(namespace).Update(ctx, deployment, metav1.UpdateOptions{})
	if err != nil {
		return &ActionResult{
			Success:   false,
			Message:   "Failed to update deployment",
			Error:     err.Error(),
			StartTime: startTime,
			EndTime:   time.Now(),
		}, err
	}

	// Wait for rollout to complete (with timeout)
	rolloutCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	err = k.waitForRollout(rolloutCtx, namespace, deploymentName)
	if err != nil {
		return &ActionResult{
			Success: false,
			Message: "Deployment updated but rollout did not complete",
			Error:   err.Error(),
			Details: map[string]interface{}{
				"old_image":    oldImage,
				"new_image":    newImage,
				"deployment":   deploymentName,
				"namespace":    namespace,
				"container":    containerName,
			},
			StartTime: startTime,
			EndTime:   time.Now(),
		}, err
	}

	return &ActionResult{
		Success: true,
		Message: fmt.Sprintf("Successfully updated deployment %s to image %s", deploymentName, newImage),
		Details: map[string]interface{}{
			"old_image":    oldImage,
			"new_image":    newImage,
			"deployment":   deploymentName,
			"namespace":    namespace,
			"container":    containerName,
		},
		StartTime: startTime,
		EndTime:   time.Now(),
	}, nil
}

// waitForRollout waits for a deployment rollout to complete
func (k *KubernetesExecutor) waitForRollout(ctx context.Context, namespace, deploymentName string) error {
	return wait.PollUntilContextTimeout(ctx, 2*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		deployment, err := k.clientset.AppsV1().Deployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		// Check if rollout is complete
		if deployment.Status.UpdatedReplicas == *deployment.Spec.Replicas &&
			deployment.Status.Replicas == *deployment.Spec.Replicas &&
			deployment.Status.AvailableReplicas == *deployment.Spec.Replicas &&
			deployment.Status.ObservedGeneration >= deployment.Generation {
			return true, nil
		}

		return false, nil
	})
}

// RolloutRestart performs a rollout restart of a deployment
func (k *KubernetesExecutor) RolloutRestart(ctx context.Context, namespace, deploymentName string) (*ActionResult, error) {
	startTime := time.Now()

	deployment, err := k.clientset.AppsV1().Deployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		return &ActionResult{
			Success:   false,
			Message:   fmt.Sprintf("Failed to get deployment %s", deploymentName),
			Error:     err.Error(),
			StartTime: startTime,
			EndTime:   time.Now(),
		}, err
	}

	// Add/update restart annotation
	if deployment.Spec.Template.Annotations == nil {
		deployment.Spec.Template.Annotations = make(map[string]string)
	}
	deployment.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)

	_, err = k.clientset.AppsV1().Deployments(namespace).Update(ctx, deployment, metav1.UpdateOptions{})
	if err != nil {
		return &ActionResult{
			Success:   false,
			Message:   "Failed to restart deployment",
			Error:     err.Error(),
			StartTime: startTime,
			EndTime:   time.Now(),
		}, err
	}

	return &ActionResult{
		Success:   true,
		Message:   fmt.Sprintf("Successfully restarted deployment %s", deploymentName),
		StartTime: startTime,
		EndTime:   time.Now(),
	}, nil
}

// RolloutUndo rolls back a deployment to the previous revision
func (k *KubernetesExecutor) RolloutUndo(ctx context.Context, namespace, deploymentName string, revision int64) (*ActionResult, error) {
	startTime := time.Now()

	// Get current deployment
	deployment, err := k.clientset.AppsV1().Deployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		return &ActionResult{
			Success:   false,
			Message:   fmt.Sprintf("Failed to get deployment %s", deploymentName),
			Error:     err.Error(),
			StartTime: startTime,
			EndTime:   time.Now(),
		}, err
	}

	// Get ReplicaSets for this deployment
	rsList, err := k.clientset.AppsV1().ReplicaSets(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(deployment.Spec.Selector),
	})
	if err != nil {
		return &ActionResult{
			Success:   false,
			Message:   "Failed to list ReplicaSets",
			Error:     err.Error(),
			StartTime: startTime,
			EndTime:   time.Now(),
		}, err
	}

	// Find the target ReplicaSet
	var targetRS *appsv1.ReplicaSet
	if revision == 0 {
		// Find previous revision (not current)
		for i := range rsList.Items {
			rs := &rsList.Items[i]
			if rs.Annotations["deployment.kubernetes.io/revision"] != deployment.Annotations["deployment.kubernetes.io/revision"] {
				if targetRS == nil {
					targetRS = rs
				} else {
					// Get the most recent one
					targetRev := rs.Annotations["deployment.kubernetes.io/revision"]
					currentRev := targetRS.Annotations["deployment.kubernetes.io/revision"]
					if targetRev > currentRev {
						targetRS = rs
					}
				}
			}
		}
	} else {
		// Find specific revision
		revStr := fmt.Sprintf("%d", revision)
		for i := range rsList.Items {
			rs := &rsList.Items[i]
			if rs.Annotations["deployment.kubernetes.io/revision"] == revStr {
				targetRS = rs
				break
			}
		}
	}

	if targetRS == nil {
		return &ActionResult{
			Success:   false,
			Message:   "No previous revision found",
			StartTime: startTime,
			EndTime:   time.Now(),
		}, fmt.Errorf("no previous revision found")
	}

	// Update deployment with target ReplicaSet's template
	deployment.Spec.Template = targetRS.Spec.Template
	_, err = k.clientset.AppsV1().Deployments(namespace).Update(ctx, deployment, metav1.UpdateOptions{})
	if err != nil {
		return &ActionResult{
			Success:   false,
			Message:   "Failed to rollback deployment",
			Error:     err.Error(),
			StartTime: startTime,
			EndTime:   time.Now(),
		}, err
	}

	return &ActionResult{
		Success: true,
		Message: fmt.Sprintf("Successfully rolled back deployment %s", deploymentName),
		Details: map[string]interface{}{
			"deployment": deploymentName,
			"namespace":  namespace,
			"revision":   targetRS.Annotations["deployment.kubernetes.io/revision"],
		},
		StartTime: startTime,
		EndTime:   time.Now(),
	}, nil
}

// GetRolloutStatus returns the current rollout status of a deployment
func (k *KubernetesExecutor) GetRolloutStatus(ctx context.Context, namespace, deploymentName string) (*ActionResult, error) {
	startTime := time.Now()

	deployment, err := k.clientset.AppsV1().Deployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		return &ActionResult{
			Success:   false,
			Message:   fmt.Sprintf("Failed to get deployment %s", deploymentName),
			Error:     err.Error(),
			StartTime: startTime,
			EndTime:   time.Now(),
		}, err
	}

	status := "in_progress"
	if deployment.Status.UpdatedReplicas == *deployment.Spec.Replicas &&
		deployment.Status.Replicas == *deployment.Spec.Replicas &&
		deployment.Status.AvailableReplicas == *deployment.Spec.Replicas {
		status = "complete"
	} else if deployment.Status.UpdatedReplicas == 0 {
		status = "unknown"
	} else {
		status = "pending"
	}

	return &ActionResult{
		Success: true,
		Message: fmt.Sprintf("Rollout status: %s", status),
		Details: map[string]interface{}{
			"status":             status,
			"desired_replicas":   *deployment.Spec.Replicas,
			"updated_replicas":   deployment.Status.UpdatedReplicas,
			"available_replicas": deployment.Status.AvailableReplicas,
			"replicas":           deployment.Status.Replicas,
		},
		StartTime: startTime,
		EndTime:   time.Now(),
	}, nil
}

// GetPodLogs retrieves logs from a pod
func (k *KubernetesExecutor) GetPodLogs(ctx context.Context, namespace, podName, containerName string, tailLines int64) (*ActionResult, error) {
	startTime := time.Now()

	opts := &corev1.PodLogOptions{
		Container: containerName,
	}
	if tailLines > 0 {
		opts.TailLines = &tailLines
	}

	req := k.clientset.CoreV1().Pods(namespace).GetLogs(podName, opts)
	logs, err := req.Stream(ctx)
	if err != nil {
		return &ActionResult{
			Success:   false,
			Message:   "Failed to get pod logs",
			Error:     err.Error(),
			StartTime: startTime,
			EndTime:   time.Now(),
		}, err
	}
	defer logs.Close()

	buf := make([]byte, 64*1024) // 64KB buffer
	n, _ := logs.Read(buf)

	return &ActionResult{
		Success:   true,
		Message:   "Successfully retrieved logs",
		Output:    string(buf[:n]),
		StartTime: startTime,
		EndTime:   time.Now(),
	}, nil
}
