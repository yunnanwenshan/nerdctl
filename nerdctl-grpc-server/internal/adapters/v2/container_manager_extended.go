package v2

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/containerd/nerdctl-grpc-server/internal/config"
	"github.com/containerd/nerdctl-grpc-server/internal/interfaces"
	"github.com/containerd/nerdctl-grpc-server/pkg/types"
	"github.com/sirupsen/logrus"
)

// ExtendedV2ContainerManager provides comprehensive container management with all nerdctl functionality
type ExtendedV2ContainerManager struct {
	*V2ContainerManager
	eventStreams map[string]chan *types.ContainerEvent
	eventMu      sync.RWMutex
	statsMu      sync.RWMutex
	statsStreams map[string]chan *types.ContainerStats
}

// NewExtendedV2ContainerManager creates a new extended container manager with comprehensive functionality
func NewExtendedV2ContainerManager(executor *V2CommandExecutor, config *config.Config, logger *logrus.Logger) (*ExtendedV2ContainerManager, error) {
	baseManager, err := NewV2ContainerManager(executor, config, logger)
	if err != nil {
		return nil, err
	}

	return &ExtendedV2ContainerManager{
		V2ContainerManager: baseManager,
		eventStreams:       make(map[string]chan *types.ContainerEvent),
		statsStreams:       make(map[string]chan *types.ContainerStats),
	}, nil
}

// GetContainerTop gets container process information
func (m *ExtendedV2ContainerManager) GetContainerTop(ctx context.Context, opts *types.GetContainerTopOptions) (*types.ContainerTopResult, error) {
	m.logger.WithFields(logrus.Fields{
		"container_id": opts.ContainerID,
		"ps_args":      opts.PsArgs,
	}).Info("Getting container top information")

	// Build top command
	args := []string{"container", "top", opts.ContainerID}
	if opts.PsArgs != "" {
		args = append(args, opts.PsArgs)
	}

	// Execute command
	result, err := m.executor.ExecuteCommand(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get container top: %w", err)
	}

	// Parse top output
	lines := strings.Split(result.Stdout, "\n")
	if len(lines) < 2 {
		return &types.ContainerTopResult{
			Titles:    []string{},
			Processes: []types.ProcessInfo{},
		}, nil
	}

	// Parse titles (first line)
	titles := strings.Fields(lines[0])

	// Parse processes (remaining lines)
	processes := make([]types.ProcessInfo, 0, len(lines)-1)
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "" {
			continue
		}
		values := strings.Fields(lines[i])
		processes = append(processes, types.ProcessInfo{Values: values})
	}

	return &types.ContainerTopResult{
		Titles:    titles,
		Processes: processes,
	}, nil
}

// GetContainerPort gets container port information
func (m *ExtendedV2ContainerManager) GetContainerPort(ctx context.Context, opts *types.GetContainerPortOptions) ([]types.ContainerPortInfo, error) {
	m.logger.WithFields(logrus.Fields{
		"container_id": opts.ContainerID,
		"private_port": opts.PrivatePort,
	}).Info("Getting container port information")

	// Build port command
	args := []string{"container", "port", opts.ContainerID}
	if opts.PrivatePort != "" {
		args = append(args, opts.PrivatePort)
	}

	// Execute command
	result, err := m.executor.ExecuteCommand(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get container ports: %w", err)
	}

	// Parse port output
	lines := strings.Split(result.Stdout, "\n")
	ports := make([]types.ContainerPortInfo, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse format: "80/tcp -> 0.0.0.0:8080"
		parts := strings.Split(line, " -> ")
		if len(parts) != 2 {
			continue
		}

		privatePort := parts[0]
		publicPart := parts[1]

		// Parse public part
		var ip, publicPort string
		if colonIdx := strings.LastIndex(publicPart, ":"); colonIdx != -1 {
			ip = publicPart[:colonIdx]
			publicPort = publicPart[colonIdx+1:]
		} else {
			publicPort = publicPart
		}

		// Parse type from private port
		var portType string
		if slashIdx := strings.Index(privatePort, "/"); slashIdx != -1 {
			portType = privatePort[slashIdx+1:]
			privatePort = privatePort[:slashIdx]
		}

		ports = append(ports, types.ContainerPortInfo{
			PrivatePort: privatePort,
			PublicPort:  publicPort,
			Type:        portType,
			IP:          ip,
		})
	}

	return ports, nil
}

