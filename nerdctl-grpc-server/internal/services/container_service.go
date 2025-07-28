package services

import (
	"context"
	"fmt"

	pb "github.com/containerd/nerdctl-grpc-server/api/proto"
	"github.com/containerd/nerdctl-grpc-server/internal/interfaces"
	"github.com/containerd/nerdctl-grpc-server/pkg/types"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ContainerServiceServer implements the gRPC ContainerService interface
// It acts as a bridge between gRPC requests and the abstract ContainerManager interface
type ContainerServiceServer struct {
	pb.UnimplementedContainerServiceServer
	containerManager interfaces.ContainerManager
	logger          *logrus.Logger
}

// NewContainerServiceServer creates a new container service server
func NewContainerServiceServer(containerManager interfaces.ContainerManager) *ContainerServiceServer {
	return &ContainerServiceServer{
		containerManager: containerManager,
		logger:          logrus.New(),
	}
}

// CreateContainer creates a new container
func (s *ContainerServiceServer) CreateContainer(ctx context.Context, req *pb.CreateContainerRequest) (*pb.CreateContainerResponse, error) {
	s.logger.WithFields(logrus.Fields{
		"image": req.Image,
		"name":  req.Name,
	}).Info("Creating container")

	// Convert protobuf request to internal type
	internalReq := s.convertCreateContainerRequest(req)
	
	// Call the container manager
	resp, err := s.containerManager.CreateContainer(ctx, internalReq)
	if err != nil {
		s.logger.WithError(err).Error("Failed to create container")
		return nil, status.Errorf(codes.Internal, "failed to create container: %v", err)
	}

	// Convert internal response to protobuf response
	return &pb.CreateContainerResponse{
		ContainerId: resp.ContainerID,
		Warnings:    resp.Warnings,
	}, nil
}

// StartContainer starts an existing container
func (s *ContainerServiceServer) StartContainer(ctx context.Context, req *pb.StartContainerRequest) (*pb.StartContainerResponse, error) {
	s.logger.WithField("container_id", req.ContainerId).Info("Starting container")

	internalReq := &types.StartContainerRequest{
		ContainerID: req.ContainerId,
		Namespace:   req.Namespace,
		Attach:      req.Attach,
		DetachKeys:  req.DetachKeys,
	}

	resp, err := s.containerManager.StartContainer(ctx, internalReq)
	if err != nil {
		s.logger.WithError(err).Error("Failed to start container")
		return nil, status.Errorf(codes.Internal, "failed to start container: %v", err)
	}

	return &pb.StartContainerResponse{
		Started:  resp.Started,
		Warnings: resp.Warnings,
	}, nil
}

// StopContainer stops a running container
func (s *ContainerServiceServer) StopContainer(ctx context.Context, req *pb.StopContainerRequest) (*pb.StopContainerResponse, error) {
	s.logger.WithField("container_id", req.ContainerId).Info("Stopping container")

	internalReq := &types.StopContainerRequest{
		ContainerID: req.ContainerId,
		Namespace:   req.Namespace,
		Timeout:     req.Timeout,
	}

	resp, err := s.containerManager.StopContainer(ctx, internalReq)
	if err != nil {
		s.logger.WithError(err).Error("Failed to stop container")
		return nil, status.Errorf(codes.Internal, "failed to stop container: %v", err)
	}

	return &pb.StopContainerResponse{
		Stopped:  resp.Stopped,
		Warnings: resp.Warnings,
	}, nil
}

// RunContainer runs a container (create + start)
func (s *ContainerServiceServer) RunContainer(ctx context.Context, req *pb.RunContainerRequest) (*pb.RunContainerResponse, error) {
	s.logger.WithFields(logrus.Fields{
		"image":  req.Image,
		"detach": req.Detached,
	}).Info("Running container")

	internalReq := s.convertRunContainerRequest(req)

	resp, err := s.containerManager.RunContainer(ctx, internalReq)
	if err != nil {
		s.logger.WithError(err).Error("Failed to run container")
		return nil, status.Errorf(codes.Internal, "failed to run container: %v", err)
	}

	return &pb.RunContainerResponse{
		ContainerId: resp.ContainerID,
		ExitCode:    resp.ExitCode,
		Output:      resp.Output,
		Warnings:    resp.Warnings,
	}, nil
}

// ListContainers lists containers
func (s *ContainerServiceServer) ListContainers(ctx context.Context, req *pb.ListContainersRequest) (*pb.ListContainersResponse, error) {
	s.logger.WithField("all", req.All).Info("Listing containers")

	internalReq := &types.ListContainersRequest{
		All:       req.All,
		Latest:    req.Latest,
		Quiet:     req.Quiet,
		Size:      req.Size,
		Filters:   req.Filters,
		Format:    req.Format,
		Namespace: req.Namespace,
		Limit:     req.Limit,
	}

	resp, err := s.containerManager.ListContainers(ctx, internalReq)
	if err != nil {
		s.logger.WithError(err).Error("Failed to list containers")
		return nil, status.Errorf(codes.Internal, "failed to list containers: %v", err)
	}

	// Convert internal response to protobuf
	containers := make([]*pb.ContainerInfo, len(resp.Containers))
	for i, container := range resp.Containers {
		containers[i] = s.convertContainerInfo(container)
	}

	return &pb.ListContainersResponse{
		Containers: containers,
	}, nil
}

// InspectContainer inspects a container
func (s *ContainerServiceServer) InspectContainer(ctx context.Context, req *pb.InspectContainerRequest) (*pb.InspectContainerResponse, error) {
	s.logger.WithField("container_id", req.ContainerId).Info("Inspecting container")

	internalReq := &types.InspectContainerRequest{
		ContainerID: req.ContainerId,
		Namespace:   req.Namespace,
		Size:        req.Size,
		Format:      req.Format,
	}

	resp, err := s.containerManager.InspectContainer(ctx, internalReq)
	if err != nil {
		s.logger.WithError(err).Error("Failed to inspect container")
		return nil, status.Errorf(codes.Internal, "failed to inspect container: %v", err)
	}

	return &pb.InspectContainerResponse{
		Container: s.convertContainerInfo(resp.Container),
	}, nil
}

// RemoveContainer removes a container
func (s *ContainerServiceServer) RemoveContainer(ctx context.Context, req *pb.RemoveContainerRequest) (*pb.RemoveContainerResponse, error) {
	s.logger.WithField("container_id", req.ContainerId).Info("Removing container")

	internalReq := &types.RemoveContainerRequest{
		ContainerID: req.ContainerId,
		Namespace:   req.Namespace,
		Force:       req.Force,
		Volumes:     req.Volumes,
		Link:        req.Link,
	}

	resp, err := s.containerManager.RemoveContainer(ctx, internalReq)
	if err != nil {
		s.logger.WithError(err).Error("Failed to remove container")
		return nil, status.Errorf(codes.Internal, "failed to remove container: %v", err)
	}

	return &pb.RemoveContainerResponse{
		Removed:  resp.Removed,
		Warnings: resp.Warnings,
	}, nil
}

// Helper methods for converting between protobuf and internal types

func (s *ContainerServiceServer) convertCreateContainerRequest(req *pb.CreateContainerRequest) *types.CreateContainerRequest {
	internalReq := &types.CreateContainerRequest{
		Image:        req.Image,
		Name:         req.Name,
		Command:      req.Command,
		Args:         req.Args,
		Env:          req.Env,
		WorkingDir:   req.WorkingDir,
		Labels:       req.Labels,
		Namespace:    req.Namespace,
		TTY:          req.Tty,
		Stdin:        req.Stdin,
		Init:         req.Init,
		InitBinary:   req.InitBinary,
		CapAdd:       req.CapAdd,
		CapDrop:      req.CapDrop,
		Devices:      req.Device,
		CgroupParent: req.CgroupParent,
		IPCMode:      req.IpcMode,
		PidMode:      req.PidMode,
		UTSMode:      req.UtsMode,
		Sysctl:       req.Sysctl,
		Tmpfs:        req.Tmpfs,
		ShmSize:      req.ShmSize,
		StopSignal:   req.StopSignal,
		StopTimeout:  req.StopTimeout,
	}

	// Convert network config
	if req.NetworkConfig != nil {
		internalReq.NetworkConfig = s.convertNetworkConfig(req.NetworkConfig)
	}

	// Convert mounts
	for _, mount := range req.Mounts {
		internalReq.Mounts = append(internalReq.Mounts, s.convertVolumeMount(mount))
	}

	// Convert resources
	if req.Resources != nil {
		internalReq.Resources = s.convertResourceLimits(req.Resources)
	}

	// Convert security options
	if req.Security != nil {
		internalReq.Security = s.convertSecurityOptions(req.Security)
	}

	return internalReq
}

func (s *ContainerServiceServer) convertRunContainerRequest(req *pb.RunContainerRequest) *types.RunContainerRequest {
	createReq := s.convertCreateContainerRequest(&pb.CreateContainerRequest{
		Image:         req.Image,
		Name:          req.Name,
		Command:       req.Command,
		Args:          req.Args,
		Env:           req.Env,
		WorkingDir:    req.WorkingDir,
		Labels:        req.Labels,
		NetworkConfig: req.NetworkConfig,
		Mounts:        req.Mounts,
		Resources:     req.Resources,
		Security:      req.Security,
		Namespace:     req.Namespace,
		Tty:           req.Tty,
		Stdin:         req.Stdin,
		Init:          req.Init,
	})

	return &types.RunContainerRequest{
		CreateContainerRequest: createReq,
		Detach:                 req.Detached,
		Remove:                 req.Remove,
		Interactive:            req.Interactive,
		Quiet:                  req.Quiet,
		Pull:                   req.Pull,
	}
}

func (s *ContainerServiceServer) convertNetworkConfig(nc *pb.NetworkConfig) *types.NetworkConfig {
	if nc == nil {
		return nil
	}

	config := &types.NetworkConfig{
		Mode:        nc.Mode,
		Networks:    nc.Networks,
		DNSServers:  nc.DnsServers,
		DNSSearch:   nc.DnsSearch,
		DNSOptions:  nc.DnsOptions,
		Hostname:    nc.Hostname,
		DomainName:  nc.Domainname,
		IP:          nc.Ip,
		IP6:         nc.Ip6,
		MacAddress:  nc.MacAddress,
		ExtraHosts:  nc.ExtraHosts,
		NetworkMode: nc.NetworkMode,
	}

	for _, port := range nc.Ports {
		config.Ports = append(config.Ports, &types.PortMapping{
			HostIP:           port.HostIp,
			HostPort:         port.HostPort,
			ContainerPort:    port.ContainerPort,
			Protocol:         port.Protocol,
			HostPortEnd:      port.HostPortEnd,
			ContainerPortEnd: port.ContainerPortEnd,
		})
	}

	return config
}

func (s *ContainerServiceServer) convertVolumeMount(vm *pb.VolumeMount) *types.VolumeMount {
	if vm == nil {
		return nil
	}

	return &types.VolumeMount{
		Source:          vm.Source,
		Destination:     vm.Destination,
		Type:            vm.Type,
		ReadOnly:        vm.ReadOnly,
		BindPropagation: vm.BindPropagation,
		TmpfsMode:       vm.TmpfsMode,
		TmpfsSize:       vm.TmpfsSize,
		MountOptions:    vm.MountOptions,
	}
}

func (s *ContainerServiceServer) convertResourceLimits(rl *pb.ResourceLimits) *types.ResourceLimits {
	if rl == nil {
		return nil
	}

	limits := &types.ResourceLimits{
		MemoryBytes:       rl.MemoryBytes,
		MemorySwapBytes:   rl.MemorySwapBytes,
		CPUQuota:          rl.CpuQuota,
		CPUPeriod:         rl.CpuPeriod,
		CPUShares:         rl.CpuShares,
		CpusetCpus:        rl.CpusetCpus,
		CpusetMems:        rl.CpusetMems,
		BlkioWeight:       rl.BlkioWeight,
		KernelMemory:      rl.KernelMemory,
		MemoryReservation: rl.MemoryReservation,
		MemorySwappiness:  rl.MemorySwappiness,
		OOMKillDisable:    rl.OomKillDisable,
		PidsLimit:         rl.PidsLimit,
	}

	return limits
}

func (s *ContainerServiceServer) convertSecurityOptions(so *pb.SecurityOptions) *types.SecurityOptions {
	if so == nil {
		return nil
	}

	return &types.SecurityOptions{
		User:            so.User,
		Groups:          so.Groups,
		CapAdd:          so.CapAdd,
		CapDrop:         so.CapDrop,
		Privileged:      so.Privileged,
		SecurityOpt:     so.SecurityOpt,
		SecurityOpts:    so.SecurityOpts,
		NoNewPrivileges: so.NoNewPrivileges,
		UsernsMode:      so.UsernsMode,
	}
}

func (s *ContainerServiceServer) convertContainerInfo(ci *types.ContainerInfo) *pb.ContainerInfo {
	if ci == nil {
		return nil
	}

	info := &pb.ContainerInfo{
		Id:       ci.ID,
		ShortId:  ci.ShortID,
		Name:     ci.Name,
		Image:    ci.Image,
		ImageId:  ci.ImageID,
		Platform: ci.Platform,
		Status:   s.convertContainerStatus(ci.Status),
		ExitCode: ci.ExitCode,
		Command:  ci.Command,
		Labels:   ci.Labels,
	}

	// Convert timestamps
	if !ci.Created.IsZero() {
		info.Created = ci.Created.Unix()
	}
	if !ci.Started.IsZero() {
		info.Started = ci.Started.Unix()
	}
	if !ci.Finished.IsZero() {
		info.Finished = ci.Finished.Unix()
	}

	return info
}

func (s *ContainerServiceServer) convertContainerStatus(status types.ContainerStatus) pb.ContainerStatus {
	switch status {
	case types.ContainerStatusCreated:
		return pb.ContainerStatus_CONTAINER_STATUS_CREATED
	case types.ContainerStatusRunning:
		return pb.ContainerStatus_CONTAINER_STATUS_RUNNING
	case types.ContainerStatusPaused:
		return pb.ContainerStatus_CONTAINER_STATUS_PAUSED
	case types.ContainerStatusRestarting:
		return pb.ContainerStatus_CONTAINER_STATUS_RESTARTING
	case types.ContainerStatusRemoving:
		return pb.ContainerStatus_CONTAINER_STATUS_REMOVING
	case types.ContainerStatusExited:
		return pb.ContainerStatus_CONTAINER_STATUS_EXITED
	case types.ContainerStatusDead:
		return pb.ContainerStatus_CONTAINER_STATUS_DEAD
	default:
		return pb.ContainerStatus_CONTAINER_STATUS_UNSPECIFIED
	}
}