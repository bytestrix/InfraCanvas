package actions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
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
	case ActionRestartService, ActionStopService, ActionStartService:
		if action.Target.EntityID == "" {
			return fmt.Errorf("service name is required")
		}
		return h.validateServiceExists(action.Target.EntityID)

	case ActionKillProcess:
		pid, err := strconv.Atoi(action.Target.EntityID)
		if err != nil || pid <= 1 {
			return fmt.Errorf("invalid pid %q", action.Target.EntityID)
		}
		if pid == os.Getpid() {
			return fmt.Errorf("refusing to kill the InfraCanvas agent itself")
		}
		return nil

	case ActionUpdateAgent:
		return nil

	case ActionHostRunCommand:
		if action.Parameters["command"] == "" {
			return fmt.Errorf("command is required")
		}
		return nil

	case ActionHostDiskUsage, ActionHostTopProcesses, ActionHostJournalctl:
		return nil

	case ActionHostSvcStatus:
		return nil

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

	case ActionStopService:
		return h.systemctlVerb(ctx, "stop", action.Target.EntityID, startTime)

	case ActionStartService:
		return h.systemctlVerb(ctx, "start", action.Target.EntityID, startTime)

	case ActionKillProcess:
		return h.killProcess(action.Target.EntityID, action.Parameters["signal"], startTime)

	case ActionUpdateAgent:
		return h.updateAgent(ctx, action.Parameters, startTime)

	case ActionHostRunCommand:
		return h.runCommand(ctx, action.Parameters["command"], startTime)

	case ActionHostDiskUsage:
		return h.runCommand(ctx, "df -h", startTime)

	case ActionHostTopProcesses:
		return h.runCommand(ctx, "ps aux --sort=-%cpu | head -20", startTime)

	case ActionHostSvcStatus:
		svc := action.Parameters["service"]
		if svc == "" {
			return h.runCommand(ctx, "systemctl list-units --type=service --state=running --no-pager --no-legend", startTime)
		}
		return h.runArgv(ctx, startTime, "systemctl", "status", svc, "--no-pager")

	case ActionHostJournalctl:
		unit := action.Parameters["unit"]
		lines := action.Parameters["lines"]
		if lines == "" {
			lines = "200"
		}
		if _, err := strconv.Atoi(lines); err != nil {
			return &ActionResult{
				Success: false, Message: "Invalid lines parameter", Error: "lines must be a number",
				StartTime: startTime, EndTime: time.Now(),
			}, nil
		}
		if unit != "" {
			return h.runArgv(ctx, startTime, "journalctl", "-u", unit, "-n", lines, "--no-pager")
		}
		return h.runArgv(ctx, startTime, "journalctl", "-n", lines, "--no-pager")

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

// systemctlVerb runs `systemctl <verb> <service>` (stop/start).
func (h *HostExecutor) systemctlVerb(ctx context.Context, verb, serviceName string, startTime time.Time) (*ActionResult, error) {
	cmd := exec.CommandContext(ctx, "systemctl", verb, serviceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return &ActionResult{
			Success:   false,
			Message:   fmt.Sprintf("Failed to %s service %s", verb, serviceName),
			Output:    string(output),
			Error:     err.Error(),
			StartTime: startTime,
			EndTime:   time.Now(),
		}, err
	}
	return &ActionResult{
		Success:   true,
		Message:   fmt.Sprintf("Successfully ran %s on service %s", verb, serviceName),
		Output:    string(output),
		StartTime: startTime,
		EndTime:   time.Now(),
	}, nil
}

