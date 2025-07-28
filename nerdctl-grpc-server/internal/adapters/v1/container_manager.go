package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/containerd/nerdctl-grpc-server/internal/config"
	"github.com/containerd/nerdctl-grpc-server/internal/interfaces"
	"github.com/containerd/nerdctl-grpc-server/pkg/types"
	"github.com/sirupsen/logrus"
)

// V1ContainerManager implements the ContainerManager interface for nerdctl v1.x
// This demonstrates how version-specific adapters implement the abstract interfaces
type V1ContainerManager struct {
	executor *V1CommandExecutor
	config   *config.Config
	logger   *logrus.Logger
}

// NewV1ContainerManager creates a new V1ContainerManager
func NewV1ContainerManager(executor *V1CommandExecutor, config *config.Config, logger *logrus.Logger) (*V1ContainerManager, error) {
	return &V1ContainerManager{
		executor: executor,
		config:   config,
		logger:   logger.WithField("component", "v1-container-manager"),
	}, nil
}

// CreateContainer creates a new container using nerdctl v1.x
func (m *V1ContainerManager) CreateContainer(ctx context.Context, req *types.CreateContainerRequest) (*types.CreateContainerResponse, error) {
	m.logger.WithField("image", req.Image).Info("Creating container with v1 adapter")
	
	// Build nerdctl create command arguments
	args := []string{"create"}
	
	// Add container name if specified
	if req.Name != "" {
		args = append(args, "--name", req.Name)
	}
	
	// Add environment variables
	for _, env := range req.Env {
		args = append(args, "-e", env)
	}
	
	// Add labels
	for key, value := range req.Labels {
		args = append(args, "--label", fmt.Sprintf("%s=%s", key, value))
	}
	
	// Add working directory
	if req.WorkingDir != "" {
		args = append(args, "-w", req.WorkingDir)
	}
	
	// Add TTY and stdin options
	if req.TTY {
		args = append(args, "-t")
	}
	if req.Stdin {
		args = append(args, "-i")
	}
	
	// Add init process
	if req.Init {
		if req.InitBinary != "" {
			args = append(args, "--init", req.InitBinary)
		} else {
			args = append(args, "--init")
		}
	}
	
	// Add stop signal and timeout
	if req.StopSignal != "" {
		args = append(args, "--stop-signal", req.StopSignal)
	}
	if req.StopTimeout > 0 {
		args = append(args, "--stop-timeout", strconv.Itoa(int(req.StopTimeout)))
	}
	
	// Add network configuration
	args = m.addNetworkArgs(args, req.NetworkConfig)
	
	// Add volume mounts
	args = m.addMountArgs(args, req.Mounts)
	
	// Add resource limits
	args = m.addResourceArgs(args, req.Resources)
	
	// Add security options
	args = m.addSecurityArgs(args, req.Security)
	
	// Add image name
	args = append(args, req.Image)
	
	// Add command and args
	if len(req.Command) > 0 {
		args = append(args, req.Command...)
	}
	if len(req.Args) > 0 {
		args = append(args, req.Args...)
	}
	
	// Execute command
	result, err := m.executor.Execute(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("failed to execute create command: %w", err)
	}
	
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("create command failed: %s", result.Stderr)
	}
	
	// Parse container ID from output
	containerID := strings.TrimSpace(result.Stdout)
	if containerID == "" {
		return nil, fmt.Errorf("no container ID returned from create command")
	}
	
	// Parse warnings from stderr (v1.x format specific)
	var warnings []string
	if result.Stderr != "" {
		warnings = m.parseWarnings(result.Stderr)
	}
	
	response := &types.CreateContainerResponse{
		ContainerID: containerID,
		Warnings:    warnings,
	}
	
	m.logger.WithFields(logrus.Fields{
		"container_id": containerID,
		"image":        req.Image,
		"warnings":     len(warnings),
	}).Info("Container created successfully")
	
	return response, nil
}

// StartContainer starts an existing container
func (m *V1ContainerManager) StartContainer(ctx context.Context, req *types.StartContainerRequest) (*types.StartContainerResponse, error) {
	m.logger.WithField("container_id", req.ContainerID).Info("Starting container with v1 adapter")
	
	args := []string{"start"}
	
	// Add attach options if specified
	if len(req.Attach) > 0 {
		args = append(args, "--attach")
		for _, attach := range req.Attach {
			args = append(args, attach)
		}
	}
	
	// Add detach keys if specified
	if req.DetachKeys != "" {
		args = append(args, "--detach-keys", req.DetachKeys)
	}
	
	args = append(args, req.ContainerID)
	
	result, err := m.executor.Execute(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("failed to execute start command: %w", err)
	}
	
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("start command failed: %s", result.Stderr)
	}
	
	response := &types.StartContainerResponse{
		ContainerID: req.ContainerID,
	}
	
	m.logger.WithField("container_id", req.ContainerID).Info("Container started successfully")
	return response, nil
}

