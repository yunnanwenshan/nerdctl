package services

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/containerd/nerdctl-grpc-server/internal/interfaces"
	pb "github.com/containerd/nerdctl-grpc-server/api/proto"
	"github.com/containerd/nerdctl-grpc-server/pkg/types"
)

// NetworkServiceServer implements the comprehensive NetworkServiceExtended gRPC service
// This service provides complete network management functionality for nerdctl
type NetworkServiceServer struct {
	pb.UnimplementedNetworkServiceExtendedServer
	manager interfaces.NetworkManager
	
	// Service state management
	mu           sync.RWMutex
	eventStreams map[string]chan *pb.NetworkEvent
	statsStreams map[string]chan *pb.NetworkStatsResponse
	
	// Configuration
	maxConcurrency   int
	defaultTimeout   time.Duration
	enableMetrics    bool
	enableDiagnostics bool
}

// NewNetworkServiceServer creates a new network service server instance
func NewNetworkServiceServer(manager interfaces.NetworkManager) *NetworkServiceServer {
	return &NetworkServiceServer{
		manager:           manager,
		eventStreams:     make(map[string]chan *pb.NetworkEvent),
		statsStreams:     make(map[string]chan *pb.NetworkStatsResponse),
		maxConcurrency:   10,
		defaultTimeout:   30 * time.Second,
		enableMetrics:    true,
		enableDiagnostics: true,
	}
}

// CreateNetwork creates a new network with comprehensive configuration options
func (s *NetworkServiceServer) CreateNetwork(ctx context.Context, req *pb.CreateNetworkExtendedRequest) (*pb.CreateNetworkExtendedResponse, error) {
	log.Printf("Creating network: %s in namespace: %s", req.Name, req.Namespace)
	
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "network name is required")
	}
	
	// Convert protobuf request to internal types
	internalReq := &types.CreateNetworkRequest{
		Name:              req.Name,
		Namespace:         req.Namespace,
		Driver:           req.Driver,
		Internal:         req.Internal,
		Attachable:       req.Attachable,
		Ingress:          req.Ingress,
		IPv6:             req.Ipv6,
		EnableICC:        req.EnableIcc,
		Options:          req.Options,
		Labels:           req.Labels,
		MTU:              int(req.Mtu),
		ParentInterface:  req.ParentInterface,
		VlanID:           int(req.VlanId),
		FirewallRules:    req.FirewallRules,
		IsolationMode:    req.IsolationMode,
		EnableEncryption: req.EnableEncryption,
		ConfigFile:       req.ConfigFile,
		ConfigDir:        req.ConfigDir,
		ValidateConfig:   req.ValidateConfig,
	}
	
	// Convert IPAM configuration
	if req.Ipam != nil {
		internalReq.IPAM = convertIPAMConfig(req.Ipam)
	}
	
	// Set timeout context
	timeout := s.defaultTimeout
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	
	// Create the network
	resp, err := s.manager.CreateNetwork(ctx, internalReq)
	if err != nil {
		log.Printf("Error creating network %s: %v", req.Name, err)
		return nil, handleNetworkError(err, "create network")
	}
	
	// Emit network creation event
	s.emitNetworkEvent(&pb.NetworkEvent{
		EventType:   "create",
		NetworkId:   resp.NetworkID,
		NetworkName: req.Name,
		Timestamp:   timestampNow(),
		Message:     fmt.Sprintf("Network %s created", req.Name),
		CreateEvent: &pb.NetworkCreateEvent{
			Network:    convertNetworkInfo(resp.Network),
			DriverUsed: req.Driver,
		},
	})
	
	return &pb.CreateNetworkExtendedResponse{
		NetworkId: resp.NetworkID,
		Warnings:  resp.Warnings,
		Network:   convertNetworkInfo(resp.Network),
		Metadata:  createOperationMetadata("create_network", time.Now()),
	}, nil
}

