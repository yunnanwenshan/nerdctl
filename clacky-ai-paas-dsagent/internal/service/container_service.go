package service

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/log"

	pb "dsagent/api/container/v1"
	"github.com/containerd/nerdctl/v2/pkg/api/types"
	"github.com/containerd/nerdctl/v2/pkg/clientutil"
	"github.com/containerd/nerdctl/v2/pkg/cmd/container"
	"github.com/containerd/nerdctl/v2/pkg/containerutil"
	"github.com/containerd/nerdctl/v2/pkg/taskutil"
)

// ContainerService implements the gRPC container service interface
type ContainerService struct {
	pb.UnimplementedContainerServiceServer
}

// NewContainerService creates a new container service instance
func NewContainerService() *ContainerService {
	return &ContainerService{}
}

// CreateContainer creates a new container
func (s *ContainerService) CreateContainer(ctx context.Context, req *pb.CreateContainerRequest) (*pb.CreateContainerResponse, error) {
	log.L.Debugf("Creating container with name: %s", req.Name)
	
	// Convert protobuf request to nerdctl options and network options
	createOpt, netOpts, err := s.convertCreateRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to convert create request: %w", err)
	}

	// Create containerd client
	client, cancel, err := s.createClient(createOpt.GOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create containerd client: %w", err)
	}
	defer cancel()

	// Setup network manager
	netManager, err := containerutil.NewNetworkingOptionsManager(createOpt.GOptions, netOpts, client)
	if err != nil {
		return nil, fmt.Errorf("failed to setup network manager: %w", err)
	}

	// Prepare arguments for nerdctl create command
	args := []string{req.Config.Image}
	args = append(args, req.Config.Command...)

	// Create container using nerdctl's create function
	container, gc, err := container.Create(ctx, client, args, netManager, createOpt)
	if err != nil {
		if gc != nil {
			gc()
		}
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	// Return response
	return &pb.CreateContainerResponse{
		Id: container.ID(),
		Warnings: []string{}, // TODO: Add warnings if any
	}, nil
}

// StartContainer starts a container
func (s *ContainerService) StartContainer(ctx context.Context, req *pb.StartContainerRequest) (*pb.StartContainerResponse, error) {
	log.L.Debugf("Starting container: %s", req.ContainerId)

	// Convert request to nerdctl options
	startOpt, err := s.convertStartRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to convert start request: %w", err)
	}

	// Create containerd client
	client, cancel, err := s.createClient(startOpt.GOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create containerd client: %w", err)
	}
	defer cancel()

	// Start container using nerdctl's start function
	err = container.Start(ctx, client, []string{req.ContainerId}, startOpt)
	if err != nil {
		return &pb.StartContainerResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to start container: %v", err),
		}, nil
	}

	return &pb.StartContainerResponse{
		Success: true,
		Message: "Container started successfully",
	}, nil
}

// StopContainer stops a container
func (s *ContainerService) StopContainer(ctx context.Context, req *pb.StopContainerRequest) (*pb.StopContainerResponse, error) {
	log.L.Debugf("Stopping container: %s", req.ContainerId)

	// Convert request to nerdctl options
	stopOpt, err := s.convertStopRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to convert stop request: %w", err)
	}

	// Create containerd client
	client, cancel, err := s.createClient(stopOpt.GOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create containerd client: %w", err)
	}
	defer cancel()

	// Stop container using nerdctl's stop function
	err = container.Stop(ctx, client, []string{req.ContainerId}, stopOpt)
	if err != nil {
		return &pb.StopContainerResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to stop container: %v", err),
		}, nil
	}

	return &pb.StopContainerResponse{
		Success: true,
		Message: "Container stopped successfully",
	}, nil
}

