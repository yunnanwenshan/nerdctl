package v2

import (
	"context"
	"fmt"
	"strings"

	"github.com/containerd/nerdctl-grpc-server/internal/config"
	"github.com/containerd/nerdctl-grpc-server/internal/interfaces"
	"github.com/containerd/nerdctl-grpc-server/pkg/types"
	"github.com/sirupsen/logrus"
)

// V2ImageManager implements image management for nerdctl v2.x with enhanced features
type V2ImageManager struct {
	executor *V2CommandExecutor
	config   *config.Config
	logger   *logrus.Entry
}

// NewV2ImageManager creates a new V2ImageManager
func NewV2ImageManager(executor *V2CommandExecutor, config *config.Config, logger *logrus.Logger) (*V2ImageManager, error) {
	return &V2ImageManager{
		executor: executor,
		config:   config,
		logger:   logger.WithField("component", "v2-image-manager"),
	}, nil
}

// PullImage pulls an image using v2.x enhanced pull mechanisms
func (m *V2ImageManager) PullImage(ctx context.Context, request *types.PullImageRequest) (*interfaces.ImagePullStream, error) {
	m.logger.WithFields(logrus.Fields{
		"image":    request.Image,
		"platform": request.Platform,
	}).Info("Pulling image with v2 enhanced features")

	args := []string{"image", "pull"}
	
	// v2.x enhanced platform support
	if request.Platform != "" {
		args = append(args, "--platform", request.Platform)
	}
	
	// v2.x enhanced authentication
	if request.Auth != nil && request.Auth.Username != "" {
		// Handle authentication in v2.x way
		args = append(args, "--username", request.Auth.Username)
	}
	
	// v2.x enhanced progress tracking
	args = append(args, "--progress", "auto")
	
	args = append(args, request.Image)
	
	// Use v2 enhanced streaming for pull progress
	streamResult, err := m.executor.ExecuteStream(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("failed to pull image: %w", err)
	}
	
	return &interfaces.ImagePullStream{
		ProgressChan: streamResult.LinesChan,
		ErrorsChan:   streamResult.ErrorsChan,
		Cancel:       streamResult.Cancel,
	}, nil
}

// ListImages lists images with v2.x enhanced metadata
func (m *V2ImageManager) ListImages(ctx context.Context, request *types.ListImagesRequest) (*types.ListImagesResponse, error) {
	args := []string{"image", "ls", "--format", "json"}
	
	if request.All {
		args = append(args, "--all")
	}
	
	// v2.x enhanced digest information
	if request.Digests {
		args = append(args, "--digests")
	}
	
	result, err := m.executor.Execute(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("failed to list images: %w", err)
	}
	
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("list images failed: %s", result.Stderr)
	}
	
	// Parse v2.x enhanced JSON output
	var images []*types.Image
	if err := m.executor.ParseJSONOutput(result.Stdout, &images); err != nil {
		return nil, fmt.Errorf("failed to parse v2 image list: %w", err)
	}
	
	return &types.ListImagesResponse{
		Images:   images,
		Metadata: result.Metadata,
	}, nil
}

// RemoveImage removes an image with v2.x enhanced cleanup
func (m *V2ImageManager) RemoveImage(ctx context.Context, request *types.RemoveImageRequest) (*types.RemoveImageResponse, error) {
	args := []string{"image", "rm"}
	
	if request.Force {
		args = append(args, "--force")
	}
	
	// v2.x enhanced pruning
	if request.NoPrune {
		args = append(args, "--no-prune")
	}
	
	args = append(args, request.ImageID)
	
	result, err := m.executor.Execute(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("failed to remove image: %w", err)
	}
	
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("remove image failed: %s", result.Stderr)
	}
	
	return &types.RemoveImageResponse{
		Metadata: result.Metadata,
	}, nil
}

// InspectImage inspects an image with v2.x enhanced details
func (m *V2ImageManager) InspectImage(ctx context.Context, request *types.InspectImageRequest) (*types.InspectImageResponse, error) {
	args := []string{"image", "inspect", request.ImageID}
	
	result, err := m.executor.Execute(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect image: %w", err)
	}
	
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("inspect image failed: %s", result.Stderr)
	}
	
	var images []*types.Image
	if err := m.executor.ParseJSONOutput(result.Stdout, &images); err != nil {
		return nil, fmt.Errorf("failed to parse v2 image inspect: %w", err)
	}
	
	if len(images) == 0 {
		return nil, fmt.Errorf("image not found")
	}
	
	return &types.InspectImageResponse{
		Image:    images[0],
		Metadata: result.Metadata,
	}, nil
}