// killProcess sends a signal to a pid (default SIGTERM; "KILL" forces).
func (h *HostExecutor) killProcess(pidStr, signal string, startTime time.Time) (*ActionResult, error) {
	pid, err := strconv.Atoi(pidStr)
	if err != nil || pid <= 1 {
		return &ActionResult{
			Success: false, Message: "Invalid pid", Error: fmt.Sprintf("pid %q", pidStr),
			StartTime: startTime, EndTime: time.Now(),
		}, fmt.Errorf("invalid pid %q", pidStr)
	}
	sig := syscall.SIGTERM
	if strings.EqualFold(signal, "KILL") || signal == "9" {
		sig = syscall.SIGKILL
	}
	if err := syscall.Kill(pid, sig); err != nil {
		return &ActionResult{
			Success: false, Message: fmt.Sprintf("Failed to signal pid %d", pid),
			Error: err.Error(), StartTime: startTime, EndTime: time.Now(),
		}, err
	}
	return &ActionResult{
		Success: true, Message: fmt.Sprintf("Sent %s to pid %d", sig.String(), pid),
		StartTime: startTime, EndTime: time.Now(),
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

	// Determine download URL. A custom binary_url is a caller-supplied string
	// with no restriction otherwise — reject anything that isn't https so a
	// downgrade to plaintext http can't be used to tamper with the binary
	// in transit even before the checksum check below.
	const defaultBase = "https://github.com/bytestrix/InfraCanvas/releases/latest/download/"
	binaryURL := params["binary_url"]
	usingDefault := binaryURL == ""
	if usingDefault {
		binaryURL = fmt.Sprintf(defaultBase+"infracanvas-%s-%s", runtime.GOOS, runtime.GOARCH)
	}
	if !strings.HasPrefix(binaryURL, "https://") {
		return fail("invalid binary_url", "must be an https:// URL")
	}

	// Resolve the expected sha256 before downloading anything. For the
	// default GitHub path, fetch the release's own published checksums.txt
	// (every release ships one — see CLAUDE.md release checklist). For a
	// caller-supplied binary_url we have no co-located checksums file to
	// trust, so the caller must supply the expected hash explicitly; refuse
	// otherwise rather than silently skipping verification.
	var wantSHA256 string
	if usingDefault {
		sum, err := fetchExpectedSHA256(ctx, defaultBase+"checksums.txt", fmt.Sprintf("infracanvas-%s-%s", runtime.GOOS, runtime.GOARCH))
		if err != nil {
			return fail("fetch checksums", err.Error())
		}
		wantSHA256 = sum
	} else {
		wantSHA256 = params["sha256"]
		if wantSHA256 == "" {
			return fail("missing sha256", "binary_url requires an explicit sha256 parameter to verify against")
		}
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
	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0700)
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

	hasher := sha256.New()
	if _, err := io.Copy(io.MultiWriter(f, hasher), resp.Body); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fail("write binary", err.Error())
	}
	f.Close()

	gotSHA256 := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(gotSHA256, wantSHA256) {
		os.Remove(tmpPath)
		return fail("checksum mismatch", fmt.Sprintf("downloaded binary does not match expected sha256 (got %s, want %s) — refusing to install", gotSHA256, wantSHA256))
	}
	if err := os.Chmod(tmpPath, 0o700); err != nil {
		os.Remove(tmpPath)
		return fail("chmod binary", err.Error())
	}

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

// fetchExpectedSHA256 downloads a sha256sum(1)-format checksums file (lines
// of "<hex digest>  <filename>") and returns the digest for the given
// filename, or an error if the file can't be fetched or has no matching
// entry. Used so updateAgent never installs a binary whose hash it never
// actually checked.
func fetchExpectedSHA256(ctx context.Context, checksumsURL, filename string) (string, error) {
	fetchCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(fetchCtx, http.MethodGet, checksumsURL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch %s: %w", checksumsURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch %s: HTTP %d", checksumsURL, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB cap — this is a tiny text file
	if err != nil {
		return "", fmt.Errorf("read checksums: %w", err)
	}
	for _, line := range strings.Split(string(body), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		digest, name := fields[0], strings.TrimPrefix(fields[1], "*")
		if name == filename {
			return digest, nil
		}
	}
	return "", fmt.Errorf("no checksum entry for %s in %s", filename, checksumsURL)
}

// runArgv runs a command directly (no shell involved), so caller-supplied
// parameters interpolated into argv can never break out via shell
// metacharacters (`;`, `|`, `#`, backticks, ...) — unlike runCommand below,
// which is only safe for fixed, hardcoded strings with no user input.
func (h *HostExecutor) runArgv(ctx context.Context, startTime time.Time, name string, args ...string) (*ActionResult, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return &ActionResult{
			Success:   false,
			Message:   "Command failed",
			Output:    string(output),
			Error:     err.Error(),
			StartTime: startTime,
			EndTime:   time.Now(),
		}, nil
	}
	return &ActionResult{
		Success:   true,
		Message:   "OK",
		Output:    string(output),
		StartTime: startTime,
		EndTime:   time.Now(),
	}, nil
}

// runCommand runs a shell command and returns its combined output. Only ever
// call this with a fixed, hardcoded command string — never with anything
// derived from caller-supplied parameters. Use runArgv for that.
func (h *HostExecutor) runCommand(ctx context.Context, command string, startTime time.Time) (*ActionResult, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return &ActionResult{
			Success:   false,
			Message:   "Command failed",
			Output:    string(output),
			Error:     err.Error(),
			StartTime: startTime,
			EndTime:   time.Now(),
		}, nil
	}
	return &ActionResult{
		Success:   true,
		Message:   "OK",
		Output:    string(output),
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