// RestartContainer restarts a container
func (s *ContainerService) RestartContainer(ctx context.Context, req *pb.RestartContainerRequest) (*pb.RestartContainerResponse, error) {
	log.L.Debugf("Restarting container: %s", req.ContainerId)

	// Convert request to nerdctl options
	restartOpt, err := s.convertRestartRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to convert restart request: %w", err)
	}

	// Create containerd client
	client, cancel, err := s.createClient(restartOpt.GOption)
	if err != nil {
		return nil, fmt.Errorf("failed to create containerd client: %w", err)
	}
	defer cancel()

	// Restart container using nerdctl's restart function
	err = container.Restart(ctx, client, []string{req.ContainerId}, restartOpt)
	if err != nil {
		return &pb.RestartContainerResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to restart container: %v", err),
		}, nil
	}

	return &pb.RestartContainerResponse{
		Success: true,
		Message: "Container restarted successfully",
	}, nil
}

// RemoveContainer removes a container
func (s *ContainerService) RemoveContainer(ctx context.Context, req *pb.RemoveContainerRequest) (*pb.RemoveContainerResponse, error) {
	log.L.Debugf("Removing container: %s", req.ContainerId)

	// Convert request to nerdctl options
	removeOpt, err := s.convertRemoveRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to convert remove request: %w", err)
	}

	// Create containerd client
	client, cancel, err := s.createClient(removeOpt.GOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create containerd client: %w", err)
	}
	defer cancel()

	// Remove container using nerdctl's remove function
	err = container.Remove(ctx, client, []string{req.ContainerId}, removeOpt)
	if err != nil {
		return &pb.RemoveContainerResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to remove container: %v", err),
		}, nil
	}

	return &pb.RemoveContainerResponse{
		Success: true,
		Message: "Container removed successfully",
	}, nil
}

// Helper methods for converting protobuf requests to nerdctl types

func (s *ContainerService) createClient(gOptions types.GlobalCommandOptions) (*containerd.Client, func(), error) {
	client, _, cancel, err := clientutil.NewClient(context.Background(), gOptions.Namespace, gOptions.Address)
	if err != nil {
		return nil, nil, err
	}
	// Return client and cancel function
	return client, cancel, nil
}

func (s *ContainerService) convertCreateRequest(req *pb.CreateContainerRequest) (types.ContainerCreateOptions, types.NetworkOptions, error) {
	opt := types.ContainerCreateOptions{
		Stdout: io.Discard, // TODO: Configure properly
		Stderr: io.Discard, // TODO: Configure properly
	}

	// Set global options with sane defaults
	opt.GOptions = types.GlobalCommandOptions{
		Address:      "/run/containerd/containerd.sock",
		Namespace:    "default",
		DataRoot:     "/var/lib/nerdctl",
		CgroupManager: "systemd", 
	}

	// Initialize network options separately 
	netOpts := types.NetworkOptions{
		Hostname: "localhost",
	}

	// Configure container options
	opt.Platform = req.Platform
	
	// Basic container configuration
	if req.Config != nil {
		opt.Restart = "no" // Default restart policy
		opt.StopSignal = req.Config.StopSignal
		if req.Config.StopTimeout != 0 {
			opt.StopTimeout = int(req.Config.StopTimeout)
		}

		// Environment variables
		if req.Config.Env != nil {
			opt.Env = make([]string, 0, len(req.Config.Env))
			for k, v := range req.Config.Env {
				opt.Env = append(opt.Env, fmt.Sprintf("%s=%s", k, v))
			}
		}

		// Labels
		if req.Config.Labels != nil {
			opt.Label = make([]string, 0, len(req.Config.Labels))
			for k, v := range req.Config.Labels {
				opt.Label = append(opt.Label, fmt.Sprintf("%s=%s", k, v))
			}
		}

		// Working directory
		if req.Config.WorkingDir != "" {
			opt.Workdir = req.Config.WorkingDir
		}

		// User
		if req.Config.User != "" {
			opt.User = req.Config.User
		}

		// Hostname (set in network options)
		if req.Config.Hostname != "" {
			netOpts.Hostname = req.Config.Hostname
		}
	}

	// Host configuration
	if req.HostConfig != nil {
		opt.Rm = req.HostConfig.AutoRemove
		opt.Privileged = req.HostConfig.Privileged
		opt.ReadOnly = req.HostConfig.ReadOnlyRootfs

		// Resource limits
		if req.HostConfig.Memory > 0 {
			opt.Memory = fmt.Sprintf("%d", req.HostConfig.Memory)
		}
		
		if req.HostConfig.Cpus != "" {
			var err error
			opt.CPUs, err = strconv.ParseFloat(req.HostConfig.Cpus, 64)
			if err != nil {
				return opt, netOpts, fmt.Errorf("invalid CPUs value: %w", err)
			}
		}

		if req.HostConfig.CpuShares > 0 {
			opt.CPUShares = uint64(req.HostConfig.CpuShares)
		}

		// Networking
		if req.HostConfig.NetworkMode != "" {
			netOpts.NetworkSlice = []string{req.HostConfig.NetworkMode}
		}

		// Volume mounts
		if req.HostConfig.Binds != nil {
			opt.Volume = req.HostConfig.Binds
		}

		// Security options
		if req.HostConfig.CapAdd != nil {
			opt.CapAdd = req.HostConfig.CapAdd
		}
		if req.HostConfig.CapDrop != nil {
			opt.CapDrop = req.HostConfig.CapDrop
		}
		if req.HostConfig.SecurityOpt != nil {
			opt.SecurityOpt = req.HostConfig.SecurityOpt
		}

		// Runtime
		if req.HostConfig.Runtime != "" {
			opt.Runtime = req.HostConfig.Runtime
		}
	}

	// Set container name
	if req.Name != "" {
		opt.Name = req.Name
	}

	return opt, netOpts, nil
}

