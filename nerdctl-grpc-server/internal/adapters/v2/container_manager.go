package v2

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/containerd/nerdctl-grpc-server/internal/config"
	"github.com/containerd/nerdctl-grpc-server/internal/interfaces"
	"github.com/containerd/nerdctl-grpc-server/pkg/types"
	"github.com/sirupsen/logrus"
)

// V2ContainerManager implements container management for nerdctl v2.x
// This provides enhanced container lifecycle management with v2.x specific features
type V2ContainerManager struct {
	executor *V2CommandExecutor
	config   *config.Config
	logger   *logrus.Entry
}

// V2ContainerInfo represents enhanced container information in v2.x format
type V2ContainerInfo struct {
	ID           string                 `json:"Id"`
	Names        []string               `json:"Names"`
	Image        string                 `json:"Image"`
	ImageID      string                 `json:"ImageID"`
	Command      string                 `json:"Command"`
	CreatedAt    string                 `json:"CreatedAt"`
	RunningFor   string                 `json:"RunningFor"`
	Ports        []V2PortBinding        `json:"Ports"`
	Status       string                 `json:"Status"`
	Size         string                 `json:"Size"`
	Labels       map[string]string      `json:"Labels"`
	Mounts       []V2MountPoint         `json:"Mounts"`
	Networks     map[string]interface{} `json:"Networks"`
	
	// v2.x enhanced fields
	Platform     string                 `json:"Platform,omitempty"`
	HostConfig   map[string]interface{} `json:"HostConfig,omitempty"`
	NetworkMode  string                 `json:"NetworkMode,omitempty"`
	State        V2ContainerState       `json:"State,omitempty"`
	
	// v2.x metadata
	Metadata     map[string]interface{} `json:"Metadata,omitempty"`
}

// V2ContainerState represents enhanced container state in v2.x
type V2ContainerState struct {
	Status      string    `json:"Status"`
	Running     bool      `json:"Running"`
	Paused      bool      `json:"Paused"`
	Restarting  bool      `json:"Restarting"`
	OOMKilled   bool      `json:"OOMKilled"`
	Dead        bool      `json:"Dead"`
	Pid         int       `json:"Pid"`
	ExitCode    int       `json:"ExitCode"`
	Error       string    `json:"Error"`
	StartedAt   time.Time `json:"StartedAt"`
	FinishedAt  time.Time `json:"FinishedAt"`
	Health      *V2Health `json:"Health,omitempty"`
}

// V2Health represents enhanced health check information in v2.x
type V2Health struct {
	Status        string      `json:"Status"`
	FailingStreak int         `json:"FailingStreak"`
	Log           []V2HealthLog `json:"Log"`
}

// V2HealthLog represents health check log entries
type V2HealthLog struct {
	Start    time.Time `json:"Start"`
	End      time.Time `json:"End"`
	ExitCode int       `json:"ExitCode"`
	Output   string    `json:"Output"`
}

// V2PortBinding represents enhanced port binding information
type V2PortBinding struct {
	IP          string `json:"IP"`
	PrivatePort uint16 `json:"PrivatePort"`
	PublicPort  uint16 `json:"PublicPort"`
	Type        string `json:"Type"`
}

// V2MountPoint represents enhanced mount point information
type V2MountPoint struct {
	Type        string `json:"Type"`
	Name        string `json:"Name,omitempty"`
	Source      string `json:"Source"`
	Destination string `json:"Destination"`
	Driver      string `json:"Driver,omitempty"`
	Mode        string `json:"Mode"`
	RW          bool   `json:"RW"`
	Propagation string `json:"Propagation"`
}

// NewV2ContainerManager creates a new V2ContainerManager
func NewV2ContainerManager(executor *V2CommandExecutor, config *config.Config, logger *logrus.Logger) (*V2ContainerManager, error) {
	return &V2ContainerManager{
		executor: executor,
		config:   config,
		logger:   logger.WithField("component", "v2-container-manager"),
	}, nil
}

