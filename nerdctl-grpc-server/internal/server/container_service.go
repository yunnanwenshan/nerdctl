package server

import (
	"context"
	"fmt"
	"io"

	"github.com/containerd/nerdctl-grpc-server/internal/interfaces"
	"github.com/containerd/nerdctl-grpc-server/pkg/types"
	pb "github.com/containerd/nerdctl-grpc-server/api/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ContainerServiceServer implements the gRPC ContainerService using abstract interfaces
// This demonstrates the decoupled architecture - the gRPC service doesn't know about
// specific nerdctl versions, it only uses the abstract ContainerManager interface
type ContainerServiceServer struct {
	pb.UnimplementedContainerServiceServer
	containerManager interfaces.ContainerManager
}

// NewContainerServiceServer creates a new ContainerServiceServer
func NewContainerServiceServer(containerManager interfaces.ContainerManager) *ContainerServiceServer {
	return &ContainerServiceServer{
		containerManager: containerManager,
	}
}

// CreateContainer creates a new container
func (s *ContainerServiceServer) CreateContainer(ctx context.Context, req *pb.CreateContainerRequest) (*pb.CreateContainerResponse, error) {
	// Convert gRPC request to internal types
	internalReq := &types.CreateContainerRequest{
		Image:      req.Image,
		Name:       req.Name,
		Command:    req.Command,
		Args:       req.Args,
		Env:        req.Env,
		WorkingDir: req.WorkingDir,
		Labels:     req.Labels,
		Namespace:  req.Namespace,
		TTY:        req.Tty,
		Stdin:      req.Stdin,
		Init:       req.Init,
		InitBinary: req.InitBinary,
		StopSignal: req.StopSignal,
		StopTimeout: req.StopTimeout,
	}

	// Convert network configuration
	if req.NetworkConfig != nil {
		internalReq.NetworkConfig = convertNetworkConfig(req.NetworkConfig)
	}

	// Convert volume mounts
	for _, mount := range req.Mounts {
		internalReq.Mounts = append(internalReq.Mounts, convertVolumeMount(mount))
	}

	// Convert resource limits
	if req.Resources != nil {
		internalReq.Resources = convertResourceLimits(req.Resources)
	}

	// Convert security options
	if req.Security != nil {
		internalReq.Security = convertSecurityOptions(req.Security)
	}

	// Call the adapter through the abstract interface
	resp, err := s.containerManager.CreateContainer(ctx, internalReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create container: %v", err)
	}

	// Convert response back to gRPC format
	return &pb.CreateContainerResponse{
		ContainerId: resp.ContainerID,
		Warnings:    resp.Warnings,
	}, nil
}

// StartContainer starts an existing container
func (s *ContainerServiceServer) StartContainer(ctx context.Context, req *pb.StartContainerRequest) (*pb.StartContainerResponse, error) {
	internalReq := &types.StartContainerRequest{
		ContainerID: req.ContainerId,
		Namespace:   req.Namespace,
		Attach:      req.Attach,
		DetachKeys:  req.DetachKeys,
	}

	resp, err := s.containerManager.StartContainer(ctx, internalReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to start container: %v", err)
	}

	return &pb.StartContainerResponse{
		ContainerId: resp.ContainerID,
	}, nil
}

// StopContainer stops a running container
func (s *ContainerServiceServer) StopContainer(ctx context.Context, req *pb.StopContainerRequest) (*pb.StopContainerResponse, error) {
	internalReq := &types.StopContainerRequest{
		ContainerID: req.ContainerId,
		Namespace:   req.Namespace,
		Timeout:     req.Timeout,
		Signal:      req.Signal,
	}

	resp, err := s.containerManager.StopContainer(ctx, internalReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to stop container: %v", err)
	}

	return &pb.StopContainerResponse{
		ContainerId: resp.ContainerID,
	}, nil
}

// RestartContainer restarts a container
func (s *ContainerServiceServer) RestartContainer(ctx context.Context, req *pb.RestartContainerRequest) (*pb.RestartContainerResponse, error) {
	internalReq := &types.RestartContainerRequest{
		ContainerID: req.ContainerId,
		Namespace:   req.Namespace,
		Timeout:     req.Timeout,
	}

	resp, err := s.containerManager.RestartContainer(ctx, internalReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to restart container: %v", err)
	}

	return &pb.RestartContainerResponse{
		ContainerId: resp.ContainerID,
	}, nil
}

// RemoveContainer removes a container
func (s *ContainerServiceServer) RemoveContainer(ctx context.Context, req *pb.RemoveContainerRequest) (*pb.RemoveContainerResponse, error) {
	internalReq := &types.RemoveContainerRequest{
		ContainerID: req.ContainerId,
		Namespace:   req.Namespace,
		Force:       req.Force,
		Volumes:     req.Volumes,
		Link:        req.Link,
	}

	resp, err := s.containerManager.RemoveContainer(ctx, internalReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to remove container: %v", err)
	}

	return &pb.RemoveContainerResponse{
		ContainerId: resp.ContainerID,
	}, nil
}

// ListContainers lists containers
func (s *ContainerServiceServer) ListContainers(ctx context.Context, req *pb.ListContainersRequest) (*pb.ListContainersResponse, error) {
	internalReq := &types.ListContainersRequest{
		All:       req.All,
		Latest:    req.Latest,
		Last:      req.Last,
		Size:      req.Size,
		Namespace: req.Namespace,
		Names:     req.Names,
		Filters:   req.Filters,
		Format:    req.Format,
	}

	resp, err := s.containerManager.ListContainers(ctx, internalReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list containers: %v", err)
	}

	// Convert internal response to gRPC format
	var containers []*pb.ContainerInfo
	for _, container := range resp.Containers {
		containers = append(containers, convertContainerInfo(&container))
	}

	return &pb.ListContainersResponse{
		Containers: containers,
	}, nil
}

// InspectContainer inspects a container
func (s *ContainerServiceServer) InspectContainer(ctx context.Context, req *pb.InspectContainerRequest) (*pb.InspectContainerResponse, error) {
	internalReq := &types.InspectContainerRequest{
		ContainerID: req.ContainerId,
		Namespace:   req.Namespace,
		Size:        req.Size,
	}

	resp, err := s.containerManager.InspectContainer(ctx, internalReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to inspect container: %v", err)
	}

	return &pb.InspectContainerResponse{
		Container: convertContainerInfo(resp.Container),
	}, nil
}

// RunContainer runs a container (create + start)
func (s *ContainerServiceServer) RunContainer(ctx context.Context, req *pb.RunContainerRequest) (*pb.RunContainerResponse, error) {
	internalReq := &types.RunContainerRequest{
		Image:       req.Image,
		Name:        req.Name,
		Command:     req.Command,
		Args:        req.Args,
		Env:         req.Env,
		WorkingDir:  req.WorkingDir,
		Labels:      req.Labels,
		Namespace:   req.Namespace,
		Detached:    req.Detached,
		TTY:         req.Tty,
		Interactive: req.Interactive,
		Remove:      req.Remove,
		Pull:        req.Pull,
	}

	// Convert network configuration
	if req.NetworkConfig != nil {
		internalReq.NetworkConfig = convertNetworkConfig(req.NetworkConfig)
	}

	// Convert volume mounts
	for _, mount := range req.Mounts {
		internalReq.Mounts = append(internalReq.Mounts, convertVolumeMount(mount))
	}

	// Convert resource limits
	if req.Resources != nil {
		internalReq.Resources = convertResourceLimits(req.Resources)
	}

	// Convert security options
	if req.Security != nil {
		internalReq.Security = convertSecurityOptions(req.Security)
	}

	resp, err := s.containerManager.RunContainer(ctx, internalReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to run container: %v", err)
	}

	return &pb.RunContainerResponse{
		ContainerId: resp.ContainerID,
		ExitCode:    resp.ExitCode,
		Output:      resp.Output,
		Error:       resp.Error,
	}, nil
}

// GetContainerLogs streams container logs
func (s *ContainerServiceServer) GetContainerLogs(req *pb.GetContainerLogsRequest, stream pb.ContainerService_GetContainerLogsServer) error {
	ctx := stream.Context()
	
	internalReq := &types.GetContainerLogsRequest{
		ContainerID: req.ContainerId,
		Namespace:   req.Namespace,
		Follow:      req.Follow,
		Stdout:      req.Stdout,
		Stderr:      req.Stderr,
		Since:       req.Since,
		Until:       req.Until,
		Timestamps:  req.Timestamps,
		Tail:        req.Tail,
	}

	logChan, err := s.containerManager.GetContainerLogs(ctx, internalReq)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to get container logs: %v", err)
	}

	// Stream logs to client
	for logEntry := range logChan {
		pbLogEntry := &pb.LogEntry{
			Timestamp: timestamppb.New(logEntry.Timestamp),
			Source:    logEntry.Source,
			Content:   logEntry.Content,
		}

		if err := stream.Send(pbLogEntry); err != nil {
			return status.Errorf(codes.Internal, "failed to send log entry: %v", err)
		}
	}

	return nil
}