func (s *ContainerService) convertStartRequest(req *pb.StartContainerRequest) (types.ContainerStartOptions, error) {
	opt := types.ContainerStartOptions{
		Stdout: io.Discard,
	}

	// Set global options
	opt.GOptions = types.GlobalCommandOptions{
		Address:   "/run/containerd/containerd.sock",
		Namespace: "default",
		DataRoot:  "/var/lib/nerdctl",
	}

	// Note: DetachKeys is not available in ExecContainerRequest
	// This field would need to be added to the proto if needed

	return opt, nil
}

func (s *ContainerService) convertStopRequest(req *pb.StopContainerRequest) (types.ContainerStopOptions, error) {
	opt := types.ContainerStopOptions{
		Stdout: io.Discard,
		Stderr: io.Discard,
	}

	// Set global options
	opt.GOptions = types.GlobalCommandOptions{
		Address:   "/run/containerd/containerd.sock",
		Namespace: "default", 
		DataRoot:  "/var/lib/nerdctl",
	}

	// Set timeout if provided
	if req.Timeout > 0 {
		timeout := time.Duration(req.Timeout) * time.Second
		opt.Timeout = &timeout
	}

	return opt, nil
}

func (s *ContainerService) convertRestartRequest(req *pb.RestartContainerRequest) (types.ContainerRestartOptions, error) {
	opt := types.ContainerRestartOptions{
		Stdout: io.Discard,
	}

	// Set global options
	opt.GOption = types.GlobalCommandOptions{
		Address:   "/run/containerd/containerd.sock",
		Namespace: "default",
		DataRoot:  "/var/lib/nerdctl",
	}

	// Set timeout if provided
	if req.Timeout > 0 {
		timeout := time.Duration(req.Timeout) * time.Second 
		opt.Timeout = &timeout
	}

	return opt, nil
}

func (s *ContainerService) convertRemoveRequest(req *pb.RemoveContainerRequest) (types.ContainerRemoveOptions, error) {
	opt := types.ContainerRemoveOptions{
		Stdout: io.Discard,
	}

	// Set global options
	opt.GOptions = types.GlobalCommandOptions{
		Address:   "/run/containerd/containerd.sock",
		Namespace: "default",
		DataRoot:  "/var/lib/nerdctl",
	}

	// Set remove options
	opt.Force = req.Force
	opt.Volumes = req.Volumes

	return opt, nil
}

// TODO: Implement the remaining methods for container operations
// These methods will follow the same pattern as above

