package actions

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"time"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
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
	if action.Target.EntityID == "" {
		return fmt.Errorf("entity ID is required")
	}
	return nil
}

// ExecuteAction executes a Docker action
func (d *DockerExecutor) ExecuteAction(ctx context.Context, action *Action) (*ActionResult, error) {
	startTime := time.Now()
	id := action.Target.EntityID

	switch action.Type {
	case ActionRestartContainer:
		return d.restartContainer(ctx, id, startTime)
	case ActionStopContainer:
		return d.stopContainer(ctx, id, startTime)
	case ActionStartContainer:
		return d.startContainer(ctx, id, startTime)
	case ActionDockerUpdateImage:
		return d.updateContainerImage(ctx, id, action.Parameters["image"], startTime)
	case ActionDockerPullImage:
		return d.pullImage(ctx, action.Parameters["image"], startTime)
	case ActionDockerRemoveImage:
		return d.removeImage(ctx, id, startTime)
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
		AttachStderr: false, // merged into stdout by TTY
		Tty:          true,
		Env:          []string{"TERM=xterm-256color", "COLORTERM=truecolor"},
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

// pullImage pulls a Docker image by reference (e.g. "nginx:1.25").
func (d *DockerExecutor) pullImage(ctx context.Context, imageRef string, startTime time.Time) (*ActionResult, error) {
	if imageRef == "" {
		return &ActionResult{Success: false, Message: "image reference is required", StartTime: startTime, EndTime: time.Now()}, fmt.Errorf("image reference required")
	}
	pullCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	reader, err := d.client.ImagePull(pullCtx, imageRef, image.PullOptions{})
	if err != nil {
		return &ActionResult{Success: false, Message: fmt.Sprintf("Failed to pull %s", imageRef), Error: err.Error(), StartTime: startTime, EndTime: time.Now()}, err
	}
	defer reader.Close()
	// Drain the response so the pull completes
	_, _ = io.Copy(io.Discard, reader)
	return &ActionResult{Success: true, Message: fmt.Sprintf("Pulled %s", imageRef), StartTime: startTime, EndTime: time.Now()}, nil
}

// removeImage removes a Docker image by ID or reference.
func (d *DockerExecutor) removeImage(ctx context.Context, imageID string, startTime time.Time) (*ActionResult, error) {
	_, err := d.client.ImageRemove(ctx, imageID, image.RemoveOptions{Force: false, PruneChildren: false})
	if err != nil {
		return &ActionResult{Success: false, Message: fmt.Sprintf("Failed to remove image %s", imageID), Error: err.Error(), StartTime: startTime, EndTime: time.Now()}, err
	}
	return &ActionResult{Success: true, Message: fmt.Sprintf("Removed image %s", imageID), StartTime: startTime, EndTime: time.Now()}, nil
}

// updateContainerImage pulls a new image then recreates the container with the same config.
func (d *DockerExecutor) updateContainerImage(ctx context.Context, containerID, newImage string, startTime time.Time) (*ActionResult, error) {
	if newImage == "" {
		return &ActionResult{Success: false, Message: "new image is required", StartTime: startTime, EndTime: time.Now()}, fmt.Errorf("new image required")
	}

	// Inspect existing container to clone its config
	info, err := d.client.ContainerInspect(ctx, containerID)
	if err != nil {
		return &ActionResult{Success: false, Message: "Failed to inspect container", Error: err.Error(), StartTime: startTime, EndTime: time.Now()}, err
	}

	// Pull new image
	if _, err := d.pullImage(ctx, newImage, startTime); err != nil {
		return &ActionResult{Success: false, Message: fmt.Sprintf("Failed to pull %s", newImage), Error: err.Error(), StartTime: startTime, EndTime: time.Now()}, err
	}

	// Stop old container
	timeout := 10
	_ = d.client.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout})

	// Rename old container out of the way
	oldName := info.Name
	tmpName := oldName + "_old"
	_ = d.client.ContainerRename(ctx, containerID, tmpName)

	// Build new config with updated image
	cfg := info.Config
	cfg.Image = newImage
	hostCfg := info.HostConfig
	networkCfg := &network.NetworkingConfig{EndpointsConfig: info.NetworkSettings.Networks}

	name := oldName
	if len(name) > 0 && name[0] == '/' {
		name = name[1:]
	}
	resp, err := d.client.ContainerCreate(ctx, cfg, hostCfg, networkCfg, nil, name)
	if err != nil {
		// Roll back: rename old container back
		_ = d.client.ContainerRename(ctx, containerID, oldName)
		_ = d.client.ContainerStart(ctx, containerID, container.StartOptions{})
		return &ActionResult{Success: false, Message: "Failed to create updated container (rolled back)", Error: err.Error(), StartTime: startTime, EndTime: time.Now()}, err
	}

	if err := d.client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		_ = d.client.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		_ = d.client.ContainerRename(ctx, containerID, oldName)
		_ = d.client.ContainerStart(ctx, containerID, container.StartOptions{})
		return &ActionResult{Success: false, Message: "Failed to start updated container (rolled back)", Error: err.Error(), StartTime: startTime, EndTime: time.Now()}, err
	}

	// Remove old container
	_ = d.client.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})

	return &ActionResult{
		Success: true,
		Message: fmt.Sprintf("Container updated to %s", newImage),
		Details: map[string]interface{}{"old_image": info.Config.Image, "new_image": newImage, "new_container_id": resp.ID[:12]},
		StartTime: startTime, EndTime: time.Now(),
	}, nil
}

// PruneImages removes unused images.
func (d *DockerExecutor) PruneImages(ctx context.Context) (*ActionResult, error) {
	startTime := time.Now()
	report, err := d.client.ImagesPrune(ctx, filters.Args{})
	if err != nil {
		return &ActionResult{Success: false, Message: "Failed to prune images", Error: err.Error(), StartTime: startTime, EndTime: time.Now()}, err
	}
	return &ActionResult{
		Success: true,
		Message: fmt.Sprintf("Pruned %d images, reclaimed %d bytes", len(report.ImagesDeleted), report.SpaceReclaimed),
		StartTime: startTime, EndTime: time.Now(),
	}, nil
}