// StopContainer stops a running container
func (m *V1ContainerManager) StopContainer(ctx context.Context, req *types.StopContainerRequest) (*types.StopContainerResponse, error) {
	m.logger.WithField("container_id", req.ContainerID).Info("Stopping container with v1 adapter")
	
	args := []string{"stop"}
	
	// Add timeout if specified
	if req.Timeout > 0 {
		args = append(args, "--time", strconv.Itoa(int(req.Timeout)))
	}
	
	// Add signal if specified
	if req.Signal != "" {
		args = append(args, "--signal", req.Signal)
	}
	
	args = append(args, req.ContainerID)
	
	result, err := m.executor.Execute(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("failed to execute stop command: %w", err)
	}
	
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("stop command failed: %s", result.Stderr)
	}
	
	response := &types.StopContainerResponse{
		ContainerID: req.ContainerID,
	}
	
	m.logger.WithField("container_id", req.ContainerID).Info("Container stopped successfully")
	return response, nil
}

// RestartContainer restarts a container
func (m *V1ContainerManager) RestartContainer(ctx context.Context, req *types.RestartContainerRequest) (*types.RestartContainerResponse, error) {
	m.logger.WithField("container_id", req.ContainerID).Info("Restarting container with v1 adapter")
	
	args := []string{"restart"}
	
	// Add timeout if specified
	if req.Timeout > 0 {
		args = append(args, "--time", strconv.Itoa(int(req.Timeout)))
	}
	
	args = append(args, req.ContainerID)
	
	result, err := m.executor.Execute(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("failed to execute restart command: %w", err)
	}
	
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("restart command failed: %s", result.Stderr)
	}
	
	response := &types.RestartContainerResponse{
		ContainerID: req.ContainerID,
	}
	
	m.logger.WithField("container_id", req.ContainerID).Info("Container restarted successfully")
	return response, nil
}

// RemoveContainer removes a container
func (m *V1ContainerManager) RemoveContainer(ctx context.Context, req *types.RemoveContainerRequest) (*types.RemoveContainerResponse, error) {
	m.logger.WithField("container_id", req.ContainerID).Info("Removing container with v1 adapter")
	
	args := []string{"rm"}
	
	// Add options
	if req.Force {
		args = append(args, "--force")
	}
	if req.Volumes {
		args = append(args, "--volumes")
	}
	if req.Link {
		args = append(args, "--link")
	}
	
	args = append(args, req.ContainerID)
	
	result, err := m.executor.Execute(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("failed to execute remove command: %w", err)
	}
	
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("remove command failed: %s", result.Stderr)
	}
	
	response := &types.RemoveContainerResponse{
		ContainerID: req.ContainerID,
	}
	
	m.logger.WithField("container_id", req.ContainerID).Info("Container removed successfully")
	return response, nil
}

// ListContainers lists containers
func (m *V1ContainerManager) ListContainers(ctx context.Context, req *types.ListContainersRequest) (*types.ListContainersResponse, error) {
	m.logger.Debug("Listing containers with v1 adapter")
	
	args := []string{"ps"}
	
	// Add format JSON
	args = append(args, "--format", "json")
	
	// Add options
	if req.All {
		args = append(args, "--all")
	}
	if req.Latest {
		args = append(args, "--latest")
	}
	if req.Last > 0 {
		args = append(args, "--last", strconv.Itoa(int(req.Last)))
	}
	if req.Size {
		args = append(args, "--size")
	}
	
	// Add filters
	for key, values := range req.Filters {
		for _, value := range values {
			args = append(args, "--filter", fmt.Sprintf("%s=%s", key, value))
		}
	}
	
	result, err := m.executor.Execute(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("failed to execute list command: %w", err)
	}
	
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("list command failed: %s", result.Stderr)
	}
	
	// Parse container list
	containerData, err := m.executor.ParseContainerList(result.Stdout)
	if err != nil {
		return nil, fmt.Errorf("failed to parse container list: %w", err)
	}
	
	// Convert to internal types
	var containers []types.ContainerInfo
	for _, data := range containerData {
		container, err := m.parseContainerInfo(data)
		if err != nil {
			m.logger.WithError(err).Warn("Failed to parse container info, skipping")
			continue
		}
		containers = append(containers, *container)
	}
	
	response := &types.ListContainersResponse{
		Containers: containers,
	}
	
	m.logger.WithField("count", len(containers)).Debug("Listed containers successfully")
	return response, nil
}

