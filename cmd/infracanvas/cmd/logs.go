package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	logsNamespace  string
	logsContainer  string
	logsSince      string
	logsTail       int64
	logsFollow     bool
	logsPrevious   bool
	logsUnit       string
	logsPriority   string
)

// logsCmd represents the logs command
var logsCmd = &cobra.Command{
	Use:   "logs [TYPE] [NAME]",
	Short: "Retrieve logs from various sources",
	Long: `Retrieve logs from host (journald), Docker containers, or Kubernetes pods.

Examples:
  # Host logs
  rix logs host --unit docker.service --follow

  # Docker container logs
  rix logs container my-container --tail 100 --follow

  # Kubernetes pod logs
  rix logs pod my-pod -n default --container app --follow`,
	Args: cobra.ExactArgs(2),
	RunE: runLogs,
}

func init() {
	rootCmd.AddCommand(logsCmd)

	logsCmd.Flags().StringVarP(&logsNamespace, "namespace", "n", "default", "Kubernetes namespace")
	logsCmd.Flags().StringVarP(&logsContainer, "container", "c", "", "Container name (for pods with multiple containers)")
	logsCmd.Flags().StringVar(&logsSince, "since", "", "Show logs since timestamp (e.g., 2023-01-01T00:00:00Z) or duration (e.g., 10m)")
	logsCmd.Flags().Int64Var(&logsTail, "tail", 100, "Number of lines to show from the end of logs")
	logsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "Follow log output")
	logsCmd.Flags().BoolVarP(&logsPrevious, "previous", "p", false, "Show logs from previous container instance")
	logsCmd.Flags().StringVar(&logsUnit, "unit", "", "Systemd unit name (for host logs)")
	logsCmd.Flags().StringVar(&logsPriority, "priority", "", "Log priority level (for host logs: emerg, alert, crit, err, warning, notice, info, debug)")
}

func runLogs(cmd *cobra.Command, args []string) error {
	logType := args[0]
	name := args[1]

	switch logType {
	case "host":
		return getHostLogs()
	case "container":
		return getContainerLogs(name)
	case "pod":
		return getPodLogs(name)
	default:
		return fmt.Errorf("unsupported log type: %s (supported: host, container, pod)", logType)
	}
}

func getHostLogs() error {
	args := []string{}

	// Add unit filter
	if logsUnit != "" {
		args = append(args, "-u", logsUnit)
	}

	// Add priority filter
	if logsPriority != "" {
		args = append(args, "-p", logsPriority)
	}

	// Add since filter
	if logsSince != "" {
		args = append(args, "--since", logsSince)
	}

	// Add tail
	if logsTail > 0 && !logsFollow {
		args = append(args, "-n", fmt.Sprintf("%d", logsTail))
	}

	// Add follow
	if logsFollow {
		args = append(args, "-f")
	}

	// Execute journalctl
	cmd := exec.Command("journalctl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func getContainerLogs(containerName string) error {
	// Initialize Docker client
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer cli.Close()

	ctx := context.Background()

	// Build log options
	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     logsFollow,
		Tail:       fmt.Sprintf("%d", logsTail),
	}

	if logsSince != "" {
		// Parse since as duration or timestamp
		if duration, err := time.ParseDuration(logsSince); err == nil {
			options.Since = time.Now().Add(-duration).Format(time.RFC3339)
		} else {
			options.Since = logsSince
		}
	}

	// Get logs
	reader, err := cli.ContainerLogs(ctx, containerName, options)
	if err != nil {
		return fmt.Errorf("failed to get container logs: %w", err)
	}
	defer reader.Close()

	// Stream logs to stdout
	_, err = io.Copy(os.Stdout, reader)
	return err
}

func getPodLogs(podName string) error {
	// Load kubeconfig
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	config, err := kubeConfig.ClientConfig()
	if err != nil {
		return fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	// Create clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	ctx := context.Background()

	// Build log options
	options := &corev1.PodLogOptions{
		Follow:    logsFollow,
		TailLines: &logsTail,
		Previous:  logsPrevious,
	}

	if logsContainer != "" {
		options.Container = logsContainer
	}

	if logsSince != "" {
		// Parse since as duration or timestamp
		if duration, err := time.ParseDuration(logsSince); err == nil {
			sinceSeconds := int64(duration.Seconds())
			options.SinceSeconds = &sinceSeconds
		} else if timestamp, err := time.Parse(time.RFC3339, logsSince); err == nil {
			sinceTime := metav1.NewTime(timestamp)
			options.SinceTime = &sinceTime
		}
	}

	// Get logs
	req := clientset.CoreV1().Pods(logsNamespace).GetLogs(podName, options)
	stream, err := req.Stream(ctx)
	if err != nil {
		return fmt.Errorf("failed to get pod logs: %w", err)
	}
	defer stream.Close()

	// Stream logs to stdout
	scanner := bufio.NewScanner(stream)
	for scanner.Scan() {
		fmt.Println(scanner.Text())
	}

	return scanner.Err()
}