// Helper functions to convert between gRPC and internal types

func convertNetworkConfig(netConfig *pb.NetworkConfig) types.NetworkConfig {
	var ports []types.PortMapping
	for _, port := range netConfig.Ports {
		ports = append(ports, types.PortMapping{
			HostIP:             port.HostIp,
			HostPort:           port.HostPort,
			ContainerPort:      port.ContainerPort,
			Protocol:           port.Protocol,
			HostPortEnd:        port.HostPortEnd,
			ContainerPortEnd:   port.ContainerPortEnd,
		})
	}

	return types.NetworkConfig{
		Mode:        netConfig.Mode,
		Networks:    netConfig.Networks,
		Ports:       ports,
		DNSServers:  netConfig.DnsServers,
		DNSSearch:   netConfig.DnsSearch,
		DNSOptions:  netConfig.DnsOptions,
		Hostname:    netConfig.Hostname,
		Domainname:  netConfig.Domainname,
		IP:          netConfig.Ip,
		IP6:         netConfig.Ip6,
		MacAddress:  netConfig.MacAddress,
		ExtraHosts:  netConfig.ExtraHosts,
	}
}

func convertVolumeMount(mount *pb.VolumeMount) types.VolumeMount {
	return types.VolumeMount{
		Source:          mount.Source,
		Destination:     mount.Destination,
		Type:            mount.Type,
		ReadOnly:        mount.ReadOnly,
		BindPropagation: mount.BindPropagation,
		TmpfsMode:       mount.TmpfsMode,
		TmpfsSize:       mount.TmpfsSize,
		MountOptions:    mount.MountOptions,
	}
}