// RemoveNetwork removes one or more networks with advanced options
func (s *NetworkServiceServer) RemoveNetwork(ctx context.Context, req *pb.RemoveNetworkExtendedRequest) (*pb.RemoveNetworkExtendedResponse, error) {
	log.Printf("Removing networks: %v in namespace: %s", req.Networks, req.Namespace)
	
	if len(req.Networks) == 0 {
		return nil, status.Error(codes.InvalidArgument, "at least one network must be specified")
	}
	
	// Convert protobuf request to internal types
	internalReq := &types.RemoveNetworkRequest{
		Networks:          req.Networks,
		Namespace:         req.Namespace,
		Force:            req.Force,
		RemoveContainers: req.RemoveContainers,
		TimeoutSeconds:   int(req.TimeoutSeconds),
		DryRun:           req.DryRun,
	}
	
	// Set timeout context
	timeout := time.Duration(req.TimeoutSeconds)*time.Second
	if timeout == 0 {
		timeout = s.defaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	
	// Remove the networks
	resp, err := s.manager.RemoveNetwork(ctx, internalReq)
	if err != nil {
		log.Printf("Error removing networks %v: %v", req.Networks, err)
		return nil, handleNetworkError(err, "remove networks")
	}
	
	// Emit network removal events
	for _, networkID := range resp.Removed {
		s.emitNetworkEvent(&pb.NetworkEvent{
			EventType:   "remove",
			NetworkId:   networkID,
			Timestamp:   timestampNow(),
			Message:     fmt.Sprintf("Network %s removed", networkID),
			RemoveEvent: &pb.NetworkRemoveEvent{
				NetworkId:   networkID,
				NetworkName: networkID, // Assuming ID as name for now
				ForceRemoved: req.Force,
			},
		})
	}
	
	return &pb.RemoveNetworkExtendedResponse{
		Removed:   resp.Removed,
		Failed:    resp.Failed,
		Warnings:  resp.Warnings,
		Results:   convertOperationResults(resp.Results),
		Metadata:  createOperationMetadata("remove_network", time.Now()),
	}, nil
}

// ListNetworks lists networks with filtering, pagination, and comprehensive information
func (s *NetworkServiceServer) ListNetworks(ctx context.Context, req *pb.ListNetworksExtendedRequest) (*pb.ListNetworksExtendedResponse, error) {
	log.Printf("Listing networks in namespace: %s with filters: %v", req.Namespace, req.Filters)
	
	// Convert protobuf request to internal types
	internalReq := &types.ListNetworksRequest{
		Namespace:        req.Namespace,
		Filters:          req.Filters,
		Quiet:            req.Quiet,
		Format:           req.Format,
		NoTrunc:          req.NoTrunc,
		All:              req.All,
		IncludeSystem:    req.IncludeSystem,
		IncludeMetrics:   req.IncludeMetrics,
		IncludeContainers: req.IncludeContainers,
		Fields:           req.Fields,
		Limit:            int(req.Limit),
		ContinueToken:    req.ContinueToken,
		SortBy:           req.SortBy,
		ReverseSort:      req.ReverseSort,
	}
	
	// Set timeout context
	timeout := s.defaultTimeout
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	
	// List the networks
	resp, err := s.manager.ListNetworks(ctx, internalReq)
	if err != nil {
		log.Printf("Error listing networks: %v", err)
		return nil, handleNetworkError(err, "list networks")
	}
	
	// Convert response
	networks := make([]*pb.NetworkInfoExtended, len(resp.Networks))
	for i, network := range resp.Networks {
		networks[i] = convertNetworkInfo(network)
	}
	
	return &pb.ListNetworksExtendedResponse{
		Networks:      networks,
		TotalCount:    int32(resp.TotalCount),
		ContinueToken: resp.ContinueToken,
		Warnings:      resp.Warnings,
		Metadata:      createOperationMetadata("list_networks", time.Now()),
	}, nil
}

// InspectNetwork provides detailed information about a specific network
func (s *NetworkServiceServer) InspectNetwork(ctx context.Context, req *pb.InspectNetworkExtendedRequest) (*pb.InspectNetworkExtendedResponse, error) {
	log.Printf("Inspecting network: %s in namespace: %s", req.Network, req.Namespace)
	
	if req.Network == "" {
		return nil, status.Error(codes.InvalidArgument, "network name or ID is required")
	}
	
	// Convert protobuf request to internal types
	internalReq := &types.InspectNetworkRequest{
		Network:            req.Network,
		Namespace:          req.Namespace,
		Format:             req.Format,
		Verbose:            req.Verbose,
		IncludeContainers:  req.IncludeContainers,
		IncludeEndpoints:   req.IncludeEndpoints,
		IncludeMetrics:     req.IncludeMetrics,
		Fields:             req.Fields,
	}
	
	// Set timeout context
	timeout := s.defaultTimeout
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	
	// Inspect the network
	resp, err := s.manager.InspectNetwork(ctx, internalReq)
	if err != nil {
		log.Printf("Error inspecting network %s: %v", req.Network, err)
		return nil, handleNetworkError(err, "inspect network")
	}
	
	return &pb.InspectNetworkExtendedResponse{
		Network:  convertNetworkInfo(resp.Network),
		Warnings: resp.Warnings,
		Metadata: createOperationMetadata("inspect_network", time.Now()),
	}, nil
}

// ConnectNetwork connects a container to a network with advanced configuration
func (s *NetworkServiceServer) ConnectNetwork(ctx context.Context, req *pb.ConnectNetworkExtendedRequest) (*pb.ConnectNetworkExtendedResponse, error) {
	log.Printf("Connecting container %s to network %s in namespace: %s", req.Container, req.Network, req.Namespace)
	
	if req.Network == "" || req.Container == "" {
		return nil, status.Error(codes.InvalidArgument, "network and container are required")
	}
	
	// Convert protobuf request to internal types
	internalReq := &types.ConnectNetworkRequest{
		Network:               req.Network,
		Container:             req.Container,
		Namespace:             req.Namespace,
		Force:                req.Force,
		TimeoutSeconds:       int(req.TimeoutSeconds),
		ValidateConnectivity: req.ValidateConnectivity,
	}
	
	// Convert endpoint configuration
	if req.EndpointConfig != nil {
		internalReq.EndpointConfig = convertEndpointConfig(req.EndpointConfig)
	}
	
	// Set timeout context
	timeout := time.Duration(req.TimeoutSeconds)*time.Second
	if timeout == 0 {
		timeout = s.defaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	
	// Connect to the network
	resp, err := s.manager.ConnectNetwork(ctx, internalReq)
	if err != nil {
		log.Printf("Error connecting container %s to network %s: %v", req.Container, req.Network, err)
		return nil, handleNetworkError(err, "connect network")
	}
	
	// Emit network connection event
	s.emitNetworkEvent(&pb.NetworkEvent{
		EventType:     "connect",
		NetworkId:     req.Network,
		ContainerId:   req.Container,
		Timestamp:     timestampNow(),
		Message:       fmt.Sprintf("Container %s connected to network %s", req.Container, req.Network),
		ConnectEvent: &pb.NetworkConnectEvent{
			NetworkId:   req.Network,
			ContainerId: req.Container,
			EndpointId:  resp.EndpointID,
		},
	})
	
	return &pb.ConnectNetworkExtendedResponse{
		Connected:  resp.Connected,
		EndpointId: resp.EndpointID,
		Warnings:   resp.Warnings,
		Endpoint:   convertNetworkEndpoint(resp.Endpoint),
		Metadata:   createOperationMetadata("connect_network", time.Now()),
	}, nil
}

// DisconnectNetwork disconnects a container from a network
func (s *NetworkServiceServer) DisconnectNetwork(ctx context.Context, req *pb.DisconnectNetworkExtendedRequest) (*pb.DisconnectNetworkExtendedResponse, error) {
	log.Printf("Disconnecting container %s from network %s in namespace: %s", req.Container, req.Network, req.Namespace)
	
	if req.Network == "" || req.Container == "" {
		return nil, status.Error(codes.InvalidArgument, "network and container are required")
	}
	
	// Convert protobuf request to internal types
	internalReq := &types.DisconnectNetworkRequest{
		Network:          req.Network,
		Container:        req.Container,
		Namespace:        req.Namespace,
		Force:           req.Force,
		TimeoutSeconds:  int(req.TimeoutSeconds),
		CleanupResources: req.CleanupResources,
	}
	
	// Set timeout context
	timeout := time.Duration(req.TimeoutSeconds)*time.Second
	if timeout == 0 {
		timeout = s.defaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	
	// Disconnect from the network
	resp, err := s.manager.DisconnectNetwork(ctx, internalReq)
	if err != nil {
		log.Printf("Error disconnecting container %s from network %s: %v", req.Container, req.Network, err)
		return nil, handleNetworkError(err, "disconnect network")
	}
	
	// Emit network disconnection event
	s.emitNetworkEvent(&pb.NetworkEvent{
		EventType:        "disconnect",
		NetworkId:        req.Network,
		ContainerId:      req.Container,
		Timestamp:        timestampNow(),
		Message:          fmt.Sprintf("Container %s disconnected from network %s", req.Container, req.Network),
		DisconnectEvent: &pb.NetworkDisconnectEvent{
			NetworkId:        req.Network,
			ContainerId:      req.Container,
			ForceDisconnected: req.Force,
		},
	})
	
	return &pb.DisconnectNetworkExtendedResponse{
		Disconnected:    resp.Disconnected,
		Warnings:        resp.Warnings,
		CleanupActions:  resp.CleanupActions,
		Metadata:        createOperationMetadata("disconnect_network", time.Now()),
	}, nil
}

// PruneNetworks removes unused networks with advanced filtering options
func (s *NetworkServiceServer) PruneNetworks(ctx context.Context, req *pb.PruneNetworksExtendedRequest) (*pb.PruneNetworksExtendedResponse, error) {
	log.Printf("Pruning networks in namespace: %s with filters: %v", req.Namespace, req.Filters)
	
	// Convert protobuf request to internal types
	internalReq := &types.PruneNetworksRequest{
		Namespace:            req.Namespace,
		Filters:              req.Filters,
		Force:               req.Force,
		PruneSystemNetworks: req.PruneSystemNetworks,
		TimeoutSeconds:      int(req.TimeoutSeconds),
		DryRun:              req.DryRun,
		Until:               req.Until,
		PruneUnusedOnly:     req.PruneUnusedOnly,
		IncludeVolumes:      req.IncludeVolumes,
	}
	
	// Set timeout context
	timeout := time.Duration(req.TimeoutSeconds)*time.Second
	if timeout == 0 {
		timeout = s.defaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	
	// Prune the networks
	resp, err := s.manager.PruneNetworks(ctx, internalReq)
	if err != nil {
		log.Printf("Error pruning networks: %v", err)
		return nil, handleNetworkError(err, "prune networks")
	}
	
	return &pb.PruneNetworksExtendedResponse{
		Pruned:         resp.Pruned,
		Failed:         resp.Failed,
		SpaceReclaimed: resp.SpaceReclaimed,
		Warnings:       resp.Warnings,
		Results:        convertOperationResults(resp.Results),
		Metadata:       createOperationMetadata("prune_networks", time.Now()),
	}, nil
}

// UpdateNetwork updates network configuration
func (s *NetworkServiceServer) UpdateNetwork(ctx context.Context, req *pb.UpdateNetworkRequest) (*pb.UpdateNetworkResponse, error) {
	log.Printf("Updating network: %s in namespace: %s", req.Network, req.Namespace)
	
	if req.Network == "" {
		return nil, status.Error(codes.InvalidArgument, "network name or ID is required")
	}
	
	// Convert protobuf request to internal types
	internalReq := &types.UpdateNetworkRequest{
		Network:       req.Network,
		Namespace:     req.Namespace,
		Labels:        req.Labels,
		Options:       req.Options,
		EnableICC:     req.EnableIcc,
		MTU:           int(req.Mtu),
		FirewallRules: req.FirewallRules,
		AddIPAM:       req.AddIpam,
		UpdateMask:    req.UpdateMask,
	}
	
	// Convert IPAM configuration if provided
	if req.IpamUpdate != nil {
		internalReq.IPAMUpdate = convertIPAMConfig(req.IpamUpdate)
	}
	
	// Set timeout context
	timeout := s.defaultTimeout
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	
	// Update the network
	resp, err := s.manager.UpdateNetwork(ctx, internalReq)
	if err != nil {
		log.Printf("Error updating network %s: %v", req.Network, err)
		return nil, handleNetworkError(err, "update network")
	}
	
	// Emit network update event
	s.emitNetworkEvent(&pb.NetworkEvent{
		EventType:   "update",
		NetworkId:   req.Network,
		Timestamp:   timestampNow(),
		Message:     fmt.Sprintf("Network %s updated", req.Network),
		UpdateEvent: &pb.NetworkUpdateEvent{
			NetworkId: req.Network,
		},
	})
	
	return &pb.UpdateNetworkResponse{
		Updated:  resp.Updated,
		Network:  convertNetworkInfo(resp.Network),
		Warnings: resp.Warnings,
		Metadata: createOperationMetadata("update_network", time.Now()),
	}, nil
}

// NetworkExists checks if a network exists
func (s *NetworkServiceServer) NetworkExists(ctx context.Context, req *pb.NetworkExistsRequest) (*pb.NetworkExistsResponse, error) {
	log.Printf("Checking if network exists: %s in namespace: %s", req.Network, req.Namespace)
	
	if req.Network == "" {
		return nil, status.Error(codes.InvalidArgument, "network name or ID is required")
	}
	
	// Convert protobuf request to internal types
	internalReq := &types.NetworkExistsRequest{
		Network:   req.Network,
		Namespace: req.Namespace,
	}
	
	// Set timeout context
	timeout := s.defaultTimeout
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	
	// Check if network exists
	resp, err := s.manager.NetworkExists(ctx, internalReq)
	if err != nil {
		log.Printf("Error checking network existence %s: %v", req.Network, err)
		return nil, handleNetworkError(err, "check network existence")
	}
	
	return &pb.NetworkExistsResponse{
		Exists:    resp.Exists,
		NetworkId: resp.NetworkID,
	}, nil
}

// NetworkEvents streams network events in real-time
func (s *NetworkServiceServer) NetworkEvents(req *pb.NetworkEventsRequest, stream pb.NetworkServiceExtended_NetworkEventsServer) error {
	log.Printf("Starting network events stream for namespace: %s", req.Namespace)
	
	// Create event channel
	eventChan := make(chan *pb.NetworkEvent, 100)
	streamID := fmt.Sprintf("events_%d", time.Now().UnixNano())
	
	s.mu.Lock()
	s.eventStreams[streamID] = eventChan
	s.mu.Unlock()
	
	// Cleanup on exit
	defer func() {
		s.mu.Lock()
		delete(s.eventStreams, streamID)
		close(eventChan)
		s.mu.Unlock()
		log.Printf("Network events stream %s closed", streamID)
	}()
	
	// Stream events
	for {
		select {
		case event := <-eventChan:
			if event != nil && s.matchesEventFilter(event, req) {
				if err := stream.Send(event); err != nil {
					log.Printf("Error sending network event: %v", err)
					return err
				}
			}
		case <-stream.Context().Done():
			log.Printf("Network events stream context cancelled")
			return nil
		}
	}
}

// NetworkStats streams network statistics
func (s *NetworkServiceServer) NetworkStats(req *pb.NetworkStatsRequest, stream pb.NetworkServiceExtended_NetworkStatsServer) error {
	log.Printf("Starting network stats stream for networks: %v", req.Networks)
	
	if !s.enableMetrics {
		return status.Error(codes.Unimplemented, "network metrics are disabled")
	}
	
	// Create stats channel
	statsChan := make(chan *pb.NetworkStatsResponse, 100)
	streamID := fmt.Sprintf("stats_%d", time.Now().UnixNano())
	
	s.mu.Lock()
	s.statsStreams[streamID] = statsChan
	s.mu.Unlock()
	
	// Cleanup on exit
	defer func() {
		s.mu.Lock()
		delete(s.statsStreams, streamID)
		close(statsChan)
		s.mu.Unlock()
		log.Printf("Network stats stream %s closed", streamID)
	}()
	
	// Convert protobuf request to internal types
	internalReq := &types.NetworkStatsRequest{
		Networks:             req.Networks,
		Namespace:            req.Namespace,
		Stream:              req.Stream,
		IntervalSeconds:     int(req.IntervalSeconds),
		IncludeSystemStats:  req.IncludeSystemStats,
	}
	
	// Start stats collection
	statsResult, err := s.manager.NetworkStats(stream.Context(), internalReq)
	if err != nil {
		log.Printf("Error starting network stats collection: %v", err)
		return handleNetworkError(err, "collect network stats")
	}
	
	// Stream stats
	for stat := range statsResult {
		statsResponse := convertNetworkStats(stat)
		if err := stream.Send(statsResponse); err != nil {
			log.Printf("Error sending network stats: %v", err)
			return err
		}
	}
	
	return nil
}

// BatchNetworkOperations performs multiple network operations in batch
func (s *NetworkServiceServer) BatchNetworkOperations(req *pb.BatchNetworkRequest, stream pb.NetworkServiceExtended_BatchNetworkOperationsServer) error {
	log.Printf("Starting batch network operations with %d operations", len(req.Operations))
	
	if len(req.Operations) == 0 {
		return status.Error(codes.InvalidArgument, "at least one operation must be specified")
	}
	
	if len(req.Operations) > 100 {
		return status.Error(codes.InvalidArgument, "maximum 100 operations allowed per batch")
	}
	
	// Convert protobuf request to internal types
	internalReq := &types.BatchNetworkRequest{
		Operations:     convertBatchOperations(req.Operations),
		Namespace:      req.Namespace,
		FailFast:       req.FailFast,
		Concurrency:    int(req.Concurrency),
		TimeoutSeconds: int(req.TimeoutSeconds),
	}
	
	// Set timeout context
	timeout := time.Duration(req.TimeoutSeconds)*time.Second
	if timeout == 0 {
		timeout = s.defaultTimeout * time.Duration(len(req.Operations))
	}
	ctx, cancel := context.WithTimeout(stream.Context(), timeout)
	defer cancel()
	
	// Execute batch operations
	resultChan, err := s.manager.BatchNetworkOperations(ctx, internalReq)
	if err != nil {
		log.Printf("Error starting batch network operations: %v", err)
		return handleNetworkError(err, "batch network operations")
	}
	
	// Stream results
	for result := range resultChan {
		response := convertBatchNetworkResponse(result)
		if err := stream.Send(response); err != nil {
			log.Printf("Error sending batch operation result: %v", err)
			return err
		}
	}
	
	return nil
}

// BulkNetworkInspect inspects multiple networks in a single request
func (s *NetworkServiceServer) BulkNetworkInspect(ctx context.Context, req *pb.BulkInspectNetworkRequest) (*pb.BulkInspectNetworkResponse, error) {
	log.Printf("Bulk inspecting %d networks in namespace: %s", len(req.Networks), req.Namespace)
	
	if len(req.Networks) == 0 {
		return nil, status.Error(codes.InvalidArgument, "at least one network must be specified")
	}
	
	if len(req.Networks) > 50 {
		return nil, status.Error(codes.InvalidArgument, "maximum 50 networks allowed per bulk inspect")
	}
	
	// Convert protobuf request to internal types
	internalReq := &types.BulkInspectNetworkRequest{
		Networks:          req.Networks,
		Namespace:         req.Namespace,
		Verbose:           req.Verbose,
		IncludeContainers: req.IncludeContainers,
		IncludeMetrics:    req.IncludeMetrics,
		Fields:            req.Fields,
	}
	
	// Set timeout context
	timeout := s.defaultTimeout
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	
	// Bulk inspect networks
	resp, err := s.manager.BulkNetworkInspect(ctx, internalReq)
	if err != nil {
		log.Printf("Error bulk inspecting networks: %v", err)
		return nil, handleNetworkError(err, "bulk inspect networks")
	}
	
	// Convert results
	results := make([]*pb.NetworkInspectResult, len(resp.Results))
	for i, result := range resp.Results {
		results[i] = &pb.NetworkInspectResult{
			Network:      result.Network,
			Success:      result.Success,
			ErrorMessage: result.ErrorMessage,
			NetworkInfo:  convertNetworkInfo(result.NetworkInfo),
		}
	}
	
	return &pb.BulkInspectNetworkResponse{
		Results:      results,
		TotalCount:   int32(resp.TotalCount),
		SuccessCount: int32(resp.SuccessCount),
		ErrorCount:   int32(resp.ErrorCount),
		Warnings:     resp.Warnings,
		Metadata:     createOperationMetadata("bulk_inspect_networks", time.Now()),
	}, nil
}

// ListNetworkDrivers lists available network drivers
func (s *NetworkServiceServer) ListNetworkDrivers(ctx context.Context, req *pb.ListNetworkDriversRequest) (*pb.ListNetworkDriversResponse, error) {
	log.Printf("Listing network drivers in namespace: %s", req.Namespace)
	
	// Convert protobuf request to internal types
	internalReq := &types.ListNetworkDriversRequest{
		Namespace:         req.Namespace,
		IncludePluginInfo: req.IncludePluginInfo,
	}
	
	// Set timeout context
	timeout := s.defaultTimeout
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	
	// List network drivers
	resp, err := s.manager.ListNetworkDrivers(ctx, internalReq)
	if err != nil {
		log.Printf("Error listing network drivers: %v", err)
		return nil, handleNetworkError(err, "list network drivers")
	}
	
	// Convert drivers
	drivers := make([]*pb.NetworkDriver, len(resp.Drivers))
	for i, driver := range resp.Drivers {
		drivers[i] = convertNetworkDriver(driver)
	}
	
	return &pb.ListNetworkDriversResponse{
		Drivers:  drivers,
		Warnings: resp.Warnings,
	}, nil
}

// GetNetworkConfig retrieves network configuration
func (s *NetworkServiceServer) GetNetworkConfig(ctx context.Context, req *pb.GetNetworkConfigRequest) (*pb.GetNetworkConfigResponse, error) {
	log.Printf("Getting network config for: %s in namespace: %s", req.Network, req.Namespace)
	
	if req.Network == "" {
		return nil, status.Error(codes.InvalidArgument, "network name or ID is required")
	}
	
	// Convert protobuf request to internal types
	internalReq := &types.GetNetworkConfigRequest{
		Network:   req.Network,
		Namespace: req.Namespace,
		Format:    req.Format,
	}
	
	// Set timeout context
	timeout := s.defaultTimeout
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	
	// Get network config
	resp, err := s.manager.GetNetworkConfig(ctx, internalReq)
	if err != nil {
		log.Printf("Error getting network config for %s: %v", req.Network, err)
		return nil, handleNetworkError(err, "get network config")
	}
	
	return &pb.GetNetworkConfigResponse{
		Config:     resp.Config,
		Format:     resp.Format,
		ConfigPath: resp.ConfigPath,
		Warnings:   resp.Warnings,
	}, nil
}

// ValidateNetworkConfig validates network configuration
func (s *NetworkServiceServer) ValidateNetworkConfig(ctx context.Context, req *pb.ValidateNetworkConfigRequest) (*pb.ValidateNetworkConfigResponse, error) {
	log.Printf("Validating network config")
	
	if req.Config == "" && req.ConfigPath == "" {
		return nil, status.Error(codes.InvalidArgument, "config or config_path is required")
	}
	
	// Convert protobuf request to internal types
	internalReq := &types.ValidateNetworkConfigRequest{
		Config:           req.Config,
		ConfigPath:       req.ConfigPath,
		Format:           req.Format,
		StrictValidation: req.StrictValidation,
	}
	
	// Set timeout context
	timeout := s.defaultTimeout
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	
	// Validate network config
	resp, err := s.manager.ValidateNetworkConfig(ctx, internalReq)
	if err != nil {
		log.Printf("Error validating network config: %v", err)
		return nil, handleNetworkError(err, "validate network config")
	}
	
	// Convert validation issues
	issues := make([]*pb.ValidationIssue, len(resp.Issues))
	for i, issue := range resp.Issues {
		issues[i] = &pb.ValidationIssue{
			Severity:   issue.Severity,
			Field:      issue.Field,
			Message:    issue.Message,
			Suggestion: issue.Suggestion,
		}
	}
	
	return &pb.ValidateNetworkConfigResponse{
		Valid:    resp.Valid,
		Errors:   resp.Errors,
		Warnings: resp.Warnings,
		Issues:   issues,
	}, nil
}

// DiagnoseNetwork performs network diagnostics
func (s *NetworkServiceServer) DiagnoseNetwork(ctx context.Context, req *pb.DiagnoseNetworkRequest) (*pb.DiagnoseNetworkResponse, error) {
	log.Printf("Diagnosing network: %s in namespace: %s", req.Network, req.Namespace)
	
	if !s.enableDiagnostics {
		return nil, status.Error(codes.Unimplemented, "network diagnostics are disabled")
	}
	
	if req.Network == "" {
		return nil, status.Error(codes.InvalidArgument, "network name or ID is required")
	}
	
	// Convert protobuf request to internal types
	internalReq := &types.DiagnoseNetworkRequest{
		Network:            req.Network,
		Namespace:          req.Namespace,
		DeepInspection:     req.DeepInspection,
		CheckConnectivity:  req.CheckConnectivity,
		CheckDNS:           req.CheckDns,
		TargetContainers:   req.TargetContainers,
	}
	
	// Set timeout context with extended time for diagnostics
	timeout := s.defaultTimeout * 3 // Diagnostics can take longer
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	
	// Diagnose network
	resp, err := s.manager.DiagnoseNetwork(ctx, internalReq)
	if err != nil {
		log.Printf("Error diagnosing network %s: %v", req.Network, err)
		return nil, handleNetworkError(err, "diagnose network")
	}
	
	// Convert diagnostic results
	results := make([]*pb.DiagnosticResult, len(resp.Results))
	for i, result := range resp.Results {
		results[i] = &pb.DiagnosticResult{
			TestName:        result.TestName,
			Status:          result.Status,
			Message:         result.Message,
			Details:         result.Details,
			ExecutionTimeMs: result.ExecutionTimeMs,
		}
	}
	
	return &pb.DiagnoseNetworkResponse{
		NetworkId:       resp.NetworkID,
		NetworkName:     resp.NetworkName,
		OverallStatus:   resp.OverallStatus,
		Results:         results,
		Recommendations: resp.Recommendations,
		DebugInfo:       resp.DebugInfo,
	}, nil
}

// NetworkConnectivity tests connectivity between networks
func (s *NetworkServiceServer) NetworkConnectivity(ctx context.Context, req *pb.NetworkConnectivityRequest) (*pb.NetworkConnectivityResponse, error) {
	log.Printf("Testing network connectivity from %s to %s in namespace: %s", req.SourceNetwork, req.TargetNetwork, req.Namespace)
	
	if req.SourceNetwork == "" || req.TargetNetwork == "" {
		return nil, status.Error(codes.InvalidArgument, "source_network and target_network are required")
	}
	
	// Convert protobuf request to internal types
	internalReq := &types.NetworkConnectivityRequest{
		SourceNetwork:    req.SourceNetwork,
		TargetNetwork:    req.TargetNetwork,
		Namespace:        req.Namespace,
		SourceContainer:  req.SourceContainer,
		TargetContainer:  req.TargetContainer,
		Protocols:        req.Protocols,
		Ports:           convertPorts(req.Ports),
		TimeoutSeconds:  int(req.TimeoutSeconds),
		RetryCount:      int(req.RetryCount),
	}
	
	// Set timeout context
	timeout := time.Duration(req.TimeoutSeconds)*time.Second
	if timeout == 0 {
		timeout = s.defaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	
	// Test network connectivity
	resp, err := s.manager.NetworkConnectivity(ctx, internalReq)
	if err != nil {
		log.Printf("Error testing network connectivity: %v", err)
		return nil, handleNetworkError(err, "test network connectivity")
	}
	
	// Convert connectivity results
	results := make([]*pb.ConnectivityResult, len(resp.Results))
	for i, result := range resp.Results {
		results[i] = &pb.ConnectivityResult{
			Protocol:     result.Protocol,
			Port:         int32(result.Port),
			Success:      result.Success,
			LatencyMs:    result.LatencyMs,
			ErrorMessage: result.ErrorMessage,
			AttemptCount: int32(result.AttemptCount),
		}
	}
	
	return &pb.NetworkConnectivityResponse{
		Reachable:          resp.Reachable,
		Results:            results,
		Summary:            resp.Summary,
		AverageLatencyMs:   resp.AverageLatencyMs,
		PacketLossRate:     resp.PacketLossRate,
		Warnings:           resp.Warnings,
	}, nil
}

// GetNetworkSupportedFeatures returns supported network features
func (s *NetworkServiceServer) GetNetworkSupportedFeatures(ctx context.Context, req *pb.GetNetworkSupportedFeaturesRequest) (*pb.GetNetworkSupportedFeaturesResponse, error) {
	log.Printf("Getting network supported features for namespace: %s", req.Namespace)
	
	// Convert protobuf request to internal types
	internalReq := &types.GetNetworkSupportedFeaturesRequest{
		Namespace: req.Namespace,
	}
	
	// Set timeout context
	timeout := s.defaultTimeout
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	
	// Get supported features
	resp, err := s.manager.GetSupportedFeatures(ctx)
	if err != nil {
		log.Printf("Error getting network supported features: %v", err)
		return nil, handleNetworkError(err, "get network supported features")
	}
	
	features := convertNetworkSupportedFeatures(resp)
	
	return &pb.GetNetworkSupportedFeaturesResponse{
		Features: features,
	}, nil
}

// Helper methods

func (s *NetworkServiceServer) emitNetworkEvent(event *pb.NetworkEvent) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	for _, eventChan := range s.eventStreams {
		select {
		case eventChan <- event:
		default:
			// Channel is full, skip this stream
			log.Printf("Warning: Network event stream buffer full, dropping event")
		}
	}
}

func (s *NetworkServiceServer) matchesEventFilter(event *pb.NetworkEvent, req *pb.NetworkEventsRequest) bool {
	// Check timestamp filters
	if req.Since != nil && event.Timestamp.AsTime().Before(req.Since.AsTime()) {
		return false
	}
	if req.Until != nil && event.Timestamp.AsTime().After(req.Until.AsTime()) {
		return false
	}
	
	// Check event type filter
	if len(req.EventTypes) > 0 {
		found := false
		for _, eventType := range req.EventTypes {
			if event.EventType == eventType {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	
	// Check other filters
	for key, value := range req.Filters {
		switch key {
		case "network":
			if event.NetworkName != value && event.NetworkId != value {
				return false
			}
		case "container":
			if event.ContainerName != value && event.ContainerId != value {
				return false
			}
		case "event":
			if event.EventType != value {
				return false
			}
		}
	}
	
	return true
}

// Conversion helper functions

func convertIPAMConfig(ipam *pb.IPAMConfigExtended) *types.IPAMConfig {
	if ipam == nil {
		return nil
	}
	
	subnets := make([]*types.IPAMSubnet, len(ipam.Subnets))
	for i, subnet := range ipam.Subnets {
		subnets[i] = &types.IPAMSubnet{
			Subnet:             subnet.Subnet,
			Gateway:            subnet.Gateway,
			IPRange:            subnet.IpRange,
			AuxAddresses:       subnet.AuxAddresses,
			VlanID:             int(subnet.VlanId),
			AvailableIPs:       int(subnet.AvailableIps),
			AllocatedIPs:       int(subnet.AllocatedIps),
			AllocatedAddresses: subnet.AllocatedAddresses,
		}
	}
	
	return &types.IPAMConfig{
		Driver:          ipam.Driver,
		Subnets:         subnets,
		Options:         ipam.Options,
		DriverOpts:      ipam.DriverOpts,
		EnableDHCP:      ipam.EnableDhcp,
		DHCPRangeStart:  ipam.DhcpRangeStart,
		DHCPRangeEnd:    ipam.DhcpRangeEnd,
		ReservedIPs:     ipam.ReservedIps,
		DNSServer:       ipam.DnsServer,
		DNSSearch:       ipam.DnsSearch,
	}
}

func convertEndpointConfig(config *pb.NetworkEndpointConfigExtended) *types.NetworkEndpointConfig {
	if config == nil {
		return nil
	}
	
	portMappings := make([]*types.PortMapping, len(config.PortMappings))
	for i, mapping := range config.PortMappings {
		portMappings[i] = &types.PortMapping{
			ContainerPort: int(mapping.ContainerPort),
			HostPort:      int(mapping.HostPort),
			Protocol:      mapping.Protocol,
			HostIP:        mapping.HostIp,
			Description:   mapping.Description,
		}
	}
	
	return &types.NetworkEndpointConfig{
		IPv4Address:    config.Ipv4Address,
		IPv6Address:    config.Ipv6Address,
		Aliases:        config.Aliases,
		Links:          config.Links,
		DriverOpts:     config.DriverOpts,
		MacAddress:     config.MacAddress,
		DNSNames:       config.DnsNames,
		DNSServers:     config.DnsServers,
		DNSSearch:      config.DnsSearch,
		ExtraHosts:     config.ExtraHosts,
		PortMappings:   portMappings,
		Capabilities:   config.Capabilities,
		Privileged:     config.Privileged,
		UserNsMode:     config.UserNsMode,
	}
}

func convertNetworkInfo(network *types.NetworkInfoExtended) *pb.NetworkInfoExtended {
	if network == nil {
		return nil
	}
	
	// Convert containers
	containers := make([]*pb.NetworkContainerExtended, len(network.Containers))
	for i, container := range network.Containers {
		containers[i] = convertNetworkContainer(container)
	}
	
	// Convert endpoints
	endpoints := make([]*pb.NetworkEndpoint, len(network.Endpoints))
	for i, endpoint := range network.Endpoints {
		endpoints[i] = convertNetworkEndpoint(endpoint)
	}
	
	return &pb.NetworkInfoExtended{
		Id:               network.ID,
		Name:             network.Name,
		Driver:           network.Driver,
		Scope:            network.Scope,
		Ipam:             convertIPAMConfigToPb(network.IPAM),
		Options:          network.Options,
		Labels:           network.Labels,
		Internal:         network.Internal,
		Attachable:       network.Attachable,
		Ingress:          network.Ingress,
		Ipv6Enabled:      network.IPv6Enabled,
		EnableIcc:        network.EnableICC,
		Created:          timestampProto(network.Created),
		Updated:          timestampProto(network.Updated),
		Containers:       containers,
		Endpoints:        endpoints,
		State:            convertNetworkState(network.State),
		Config:           convertNetworkConfig(network.Config),
		Metrics:          convertNetworkMetrics(network.Metrics),
		DriverData:       network.DriverData,
		SubnetRanges:     network.SubnetRanges,
		GatewayAddresses: network.GatewayAddresses,
		FirewallRules:    network.FirewallRules,
		IsolationMode:    network.IsolationMode,
	}
}

func convertNetworkContainer(container *types.NetworkContainerExtended) *pb.NetworkContainerExtended {
	if container == nil {
		return nil
	}
	
	portMappings := make([]*pb.PortMapping, len(container.PortMappings))
	for i, mapping := range container.PortMappings {
		portMappings[i] = &pb.PortMapping{
			ContainerPort: int32(mapping.ContainerPort),
			HostPort:      int32(mapping.HostPort),
			Protocol:      mapping.Protocol,
			HostIp:        mapping.HostIP,
			Description:   mapping.Description,
		}
	}
	
	return &pb.NetworkContainerExtended{
		ContainerId:  container.ContainerID,
		Name:         container.Name,
		Image:        container.Image,
		Ipv4Address:  container.IPv4Address,
		Ipv6Address:  container.IPv6Address,
		MacAddress:   container.MacAddress,
		Aliases:      container.Aliases,
		Links:        container.Links,
		Endpoints:    container.Endpoints,
		State:        convertNetworkEndpointState(container.State),
		ConnectedAt:  timestampProto(container.ConnectedAt),
		PortMappings: portMappings,
	}
}

func convertNetworkEndpoint(endpoint *types.NetworkEndpoint) *pb.NetworkEndpoint {
	if endpoint == nil {
		return nil
	}
	
	return &pb.NetworkEndpoint{
		EndpointId:  endpoint.EndpointID,
		Name:        endpoint.Name,
		NetworkId:   endpoint.NetworkID,
		ContainerId: endpoint.ContainerID,
		Ipv4Address: endpoint.IPv4Address,
		Ipv6Address: endpoint.IPv6Address,
		MacAddress:  endpoint.MacAddress,
		DnsNames:    endpoint.DNSNames,
		Aliases:     endpoint.Aliases,
		Options:     endpoint.Options,
		State:       convertNetworkEndpointState(endpoint.State),
		Created:     timestampProto(endpoint.Created),
	}
}

func convertNetworkState(state *types.NetworkState) *pb.NetworkState {
	if state == nil {
		return nil
	}
	
	return &pb.NetworkState{
		Status:           state.Status,
		Health:           state.Health,
		LastError:        state.LastError,
		ContainerCount:   int32(state.ContainerCount),
		EndpointCount:    int32(state.EndpointCount),
		LastActivity:     timestampProto(state.LastActivity),
		BridgeUp:         state.BridgeUp,
		DhcpActive:       state.DHCPActive,
		ActiveInterfaces: state.ActiveInterfaces,
	}
}

func convertNetworkConfig(config *types.NetworkConfig) *pb.NetworkConfig {
	if config == nil {
		return nil
	}
	
	return &pb.NetworkConfig{
		CniVersion:              config.CNIVersion,
		CniConfigPath:           config.CNIConfigPath,
		CniPlugins:              config.CNIPlugins,
		PrimaryDriver:           config.PrimaryDriver,
		AvailableDrivers:        config.AvailableDrivers,
		DriverConfig:            config.DriverConfig,
		Mtu:                     int32(config.MTU),
		HairpinMode:             config.HairpinMode,
		BridgeNfCallIptables:    config.BridgeNfCallIptables,
		BridgeNfCallIp6Tables:   config.BridgeNfCallIp6tables,
		IpForward:               config.IPForward,
		IpMasq:                  config.IPMasq,
		VlanId:                  int32(config.VlanID),
		ParentInterface:         config.ParentInterface,
		AllowedProtocols:        config.AllowedProtocols,
		BlockedPorts:            config.BlockedPorts,
		EnableEncryption:        config.EnableEncryption,
	}
}

func convertNetworkMetrics(metrics *types.NetworkMetrics) *pb.NetworkMetrics {
	if metrics == nil {
		return nil
	}
	
	return &pb.NetworkMetrics{
		BytesSent:         metrics.BytesSent,
		BytesReceived:     metrics.BytesReceived,
		PacketsSent:       metrics.PacketsSent,
		PacketsReceived:   metrics.PacketsReceived,
		TxErrors:          metrics.TxErrors,
		RxErrors:          metrics.RxErrors,
		TxDropped:         metrics.TxDropped,
		RxDropped:         metrics.RxDropped,
		CurrentConnections: int32(metrics.CurrentConnections),
		PeakConnections:   int32(metrics.PeakConnections),
		TotalConnections:  metrics.TotalConnections,
		LatencyMs:         metrics.LatencyMs,
		ThroughputMbps:    metrics.ThroughputMbps,
		PacketLossRate:    metrics.PacketLossRate,
		CollectedAt:       timestampProto(metrics.CollectedAt),
	}
}

func convertNetworkEndpointState(state types.NetworkEndpointState) pb.NetworkEndpointState {
	switch state {
	case types.NetworkEndpointStateCreating:
		return pb.NetworkEndpointState_ENDPOINT_CREATING
	case types.NetworkEndpointStateActive:
		return pb.NetworkEndpointState_ENDPOINT_ACTIVE
	case types.NetworkEndpointStateInactive:
		return pb.NetworkEndpointState_ENDPOINT_INACTIVE
	case types.NetworkEndpointStateError:
		return pb.NetworkEndpointState_ENDPOINT_ERROR
	case types.NetworkEndpointStateRemoving:
		return pb.NetworkEndpointState_ENDPOINT_REMOVING
	default:
		return pb.NetworkEndpointState_ENDPOINT_UNKNOWN
	}
}

func convertIPAMConfigToPb(ipam *types.IPAMConfig) *pb.IPAMConfigExtended {
	if ipam == nil {
		return nil
	}
	
	subnets := make([]*pb.IPAMSubnet, len(ipam.Subnets))
	for i, subnet := range ipam.Subnets {
		subnets[i] = &pb.IPAMSubnet{
			Subnet:             subnet.Subnet,
			Gateway:            subnet.Gateway,
			IpRange:            subnet.IPRange,
			AuxAddresses:       subnet.AuxAddresses,
			VlanId:             int32(subnet.VlanID),
			AvailableIps:       int32(subnet.AvailableIPs),
			AllocatedIps:       int32(subnet.AllocatedIPs),
			AllocatedAddresses: subnet.AllocatedAddresses,
		}
	}
	
	return &pb.IPAMConfigExtended{
		Driver:          ipam.Driver,
		Subnets:         subnets,
		Options:         ipam.Options,
		DriverOpts:      ipam.DriverOpts,
		EnableDhcp:      ipam.EnableDHCP,
		DhcpRangeStart:  ipam.DHCPRangeStart,
		DhcpRangeEnd:    ipam.DHCPRangeEnd,
		ReservedIps:     ipam.ReservedIPs,
		DnsServer:       ipam.DNSServer,
		DnsSearch:       ipam.DNSSearch,
	}
}

func convertNetworkDriver(driver *types.NetworkDriver) *pb.NetworkDriver {
	if driver == nil {
		return nil
	}
	
	return &pb.NetworkDriver{
		Name:         driver.Name,
		Version:      driver.Version,
		Description:  driver.Description,
		Capabilities: driver.Capabilities,
		Options:      driver.Options,
		IsAvailable:  driver.IsAvailable,
		IsDefault:    driver.IsDefault,
		PluginPath:   driver.PluginPath,
		PluginConfig: driver.PluginConfig,
	}
}

func convertNetworkStats(stats *types.NetworkStatsResponse) *pb.NetworkStatsResponse {
	if stats == nil {
		return nil
	}
	
	interfaces := make([]*pb.InterfaceStats, len(stats.Interfaces))
	for i, iface := range stats.Interfaces {
		interfaces[i] = &pb.InterfaceStats{
			InterfaceName: iface.InterfaceName,
			BytesSent:     iface.BytesSent,
			BytesReceived: iface.BytesReceived,
			PacketsSent:   iface.PacketsSent,
			PacketsReceived: iface.PacketsReceived,
			Errors:        iface.Errors,
			Dropped:       iface.Dropped,
			IsUp:          iface.IsUp,
			Mtu:           int32(iface.MTU),
		}
	}
	
	return &pb.NetworkStatsResponse{
		NetworkId:   stats.NetworkID,
		NetworkName: stats.NetworkName,
		Stats:       convertNetworkMetrics(stats.Stats),
		Timestamp:   timestampProto(stats.Timestamp),
		SystemStats: stats.SystemStats,
		Interfaces:  interfaces,
	}
}

func convertBatchOperations(operations []*pb.NetworkOperation) []*types.NetworkOperation {
	result := make([]*types.NetworkOperation, len(operations))
	for i, op := range operations {
		result[i] = &types.NetworkOperation{
			OperationID: op.OperationId,
			// Note: Would need to convert specific operation types
			// This is a simplified version
		}
	}
	return result
}

func convertBatchNetworkResponse(result *types.BatchNetworkResponse) *pb.BatchNetworkResponse {
	if result == nil {
		return nil
	}
	
	return &pb.BatchNetworkResponse{
		OperationId:  result.OperationID,
		Success:      result.Success,
		ErrorMessage: result.ErrorMessage,
		Metadata:     createOperationMetadata("batch_operation", time.Now()),
	}
}

func convertNetworkSupportedFeatures(features *types.NetworkSupportedFeatures) *pb.NetworkSupportedFeatures {
	if features == nil {
		return nil
	}
	
	return &pb.NetworkSupportedFeatures{
		Drivers:                      features.Drivers,
		IpamDrivers:                  features.IPAMDrivers,
		CniPlugins:                   features.CNIPlugins,
		SupportsIpv6:                 features.SupportsIPv6,
		SupportsMacvlan:              features.SupportsMacvlan,
		SupportsOverlay:              features.SupportsOverlay,
		SupportsEncryption:           features.SupportsEncryption,
		SupportsMulticast:            features.SupportsMulticast,
		SupportsVlan:                 features.SupportsVLAN,
		SupportsBridgeNetworking:     features.SupportsBridgeNetworking,
		SupportsHostNetworking:       features.SupportsHostNetworking,
		SupportsNoneNetworking:       features.SupportsNoneNetworking,
		SupportsNetworkPolicies:      features.SupportsNetworkPolicies,
		SupportsServiceDiscovery:     features.SupportsServiceDiscovery,
		SupportsLoadBalancing:        features.SupportsLoadBalancing,
		SupportsTrafficShaping:       features.SupportsTrafficShaping,
		SupportsNetworkMonitoring:    features.SupportsNetworkMonitoring,
		SupportsNetworkDiagnostics:   features.SupportsNetworkDiagnostics,
		CniVersion:                   features.CNIVersion,
		MinNerdctlVersion:            features.MinNerdctlVersion,
		MaxNerdctlVersion:            features.MaxNerdctlVersion,
		Capabilities:                 features.Capabilities,
		ExperimentalFeatures:         features.ExperimentalFeatures,
	}
}

func convertOperationResults(results []*types.OperationResult) []*pb.OperationResult {
	converted := make([]*pb.OperationResult, len(results))
	for i, result := range results {
		converted[i] = &pb.OperationResult{
			Id:           result.ID,
			Success:      result.Success,
			ErrorMessage: result.ErrorMessage,
			Metadata:     result.Metadata,
			Timestamp:    timestampProto(result.Timestamp),
		}
	}
	return converted
}

func convertPorts(ports []int32) []int {
	result := make([]int, len(ports))
	for i, port := range ports {
		result[i] = int(port)
	}
	return result
}

// Error handling helper
func handleNetworkError(err error, operation string) error {
	if err == nil {
		return nil
	}
	
	// Map common errors to appropriate gRPC status codes
	switch {
	case isNotFoundError(err):
		return status.Errorf(codes.NotFound, "%s failed: %v", operation, err)
	case isAlreadyExistsError(err):
		return status.Errorf(codes.AlreadyExists, "%s failed: %v", operation, err)
	case isInvalidArgumentError(err):
		return status.Errorf(codes.InvalidArgument, "%s failed: %v", operation, err)
	case isPermissionError(err):
		return status.Errorf(codes.PermissionDenied, "%s failed: %v", operation, err)
	case isTimeoutError(err):
		return status.Errorf(codes.DeadlineExceeded, "%s failed: %v", operation, err)
	default:
		return status.Errorf(codes.Internal, "%s failed: %v", operation, err)
	}
}

// Helper functions for error checking
func isNotFoundError(err error) bool {
	return err != nil && (contains(err.Error(), "not found") || contains(err.Error(), "no such"))
}

func isAlreadyExistsError(err error) bool {
	return err != nil && (contains(err.Error(), "already exists") || contains(err.Error(), "conflict"))
}

func isInvalidArgumentError(err error) bool {
	return err != nil && (contains(err.Error(), "invalid") || contains(err.Error(), "malformed"))
}

func isPermissionError(err error) bool {
	return err != nil && (contains(err.Error(), "permission") || contains(err.Error(), "access denied"))
}

func isTimeoutError(err error) bool {
	return err != nil && (contains(err.Error(), "timeout") || contains(err.Error(), "deadline"))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || s[len(s)-len(substr):] == substr || s[:len(substr)] == substr || 
		(len(s) > len(substr) && (s[len(s)-len(substr)-1:len(s)-len(substr)] == " " || s[len(substr):len(substr)+1] == " ")))
}

// Utility functions
func timestampNow() *timestamppb.Timestamp {
	return timestamppb.Now()
}

func timestampProto(t time.Time) *timestamppb.Timestamp {
	return timestamppb.New(t)
}

func createOperationMetadata(operation string, startTime time.Time) *pb.OperationMetadata {
	now := time.Now()
	return &pb.OperationMetadata{
		OperationId:   fmt.Sprintf("%s_%d", operation, now.UnixNano()),
		StartedAt:     timestampProto(startTime),
		CompletedAt:   timestampProto(now),
		DurationMs:    float64(now.Sub(startTime).Nanoseconds() / 1e6),
		NerdctlVersion: "2.0.0", // Would be retrieved from version info
		CniVersion:    "1.0.0",  // Would be retrieved from CNI info
		SystemInfo:    map[string]string{},
	}
}