// BuildImage builds an image with v2.x enhanced build features
func (m *V2ImageManager) BuildImage(ctx context.Context, request *types.BuildImageRequest) (*interfaces.BuildStream, error) {
	args := []string{"image", "build"}
	
	if request.Tag != "" {
		args = append(args, "--tag", request.Tag)
	}
	
	// v2.x enhanced build features
	if request.Platform != "" {
		args = append(args, "--platform", request.Platform)
	}
	
	if request.Target != "" {
		args = append(args, "--target", request.Target)
	}
	
	// v2.x enhanced progress
	args = append(args, "--progress", "auto")
	
	args = append(args, request.ContextPath)
	
	streamResult, err := m.executor.ExecuteStream(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("failed to build image: %w", err)
	}
	
	return &interfaces.BuildStream{
		ProgressChan: streamResult.LinesChan,
		ErrorsChan:   streamResult.ErrorsChan,
		Cancel:       streamResult.Cancel,
	}, nil
}

// V2NetworkManager implements network management for nerdctl v2.x
type V2NetworkManager struct {
	executor *V2CommandExecutor
	config   *config.Config
	logger   *logrus.Entry
}

// NewV2NetworkManager creates a new V2NetworkManager
func NewV2NetworkManager(executor *V2CommandExecutor, config *config.Config, logger *logrus.Logger) (*V2NetworkManager, error) {
	return &V2NetworkManager{
		executor: executor,
		config:   config,
		logger:   logger.WithField("component", "v2-network-manager"),
	}, nil
}

// CreateNetwork creates a network with v2.x enhanced features
func (m *V2NetworkManager) CreateNetwork(ctx context.Context, request *types.CreateNetworkRequest) (*types.CreateNetworkResponse, error) {
	args := []string{"network", "create"}
	
	if request.Driver != "" {
		args = append(args, "--driver", request.Driver)
	}
	
	// v2.x enhanced IPAM configuration
	if request.IPAM != nil {
		if request.IPAM.Subnet != "" {
			args = append(args, "--subnet", request.IPAM.Subnet)
		}
		if request.IPAM.Gateway != "" {
			args = append(args, "--gateway", request.IPAM.Gateway)
		}
	}
	
	args = append(args, request.Name)
	
	result, err := m.executor.Execute(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("failed to create network: %w", err)
	}
	
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("create network failed: %s", result.Stderr)
	}
	
	return &types.CreateNetworkResponse{
		ID:       strings.TrimSpace(result.Stdout),
		Metadata: result.Metadata,
	}, nil
}

// ListNetworks lists networks with v2.x enhanced filtering
func (m *V2NetworkManager) ListNetworks(ctx context.Context, request *types.ListNetworksRequest) (*types.ListNetworksResponse, error) {
	args := []string{"network", "ls", "--format", "json"}
	
	// v2.x enhanced filtering
	for key, values := range request.Filters {
		for _, value := range values {
			args = append(args, "--filter", fmt.Sprintf("%s=%s", key, value))
		}
	}
	
	result, err := m.executor.Execute(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("failed to list networks: %w", err)
	}
	
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("list networks failed: %s", result.Stderr)
	}
	
	var networks []*types.Network
	if err := m.executor.ParseJSONOutput(result.Stdout, &networks); err != nil {
		return nil, fmt.Errorf("failed to parse v2 network list: %w", err)
	}
	
	return &types.ListNetworksResponse{
		Networks: networks,
		Metadata: result.Metadata,
	}, nil
}

// RemoveNetwork removes a network with v2.x enhanced cleanup
func (m *V2NetworkManager) RemoveNetwork(ctx context.Context, request *types.RemoveNetworkRequest) (*types.RemoveNetworkResponse, error) {
	args := []string{"network", "rm", request.ID}
	
	result, err := m.executor.Execute(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("failed to remove network: %w", err)
	}
	
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("remove network failed: %s", result.Stderr)
	}
	
	return &types.RemoveNetworkResponse{
		Metadata: result.Metadata,
	}, nil
}