func (s *ContainerService) RunContainer(ctx context.Context, req *pb.RunContainerRequest) (*pb.RunContainerResponse, error) {
	log.L.Debugf("Running container with image: %s", req.Config.Image)

	// Convert protobuf request to nerdctl options
	createOpt, netOpts, err := s.convertRunRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to convert run request: %w", err)
	}

	// Create containerd client
	client, cancel, err := s.createClient(createOpt.GOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create containerd client: %w", err)
	}
	defer cancel()

	// Setup network manager
	netManager, err := containerutil.NewNetworkingOptionsManager(createOpt.GOptions, netOpts, client)
	if err != nil {
		return nil, fmt.Errorf("failed to setup network manager: %w", err)
	}

	// Prepare arguments
	args := []string{req.Config.Image}
	args = append(args, req.Config.Command...)

	// Create container using nerdctl's create function
	c, gc, err := container.Create(ctx, client, args, netManager, createOpt)
	if err != nil {
		if gc != nil {
			gc()
		}
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	id := c.ID()

	// Start the container using nerdctl taskutil
	task, err := taskutil.NewTask(ctx, client, c, createOpt.Attach, createOpt.Interactive, createOpt.TTY, createOpt.Detach,
		nil, "", createOpt.DetachKeys, createOpt.GOptions.Namespace, make(chan struct{}))
	if err != nil {
		return nil, fmt.Errorf("failed to create task: %w", err)
	}

	if err := task.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start task: %w", err)
	}

	// Return response
	return &pb.RunContainerResponse{
		Id:       id,
		ExitCode: 0,   // For detached mode, exit code is 0
		Output:   "", // For detached mode, no output
	}, nil
}

func (s *ContainerService) ExecContainer(ctx context.Context, req *pb.ExecContainerRequest) (*pb.ExecContainerResponse, error) {
	log.L.Debugf("Executing command in container: %s", req.ContainerId)

	// Convert request to nerdctl options
	execOpt, err := s.convertExecRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to convert exec request: %w", err)
	}

	// Create containerd client
	client, cancel, err := s.createClient(execOpt.GOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create containerd client: %w", err)
	}
	defer cancel()

	// Prepare arguments for exec
	args := []string{req.ContainerId}
	args = append(args, req.Command...)

	// Execute command using nerdctl's exec function
	err = container.Exec(ctx, client, args, execOpt)
	if err != nil {
		return &pb.ExecContainerResponse{
			ExecId:   "",
			ExitCode: 1,
			Output:   fmt.Sprintf("Failed to execute command: %v", err),
		}, nil
	}

	return &pb.ExecContainerResponse{
		ExecId:   "exec-" + req.ContainerId, // Simple exec ID
		ExitCode: 0,
		Output:   "Command executed successfully",
	}, nil
}

func (s *ContainerService) PauseContainer(ctx context.Context, req *pb.PauseContainerRequest) (*pb.PauseContainerResponse, error) {
	log.L.Debugf("Pausing container: %s", req.ContainerId)

	// Convert request to nerdctl options
	pauseOpt, err := s.convertPauseRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to convert pause request: %w", err)
	}

	// Create containerd client
	client, cancel, err := s.createClient(pauseOpt.GOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create containerd client: %w", err)
	}
	defer cancel()

	// Pause container using nerdctl's pause function
	err = container.Pause(ctx, client, []string{req.ContainerId}, pauseOpt)
	if err != nil {
		return &pb.PauseContainerResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to pause container: %v", err),
		}, nil
	}

	return &pb.PauseContainerResponse{
		Success: true,
		Message: "Container paused successfully",
	}, nil
}

func (s *ContainerService) UnpauseContainer(ctx context.Context, req *pb.UnpauseContainerRequest) (*pb.UnpauseContainerResponse, error) {
	log.L.Debugf("Unpausing container: %s", req.ContainerId)

	// Convert request to nerdctl options
	unpauseOpt, err := s.convertUnpauseRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to convert unpause request: %w", err)
	}

	// Create containerd client
	client, cancel, err := s.createClient(unpauseOpt.GOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create containerd client: %w", err)
	}
	defer cancel()

	// Unpause container using nerdctl's unpause function
	err = container.Unpause(ctx, client, []string{req.ContainerId}, unpauseOpt)
	if err != nil {
		return &pb.UnpauseContainerResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to unpause container: %v", err),
		}, nil
	}

	return &pb.UnpauseContainerResponse{
		Success: true,
		Message: "Container unpaused successfully",
	}, nil
}

