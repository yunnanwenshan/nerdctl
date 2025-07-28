package interfaces

import (
	"context"

	"github.com/containerd/nerdctl-grpc-server/pkg/types"
)

// NetworkManager defines the interface for network management operations
// This abstraction allows different nerdctl versions to provide their own implementations
type NetworkManager interface {
	// Network lifecycle operations
	CreateNetwork(ctx context.Context, req *types.CreateNetworkRequest) (*types.CreateNetworkResponse, error)
	RemoveNetwork(ctx context.Context, req *types.RemoveNetworkRequest) (*types.RemoveNetworkResponse, error)
	
	// Network information operations
	ListNetworks(ctx context.Context, req *types.ListNetworksRequest) (*types.ListNetworksResponse, error)
	InspectNetwork(ctx context.Context, req *types.InspectNetworkRequest) (*types.InspectNetworkResponse, error)
	
	// Network container operations
	ConnectContainer(ctx context.Context, req *types.ConnectContainerRequest) (*types.ConnectContainerResponse, error)
	DisconnectContainer(ctx context.Context, req *types.DisconnectContainerRequest) (*types.DisconnectContainerResponse, error)
	
	// Network cleanup operations
	PruneNetworks(ctx context.Context, req *types.PruneNetworksRequest) (*types.PruneNetworksResponse, error)
}

// VolumeManager defines the interface for volume management operations
type VolumeManager interface {
	// Volume lifecycle operations
	CreateVolume(ctx context.Context, req *types.CreateVolumeRequest) (*types.CreateVolumeResponse, error)
	RemoveVolume(ctx context.Context, req *types.RemoveVolumeRequest) (*types.RemoveVolumeResponse, error)
	
	// Volume information operations
	ListVolumes(ctx context.Context, req *types.ListVolumesRequest) (*types.ListVolumesResponse, error)
	InspectVolume(ctx context.Context, req *types.InspectVolumeRequest) (*types.InspectVolumeResponse, error)
	
	// Volume cleanup operations
	PruneVolumes(ctx context.Context, req *types.PruneVolumesRequest) (*types.PruneVolumesResponse, error)
}

// ComposeManager defines the interface for Docker Compose operations
type ComposeManager interface {
	// Compose project lifecycle
	ComposeUp(ctx context.Context, req *types.ComposeUpRequest, stream chan<- *types.ComposeUpResponse) error
	ComposeDown(ctx context.Context, req *types.ComposeDownRequest) (*types.ComposeDownResponse, error)
	ComposeStart(ctx context.Context, req *types.ComposeStartRequest) (*types.ComposeStartResponse, error)
	ComposeStop(ctx context.Context, req *types.ComposeStopRequest) (*types.ComposeStopResponse, error)
	ComposeRestart(ctx context.Context, req *types.ComposeRestartRequest) (*types.ComposeRestartResponse, error)
	
	// Compose service operations
	ComposePs(ctx context.Context, req *types.ComposePsRequest) (*types.ComposePsResponse, error)
	ComposeLogs(ctx context.Context, req *types.ComposeLogsRequest, stream chan<- *types.ComposeLogsResponse) error
	ComposeExec(ctx context.Context, req *types.ComposeExecRequest) (*types.ComposeExecResponse, error)
	
	// Compose build operations
	ComposeBuild(ctx context.Context, req *types.ComposeBuildRequest, stream chan<- *types.ComposeBuildResponse) error
	ComposePull(ctx context.Context, req *types.ComposePullRequest, stream chan<- *types.ComposePullResponse) error
	ComposePush(ctx context.Context, req *types.ComposePushRequest, stream chan<- *types.ComposePushResponse) error
	
	// Compose configuration operations
	ComposeConfig(ctx context.Context, req *types.ComposeConfigRequest) (*types.ComposeConfigResponse, error)
	ComposeValidate(ctx context.Context, req *types.ComposeValidateRequest) (*types.ComposeValidateResponse, error)
	
	// Compose scaling operations
	ComposeScale(ctx context.Context, req *types.ComposeScaleRequest) (*types.ComposeScaleResponse, error)
	
	// Compose project management
	ListComposeProjects(ctx context.Context, req *types.ListComposeProjectsRequest) (*types.ListComposeProjectsResponse, error)
	RemoveComposeProject(ctx context.Context, req *types.RemoveComposeProjectRequest) (*types.RemoveComposeProjectResponse, error)
}

// SystemManager defines the interface for system-level operations
type SystemManager interface {
	// System information
	GetSystemInfo(ctx context.Context, req *types.GetSystemInfoRequest) (*types.GetSystemInfoResponse, error)
	GetVersion(ctx context.Context, req *types.GetVersionRequest) (*types.GetVersionResponse, error)
	GetSystemEvents(ctx context.Context, req *types.GetSystemEventsRequest, stream chan<- *types.SystemEvent) error
	
	// System resource operations
	GetDiskUsage(ctx context.Context, req *types.GetDiskUsageRequest) (*types.GetDiskUsageResponse, error)
	SystemPrune(ctx context.Context, req *types.SystemPruneRequest) (*types.SystemPruneResponse, error)
	
	// System configuration
	GetDaemonConfig(ctx context.Context, req *types.GetDaemonConfigRequest) (*types.GetDaemonConfigResponse, error)
	UpdateDaemonConfig(ctx context.Context, req *types.UpdateDaemonConfigRequest) (*types.UpdateDaemonConfigResponse, error)
}

// NamespaceManager defines the interface for namespace operations
type NamespaceManager interface {
	// Namespace lifecycle
	CreateNamespace(ctx context.Context, req *types.CreateNamespaceRequest) (*types.CreateNamespaceResponse, error)
	RemoveNamespace(ctx context.Context, req *types.RemoveNamespaceRequest) (*types.RemoveNamespaceResponse, error)
	
	// Namespace information
	ListNamespaces(ctx context.Context, req *types.ListNamespacesRequest) (*types.ListNamespacesResponse, error)
	InspectNamespace(ctx context.Context, req *types.InspectNamespaceRequest) (*types.InspectNamespaceResponse, error)
	
	// Namespace operations
	SetDefaultNamespace(ctx context.Context, req *types.SetDefaultNamespaceRequest) (*types.SetDefaultNamespaceResponse, error)
	GetDefaultNamespace(ctx context.Context, req *types.GetDefaultNamespaceRequest) (*types.GetDefaultNamespaceResponse, error)
}