// CreateContainer creates a new container using nerdctl v2.x enhanced features
func (m *V2ContainerManager) CreateContainer(ctx context.Context, request *types.CreateContainerRequest) (*types.CreateContainerResponse, error) {
	m.logger.WithFields(logrus.Fields{
		"image": request.Image,
		"name":  request.Name,
	}).Info("Creating container with v2 enhanced features")

	// Build create command with v2 enhancements
	args := []string{"container", "create"}
	
	// Add container name
	if request.Name != "" {
		args = append(args, "--name", request.Name)
	}
	
	// Add v2-specific options
	if request.Platform != "" {
		args = append(args, "--platform", request.Platform)
	}
	
	// Enhanced port bindings in v2.x
	for _, port := range request.Ports {
		portStr := fmt.Sprintf("%d:%d", port.HostPort, port.ContainerPort)
		if port.Protocol != "" {
			portStr += "/" + port.Protocol
		}
		if port.HostIP != "" {
			portStr = port.HostIP + ":" + portStr
		}
		args = append(args, "-p", portStr)
	}
	
	// Enhanced environment variables
	for key, value := range request.Environment {
		args = append(args, "-e", fmt.Sprintf("%s=%s", key, value))
	}
	
	// Enhanced volume mounts with v2 features
	for _, mount := range request.Mounts {
		mountStr := fmt.Sprintf("%s:%s", mount.Source, mount.Target)
		if mount.ReadOnly {
			mountStr += ":ro"
		}
		// v2.x specific mount options
		if mount.Type == "bind" {
			mountStr += ",bind"
		}
		args = append(args, "-v", mountStr)
	}
	
	// Enhanced resource limits in v2.x
	if request.Resources != nil {
		if request.Resources.Memory > 0 {
			args = append(args, "--memory", fmt.Sprintf("%d", request.Resources.Memory))
		}
		if request.Resources.CPUs > 0 {
			args = append(args, "--cpus", fmt.Sprintf("%.2f", request.Resources.CPUs))
		}
	}
	
	// Enhanced network configuration
	if request.NetworkMode != "" {
		args = append(args, "--network", request.NetworkMode)
	}
	
	// Enhanced security options in v2.x
	if request.SecurityOptions != nil {
		for _, opt := range request.SecurityOptions {
			args = append(args, "--security-opt", opt)
		}
	}
	
	// Add labels with v2 metadata enhancement
	for key, value := range request.Labels {
		args = append(args, "--label", fmt.Sprintf("%s=%s", key, value))
	}
	
	// Add v2-specific health check configuration
	if request.HealthCheck != nil {
		args = append(args, "--health-cmd", request.HealthCheck.Test)
		if request.HealthCheck.Interval > 0 {
			args = append(args, "--health-interval", request.HealthCheck.Interval.String())
		}
		if request.HealthCheck.Timeout > 0 {
			args = append(args, "--health-timeout", request.HealthCheck.Timeout.String())
		}
		if request.HealthCheck.Retries > 0 {
			args = append(args, "--health-retries", strconv.Itoa(request.HealthCheck.Retries))
		}
	}
	
	// Add image and command
	args = append(args, request.Image)
	if len(request.Command) > 0 {
		args = append(args, request.Command...)
	}
	
	// Execute create command with v2 enhancements
	result, err := m.executor.Execute(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}
	
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("create container failed with exit code %d: %s", result.ExitCode, result.Stderr)
	}
	
	// Extract container ID from output
	containerID := strings.TrimSpace(result.Stdout)
	
	m.logger.WithFields(logrus.Fields{
		"container_id": containerID,
		"image":        request.Image,
		"name":         request.Name,
		"metadata_size": len(result.Metadata),
	}).Info("Container created successfully with v2 features")
	
	return &types.CreateContainerResponse{
		ID:       containerID,
		Warnings: []string{}, // v2.x provides better warning handling
		Metadata: result.Metadata,
	}, nil
}

// StartContainer starts a container with v2.x enhanced monitoring
func (m *V2ContainerManager) StartContainer(ctx context.Context, request *types.StartContainerRequest) (*types.StartContainerResponse, error) {
	m.logger.WithField("container_id", request.ID).Info("Starting container with v2 monitoring")

	args := []string{"container", "start"}
	
	// v2.x enhanced attach options
	if request.AttachStdout || request.AttachStderr {
		args = append(args, "--attach")
	}
	
	if request.AttachStdin {
		args = append(args, "--interactive")
	}
	
	args = append(args, request.ID)
	
	result, err := m.executor.Execute(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("failed to start container: %w", err)
	}
	
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("start container failed with exit code %d: %s", result.ExitCode, result.Stderr)
	}
	
	return &types.StartContainerResponse{
		Metadata: result.Metadata,
	}, nil
}

