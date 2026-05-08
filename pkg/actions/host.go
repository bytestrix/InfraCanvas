package actions

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// HostExecutor handles actions on the host system
type HostExecutor struct{}

// NewHostExecutor creates a new host executor
func NewHostExecutor() *HostExecutor {
	return &HostExecutor{}
}

// ValidateAction validates a host action
func (h *HostExecutor) ValidateAction(action *Action) error {
	switch action.Type {
	case ActionRestartService:
		if action.Target.EntityID == "" {
			return fmt.Errorf("service name is required")
		}
		return h.validateServiceExists(action.Target.EntityID)

	case ActionUpdateAgent:
		return nil // no pre-validation needed

	default:
		return fmt.Errorf("unsupported host action type: %s", action.Type)
	}
}

// validateServiceExists checks if a systemd service exists
func (h *HostExecutor) validateServiceExists(serviceName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use systemctl show to check if service exists
	cmd := exec.CommandContext(ctx, "systemctl", "show", serviceName, "--property=LoadState")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to check service existence: %w", err)
	}

	// Parse output to check LoadState
	outputStr := strings.TrimSpace(string(output))
	if strings.Contains(outputStr, "LoadState=not-found") {
		return fmt.Errorf("service %s not found", serviceName)
	}

	return nil
}

// ExecuteAction executes a host action
func (h *HostExecutor) ExecuteAction(ctx context.Context, action *Action) (*ActionResult, error) {
	startTime := time.Now()

	switch action.Type {
	case ActionRestartService:
		return h.restartService(ctx, action.Target.EntityID, startTime)

	case ActionUpdateAgent:
		return h.updateAgent(ctx, action.Parameters, startTime)

	default:
		return &ActionResult{
			Success:   false,
			Message:   "Unsupported action type",
			Error:     fmt.Sprintf("unsupported host action type: %s", action.Type),
			StartTime: startTime,
			EndTime:   time.Now(),
		}, fmt.Errorf("unsupported host action type: %s", action.Type)
	}
}

// restartService restarts a systemd service
func (h *HostExecutor) restartService(ctx context.Context, serviceName string, startTime time.Time) (*ActionResult, error) {
	// Execute systemctl restart
	cmd := exec.CommandContext(ctx, "systemctl", "restart", serviceName)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return &ActionResult{
			Success:   false,
			Message:   fmt.Sprintf("Failed to restart service %s", serviceName),
			Output:    string(output),
			Error:     err.Error(),
			StartTime: startTime,
			EndTime:   time.Now(),
		}, err
	}

	// Verify service is running
	statusCmd := exec.CommandContext(ctx, "systemctl", "is-active", serviceName)
	statusOutput, statusErr := statusCmd.CombinedOutput()
	status := strings.TrimSpace(string(statusOutput))

	if statusErr != nil || status != "active" {
		return &ActionResult{
			Success:   false,
			Message:   fmt.Sprintf("Service %s restarted but is not active (status: %s)", serviceName, status),
			Output:    string(output),
			Error:     fmt.Sprintf("service status: %s", status),
			StartTime: startTime,
			EndTime:   time.Now(),
		}, fmt.Errorf("service not active after restart: %s", status)
	}

	return &ActionResult{
		Success:   true,
		Message:   fmt.Sprintf("Successfully restarted service %s", serviceName),
		Output:    string(output),
		StartTime: startTime,
		EndTime:   time.Now(),
	}, nil
}

// updateAgent downloads the latest binary and hot-swaps it, then restarts the service.
func (h *HostExecutor) updateAgent(ctx context.Context, params map[string]string, startTime time.Time) (*ActionResult, error) {
	fail := func(msg, detail string) (*ActionResult, error) {
		err := fmt.Errorf("%s: %s", msg, detail)
		return &ActionResult{
			Success: false, Message: msg, Error: detail,
			StartTime: startTime, EndTime: time.Now(),
		}, err
	}

	// Determine download URL
	binaryURL := params["binary_url"]
	if binaryURL == "" {
		binaryURL = fmt.Sprintf(
			"https://github.com/bytestrix/InfraCanvas/releases/latest/download/infracanvas-%s-%s",
			runtime.GOOS, runtime.GOARCH,
		)
	}

	// Find current executable (resolve symlinks)
	exe, err := os.Executable()
	if err != nil {
		return fail("find executable", err.Error())
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return fail("resolve symlinks", err.Error())
	}

	// Download new binary alongside current one (same filesystem = rename works)
	tmpPath := exe + ".update-tmp"
	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return fail("create temp file", err.Error())
	}

	downloadCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	req, _ := http.NewRequestWithContext(downloadCtx, http.MethodGet, binaryURL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fail("download", err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		f.Close()
		os.Remove(tmpPath)
		return fail("download", fmt.Sprintf("HTTP %d from %s", resp.StatusCode, binaryURL))
	}

	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fail("write binary", err.Error())
	}
	f.Close()

	// Atomically replace (works on Linux even while old binary is running)
	if err := os.Rename(tmpPath, exe); err != nil {
		os.Remove(tmpPath)
		return fail("replace binary", err.Error())
	}

	// Detect running service and schedule restart after result is delivered
	svcName := h.detectServiceName()
	go func() {
		time.Sleep(500 * time.Millisecond)
		exec.Command("systemctl", "restart", svcName).Run() //nolint:errcheck
	}()

	return &ActionResult{
		Success:   true,
		Message:   fmt.Sprintf("Binary updated, restarting %s", svcName),
		Output:    fmt.Sprintf("Downloaded from %s, replaced %s", binaryURL, exe),
		StartTime: startTime,
		EndTime:   time.Now(),
	}, nil
}

// detectServiceName finds the currently active infracanvas service.
func (h *HostExecutor) detectServiceName() string {
	for _, svc := range []string{"infracanvas-agent", "infracanvas"} {
		out, _ := exec.Command("systemctl", "is-active", svc).Output()
		if strings.TrimSpace(string(out)) == "active" {
			return svc
		}
	}
	return "infracanvas-agent"
}
