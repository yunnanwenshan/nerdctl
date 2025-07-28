package interfaces

import (
	"context"
	"io"

	"github.com/containerd/nerdctl-grpc-server/pkg/types"
)

// ImageManager defines the interface for image management operations
// This abstraction allows different nerdctl versions to provide their own implementations
type ImageManager interface {
	// Image retrieval operations
	PullImage(ctx context.Context, req *types.PullImageRequest, stream chan<- *types.PullImageResponse) error
	PushImage(ctx context.Context, req *types.PushImageRequest, stream chan<- *types.PushImageResponse) error
	LoadImage(ctx context.Context, req *types.LoadImageRequest, stream <-chan *types.LoadImageChunk) (*types.LoadImageResponse, error)
	SaveImage(ctx context.Context, req *types.SaveImageRequest, stream chan<- *types.SaveImageResponse) error
	ImportImage(ctx context.Context, req *types.ImportImageRequest, stream <-chan *types.ImportImageChunk) (*types.ImportImageResponse, error)
	
	// Image information operations
	ListImages(ctx context.Context, req *types.ListImagesRequest) (*types.ListImagesResponse, error)
	InspectImage(ctx context.Context, req *types.InspectImageRequest) (*types.InspectImageResponse, error)
	GetImageHistory(ctx context.Context, req *types.GetImageHistoryRequest) (*types.GetImageHistoryResponse, error)
	
	// Image building operations
	BuildImage(ctx context.Context, req *types.BuildImageRequest, stream chan<- *types.BuildImageResponse) error
	
	// Image modification operations
	TagImage(ctx context.Context, req *types.TagImageRequest) (*types.TagImageResponse, error)
	UntagImage(ctx context.Context, req *types.UntagImageRequest) (*types.UntagImageResponse, error)
	RemoveImage(ctx context.Context, req *types.RemoveImageRequest) (*types.RemoveImageResponse, error)
	
	// Image cleanup operations
	PruneImages(ctx context.Context, req *types.PruneImagesRequest) (*types.PruneImagesResponse, error)
	
	// Image conversion and manipulation
	ConvertImage(ctx context.Context, req *types.ConvertImageRequest) (*types.ConvertImageResponse, error)
	CompressImage(ctx context.Context, req *types.CompressImageRequest) (*types.CompressImageResponse, error)
	
	// Image security operations
	SignImage(ctx context.Context, req *types.SignImageRequest) (*types.SignImageResponse, error)
	VerifyImage(ctx context.Context, req *types.VerifyImageRequest) (*types.VerifyImageResponse, error)
	ScanImage(ctx context.Context, req *types.ScanImageRequest) (*types.ScanImageResponse, error)
	
	// Image registry operations  
	SearchImages(ctx context.Context, req *types.SearchImagesRequest) (*types.SearchImagesResponse, error)
}

// ImageStreamer defines streaming operations for images
type ImageStreamer interface {
	// Streaming pull with progress
	StreamPull(ctx context.Context, imageRef string, options *types.PullOptions, output io.Writer) error
	
	// Streaming push with progress
	StreamPush(ctx context.Context, imageRef string, options *types.PushOptions, output io.Writer) error
	
	// Streaming build with logs
	StreamBuild(ctx context.Context, buildContext io.Reader, dockerfile string, options *types.BuildOptions, output io.Writer) error
	
	// Streaming save/load operations
	StreamSave(ctx context.Context, imageRefs []string, output io.Writer) error
	StreamLoad(ctx context.Context, input io.Reader, output io.Writer) error
}

// ImageRegistry defines registry interaction operations
type ImageRegistry interface {
	// Registry authentication
	Login(ctx context.Context, registry, username, password string) error
	Logout(ctx context.Context, registry string) error
	
	// Registry information
	GetRegistryInfo(ctx context.Context, registry string) (*types.RegistryInfo, error)
	ListRepositories(ctx context.Context, registry string) ([]string, error)
	ListTags(ctx context.Context, repository string) ([]string, error)
	
	// Image manifest operations
	GetManifest(ctx context.Context, imageRef string) (*types.ImageManifest, error)
	PutManifest(ctx context.Context, imageRef string, manifest *types.ImageManifest) error
	
	// Blob operations
	GetBlob(ctx context.Context, registry, digest string) (io.ReadCloser, error)
	PutBlob(ctx context.Context, registry string, content io.Reader) (string, error)
}

// ImageCache defines image caching operations
type ImageCache interface {
	// Cache management
	GetCacheInfo(ctx context.Context) (*types.CacheInfo, error)
	ClearCache(ctx context.Context, filters map[string]string) error
	PruneCache(ctx context.Context, options *types.PruneCacheOptions) (*types.PruneCacheResponse, error)
	
	// Cache optimization
	OptimizeCache(ctx context.Context) error
	ValidateCache(ctx context.Context) (*types.CacheValidationResult, error)
}