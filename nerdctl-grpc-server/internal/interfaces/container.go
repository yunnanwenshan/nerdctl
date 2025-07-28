package interfaces

import (
	"context"
	"io"

	"github.com/containerd/nerdctl-grpc-server/pkg/types"
)

// ContainerManager defines the interface for container management operations
// This abstraction allows different nerdctl versions to provide their own implementations
type ContainerManager interface {
	// Container lifecycle operations
	CreateContainer(ctx context.Context, req *types.CreateContainerRequest) (*types.CreateContainerResponse, error)
	StartContainer(ctx context.Context, req *types.StartContainerRequest) (*types.StartContainerResponse, error)
	StopContainer(ctx context.Context, req *types.StopContainerRequest) (*types.StopContainerResponse, error)
	RestartContainer(ctx context.Context, req *types.RestartContainerRequest) (*types.RestartContainerResponse, error)
	RemoveContainer(ctx context.Context, req *types.RemoveContainerRequest) (*types.RemoveContainerResponse, error)
	KillContainer(ctx context.Context, req *types.KillContainerRequest) (*types.KillContainerResponse, error)
	
	// Container state operations
	PauseContainer(ctx context.Context, req *types.PauseContainerRequest) (*types.PauseContainerResponse, error)
	UnpauseContainer(ctx context.Context, req *types.UnpauseContainerRequest) (*types.UnpauseContainerResponse, error)
	
	// Combined operations
	RunContainer(ctx context.Context, req *types.RunContainerRequest) (*types.RunContainerResponse, error)
	RunContainerStream(ctx context.Context, req *types.RunContainerRequest, stream chan<- *types.RunContainerStreamResponse) error
	
	// Information and monitoring
	ListContainers(ctx context.Context, req *types.ListContainersRequest) (*types.ListContainersResponse, error)
	InspectContainer(ctx context.Context, req *types.InspectContainerRequest) (*types.InspectContainerResponse, error)
	GetContainerLogs(ctx context.Context, req *types.GetContainerLogsRequest, stream chan<- *types.LogEntry) error
	GetContainerStats(ctx context.Context, req *types.GetContainerStatsRequest, stream chan<- *types.ContainerStats) error
	WaitContainer(ctx context.Context, req *types.WaitContainerRequest) (*types.WaitContainerResponse, error)
	
	// Interactive operations
	AttachContainer(ctx context.Context, stream chan<- *types.AttachContainerResponse) error
	ExecContainer(ctx context.Context, req *types.ExecContainerRequest) (*types.ExecContainerResponse, error)
	ExecContainerStream(ctx context.Context, req *types.ExecContainerStreamRequest, stream chan<- *types.ExecContainerStreamResponse) error
	
	// Management operations
	RenameContainer(ctx context.Context, req *types.RenameContainerRequest) (*types.RenameContainerResponse, error)
	UpdateContainer(ctx context.Context, req *types.UpdateContainerRequest) (*types.UpdateContainerResponse, error)
	
	// File operations
	CopyToContainer(ctx context.Context, stream <-chan *types.CopyToContainerRequest) (*types.CopyToContainerResponse, error)
	CopyFromContainer(ctx context.Context, req *types.CopyFromContainerRequest, stream chan<- *types.CopyFromContainerResponse) error
	
	// Export and snapshot operations
	ExportContainer(ctx context.Context, req *types.ExportContainerRequest, stream chan<- *types.ExportContainerResponse) error
	DiffContainer(ctx context.Context, req *types.DiffContainerRequest) (*types.DiffContainerResponse, error)
	CommitContainer(ctx context.Context, req *types.CommitContainerRequest) (*types.CommitContainerResponse, error)
	
	// Cleanup operations
	PruneContainers(ctx context.Context, req *types.PruneContainersRequest) (*types.PruneContainersResponse, error)
}

// ContainerStreamer defines streaming operations for containers
type ContainerStreamer interface {
	// Streaming operations
	StreamLogs(ctx context.Context, containerID string, options *types.LogOptions, output io.Writer) error
	StreamStats(ctx context.Context, containerID string, options *types.StatsOptions, output io.Writer) error
	StreamAttach(ctx context.Context, containerID string, options *types.AttachOptions, stdin io.Reader, stdout, stderr io.Writer) error
	StreamExec(ctx context.Context, containerID string, cmd []string, options *types.ExecOptions, stdin io.Reader, stdout, stderr io.Writer) error
}

// ContainerEventListener defines container event handling
type ContainerEventListener interface {
	// Event handling
	ListenForEvents(ctx context.Context, filters map[string]string, eventChan chan<- *types.ContainerEvent) error
	GetEventHistory(ctx context.Context, since, until string, filters map[string]string) ([]*types.ContainerEvent, error)
}