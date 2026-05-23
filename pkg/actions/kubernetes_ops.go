package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

// ── ConfigMap ────────────────────────────────────────────────────────────────

func (k *KubernetesExecutor) getConfigMap(ctx context.Context, namespace, name string, startTime time.Time) (*ActionResult, error) {
	cm, err := k.clientset.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return &ActionResult{Success: false, Message: fmt.Sprintf("Failed to get configmap %s", name), Error: err.Error(), StartTime: startTime, EndTime: time.Now()}, err
	}
	raw, _ := json.Marshal(cm.Data)
	return &ActionResult{
		Success:   true,
		Message:   fmt.Sprintf("ConfigMap %s/%s", namespace, name),
		Output:    string(raw),
		Details:   map[string]interface{}{"keys": len(cm.Data), "namespace": namespace, "name": name},
		StartTime: startTime, EndTime: time.Now(),
	}, nil
}

func (k *KubernetesExecutor) updateConfigMap(ctx context.Context, namespace, name string, data map[string]string, startTime time.Time) (*ActionResult, error) {
	cm, err := k.clientset.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return &ActionResult{Success: false, Message: fmt.Sprintf("Failed to get configmap %s", name), Error: err.Error(), StartTime: startTime, EndTime: time.Now()}, err
	}
	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}
	for k2, v := range data {
		cm.Data[k2] = v
	}
	_, err = k.clientset.CoreV1().ConfigMaps(namespace).Update(ctx, cm, metav1.UpdateOptions{})
	if err != nil {
		return &ActionResult{Success: false, Message: fmt.Sprintf("Failed to update configmap %s", name), Error: err.Error(), StartTime: startTime, EndTime: time.Now()}, err
	}
	return &ActionResult{Success: true, Message: fmt.Sprintf("ConfigMap %s/%s updated", namespace, name), StartTime: startTime, EndTime: time.Now()}, nil
}

// ── Secret ───────────────────────────────────────────────────────────────────

func (k *KubernetesExecutor) getSecret(ctx context.Context, namespace, name string, startTime time.Time) (*ActionResult, error) {
	secret, err := k.clientset.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return &ActionResult{Success: false, Message: fmt.Sprintf("Failed to get secret %s", name), Error: err.Error(), StartTime: startTime, EndTime: time.Now()}, err
	}
	// Convert []byte values to strings for JSON output
	decoded := make(map[string]string, len(secret.Data))
	for k2, v := range secret.Data {
		decoded[k2] = string(v)
	}
	raw, _ := json.Marshal(decoded)
	return &ActionResult{
		Success:   true,
		Message:   fmt.Sprintf("Secret %s/%s (type: %s)", namespace, name, secret.Type),
		Output:    string(raw),
		Details:   map[string]interface{}{"keys": len(secret.Data), "namespace": namespace, "name": name, "type": string(secret.Type)},
		StartTime: startTime, EndTime: time.Now(),
	}, nil
}

func (k *KubernetesExecutor) updateSecret(ctx context.Context, namespace, name string, data map[string]string, startTime time.Time) (*ActionResult, error) {
	secret, err := k.clientset.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return &ActionResult{Success: false, Message: fmt.Sprintf("Failed to get secret %s", name), Error: err.Error(), StartTime: startTime, EndTime: time.Now()}, err
	}
	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}
	for k2, v := range data {
		secret.Data[k2] = []byte(v)
	}
	_, err = k.clientset.CoreV1().Secrets(namespace).Update(ctx, secret, metav1.UpdateOptions{})
	if err != nil {
		return &ActionResult{Success: false, Message: fmt.Sprintf("Failed to update secret %s", name), Error: err.Error(), StartTime: startTime, EndTime: time.Now()}, err
	}
	return &ActionResult{Success: true, Message: fmt.Sprintf("Secret %s/%s updated", namespace, name), StartTime: startTime, EndTime: time.Now()}, nil
}

// ── Apply manifest ────────────────────────────────────────────────────────────