func (s *ContainerService) ListContainers(ctx context.Context, req *pb.ListContainersRequest) (*pb.ListContainersResponse, error) {
	return nil, fmt.Errorf("ListContainers not implemented yet")
}

func (s *ContainerService) GetContainerLogs(req *pb.GetContainerLogsRequest, stream pb.ContainerService_GetContainerLogsServer) error {
	return fmt.Errorf("GetContainerLogs not implemented yet")
}

func (s *ContainerService) GetContainerStats(req *pb.GetContainerStatsRequest, stream pb.ContainerService_GetContainerStatsServer) error {
	return fmt.Errorf("GetContainerStats not implemented yet")
}

func (s *ContainerService) HealthCheck(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	return nil, fmt.Errorf("HealthCheck not implemented yet")
}

func (s *ContainerService) RenameContainer(ctx context.Context, req *pb.RenameContainerRequest) (*pb.RenameContainerResponse, error) {
	return nil, fmt.Errorf("RenameContainer not implemented yet")
}

func (s *ContainerService) KillContainer(ctx context.Context, req *pb.KillContainerRequest) (*pb.KillContainerResponse, error) {
	return nil, fmt.Errorf("KillContainer not implemented yet")
}

func (s *ContainerService) GetContainerPort(ctx context.Context, req *pb.GetContainerPortRequest) (*pb.GetContainerPortResponse, error) {
	return nil, fmt.Errorf("GetContainerPort not implemented yet")
}

func (s *ContainerService) WaitContainer(ctx context.Context, req *pb.WaitContainerRequest) (*pb.WaitContainerResponse, error) {
	return nil, fmt.Errorf("WaitContainer not implemented yet")
}

func (s *ContainerService) UpdateContainer(ctx context.Context, req *pb.UpdateContainerRequest) (*pb.UpdateContainerResponse, error) {
	return nil, fmt.Errorf("UpdateContainer not implemented yet")
}

func (s *ContainerService) InspectContainer(ctx context.Context, req *pb.InspectContainerRequest) (*pb.InspectContainerResponse, error) {
	return nil, fmt.Errorf("InspectContainer not implemented yet")
}

// Additional converter functions for runtime control methods