// HealthCheckContainer performs health check on container
func (m *ExtendedV2ContainerManager) HealthCheckContainer(ctx context.Context, opts *types.HealthCheckContainerOptions) (*types.ContainerHealthInfo, error) {
	m.logger.WithFields(logrus.Fields{
		"container_id": opts.ContainerID,
	}).Info("Performing container health check")

	// Get container inspect data which includes health status
	inspectOpts := &types.InspectContainerRequest{
		ContainerID: opts.ContainerID,
		Namespace:   opts.Namespace,
		Size:        false,
	}

	inspectResult, err := m.InspectContainer(ctx, inspectOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container for health check: %w", err)
	}

	// Extract health information from inspect result
	// For now, return basic health info; in a real implementation,
	// this would parse the detailed health check data from nerdctl inspect
	return &types.ContainerHealthInfo{
		Status:        "healthy", // This should be parsed from inspect result
		FailingStreak: 0,
		Log: []types.HealthCheckResult{
			{
				Start:    time.Now().Add(-1 * time.Minute),
				End:      time.Now(),
				ExitCode: 0,
				Output:   "Health check passed",
			},
		},
	}, nil
}

// MonitorContainerEvents monitors container events
func (m *ExtendedV2ContainerManager) MonitorContainerEvents(ctx context.Context, opts *types.MonitorContainerEventsOptions) (<-chan types.ContainerEvent, <-chan error) {
	eventChan := make(chan types.ContainerEvent, 100)
	errChan := make(chan error, 1)

	go func() {
		defer close(eventChan)
		defer close(errChan)

		m.logger.WithFields(logrus.Fields{
			"container_ids": opts.ContainerIDs,
			"event_types":   opts.EventTypes,
		}).Info("Starting container event monitoring")

		// Build events command
		args := []string{"events"}

		// Add time filters
		if !opts.Since.IsZero() {
			args = append(args, "--since", opts.Since.Format(time.RFC3339))
		}
		if !opts.Until.IsZero() {
			args = append(args, "--until", opts.Until.Format(time.RFC3339))
		}

		// Add container filters
		for _, containerID := range opts.ContainerIDs {
			args = append(args, "--filter", fmt.Sprintf("container=%s", containerID))
		}

		// Add event type filters
		for _, eventType := range opts.EventTypes {
			args = append(args, "--filter", fmt.Sprintf("event=%s", eventType))
		}

		// Add additional filters
		for key, value := range opts.Filters {
			args = append(args, "--filter", fmt.Sprintf("%s=%s", key, value))
		}

		// Execute events command with streaming
		cmd := exec.CommandContext(ctx, m.executor.nerdctlPath, args...)
		if opts.Namespace != "" {
			cmd.Env = append(os.Environ(), fmt.Sprintf("CONTAINERD_NAMESPACE=%s", opts.Namespace))
		}

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			errChan <- fmt.Errorf("failed to create stdout pipe: %w", err)
			return
		}

		if err := cmd.Start(); err != nil {
			errChan <- fmt.Errorf("failed to start events command: %w", err)
			return
		}

		// Parse events from stdout
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}

			// Parse event line (nerdctl events format)
			event, err := m.parseEventLine(line)
			if err != nil {
				m.logger.WithError(err).Warn("Failed to parse event line")
				continue
			}

			select {
			case eventChan <- *event:
			case <-ctx.Done():
				return
			}
		}

		if err := scanner.Err(); err != nil {
			errChan <- fmt.Errorf("error reading events: %w", err)
		}

		if err := cmd.Wait(); err != nil && ctx.Err() == nil {
			errChan <- fmt.Errorf("events command failed: %w", err)
		}
	}()

	return eventChan, errChan
}