// InspectNetwork inspects a network with v2.x enhanced details
func (m *V2NetworkManager) InspectNetwork(ctx context.Context, request *types.InspectNetworkRequest) (*types.InspectNetworkResponse, error) {
	args := []string{"network", "inspect", request.ID}
	
	result, err := m.executor.Execute(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect network: %w", err)
	}
	
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("inspect network failed: %s", result.Stderr)
	}
	
	var networks []*types.Network
	if err := m.executor.ParseJSONOutput(result.Stdout, &networks); err != nil {
		return nil, fmt.Errorf("failed to parse v2 network inspect: %w", err)
	}
	
	if len(networks) == 0 {
		return nil, fmt.Errorf("network not found")
	}
	
	return &types.InspectNetworkResponse{
		Network:  networks[0],
		Metadata: result.Metadata,
	}, nil
}

// V2VolumeManager implements volume management for nerdctl v2.x
type V2VolumeManager struct {
	executor *V2CommandExecutor
	config   *config.Config
	logger   *logrus.Entry
}

// NewV2VolumeManager creates a new V2VolumeManager
func NewV2VolumeManager(executor *V2CommandExecutor, config *config.Config, logger *logrus.Logger) (*V2VolumeManager, error) {
	return &V2VolumeManager{
		executor: executor,
		config:   config,
		logger:   logger.WithField("component", "v2-volume-manager"),
	}, nil
}

// CreateVolume creates a volume with v2.x enhanced features
func (m *V2VolumeManager) CreateVolume(ctx context.Context, request *types.CreateVolumeRequest) (*types.CreateVolumeResponse, error) {
	args := []string{"volume", "create"}
	
	if request.Driver != "" {
		args = append(args, "--driver", request.Driver)
	}
	
	// v2.x enhanced driver options
	for key, value := range request.DriverOpts {
		args = append(args, "--opt", fmt.Sprintf("%s=%s", key, value))
	}
	
	args = append(args, request.Name)
	
	result, err := m.executor.Execute(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("failed to create volume: %w", err)
	}
	
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("create volume failed: %s", result.Stderr)
	}
	
	return &types.CreateVolumeResponse{
		Name:     strings.TrimSpace(result.Stdout),
		Metadata: result.Metadata,
	}, nil
}

// ListVolumes lists volumes with v2.x enhanced metadata
func (m *V2VolumeManager) ListVolumes(ctx context.Context, request *types.ListVolumesRequest) (*types.ListVolumesResponse, error) {
	args := []string{"volume", "ls", "--format", "json"}
	
	// v2.x enhanced filtering
	for key, values := range request.Filters {
		for _, value := range values {
			args = append(args, "--filter", fmt.Sprintf("%s=%s", key, value))
		}
	}
	
	result, err := m.executor.Execute(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("failed to list volumes: %w", err)
	}
	
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("list volumes failed: %s", result.Stderr)
	}
	
	var volumes []*types.Volume
	if err := m.executor.ParseJSONOutput(result.Stdout, &volumes); err != nil {
		return nil, fmt.Errorf("failed to parse v2 volume list: %w", err)
	}
	
	return &types.ListVolumesResponse{
		Volumes:  volumes,
		Metadata: result.Metadata,
	}, nil
}

// RemoveVolume removes a volume with v2.x enhanced cleanup
func (m *V2VolumeManager) RemoveVolume(ctx context.Context, request *types.RemoveVolumeRequest) (*types.RemoveVolumeResponse, error) {
	args := []string{"volume", "rm"}
	
	if request.Force {
		args = append(args, "--force")
	}
	
	args = append(args, request.Name)
	
	result, err := m.executor.Execute(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("failed to remove volume: %w", err)
	}
	
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("remove volume failed: %s", result.Stderr)
	}
	
	return &types.RemoveVolumeResponse{
		Metadata: result.Metadata,
	}, nil
}

// InspectVolume inspects a volume with v2.x enhanced details
func (m *V2VolumeManager) InspectVolume(ctx context.Context, request *types.InspectVolumeRequest) (*types.InspectVolumeResponse, error) {
	args := []string{"volume", "inspect", request.Name}
	
	result, err := m.executor.Execute(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect volume: %w", err)
	}
	
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("inspect volume failed: %s", result.Stderr)
	}
	
	var volumes []*types.Volume
	if err := m.executor.ParseJSONOutput(result.Stdout, &volumes); err != nil {
		return nil, fmt.Errorf("failed to parse v2 volume inspect: %w", err)
	}
	
	if len(volumes) == 0 {
		return nil, fmt.Errorf("volume not found")
	}
	
	return &types.InspectVolumeResponse{
		Volume:   volumes[0],
		Metadata: result.Metadata,
	}, nil
}