// StopContainer stops a container with v2.x enhanced graceful shutdown
func (m *V2ContainerManager) StopContainer(ctx context.Context, request *types.StopContainerRequest) (*types.StopContainerResponse, error) {
	m.logger.WithFields(logrus.Fields{
		"container_id": request.ID,
		"timeout":      request.Timeout,
	}).Info("Stopping container with v2 enhanced shutdown")

	args := []string{"container", "stop"}
	
	// v2.x enhanced timeout handling
	if request.Timeout > 0 {
		args = append(args, "--time", strconv.Itoa(int(request.Timeout)))
	}
	
	args = append(args, request.ID)
	
	result, err := m.executor.Execute(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("failed to stop container: %w", err)
	}
	
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("stop container failed with exit code %d: %s", result.ExitCode, result.Stderr)
	}
	
	return &types.StopContainerResponse{
		Metadata: result.Metadata,
	}, nil
}

// RemoveContainer removes a container with v2.x enhanced cleanup
func (m *V2ContainerManager) RemoveContainer(ctx context.Context, request *types.RemoveContainerRequest) (*types.RemoveContainerResponse, error) {
	m.logger.WithFields(logrus.Fields{
		"container_id":   request.ID,
		"remove_volumes": request.RemoveVolumes,
		"force":          request.Force,
	}).Info("Removing container with v2 enhanced cleanup")

	args := []string{"container", "rm"}
	
	if request.Force {
		args = append(args, "--force")
	}
	
	if request.RemoveVolumes {
		args = append(args, "--volumes")
	}
	
	// v2.x specific: enhanced link removal
	if request.RemoveLinks {
		args = append(args, "--link")
	}
	
	args = append(args, request.ID)
	
	result, err := m.executor.Execute(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("failed to remove container: %w", err)
	}
	
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("remove container failed with exit code %d: %s", result.ExitCode, result.Stderr)
	}
	
	return &types.RemoveContainerResponse{
		Metadata: result.Metadata,
	}, nil
}

// ListContainers lists containers with v2.x enhanced filtering and metadata
func (m *V2ContainerManager) ListContainers(ctx context.Context, request *types.ListContainersRequest) (*types.ListContainersResponse, error) {
	m.logger.WithFields(logrus.Fields{
		"all":     request.All,
		"filters": len(request.Filters),
	}).Debug("Listing containers with v2 enhanced metadata")

	args := []string{"container", "ls"}
	
	if request.All {
		args = append(args, "--all")
	}
	
	// v2.x enhanced size information
	if request.Size {
		args = append(args, "--size")
	}
	
	// Enhanced filtering in v2.x
	for key, values := range request.Filters {
		for _, value := range values {
			args = append(args, "--filter", fmt.Sprintf("%s=%s", key, value))
		}
	}
	
	// Force JSON output for v2.x enhanced parsing
	args = append(args, "--format", "json")
	
	result, err := m.executor.Execute(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}
	
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("list containers failed with exit code %d: %s", result.ExitCode, result.Stderr)
	}
	
	// Parse v2.x enhanced JSON output
	var v2Containers []V2ContainerInfo
	if err := m.executor.ParseJSONOutput(result.Stdout, &v2Containers); err != nil {
		return nil, fmt.Errorf("failed to parse v2 container list: %w", err)
	}
	
	// Convert to generic container format
	containers := make([]*types.Container, len(v2Containers))
	for i, v2Container := range v2Containers {
		containers[i] = m.convertV2ContainerInfo(&v2Container)
	}
	
	m.logger.WithFields(logrus.Fields{
		"count":         len(containers),
		"metadata_size": len(result.Metadata),
	}).Debug("Successfully listed containers with v2 enhancements")
	
	return &types.ListContainersResponse{
		Containers: containers,
		Metadata:   result.Metadata,
	}, nil
}

// InspectContainer inspects a container with v2.x enhanced details
func (m *V2ContainerManager) InspectContainer(ctx context.Context, request *types.InspectContainerRequest) (*types.InspectContainerResponse, error) {
	m.logger.WithField("container_id", request.ID).Debug("Inspecting container with v2 enhanced details")

	args := []string{"container", "inspect", request.ID}
	
	result, err := m.executor.Execute(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}
	
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("inspect container failed with exit code %d: %s", result.ExitCode, result.Stderr)
	}
	
	// Parse v2.x enhanced JSON output
	var v2Containers []V2ContainerInfo
	if err := m.executor.ParseJSONOutput(result.Stdout, &v2Containers); err != nil {
		return nil, fmt.Errorf("failed to parse v2 container inspect: %w", err)
	}
	
	if len(v2Containers) == 0 {
		return nil, fmt.Errorf("container not found")
	}
	
	container := m.convertV2ContainerInfo(&v2Containers[0])
	
	return &types.InspectContainerResponse{
		Container: container,
		Metadata:  result.Metadata,
	}, nil
}