func convertResourceLimits(resources *pb.ResourceLimits) types.ResourceLimits {
	var ulimits []types.UlimitSpec
	for _, ulimit := range resources.Ulimits {
		ulimits = append(ulimits, types.UlimitSpec{
			Name: ulimit.Name,
			Soft: ulimit.Soft,
			Hard: ulimit.Hard,
		})
	}

	return types.ResourceLimits{
		MemoryBytes:     resources.MemoryBytes,
		MemorySwapBytes: resources.MemorySwapBytes,
		CPUQuota:        resources.CpuQuota,
		CPUPeriod:       resources.CpuPeriod,
		CPUShares:       resources.CpuShares,
		CPUSetCPUs:      resources.CpusetCpus,
		CPUSetMems:      resources.CpusetMems,
		BlkioWeight:     resources.BlkioWeight,
		PidsLimit:       resources.PidsLimit,
		Ulimits:         ulimits,
	}
}

func convertSecurityOptions(security *pb.SecurityOptions) types.SecurityOptions {
	return types.SecurityOptions{
		User:            security.User,
		Groups:          security.Groups,
		CapAdd:          security.CapAdd,
		CapDrop:         security.CapDrop,
		Privileged:      security.Privileged,
		SecurityOpts:    security.SecurityOpts,
		NoNewPrivileges: security.NoNewPrivileges,
		UsernsMode:      security.UsernsMode,
	}
}

func convertContainerInfo(container *types.ContainerInfo) *pb.ContainerInfo {
	// Convert ports
	var ports []*pb.PortMapping
	for _, port := range container.NetworkConfig.Ports {
		ports = append(ports, &pb.PortMapping{
			HostIp:             port.HostIP,
			HostPort:           port.HostPort,
			ContainerPort:      port.ContainerPort,
			Protocol:           port.Protocol,
			HostPortEnd:        port.HostPortEnd,
			ContainerPortEnd:   port.ContainerPortEnd,
		})
	}

	// Convert mounts
	var mounts []*pb.VolumeMount
	for _, mount := range container.Mounts {
		mounts = append(mounts, &pb.VolumeMount{
			Source:          mount.Source,
			Destination:     mount.Destination,
			Type:            mount.Type,
			ReadOnly:        mount.ReadOnly,
			BindPropagation: mount.BindPropagation,
			TmpfsMode:       mount.TmpfsMode,
			TmpfsSize:       mount.TmpfsSize,
			MountOptions:    mount.MountOptions,
		})
	}

	return &pb.ContainerInfo{
		Id:          container.ID,
		ShortId:     container.ShortID,
		Name:        container.Name,
		Image:       container.Image,
		ImageId:     container.ImageID,
		Platform:    container.Platform,
		Status:      pb.ContainerStatus(container.Status),
		ExitCode:    container.ExitCode,
		Created:     timestamppb.New(container.Created),
		Started:     timestamppb.New(container.Started),
		Finished:    timestamppb.New(container.Finished),
		Command:     container.Command,
		Labels:      container.Labels,
		NetworkConfig: &pb.NetworkConfig{
			Mode:        container.NetworkConfig.Mode,
			Networks:    container.NetworkConfig.Networks,
			Ports:       ports,
			DnsServers:  container.NetworkConfig.DNSServers,
			DnsSearch:   container.NetworkConfig.DNSSearch,
			DnsOptions:  container.NetworkConfig.DNSOptions,
			Hostname:    container.NetworkConfig.Hostname,
			Domainname:  container.NetworkConfig.Domainname,
			Ip:          container.NetworkConfig.IP,
			Ip6:         container.NetworkConfig.IP6,
			MacAddress:  container.NetworkConfig.MacAddress,
			ExtraHosts:  container.NetworkConfig.ExtraHosts,
		},
		Mounts: mounts,
	}
}