package actions

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"time"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// DockerExecutor handles actions on Docker containers
type DockerExecutor struct {
	client *client.Client
}

// NewDockerExecutor creates a new Docker executor
func NewDockerExecutor() (*DockerExecutor, error) {
	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	return &DockerExecutor{
		client: cli,
	}, nil
}

// ValidateAction validates a Docker action
func (d *DockerExecutor) ValidateAction(action *Action) error {
	switch action.Type {
	case ActionRestartContainer, ActionStopContainer, ActionStartContainer:
		if action.Target.EntityID == "" {
			return fmt.Errorf("container ID or name is required")
		}
		// Check if container exists
		return d.validateContainerExists(action.Target.EntityID)

	default:
		return fmt.Errorf("unsupported docker action type: %s", action.Type)
	}
}

// validateContainerExists checks if a container exists
func (d *DockerExecutor) validateContainerExists(containerID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := d.client.ContainerInspect(ctx, containerID)
	if err != nil {
		return fmt.Errorf("container %s not found: %w", containerID, err)
	}

	return nil
}

// ExecuteAction executes a Docker action
func (d *DockerExecutor) ExecuteAction(ctx context.Context, action *Action) (*ActionResult, error) {
	startTime := time.Now()

	switch action.Type {
	case ActionRestartContainer:
		return d.restartContainer(ctx, action.Target.EntityID, startTime)

	case ActionStopContainer:
		return d.stopContainer(ctx, action.Target.EntityID, startTime)

	case ActionStartContainer:
		return d.startContainer(ctx, action.Target.EntityID, startTime)

	default:
		return &ActionResult{
			Success:   false,
			Message:   "Unsupported action type",
			Error:     fmt.Sprintf("unsupported docker action type: %s", action.Type),
			StartTime: startTime,
			EndTime:   time.Now(),
		}, fmt.Errorf("unsupported docker action type: %s", action.Type)
	}
}

// restartContainer restarts a Docker container
func (d *DockerExecutor) restartContainer(ctx context.Context, containerID string, startTime time.Time) (*ActionResult, error) {
	timeout := 10 // seconds
	err := d.client.ContainerRestart(ctx, containerID, container.StopOptions{Timeout: &timeout})

	if err != nil {
		return &ActionResult{
			Success:   false,
			Message:   fmt.Sprintf("Failed to restart container %s", containerID),
			Error:     err.Error(),
			StartTime: startTime,
			EndTime:   time.Now(),
		}, err
	}

	return &ActionResult{
		Success:   true,
		Message:   fmt.Sprintf("Successfully restarted container %s", containerID),
		StartTime: startTime,
		EndTime:   time.Now(),
	}, nil
}

// stopContainer stops a Docker container
func (d *DockerExecutor) stopContainer(ctx context.Context, containerID string, startTime time.Time) (*ActionResult, error) {
	timeout := 10 // seconds
	err := d.client.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout})

	if err != nil {
		return &ActionResult{
			Success:   false,
			Message:   fmt.Sprintf("Failed to stop container %s", containerID),
			Error:     err.Error(),
			StartTime: startTime,
			EndTime:   time.Now(),
		}, err
	}

	return &ActionResult{
		Success:   true,
		Message:   fmt.Sprintf("Successfully stopped container %s", containerID),
		StartTime: startTime,
		EndTime:   time.Now(),
	}, nil
}

// startContainer starts a Docker container
func (d *DockerExecutor) startContainer(ctx context.Context, containerID string, startTime time.Time) (*ActionResult, error) {
	err := d.client.ContainerStart(ctx, containerID, container.StartOptions{})

	if err != nil {
		return &ActionResult{
			Success:   false,
			Message:   fmt.Sprintf("Failed to start container %s", containerID),
			Error:     err.Error(),
			StartTime: startTime,
			EndTime:   time.Now(),
		}, err
	}

	return &ActionResult{
		Success:   true,
		Message:   fmt.Sprintf("Successfully started container %s", containerID),
		StartTime: startTime,
		EndTime:   time.Now(),
	}, nil
}

// GetContainerLogs returns a streaming reader for container logs.
// The caller is responsible for closing the returned ReadCloser.
func (d *DockerExecutor) GetContainerLogs(ctx context.Context, containerID string, tail int, follow bool) (io.ReadCloser, error) {
	opts := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       strconv.Itoa(tail),
		Follow:     follow,
		Timestamps: true,
	}
	return d.client.ContainerLogs(ctx, containerID, opts)
}

// ExecSession holds state for an active exec session.
type ExecSession struct {
	ExecID  string
	Attach  dockertypes.HijackedResponse
}

// ExecCreate creates an exec instance and attaches to it, returning an ExecSession.
func (d *DockerExecutor) ExecCreate(ctx context.Context, containerID string, cmd []string) (*ExecSession, error) {
	execResp, err := d.client.ContainerExecCreate(ctx, containerID, container.ExecOptions{
		Cmd:          cmd,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("exec create: %w", err)
	}

	attach, err := d.client.ContainerExecAttach(ctx, execResp.ID, container.ExecAttachOptions{Tty: true})
	if err != nil {
		return nil, fmt.Errorf("exec attach: %w", err)
	}

	return &ExecSession{ExecID: execResp.ID, Attach: attach}, nil
}

// ExecResize resizes the TTY for an exec session.
func (d *DockerExecutor) ExecResize(ctx context.Context, execID string, rows, cols uint) error {
	return d.client.ContainerExecResize(ctx, execID, container.ResizeOptions{
		Height: rows,
		Width:  cols,
	})
}