// V2ComposeManager implements compose management for nerdctl v2.x
type V2ComposeManager struct {
	executor *V2CommandExecutor
	config   *config.Config
	logger   *logrus.Entry
}

// NewV2ComposeManager creates a new V2ComposeManager
func NewV2ComposeManager(executor *V2CommandExecutor, config *config.Config, logger *logrus.Logger) (*V2ComposeManager, error) {
	return &V2ComposeManager{
		executor: executor,
		config:   config,
		logger:   logger.WithField("component", "v2-compose-manager"),
	}, nil
}

// Up starts compose services with v2.x enhanced orchestration
func (m *V2ComposeManager) Up(ctx context.Context, request *types.ComposeUpRequest) (*interfaces.ComposeStream, error) {
	args := []string{"compose"}
	
	// v2.x enhanced compose file support
	for _, file := range request.ComposeFiles {
		args = append(args, "-f", file)
	}
	
	args = append(args, "up")
	
	if request.Detach {
		args = append(args, "--detach")
	}
	
	// v2.x enhanced service selection
	args = append(args, request.Services...)
	
	streamResult, err := m.executor.ExecuteStream(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("failed to run compose up: %w", err)
	}
	
	return &interfaces.ComposeStream{
		OutputChan: streamResult.LinesChan,
		ErrorsChan: streamResult.ErrorsChan,
		Cancel:     streamResult.Cancel,
	}, nil
}

// Down stops compose services with v2.x enhanced cleanup
func (m *V2ComposeManager) Down(ctx context.Context, request *types.ComposeDownRequest) (*types.ComposeDownResponse, error) {
	args := []string{"compose"}
	
	for _, file := range request.ComposeFiles {
		args = append(args, "-f", file)
	}
	
	args = append(args, "down")
	
	if request.RemoveVolumes {
		args = append(args, "--volumes")
	}
	
	if request.RemoveImages != "" {
		args = append(args, "--rmi", request.RemoveImages)
	}
	
	result, err := m.executor.Execute(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("failed to run compose down: %w", err)
	}
	
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("compose down failed: %s", result.Stderr)
	}
	
	return &types.ComposeDownResponse{
		Metadata: result.Metadata,
	}, nil
}

// V2SystemManager implements system management for nerdctl v2.x
type V2SystemManager struct {
	executor *V2CommandExecutor
	config   *config.Config
	logger   *logrus.Entry
}

// NewV2SystemManager creates a new V2SystemManager
func NewV2SystemManager(executor *V2CommandExecutor, config *config.Config, logger *logrus.Logger) (*V2SystemManager, error) {
	return &V2SystemManager{
		executor: executor,
		config:   config,
		logger:   logger.WithField("component", "v2-system-manager"),
	}, nil
}

// GetSystemInfo gets system information with v2.x enhanced details
func (m *V2SystemManager) GetSystemInfo(ctx context.Context, request *types.GetSystemInfoRequest) (*types.GetSystemInfoResponse, error) {
	args := []string{"system", "info", "--format", "json"}
	
	result, err := m.executor.Execute(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("failed to get system info: %w", err)
	}
	
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("system info failed: %s", result.Stderr)
	}
	
	var systemInfo *types.SystemInfo
	if err := m.executor.ParseJSONOutput(result.Stdout, &systemInfo); err != nil {
		return nil, fmt.Errorf("failed to parse v2 system info: %w", err)
	}
	
	return &types.GetSystemInfoResponse{
		SystemInfo: systemInfo,
		Metadata:   result.Metadata,
	}, nil
}

// Prune prunes system resources with v2.x enhanced cleanup
func (m *V2SystemManager) Prune(ctx context.Context, request *types.SystemPruneRequest) (*types.SystemPruneResponse, error) {
	args := []string{"system", "prune"}
	
	if request.All {
		args = append(args, "--all")
	}
	
	if request.Force {
		args = append(args, "--force")
	}
	
	// v2.x enhanced volume pruning
	if request.Volumes {
		args = append(args, "--volumes")
	}
	
	result, err := m.executor.Execute(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("failed to prune system: %w", err)
	}
	
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("system prune failed: %s", result.Stderr)
	}
	
	return &types.SystemPruneResponse{
		SpaceReclaimed: result.Stdout,
		Metadata:       result.Metadata,
	}, nil
}