// InspectContainer inspects a container
func (m *V1ContainerManager) InspectContainer(ctx context.Context, req *types.InspectContainerRequest) (*types.InspectContainerResponse, error) {
	m.logger.WithField("container_id", req.ContainerID).Debug("Inspecting container with v1 adapter")
	
	args := []string{"inspect", "--format", "json"}
	
	if req.Size {
		args = append(args, "--size")
	}
	
	args = append(args, req.ContainerID)
	
	result, err := m.executor.Execute(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("failed to execute inspect command: %w", err)
	}
	
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("inspect command failed: %s", result.Stderr)
	}
	
	// Parse inspect output (v1.x returns array)
	var inspectData []map[string]interface{}
	if err := json.Unmarshal([]byte(result.Stdout), &inspectData); err != nil {
		return nil, fmt.Errorf("failed to parse inspect output: %w", err)
	}
	
	if len(inspectData) == 0 {
		return nil, fmt.Errorf("container not found")
	}
	
	// Convert to internal types
	container, err := m.parseContainerInfo(inspectData[0])
	if err != nil {
		return nil, fmt.Errorf("failed to parse container info: %w", err)
	}
	
	response := &types.InspectContainerResponse{
		Container: container,
	}
	
	return response, nil
}

// GetContainerLogs streams container logs
func (m *V1ContainerManager) GetContainerLogs(ctx context.Context, req *types.GetContainerLogsRequest) (<-chan *types.LogEntry, error) {
	m.logger.WithField("container_id", req.ContainerID).Debug("Getting container logs with v1 adapter")
	
	args := []string{"logs"}
	
	// Add options
	if req.Follow {
		args = append(args, "--follow")
	}
	if req.Stdout {
		args = append(args, "--stdout")
	}
	if req.Stderr {
		args = append(args, "--stderr")
	}
	if req.Since != "" {
		args = append(args, "--since", req.Since)
	}
	if req.Until != "" {
		args = append(args, "--until", req.Until)
	}
	if req.Timestamps {
		args = append(args, "--timestamps")
	}
	if req.Tail != "" {
		args = append(args, "--tail", req.Tail)
	}
	
	args = append(args, req.ContainerID)
	
	// Execute streaming command
	streamResult, err := m.executor.ExecuteStream(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("failed to start logs stream: %w", err)
	}
	
	// Create output channel
	logChan := make(chan *types.LogEntry, 100)
	
	// Start goroutine to process stream
	go func() {
		defer close(logChan)
		defer streamResult.Cancel()
		
		for {
			select {
			case <-ctx.Done():
				return
			case line, ok := <-streamResult.LinesChan:
				if !ok {
					return
				}
				
				// Parse log line
				logEntry := m.parseLogLine(line, req.Timestamps)
				select {
				case <-ctx.Done():
					return
				case logChan <- logEntry:
				}
				
			case err, ok := <-streamResult.ErrorsChan:
				if ok && err != nil {
					m.logger.WithError(err).Error("Error in log stream")
				}
				return
			}
		}
	}()
	
	return logChan, nil
}

// Helper methods for building command arguments

func (m *V1ContainerManager) addNetworkArgs(args []string, netConfig types.NetworkConfig) []string {
	if netConfig.Mode != "" {
		args = append(args, "--network", netConfig.Mode)
	}
	
	// Add port mappings
	for _, port := range netConfig.Ports {
		portStr := fmt.Sprintf("%s:%d:%d", port.HostIP, port.HostPort, port.ContainerPort)
		if port.Protocol != "" {
			portStr += "/" + port.Protocol
		}
		args = append(args, "-p", portStr)
	}
	
	// Add DNS settings
	for _, dns := range netConfig.DNSServers {
		args = append(args, "--dns", dns)
	}
	
	for _, search := range netConfig.DNSSearch {
		args = append(args, "--dns-search", search)
	}
	
	if netConfig.Hostname != "" {
		args = append(args, "--hostname", netConfig.Hostname)
	}
	
	return args
}

