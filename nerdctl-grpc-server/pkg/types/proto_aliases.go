package types

// Proto type aliases to bridge proto package and types package
// This file resolves the missing type definitions by creating aliases to the proto types

import (
	pb "github.com/containerd/nerdctl-grpc-server/api/proto"
)

// Request type aliases
type (
	GetContainerStatsRequest   = pb.GetContainerStatsRequest
	WaitContainerRequest      = pb.WaitContainerRequest
	AttachContainerRequest    = pb.AttachContainerRequest
	ExecContainerRequest      = pb.ExecContainerRequest
	ExecContainerStreamRequest = pb.ExecContainerStreamRequest
	RenameContainerRequest    = pb.RenameContainerRequest
	UpdateContainerRequest    = pb.UpdateContainerRequest
	CopyToContainerRequest    = pb.CopyToContainerRequest
	CopyFromContainerRequest  = pb.CopyFromContainerRequest
	ExportContainerRequest    = pb.ExportContainerRequest
	DiffContainerRequest      = pb.DiffContainerRequest
	CommitContainerRequest    = pb.CommitContainerRequest
	PruneContainersRequest    = pb.PruneContainersRequest
)

// Response type aliases
type (
	WaitContainerResponse         = pb.WaitContainerResponse
	AttachContainerResponse       = pb.AttachContainerResponse
	ExecContainerResponse         = pb.ExecContainerResponse
	ExecContainerStreamResponse   = pb.ExecContainerStreamResponse
	RenameContainerResponse       = pb.RenameContainerResponse
	UpdateContainerResponse       = pb.UpdateContainerResponse
	CopyToContainerResponse       = pb.CopyToContainerResponse
	CopyFromContainerResponse     = pb.CopyFromContainerResponse
	ExportContainerResponse       = pb.ExportContainerResponse
	DiffContainerResponse         = pb.DiffContainerResponse
	CommitContainerResponse       = pb.CommitContainerResponse
	PruneContainersResponse       = pb.PruneContainersResponse
)

// Additional utility types are defined in complete_aliases.go

// Image service types
type (
	PullImageRequest  = pb.PullImageRequest
	PullImageResponse = pb.PullImageResponse
	PushImageRequest  = pb.PushImageRequest
	PushImageResponse = pb.PushImageResponse
	LoadImageRequest  = pb.LoadImageRequest
	SaveImageRequest  = pb.SaveImageRequest
	ListImagesRequest = pb.ListImagesRequest
	ListImagesResponse = pb.ListImagesResponse
	InspectImageRequest = pb.InspectImageRequest
	InspectImageResponse = pb.InspectImageResponse
	RemoveImageRequest = pb.RemoveImageRequest
	RemoveImageResponse = pb.RemoveImageResponse
	TagImageRequest   = pb.TagImageRequest
	TagImageResponse  = pb.TagImageResponse
	PruneImagesRequest = pb.PruneImagesRequest
	PruneImagesResponse = pb.PruneImagesResponse
)

// Network service types
type (
	CreateNetworkRequest  = pb.CreateNetworkRequest
	CreateNetworkResponse = pb.CreateNetworkResponse
	ListNetworksRequest   = pb.ListNetworksRequest
	ListNetworksResponse  = pb.ListNetworksResponse
	InspectNetworkRequest = pb.InspectNetworkRequest
	InspectNetworkResponse = pb.InspectNetworkResponse
	RemoveNetworkRequest  = pb.RemoveNetworkRequest
	RemoveNetworkResponse = pb.RemoveNetworkResponse
	PruneNetworksRequest = pb.PruneNetworksRequest
	PruneNetworksResponse = pb.PruneNetworksResponse
)

// Additional missing types based on interfaces
type (
	LoadImageResponse       = pb.LoadImageResponse
	SaveImageResponse       = pb.SaveImageResponse
)

// Other additional types are defined in complete_aliases.go

// Note: RunContainerStreamResponse, LogEntry, ContainerEvent are already defined in other files