func (s *ContainerService) convertRunRequest(req *pb.RunContainerRequest) (types.ContainerCreateOptions, types.NetworkOptions, error) {
	// The RunContainer request has the same structure as CreateContainer,
	// but we need to configure it for running (not just creating)
	opt := types.ContainerCreateOptions{
		Stdout: io.Discard, // TODO: Configure properly for interactive sessions
		Stderr: io.Discard, // TODO: Configure properly for interactive sessions
		Detach: true,       // Default to detached mode for gRPC
	}

	// Set global options with sane defaults
	opt.GOptions = types.GlobalCommandOptions{
		Address:       "/run/containerd/containerd.sock",
		Namespace:     "default",
		DataRoot:      "/var/lib/nerdctl",
		CgroupManager: "systemd",
	}

	// Initialize network options separately 
	netOpts := types.NetworkOptions{
		Hostname: "localhost",
	}

	// Configure container options
	opt.Platform = req.Platform
	
	// Basic container configuration
	if req.Config != nil {
		opt.Restart = "no" // Default restart policy
		opt.StopSignal = req.Config.StopSignal
		if req.Config.StopTimeout != 0 {
			opt.StopTimeout = int(req.Config.StopTimeout)
		}

		// Environment variables
		if req.Config.Env != nil {
			opt.Env = make([]string, 0, len(req.Config.Env))
			for k, v := range req.Config.Env {
				opt.Env = append(opt.Env, fmt.Sprintf("%s=%s", k, v))
			}
		}

		// Labels
		if req.Config.Labels != nil {
			opt.Label = make([]string, 0, len(req.Config.Labels))
			for k, v := range req.Config.Labels {
				opt.Label = append(opt.Label, fmt.Sprintf("%s=%s", k, v))
			}
		}

		// Working directory
		if req.Config.WorkingDir != "" {
			opt.Workdir = req.Config.WorkingDir
		}

		// User
		if req.Config.User != "" {
			opt.User = req.Config.User
		}

		// Hostname (set in network options)
		if req.Config.Hostname != "" {
			netOpts.Hostname = req.Config.Hostname
		}
	}

	// Host configuration
	if req.HostConfig != nil {
		opt.Rm = req.HostConfig.AutoRemove
		opt.Privileged = req.HostConfig.Privileged
		opt.ReadOnly = req.HostConfig.ReadOnlyRootfs

		// Resource limits
		if req.HostConfig.Memory > 0 {
			opt.Memory = fmt.Sprintf("%d", req.HostConfig.Memory)
		}
		
		if req.HostConfig.Cpus != "" {
			var err error
			opt.CPUs, err = strconv.ParseFloat(req.HostConfig.Cpus, 64)
			if err != nil {
				return opt, netOpts, fmt.Errorf("invalid CPUs value: %w", err)
			}
		}

		if req.HostConfig.CpuShares > 0 {
			opt.CPUShares = uint64(req.HostConfig.CpuShares)
		}

		// Networking
		if req.HostConfig.NetworkMode != "" {
			netOpts.NetworkSlice = []string{req.HostConfig.NetworkMode}
		}

		// Volume mounts
		if req.HostConfig.Binds != nil {
			opt.Volume = req.HostConfig.Binds
		}

		// Security options
		if req.HostConfig.CapAdd != nil {
			opt.CapAdd = req.HostConfig.CapAdd
		}
		if req.HostConfig.CapDrop != nil {
			opt.CapDrop = req.HostConfig.CapDrop
		}
		if req.HostConfig.SecurityOpt != nil {
			opt.SecurityOpt = req.HostConfig.SecurityOpt
		}

		// Runtime
		if req.HostConfig.Runtime != "" {
			opt.Runtime = req.HostConfig.Runtime
		}
	}

	// Set container name
	if req.Name != "" {
		opt.Name = req.Name
	}

	return opt, netOpts, nil
}

func (s *ContainerService) convertExecRequest(req *pb.ExecContainerRequest) (types.ContainerExecOptions, error) {
	opt := types.ContainerExecOptions{
		TTY:         req.Tty,
		Interactive: req.Interactive,
		Detach:      req.Detach,
	}

	// Set global options
	opt.GOptions = types.GlobalCommandOptions{
		Address:   "/run/containerd/containerd.sock",
		Namespace: "default",
		DataRoot:  "/var/lib/nerdctl",
	}

	// Set user if provided
	if req.User != "" {
		opt.User = req.User
	}

	// Set working directory if provided
	if req.Workdir != "" {
		opt.Workdir = req.Workdir
	}

	// Set environment variables
	if req.Env != nil {
		opt.Env = req.Env
	}

	// Note: DetachKeys field is not available in this version

	return opt, nil
}

func (s *ContainerService) convertPauseRequest(req *pb.PauseContainerRequest) (types.ContainerPauseOptions, error) {
	opt := types.ContainerPauseOptions{
		Stdout: io.Discard,
	}

	// Set global options
	opt.GOptions = types.GlobalCommandOptions{
		Address:   "/run/containerd/containerd.sock",
		Namespace: "default", 
		DataRoot:  "/var/lib/nerdctl",
	}

	return opt, nil
}

func (s *ContainerService) convertUnpauseRequest(req *pb.UnpauseContainerRequest) (types.ContainerUnpauseOptions, error) {
	opt := types.ContainerUnpauseOptions{
		Stdout: io.Discard,
	}

	// Set global options
	opt.GOptions = types.GlobalCommandOptions{
		Address:   "/run/containerd/containerd.sock",
		Namespace: "default", 
		DataRoot:  "/var/lib/nerdctl",
	}

	return opt, nil
}