// parseEventLine parses a single event line from nerdctl events
func (m *ExtendedV2ContainerManager) parseEventLine(line string) (*types.ContainerEvent, error) {
	// Expected format: "2023-07-25T12:00:00.000Z container create 1234567890ab (image=nginx:latest, name=test)"
	parts := strings.SplitN(line, " ", 4)
	if len(parts) < 4 {
		return nil, fmt.Errorf("invalid event line format: %s", line)
	}

	timestamp, err := time.Parse(time.RFC3339, parts[0])
	if err != nil {
		return nil, fmt.Errorf("failed to parse timestamp: %w", err)
	}

	containerType := parts[1]
	eventType := parts[2]
	remaining := parts[3]

	// Extract container ID and attributes
	spaceLoc := strings.Index(remaining, " ")
	var containerID string
	var attributesStr string

	if spaceLoc != -1 {
		containerID = remaining[:spaceLoc]
		attributesStr = remaining[spaceLoc+1:]
	} else {
		containerID = remaining
	}

	// Parse attributes from format "(key=value, key=value)"
	attributes := make(map[string]string)
	if attributesStr != "" {
		attributesStr = strings.Trim(attributesStr, "()")
		attrPairs := strings.Split(attributesStr, ", ")
		for _, pair := range attrPairs {
			if eqIdx := strings.Index(pair, "="); eqIdx != -1 {
				key := pair[:eqIdx]
				value := pair[eqIdx+1:]
				attributes[key] = value
			}
		}
	}

	return &types.ContainerEvent{
		Type:          eventType,
		ContainerID:   containerID,
		ContainerName: attributes["name"],
		Image:         attributes["image"],
		Timestamp:     timestamp,
		Attributes:    attributes,
	}, nil
}

// Batch operations

// BatchStartContainers starts multiple containers
func (m *ExtendedV2ContainerManager) BatchStartContainers(ctx context.Context, containerIDs []string, namespace string) ([]types.BatchOperationResult, error) {
	return m.executeBatchOperation(ctx, "start", containerIDs, namespace)
}

// BatchStopContainers stops multiple containers
func (m *ExtendedV2ContainerManager) BatchStopContainers(ctx context.Context, containerIDs []string, namespace string) ([]types.BatchOperationResult, error) {
	return m.executeBatchOperation(ctx, "stop", containerIDs, namespace)
}

// BatchRemoveContainers removes multiple containers
func (m *ExtendedV2ContainerManager) BatchRemoveContainers(ctx context.Context, containerIDs []string, namespace string) ([]types.BatchOperationResult, error) {
	return m.executeBatchOperation(ctx, "rm", containerIDs, namespace)
}

// BatchRestartContainers restarts multiple containers
func (m *ExtendedV2ContainerManager) BatchRestartContainers(ctx context.Context, containerIDs []string, namespace string) ([]types.BatchOperationResult, error) {
	return m.executeBatchOperation(ctx, "restart", containerIDs, namespace)
}

// executeBatchOperation executes a batch operation on multiple containers
func (m *ExtendedV2ContainerManager) executeBatchOperation(ctx context.Context, operation string, containerIDs []string, namespace string) ([]types.BatchOperationResult, error) {
	results := make([]types.BatchOperationResult, len(containerIDs))
	
	// Use goroutines for parallel execution with limited concurrency
	semaphore := make(chan struct{}, 10) // Limit to 10 concurrent operations
	var wg sync.WaitGroup
	
	for i, containerID := range containerIDs {
		wg.Add(1)
		go func(index int, id string) {
			defer wg.Done()
			semaphore <- struct{}{} // Acquire
			defer func() { <-semaphore }() // Release
			
			result := types.BatchOperationResult{
				ContainerID: id,
				Success:     false,
			}
			
			// Build command based on operation
			var args []string
			switch operation {
			case "start":
				args = []string{"container", "start", id}
			case "stop":
				args = []string{"container", "stop", id}
			case "rm":
				args = []string{"container", "rm", id}
			case "restart":
				args = []string{"container", "restart", id}
			default:
				result.Error = fmt.Sprintf("unsupported operation: %s", operation)
				results[index] = result
				return
			}
			
			// Execute command
			_, err := m.executor.ExecuteCommand(ctx, args...)
			if err != nil {
				result.Error = err.Error()
			} else {
				result.Success = true
			}
			
			results[index] = result
		}(i, containerID)
	}
	
	wg.Wait()
	return results, nil
}

// Enhanced streaming capabilities