func (k *KubernetesExecutor) applyManifest(ctx context.Context, manifestYAML string, startTime time.Time) (*ActionResult, error) {
	if manifestYAML == "" {
		return &ActionResult{Success: false, Message: "manifest is required", StartTime: startTime, EndTime: time.Now()}, fmt.Errorf("manifest required")
	}

	// Discover API groups for REST mapping
	disco := k.clientset.Discovery()
	serverGroups, serverResources, err := disco.ServerGroupsAndResources()
	if err != nil && serverGroups == nil {
		return &ActionResult{Success: false, Message: "Failed to discover API resources", Error: err.Error(), StartTime: startTime, EndTime: time.Now()}, err
	}

	// Build GVR lookup: kind → GVR
	gvrByKind := map[string]schema.GroupVersionResource{}
	for _, rList := range serverResources {
		gv, err2 := schema.ParseGroupVersion(rList.GroupVersion)
		if err2 != nil {
			continue
		}
		for _, r := range rList.APIResources {
			if strings.Contains(r.Name, "/") {
				continue // skip sub-resources
			}
			gvrByKind[strings.ToLower(r.Kind)] = schema.GroupVersionResource{Group: gv.Group, Version: gv.Version, Resource: r.Name}
		}
	}

	// Split and process multi-document YAML
	decoder := k8syaml.NewYAMLOrJSONDecoder(strings.NewReader(manifestYAML), 4096)
	applied := 0
	var lastErr error
	for {
		raw := &unstructured.Unstructured{}
		err := decoder.Decode(raw)
		if err == io.EOF {
			break
		}
		if err != nil {
			lastErr = fmt.Errorf("parse error: %w", err)
			break
		}
		if raw.GetKind() == "" {
			continue
		}

		kind := strings.ToLower(raw.GetKind())
		gvr, ok := gvrByKind[kind]
		if !ok {
			lastErr = fmt.Errorf("unknown resource kind: %s", raw.GetKind())
			continue
		}

		ns := raw.GetNamespace()
		name := raw.GetName()
		if name == "" {
			continue
		}

		rawJSON, err := json.Marshal(raw.Object)
		if err != nil {
			lastErr = err
			continue
		}

		var res *unstructured.Unstructured
		if ns != "" {
			res, err = k.dynamicClient.Resource(gvr).Namespace(ns).Patch(
				ctx, name, types.ApplyPatchType, rawJSON,
				metav1.PatchOptions{FieldManager: "infracanvas", Force: boolPtr(true)},
			)
		} else {
			res, err = k.dynamicClient.Resource(gvr).Patch(
				ctx, name, types.ApplyPatchType, rawJSON,
				metav1.PatchOptions{FieldManager: "infracanvas", Force: boolPtr(true)},
			)
		}
		if err != nil {
			lastErr = fmt.Errorf("apply %s/%s: %w", raw.GetKind(), name, err)
			continue
		}
		_ = res
		applied++
	}

	if applied == 0 && lastErr != nil {
		return &ActionResult{Success: false, Message: "Manifest apply failed", Error: lastErr.Error(), StartTime: startTime, EndTime: time.Now()}, lastErr
	}
	msg := fmt.Sprintf("Applied %d resource(s)", applied)
	if lastErr != nil {
		msg += fmt.Sprintf(" (last error: %s)", lastErr.Error())
	}
	return &ActionResult{Success: true, Message: msg, StartTime: startTime, EndTime: time.Now()}, nil
}

func boolPtr(b bool) *bool { return &b }

// ── Generic delete ────────────────────────────────────────────────────────────