// convertV2ContainerInfo converts V2ContainerInfo to generic Container format
func (m *V2ContainerManager) convertV2ContainerInfo(v2Container *V2ContainerInfo) *types.Container {
	// Convert ports
	var ports []types.Port
	for _, port := range v2Container.Ports {
		ports = append(ports, types.Port{
			IP:           port.IP,
			PrivatePort:  port.PrivatePort,
			PublicPort:   port.PublicPort,
			Type:         port.Type,
		})
	}
	
	// Convert mounts
	var mounts []types.MountPoint
	for _, mount := range v2Container.Mounts {
		mounts = append(mounts, types.MountPoint{
			Type:        mount.Type,
			Name:        mount.Name,
			Source:      mount.Source,
			Destination: mount.Destination,
			Driver:      mount.Driver,
			Mode:        mount.Mode,
			RW:          mount.RW,
			Propagation: mount.Propagation,
		})
	}
	
	container := &types.Container{
		ID:         v2Container.ID,
		Names:      v2Container.Names,
		Image:      v2Container.Image,
		ImageID:    v2Container.ImageID,
		Command:    v2Container.Command,
		CreatedAt:  v2Container.CreatedAt,
		RunningFor: v2Container.RunningFor,
		Ports:      ports,
		Status:     v2Container.Status,
		Size:       v2Container.Size,
		Labels:     v2Container.Labels,
		Mounts:     mounts,
		Networks:   v2Container.Networks,
		
		// v2.x enhanced fields
		Platform:    v2Container.Platform,
		NetworkMode: v2Container.NetworkMode,
	}
	
	// Convert v2 state information
	if v2Container.State.Status != "" {
		container.State = &types.ContainerState{
			Status:     v2Container.State.Status,
			Running:    v2Container.State.Running,
			Paused:     v2Container.State.Paused,
			Restarting: v2Container.State.Restarting,
			OOMKilled:  v2Container.State.OOMKilled,
			Dead:       v2Container.State.Dead,
			Pid:        v2Container.State.Pid,
			ExitCode:   v2Container.State.ExitCode,
			Error:      v2Container.State.Error,
			StartedAt:  v2Container.State.StartedAt,
			FinishedAt: v2Container.State.FinishedAt,
		}
		
		// Convert health information
		if v2Container.State.Health != nil {
			var healthLogs []types.HealthLog
			for _, log := range v2Container.State.Health.Log {
				healthLogs = append(healthLogs, types.HealthLog{
					Start:    log.Start,
					End:      log.End,
					ExitCode: log.ExitCode,
					Output:   log.Output,
				})
			}
			
			container.State.Health = &types.Health{
				Status:        v2Container.State.Health.Status,
				FailingStreak: v2Container.State.Health.FailingStreak,
				Log:           healthLogs,
			}
		}
	}
	
	// Add v2.x metadata
	if container.Labels == nil {
		container.Labels = make(map[string]string)
	}
	container.Labels["nerdctl.version"] = "v2.x"
	container.Labels["adapter.enhanced"] = "true"
	
	return container
}

// GetContainerLogs retrieves container logs with v2.x enhanced streaming
func (m *V2ContainerManager) GetContainerLogs(ctx context.Context, request *types.GetContainerLogsRequest) (*interfaces.LogStream, error) {
	m.logger.WithFields(logrus.Fields{
		"container_id": request.ID,
		"follow":       request.Follow,
		"timestamps":   request.Timestamps,
	}).Info("Getting container logs with v2 enhanced streaming")

	args := []string{"container", "logs"}
	
	if request.Follow {
		args = append(args, "--follow")
	}
	
	if request.Timestamps {
		args = append(args, "--timestamps")
	}
	
	if request.Since != "" {
		args = append(args, "--since", request.Since)
	}
	
	if request.Until != "" {
		args = append(args, "--until", request.Until)
	}
	
	if request.Tail != "" {
		args = append(args, "--tail", request.Tail)
	}
	
	args = append(args, request.ID)
	
	// Use v2 enhanced streaming
	streamResult, err := m.executor.ExecuteStream(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("failed to get container logs: %w", err)
	}
	
	return &interfaces.LogStream{
		LinesChan:  streamResult.LinesChan,
		ErrorsChan: streamResult.ErrorsChan,
		Cancel:     streamResult.Cancel,
	}, nil
}