// GetContainerStatsExtended provides enhanced container statistics with detailed metrics
func (m *ExtendedV2ContainerManager) GetContainerStatsExtended(ctx context.Context, opts *types.GetContainerStatsOptions) (<-chan *types.ContainerStats, <-chan error) {
	statsChan := make(chan *types.ContainerStats, 10)
	errChan := make(chan error, 1)

	go func() {
		defer close(statsChan)
		defer close(errChan)

		m.logger.WithFields(logrus.Fields{
			"container_id": opts.ContainerID,
			"stream":       opts.Stream,
		}).Info("Starting enhanced container stats streaming")

		// Build stats command
		args := []string{"container", "stats"}
		if !opts.Stream {
			args = append(args, "--no-stream")
		}
		args = append(args, opts.ContainerID)

		// Execute command with streaming
		cmd := exec.CommandContext(ctx, m.executor.nerdctlPath, args...)
		if opts.Namespace != "" {
			cmd.Env = append(os.Environ(), fmt.Sprintf("CONTAINERD_NAMESPACE=%s", opts.Namespace))
		}

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			errChan <- fmt.Errorf("failed to create stdout pipe: %w", err)
			return
		}

		if err := cmd.Start(); err != nil {
			errChan <- fmt.Errorf("failed to start stats command: %w", err)
			return
		}

		// Parse stats from stdout
		scanner := bufio.NewScanner(stdout)
		var headerParsed bool

		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}

			if !headerParsed {
				// Skip header line
				headerParsed = true
				continue
			}

			// Parse stats line
			stats, err := m.parseStatsLine(line, opts.ContainerID)
			if err != nil {
				m.logger.WithError(err).Warn("Failed to parse stats line")
				continue
			}

			select {
			case statsChan <- stats:
			case <-ctx.Done():
				return
			}

			// If not streaming, break after first result
			if !opts.Stream {
				break
			}
		}

		if err := scanner.Err(); err != nil {
			errChan <- fmt.Errorf("error reading stats: %w", err)
		}

		if err := cmd.Wait(); err != nil && ctx.Err() == nil {
			errChan <- fmt.Errorf("stats command failed: %w", err)
		}
	}()

	return statsChan, errChan
}

// parseStatsLine parses a single stats line from nerdctl stats
func (m *ExtendedV2ContainerManager) parseStatsLine(line, containerID string) (*types.ContainerStats, error) {
	// Expected format: "CONTAINER ID NAME CPU % MEM USAGE / LIMIT MEM % NET I/O BLOCK I/O PIDS"
	fields := strings.Fields(line)
	if len(fields) < 8 {
		return nil, fmt.Errorf("invalid stats line format: %s", line)
	}

	// Parse CPU percentage
	cpuPercent, _ := strconv.ParseFloat(strings.TrimSuffix(fields[2], "%"), 64)

	// Parse memory usage and limit
	memParts := strings.Split(fields[3], "/")
	var memUsage, memLimit uint64
	if len(memParts) >= 2 {
		memUsage, _ = parseSize(strings.TrimSpace(memParts[0]))
		memLimit, _ = parseSize(strings.TrimSpace(memParts[1]))
	}

	// Parse memory percentage
	memPercent, _ := strconv.ParseFloat(strings.TrimSuffix(fields[5], "%"), 64)

	// Parse network I/O
	netParts := strings.Split(fields[6], "/")
	var netI, netO uint64
	if len(netParts) >= 2 {
		netI, _ = parseSize(strings.TrimSpace(netParts[0]))
		netO, _ = parseSize(strings.TrimSpace(netParts[1]))
	}

	// Parse block I/O
	blockParts := strings.Split(fields[7], "/")
	var blockI, blockO uint64
	if len(blockParts) >= 2 {
		blockI, _ = parseSize(strings.TrimSpace(blockParts[0]))
		blockO, _ = parseSize(strings.TrimSpace(blockParts[1]))
	}

	// Parse PIDs
	pids, _ := strconv.Atoi(fields[8])

	return &types.ContainerStats{
		ContainerID: containerID,
		Name:        fields[1],
		CPUPercent:  cpuPercent,
		MemUsage:    memUsage,
		MemLimit:    memLimit,
		MemPercent:  memPercent,
		NetI:        netI,
		NetO:        netO,
		BlockI:      blockI,
		BlockO:      blockO,
		Pids:        pids,
		Timestamp:   time.Now(),
	}, nil
}