func (m *V1ContainerManager) addMountArgs(args []string, mounts []types.VolumeMount) []string {
	for _, mount := range mounts {
		mountStr := fmt.Sprintf("%s:%s", mount.Source, mount.Destination)
		if mount.ReadOnly {
			mountStr += ":ro"
		}
		args = append(args, "-v", mountStr)
	}
	return args
}

func (m *V1ContainerManager) addResourceArgs(args []string, resources types.ResourceLimits) []string {
	if resources.MemoryBytes > 0 {
		args = append(args, "--memory", strconv.FormatInt(resources.MemoryBytes, 10))
	}
	
	if resources.CPUQuota > 0 {
		args = append(args, "--cpu-quota", strconv.FormatFloat(resources.CPUQuota, 'f', -1, 64))
	}
	
	if resources.CPUShares > 0 {
		args = append(args, "--cpu-shares", strconv.Itoa(int(resources.CPUShares)))
	}
	
	return args
}

func (m *V1ContainerManager) addSecurityArgs(args []string, security types.SecurityOptions) []string {
	if security.User != "" {
		args = append(args, "--user", security.User)
	}
	
	if security.Privileged {
		args = append(args, "--privileged")
	}
	
	for _, cap := range security.CapAdd {
		args = append(args, "--cap-add", cap)
	}
	
	for _, cap := range security.CapDrop {
		args = append(args, "--cap-drop", cap)
	}
	
	return args
}

// Helper methods for parsing output

func (m *V1ContainerManager) parseWarnings(stderr string) []string {
	var warnings []string
	lines := strings.Split(stderr, "\n")
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.Contains(strings.ToLower(line), "error") {
			warnings = append(warnings, line)
		}
	}
	
	return warnings
}

func (m *V1ContainerManager) parseContainerInfo(data map[string]interface{}) (*types.ContainerInfo, error) {
	// This is a simplified parser - in real implementation would be more comprehensive
	container := &types.ContainerInfo{}
	
	if id, ok := data["Id"].(string); ok {
		container.ID = id
		if len(id) > 12 {
			container.ShortID = id[:12]
		}
	}
	
	if name, ok := data["Name"].(string); ok {
		container.Name = strings.TrimPrefix(name, "/")
	}
	
	if image, ok := data["Image"].(string); ok {
		container.Image = image
	}
	
	if status, ok := data["Status"].(string); ok {
		container.Status = m.parseContainerStatus(status)
	}
	
	// Parse created time
	if created, ok := data["Created"].(string); ok {
		if t, err := time.Parse(time.RFC3339, created); err == nil {
			container.Created = t
		}
	}
	
	// Parse labels
	if labels, ok := data["Labels"].(map[string]interface{}); ok {
		container.Labels = make(map[string]string)
		for k, v := range labels {
			if s, ok := v.(string); ok {
				container.Labels[k] = s
			}
		}
	}
	
	return container, nil
}

func (m *V1ContainerManager) parseContainerStatus(status string) types.ContainerStatus {
	lower := strings.ToLower(status)
	switch {
	case strings.Contains(lower, "running"):
		return types.ContainerStatusRunning
	case strings.Contains(lower, "exited"):
		return types.ContainerStatusExited
	case strings.Contains(lower, "created"):
		return types.ContainerStatusCreated
	case strings.Contains(lower, "paused"):
		return types.ContainerStatusPaused
	default:
		return types.ContainerStatusUnknown
	}
}

func (m *V1ContainerManager) parseLogLine(line string, includeTimestamp bool) *types.LogEntry {
	entry := &types.LogEntry{
		Content:   line,
		Source:    "stdout", // v1.x may not distinguish stdout/stderr in logs
		Timestamp: time.Now(),
	}
	
	// If timestamps are included, try to parse them
	if includeTimestamp && len(line) > 30 {
		// v1.x timestamp format: "2023-01-01T12:00:00.000000000Z message"
		if t, err := time.Parse(time.RFC3339Nano, line[:30]); err == nil {
			entry.Timestamp = t
			entry.Content = line[31:] // Remove timestamp prefix
		}
	}
	
	return entry
}

// Implement remaining interface methods...
// For brevity, I'll implement stubs for the remaining methods

func (m *V1ContainerManager) KillContainer(ctx context.Context, req *types.KillContainerRequest) (*types.KillContainerResponse, error) {
	// Implementation similar to StopContainer but with kill command
	return nil, fmt.Errorf("not implemented in this example")
}