// deleteResource deletes any K8s resource by type, namespace, and name.
// resourceType examples: "deployment", "service", "configmap", "secret", "pod", "job", "ingress"
func (k *KubernetesExecutor) deleteResource(ctx context.Context, namespace, resourceType, name string, startTime time.Time) (*ActionResult, error) {
	disco := k.clientset.Discovery()
	_, serverResources, err := disco.ServerGroupsAndResources()
	if err != nil && serverResources == nil {
		return &ActionResult{Success: false, Message: "Failed to discover API resources", Error: err.Error(), StartTime: startTime, EndTime: time.Now()}, err
	}

	want := strings.ToLower(resourceType)
	var found *schema.GroupVersionResource
	for _, rList := range serverResources {
		gv, err2 := schema.ParseGroupVersion(rList.GroupVersion)
		if err2 != nil {
			continue
		}
		for _, r := range rList.APIResources {
			if strings.Contains(r.Name, "/") {
				continue
			}
			if strings.ToLower(r.Kind) == want || strings.ToLower(r.Name) == want {
				gvr := schema.GroupVersionResource{Group: gv.Group, Version: gv.Version, Resource: r.Name}
				found = &gvr
				break
			}
		}
		if found != nil {
			break
		}
	}

	if found == nil {
		return &ActionResult{Success: false, Message: fmt.Sprintf("Unknown resource type: %s", resourceType), StartTime: startTime, EndTime: time.Now()}, fmt.Errorf("unknown resource type: %s", resourceType)
	}

	propagation := metav1.DeletePropagationForeground
	if namespace != "" {
		err = k.dynamicClient.Resource(*found).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{PropagationPolicy: &propagation})
	} else {
		err = k.dynamicClient.Resource(*found).Delete(ctx, name, metav1.DeleteOptions{PropagationPolicy: &propagation})
	}
	if err != nil {
		return &ActionResult{Success: false, Message: fmt.Sprintf("Failed to delete %s/%s", resourceType, name), Error: err.Error(), StartTime: startTime, EndTime: time.Now()}, err
	}
	return &ActionResult{Success: true, Message: fmt.Sprintf("Deleted %s %s/%s", resourceType, namespace, name), StartTime: startTime, EndTime: time.Now()}, nil
}

// ── Port-forward ──────────────────────────────────────────────────────────────

// PortForwardSession holds an active port-forward.
type PortForwardSession struct {
	StopCh    chan struct{}
	LocalPort int
}

// StartPortForward starts a port-forward from a local port to a pod port.
// If localPort is 0 a free port is picked automatically.
// Returns the session and the chosen local port.
func (k *KubernetesExecutor) StartPortForward(ctx context.Context, namespace, podName string, localPort, remotePort int) (*PortForwardSession, int, error) {
	if localPort == 0 {
		var err error
		localPort, err = getFreePort()
		if err != nil {
			return nil, 0, fmt.Errorf("could not find free port: %w", err)
		}
	}

	req := k.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(namespace).
		Name(podName).
		SubResource("portforward")

	transport, upgrader, err := spdy.RoundTripperFor(k.config)
	if err != nil {
		return nil, 0, fmt.Errorf("spdy transport: %w", err)
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, req.URL())

	stopCh := make(chan struct{})
	readyCh := make(chan struct{})
	errCh := make(chan error, 1)

	pf, err := portforward.New(dialer,
		[]string{fmt.Sprintf("%d:%d", localPort, remotePort)},
		stopCh, readyCh, io.Discard, io.Discard,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("portforward init: %w", err)
	}

	go func() {
		errCh <- pf.ForwardPorts()
	}()

	// Wait for ready or error
	select {
	case <-readyCh:
	case err := <-errCh:
		return nil, 0, fmt.Errorf("portforward failed: %w", err)
	case <-time.After(10 * time.Second):
		close(stopCh)
		return nil, 0, fmt.Errorf("portforward timed out waiting for ready")
	case <-ctx.Done():
		close(stopCh)
		return nil, 0, ctx.Err()
	}

	sess := &PortForwardSession{StopCh: stopCh, LocalPort: localPort}
	return sess, localPort, nil
}

// StopPortForward stops a port-forward session.
func StopPortForward(sess *PortForwardSession) {
	select {
	case <-sess.StopCh:
	default:
		close(sess.StopCh)
	}
}

func getFreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// ── Namespace list ────────────────────────────────────────────────────────────

// ListNamespaces returns all namespace names.
func (k *KubernetesExecutor) ListNamespaces(ctx context.Context) ([]string, error) {
	list, err := k.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	names := make([]string, len(list.Items))
	for i, ns := range list.Items {
		names[i] = ns.Name
	}
	return names, nil
}

// ── Pod container list ────────────────────────────────────────────────────────

// GetPodContainers returns container names for a pod.
func (k *KubernetesExecutor) GetPodContainers(ctx context.Context, namespace, podName string) ([]corev1.Container, error) {
	pod, err := k.clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return pod.Spec.Containers, nil
}