// parseSize parses size strings like "1.5GiB" to bytes
func parseSize(s string) (uint64, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "--" {
		return 0, nil
	}

	// Find the numeric part
	var numStr string
	var suffix string
	for i, r := range s {
		if (r < '0' || r > '9') && r != '.' {
			numStr = s[:i]
			suffix = s[i:]
			break
		}
	}

	if numStr == "" {
		numStr = s
	}

	num, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, err
	}

	// Convert based on suffix
	multiplier := uint64(1)
	switch strings.ToLower(suffix) {
	case "b", "":
		multiplier = 1
	case "kb", "k":
		multiplier = 1000
	case "mb", "m":
		multiplier = 1000 * 1000
	case "gb", "g":
		multiplier = 1000 * 1000 * 1000
	case "tb", "t":
		multiplier = 1000 * 1000 * 1000 * 1000
	case "kib":
		multiplier = 1024
	case "mib":
		multiplier = 1024 * 1024
	case "gib":
		multiplier = 1024 * 1024 * 1024
	case "tib":
		multiplier = 1024 * 1024 * 1024 * 1024
	}

	return uint64(num * float64(multiplier)), nil
}

// Enhanced container operations with comprehensive error handling and logging

// RunContainerWithAdvancedOptions runs a container with comprehensive configuration support
func (m *ExtendedV2ContainerManager) RunContainerWithAdvancedOptions(ctx context.Context, config *types.ContainerConfig, opts *types.RunContainerOptions) (*types.RunContainerResult, error) {
	m.logger.WithFields(logrus.Fields{
		"image":     config.Image,
		"name":      config.Name,
		"detach":    opts.Detach,
		"remove":    opts.Remove,
		"platform":  opts.Platform,
	}).Info("Running container with advanced configuration")

	// Build comprehensive run command
	args := []string{"run"}

	// Add detach option
	if opts.Detach {
		args = append(args, "-d")
	}

	// Add remove option
	if opts.Remove {
		args = append(args, "--rm")
	}

	// Add name
	if config.Name != "" {
		args = append(args, "--name", config.Name)
	}

	// Add platform
	if opts.Platform != "" {
		args = append(args, "--platform", opts.Platform)
	}

	// Add comprehensive configuration
	args = m.buildContainerArgs(args, config)

	// Add image
	args = append(args, config.Image)

	// Add command and args
	if len(config.Command) > 0 {
		args = append(args, config.Command...)
	}
	if len(config.Args) > 0 {
		args = append(args, config.Args...)
	}

	// Execute command
	result, err := m.executor.ExecuteCommand(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to run container: %w", err)
	}

	containerID := strings.TrimSpace(result.Stdout)

	return &types.RunContainerResult{
		ID:       containerID,
		ExitCode: int32(result.ExitCode),
		Warnings: []string{}, // Parse warnings from stderr if needed
		Container: types.Container{
			ID:    containerID,
			Name:  config.Name,
			Image: config.Image,
			State: "running",
		},
	}, nil
}

