package server

import (
	"context"

	"github.com/containerd/nerdctl-grpc-server/internal/interfaces"
	"github.com/containerd/nerdctl-grpc-server/pkg/types"
	pb "github.com/containerd/nerdctl-grpc-server/api/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// NetworkServiceServer implements the gRPC NetworkService using abstract interfaces
type NetworkServiceServer struct {
	pb.UnimplementedNetworkServiceServer
	networkManager interfaces.NetworkManager
}

// NewNetworkServiceServer creates a new NetworkServiceServer
func NewNetworkServiceServer(networkManager interfaces.NetworkManager) *NetworkServiceServer {
	return &NetworkServiceServer{
		networkManager: networkManager,
	}
}

// CreateNetwork creates a new network
func (s *NetworkServiceServer) CreateNetwork(ctx context.Context, req *pb.CreateNetworkRequest) (*pb.CreateNetworkResponse, error) {
	internalReq := &types.CreateNetworkRequest{
		Name:      req.Name,
		Driver:    req.Driver,
		Options:   req.Options,
		Labels:    req.Labels,
		Internal:  req.Internal,
		EnableIPv6: req.EnableIpv6,
		Subnet:    req.Subnet,
		IPRange:   req.IpRange,
		Gateway:   req.Gateway,
		Namespace: req.Namespace,
	}

	resp, err := s.networkManager.CreateNetwork(ctx, internalReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create network: %v", err)
	}

	return &pb.CreateNetworkResponse{
		NetworkId: resp.NetworkID,
		Warnings:  resp.Warnings,
	}, nil
}

// RemoveNetwork removes a network
func (s *NetworkServiceServer) RemoveNetwork(ctx context.Context, req *pb.RemoveNetworkRequest) (*pb.RemoveNetworkResponse, error) {
	internalReq := &types.RemoveNetworkRequest{
		NetworkName: req.NetworkName,
		Force:       req.Force,
		Namespace:   req.Namespace,
	}

	resp, err := s.networkManager.RemoveNetwork(ctx, internalReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to remove network: %v", err)
	}

	return &pb.RemoveNetworkResponse{
		NetworkId: resp.NetworkID,
	}, nil
}

// ListNetworks lists networks
func (s *NetworkServiceServer) ListNetworks(ctx context.Context, req *pb.ListNetworksRequest) (*pb.ListNetworksResponse, error) {
	internalReq := &types.ListNetworksRequest{
		Quiet:     req.Quiet,
		Filters:   req.Filters,
		Format:    req.Format,
		Namespace: req.Namespace,
	}

	resp, err := s.networkManager.ListNetworks(ctx, internalReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list networks: %v", err)
	}

	// Convert internal response to gRPC format
	var networks []*pb.NetworkInfo
	for _, network := range resp.Networks {
		networks = append(networks, convertNetworkInfo(&network))
	}

	return &pb.ListNetworksResponse{
		Networks: networks,
	}, nil
}

// InspectNetwork inspects a network
func (s *NetworkServiceServer) InspectNetwork(ctx context.Context, req *pb.InspectNetworkRequest) (*pb.InspectNetworkResponse, error) {
	internalReq := &types.InspectNetworkRequest{
		NetworkName: req.NetworkName,
		Namespace:   req.Namespace,
	}

	resp, err := s.networkManager.InspectNetwork(ctx, internalReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to inspect network: %v", err)
	}

	return &pb.InspectNetworkResponse{
		Network: convertNetworkInfo(resp.Network),
	}, nil
}

// ConnectContainerToNetwork connects a container to a network
func (s *NetworkServiceServer) ConnectContainerToNetwork(ctx context.Context, req *pb.ConnectContainerToNetworkRequest) (*pb.ConnectContainerToNetworkResponse, error) {
	internalReq := &types.ConnectContainerToNetworkRequest{
		NetworkName:   req.NetworkName,
		ContainerName: req.ContainerName,
		Aliases:       req.Aliases,
		IPAddress:     req.IpAddress,
		IPv6Address:   req.Ipv6Address,
		MacAddress:    req.MacAddress,
		Links:         req.Links,
		Namespace:     req.Namespace,
	}

	resp, err := s.networkManager.ConnectContainerToNetwork(ctx, internalReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to connect container to network: %v", err)
	}

	return &pb.ConnectContainerToNetworkResponse{
		Success: resp.Success,
	}, nil
}

// DisconnectContainerFromNetwork disconnects a container from a network
func (s *NetworkServiceServer) DisconnectContainerFromNetwork(ctx context.Context, req *pb.DisconnectContainerFromNetworkRequest) (*pb.DisconnectContainerFromNetworkResponse, error) {
	internalReq := &types.DisconnectContainerFromNetworkRequest{
		NetworkName:   req.NetworkName,
		ContainerName: req.ContainerName,
		Force:         req.Force,
		Namespace:     req.Namespace,
	}

	resp, err := s.networkManager.DisconnectContainerFromNetwork(ctx, internalReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to disconnect container from network: %v", err)
	}

	return &pb.DisconnectContainerFromNetworkResponse{
		Success: resp.Success,
	}, nil
}

// PruneNetworks removes unused networks
func (s *NetworkServiceServer) PruneNetworks(ctx context.Context, req *pb.PruneNetworksRequest) (*pb.PruneNetworksResponse, error) {
	internalReq := &types.PruneNetworksRequest{
		Filters:   req.Filters,
		Namespace: req.Namespace,
	}

	resp, err := s.networkManager.PruneNetworks(ctx, internalReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to prune networks: %v", err)
	}

	return &pb.PruneNetworksResponse{
		DeletedNetworks: resp.DeletedNetworks,
		SpaceReclaimed:  resp.SpaceReclaimed,
	}, nil
}

// Helper function to convert network info
func convertNetworkInfo(network *types.NetworkInfo) *pb.NetworkInfo {
	var containers []*pb.NetworkContainer
	for _, container := range network.Containers {
		containers = append(containers, &pb.NetworkContainer{
			ContainerId: container.ContainerID,
			Name:        container.Name,
			EndpointId:  container.EndpointID,
			MacAddress:  container.MacAddress,
			IpAddress:   container.IPAddress,
			Ipv6Address: container.IPv6Address,
		})
	}

	var ipamConfig []*pb.IPAMConfig
	for _, config := range network.IPAM.Config {
		ipamConfig = append(ipamConfig, &pb.IPAMConfig{
			Subnet:     config.Subnet,
			IpRange:    config.IPRange,
			Gateway:    config.Gateway,
			AuxAddress: config.AuxAddress,
		})
	}

	return &pb.NetworkInfo{
		Id:         network.ID,
		Name:       network.Name,
		Driver:     network.Driver,
		Scope:      network.Scope,
		Internal:   network.Internal,
		EnableIpv6: network.EnableIPv6,
		Labels:     network.Labels,
		Options:    network.Options,
		Containers: containers,
		Ipam: &pb.IPAM{
			Driver:  network.IPAM.Driver,
			Options: network.IPAM.Options,
			Config:  ipamConfig,
		},
	}
}