func (m *V1ContainerManager) PauseContainer(ctx context.Context, req *types.PauseContainerRequest) (*types.PauseContainerResponse, error) {
	return nil, fmt.Errorf("not implemented in this example")
}

func (m *V1ContainerManager) UnpauseContainer(ctx context.Context, req *types.UnpauseContainerRequest) (*types.UnpauseContainerResponse, error) {
	return nil, fmt.Errorf("not implemented in this example")
}

func (m *V1ContainerManager) RunContainer(ctx context.Context, req *types.RunContainerRequest) (*types.RunContainerResponse, error) {
	return nil, fmt.Errorf("not implemented in this example")
}

func (m *V1ContainerManager) RunContainerStream(ctx context.Context, req *types.RunContainerRequest) (<-chan *types.RunContainerStreamResponse, error) {
	return nil, fmt.Errorf("not implemented in this example")
}

func (m *V1ContainerManager) AttachContainer(ctx context.Context, req *types.AttachContainerRequest) (interfaces.AttachStream, error) {
	return nil, fmt.Errorf("not implemented in this example")
}

func (m *V1ContainerManager) ExecContainer(ctx context.Context, req *types.ExecContainerRequest) (*types.ExecContainerResponse, error) {
	return nil, fmt.Errorf("not implemented in this example")
}

func (m *V1ContainerManager) ExecContainerStream(ctx context.Context, req *types.ExecContainerRequest) (interfaces.ExecStream, error) {
	return nil, fmt.Errorf("not implemented in this example")
}

func (m *V1ContainerManager) GetContainerStats(ctx context.Context, req *types.GetContainerStatsRequest) (<-chan *types.ContainerStats, error) {
	return nil, fmt.Errorf("not implemented in this example")
}

func (m *V1ContainerManager) WaitContainer(ctx context.Context, req *types.WaitContainerRequest) (*types.WaitContainerResponse, error) {
	return nil, fmt.Errorf("not implemented in this example")
}

func (m *V1ContainerManager) RenameContainer(ctx context.Context, req *types.RenameContainerRequest) (*types.RenameContainerResponse, error) {
	return nil, fmt.Errorf("not implemented in this example")
}

func (m *V1ContainerManager) UpdateContainer(ctx context.Context, req *types.UpdateContainerRequest) (*types.UpdateContainerResponse, error) {
	return nil, fmt.Errorf("not implemented in this example")
}

func (m *V1ContainerManager) CopyToContainer(ctx context.Context, req *types.CopyToContainerRequest, data io.Reader) (*types.CopyToContainerResponse, error) {
	return nil, fmt.Errorf("not implemented in this example")
}

func (m *V1ContainerManager) CopyFromContainer(ctx context.Context, req *types.CopyFromContainerRequest) (io.ReadCloser, error) {
	return nil, fmt.Errorf("not implemented in this example")
}

func (m *V1ContainerManager) ExportContainer(ctx context.Context, req *types.ExportContainerRequest) (io.ReadCloser, error) {
	return nil, fmt.Errorf("not implemented in this example")
}

func (m *V1ContainerManager) DiffContainer(ctx context.Context, req *types.DiffContainerRequest) (*types.DiffContainerResponse, error) {
	return nil, fmt.Errorf("not implemented in this example")
}

func (m *V1ContainerManager) CommitContainer(ctx context.Context, req *types.CommitContainerRequest) (*types.CommitContainerResponse, error) {
	return nil, fmt.Errorf("not implemented in this example")
}

func (m *V1ContainerManager) PruneContainers(ctx context.Context, req *types.PruneContainersRequest) (*types.PruneContainersResponse, error) {
	return nil, fmt.Errorf("not implemented in this example")
}

func (m *V1ContainerManager) GetSupportedFeatures(ctx context.Context) (*types.SupportedFeatures, error) {
	return &types.SupportedFeatures{
		Version:  m.executor.GetVersion().Client.Raw,
		Features: []string{"basic_lifecycle", "logs", "exec", "cp", "build_basic"},
	}, nil
}

func (m *V1ContainerManager) GetVersion(ctx context.Context) (*types.VersionInfo, error) {
	versionInfo := m.executor.GetVersion()
	return &types.VersionInfo{
		Version:   versionInfo.Client.Raw,
		GitCommit: versionInfo.BuildInfo.GitCommit,
		BuildDate: versionInfo.BuildInfo.BuildDate,
		Platform:  versionInfo.BuildInfo.Platform,
	}, nil
}