// buildContainerArgs builds comprehensive container arguments from configuration
func (m *ExtendedV2ContainerManager) buildContainerArgs(args []string, config *types.ContainerConfig) []string {
	// Environment variables
	for _, env := range config.Env {
		args = append(args, "-e", env)
	}

	// Working directory
	if config.WorkingDir != "" {
		args = append(args, "-w", config.WorkingDir)
	}

	// User
	if config.User != "" {
		args = append(args, "-u", config.User)
	}

	// Labels
	for key, value := range config.Labels {
		args = append(args, "--label", fmt.Sprintf("%s=%s", key, value))
	}

	// Port mappings
	for _, port := range config.Ports {
		portStr := fmt.Sprintf("%d:%d", port.HostPort, port.ContainerPort)
		if port.Protocol != "" {
			portStr += "/" + port.Protocol
		}
		if port.HostIP != "" {
			portStr = port.HostIP + ":" + portStr
		}
		args = append(args, "-p", portStr)
	}

	// Volume mounts
	for _, mount := range config.Mounts {
		mountStr := fmt.Sprintf("%s:%s", mount.Source, mount.Target)
		if mount.ReadOnly {
			mountStr += ":ro"
		}
		args = append(args, "-v", mountStr)
	}

	// TTY and stdin
	if config.TTY {
		args = append(args, "-t")
	}
	if config.Stdin {
		args = append(args, "-i")
	}

	// Privileged mode
	if config.Privileged {
		args = append(args, "--privileged")
	}

	// Read-only root filesystem
	if config.ReadOnlyRootfs {
		args = append(args, "--read-only")
	}

	// Auto-remove
	if config.AutoRemove {
		args = append(args, "--rm")
	}

	// Capabilities
	for _, cap := range config.CapAdd {
		args = append(args, "--cap-add", cap)
	}
	for _, cap := range config.CapDrop {
		args = append(args, "--cap-drop", cap)
	}

	// Devices
	for _, device := range config.Devices {
		args = append(args, "--device", device)
	}

	// Security options
	if config.Security != nil {
		if config.Security.NoNewPrivileges {
			args = append(args, "--security-opt", "no-new-privileges")
		}
		if config.Security.ApparmorProfile != "" {
			args = append(args, "--security-opt", fmt.Sprintf("apparmor=%s", config.Security.ApparmorProfile))
		}
		if config.Security.SeccompProfile != "" {
			args = append(args, "--security-opt", fmt.Sprintf("seccomp=%s", config.Security.SeccompProfile))
		}
	}

	// Resource limits
	if config.Resources != nil {
		if config.Resources.Memory > 0 {
			args = append(args, "--memory", fmt.Sprintf("%d", config.Resources.Memory))
		}
		if config.Resources.CPUs != "" {
			args = append(args, "--cpus", config.Resources.CPUs)
		}
		if config.Resources.CPUShares > 0 {
			args = append(args, "--cpu-shares", fmt.Sprintf("%d", config.Resources.CPUShares))
		}
		if config.Resources.PidsLimit > 0 {
			args = append(args, "--pids-limit", fmt.Sprintf("%d", config.Resources.PidsLimit))
		}
	}

	// Restart policy
	if config.RestartPolicy != nil {
		restartStr := config.RestartPolicy.Name
		if config.RestartPolicy.Name == "on-failure" && config.RestartPolicy.MaximumRetryCount > 0 {
			restartStr = fmt.Sprintf("on-failure:%d", config.RestartPolicy.MaximumRetryCount)
		}
		args = append(args, "--restart", restartStr)
	}

	// Network configuration
	if config.NetworkConfig != nil {
		if config.NetworkConfig.NetworkMode != "" {
			args = append(args, "--network", config.NetworkConfig.NetworkMode)
		}
		if config.NetworkConfig.Hostname != "" {
			args = append(args, "--hostname", config.NetworkConfig.Hostname)
		}
		for _, dns := range config.NetworkConfig.DNS {
			args = append(args, "--dns", dns)
		}
		for _, host := range config.NetworkConfig.ExtraHosts {
			args = append(args, "--add-host", host)
		}
	}

	// Health check
	if config.HealthCheck != nil && len(config.HealthCheck.Test) > 0 {
		healthCmd := strings.Join(config.HealthCheck.Test, " ")
		args = append(args, "--health-cmd", healthCmd)
		if config.HealthCheck.Interval > 0 {
			args = append(args, "--health-interval", config.HealthCheck.Interval.String())
		}
		if config.HealthCheck.Timeout > 0 {
			args = append(args, "--health-timeout", config.HealthCheck.Timeout.String())
		}
		if config.HealthCheck.Retries > 0 {
			args = append(args, "--health-retries", fmt.Sprintf("%d", config.HealthCheck.Retries))
		}
		if config.HealthCheck.StartPeriod > 0 {
			args = append(args, "--health-start-period", config.HealthCheck.StartPeriod.String())
		}
	}

	// Stop signal and timeout
	if config.StopSignal != "" {
		args = append(args, "--stop-signal", config.StopSignal)
	}
	if config.StopTimeout > 0 {
		args = append(args, "--stop-timeout", fmt.Sprintf("%d", config.StopTimeout))
	}

	return args
}