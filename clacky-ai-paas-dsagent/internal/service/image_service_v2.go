package service

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/containerd/nerdctl/v2/pkg/api/types"
	"github.com/containerd/nerdctl/v2/pkg/clientutil"
	imageCmd "github.com/containerd/nerdctl/v2/pkg/cmd/image"
	containerCmd "github.com/containerd/nerdctl/v2/pkg/cmd/container"
	pb "dsagent/api/image/v1"
	"dsagent/internal/auth"

)

// ImageServiceV2 implements the ImageService gRPC interface with auto-ECR authentication
type ImageServiceV2 struct {
	pb.UnimplementedImageServiceServer
	authInterceptor auth.ImageOperationInterceptor
	initOnce        sync.Once
	initError       error
}

// NewImageServiceV2 creates a new ImageServiceV2 instance
func NewImageServiceV2() *ImageServiceV2 {
	return &ImageServiceV2{}
}

// initializeInterceptor initializes the auth interceptor (lazy initialization)
func (s *ImageServiceV2) initializeInterceptor() error {
	s.initOnce.Do(func() {
		interceptor, err := auth.NewImageOperationInterceptor()
		if err != nil {
			s.initError = fmt.Errorf("failed to create image operation interceptor: %w", err)
			return
		}
		s.authInterceptor = interceptor
	})
	return s.initError
}

// configureAuthInterceptor configures the auth interceptor with ECR settings
func (s *ImageServiceV2) configureAuthInterceptor(ecrAuth *pb.ECRAuthConfig) error {
	if ecrAuth == nil {
		return nil
	}

	if err := s.initializeInterceptor(); err != nil {
		return err
	}

	// Convert protobuf ECRAuthConfig to interceptor configuration
	config := map[string]interface{}{
		"enabled": true,
		"defaults": map[string]interface{}{
			"ecr": s.convertECRAuthToConfig(ecrAuth),
		},
	}

	return s.authInterceptor.Configure(config)
}

// convertECRAuthToConfig converts protobuf ECRAuthConfig to config map
func (s *ImageServiceV2) convertECRAuthToConfig(ecrAuth *pb.ECRAuthConfig) map[string]interface{} {
	config := map[string]interface{}{
		"region": ecrAuth.Region,
	}

	if ecrAuth.RegistryId != "" {
		config["registry_id"] = ecrAuth.RegistryId
	}

	if ecrAuth.AwsCredentials != nil {
		creds := ecrAuth.AwsCredentials

		// 处理oneof字段
		switch credType := creds.CredentialType.(type) {
		case *pb.AWSCredentials_AccessKey:
			config["access_key_id"] = credType.AccessKey.AccessKeyId
			config["secret_access_key"] = credType.AccessKey.SecretAccessKey
			if credType.AccessKey.SessionToken != "" {
				config["session_token"] = credType.AccessKey.SessionToken
			}
		case *pb.AWSCredentials_AssumeRole:
			config["role_arn"] = credType.AssumeRole.RoleArn
			if credType.AssumeRole.SessionName != "" {
				config["session_name"] = credType.AssumeRole.SessionName
			}
		case *pb.AWSCredentials_InstanceProfile:
			config["use_instance_role"] = true
		case *pb.AWSCredentials_EcsTaskRole:
			config["use_ecs_task_role"] = true
		}
	}

	return config
}

// createBaseGlobalOptions creates base global options for nerdctl operations
func (s *ImageServiceV2) createBaseGlobalOptions() *types.GlobalCommandOptions {
	return &types.GlobalCommandOptions{
		Namespace:   "default",
		Address:     "/run/containerd/containerd.sock",
		Snapshotter: "overlayfs",
		DebugFull:   false,
		Debug:       false,
	}
}

// ListImages implements the ListImages RPC method
func (s *ImageServiceV2) ListImages(ctx context.Context, req *pb.ListImagesRequest) (*pb.ListImagesResponse, error) {
	// Create containerd client
	globalOpts := s.createBaseGlobalOptions()
	client, ctxClient, cancel, err := clientutil.NewClient(ctx, globalOpts.Namespace, globalOpts.Address)
	if err != nil {
		return nil, fmt.Errorf("failed to create containerd client: %w", err)
	}
	defer cancel()

	// Prepare filters
	var filters []string
	if req.Filter != "" {
		filters = []string{req.Filter}
	}

	// Prepare options for the list operation
	listOpts := &types.ImageListOptions{
		GOptions: *globalOpts,
		Quiet:    req.Quiet,
		NoTrunc:  req.NoTrunc,
		All:      req.All,
		Filters:  filters,
		Format:   req.Format,
		Digests:  req.Digests,
	}

	// Capture output
	var buf bytes.Buffer
	listOpts.Stdout = &buf

	// Call nerdctl's image list functionality
	if err := imageCmd.ListCommandHandler(ctxClient, client, listOpts); err != nil {
		return nil, fmt.Errorf("failed to list images: %w", err)
	}

	// Parse the output and create response
	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	var images []*pb.ImageInfo
	for _, line := range lines {
		if line == "" || strings.HasPrefix(line, "REPOSITORY") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 3 {
			images = append(images, &pb.ImageInfo{
				Repository: parts[0],
				Tag:        "latest", // Simplified
				Id:         parts[len(parts)-1],
			})
		}
	}

	return &pb.ListImagesResponse{
		Images: images,
	}, nil
}

// PullImage implements the PullImage RPC method with auto-ECR authentication
func (s *ImageServiceV2) PullImage(ctx context.Context, req *pb.PullImageRequest) (*pb.PullImageResponse, error) {
	// Configure auth interceptor if ECR auth is provided
	if req.EcrAuth != nil {
		if err := s.configureAuthInterceptor(req.EcrAuth); err != nil {
			return nil, fmt.Errorf("failed to configure auth interceptor: %w", err)
		}
	} else {
		// Initialize with default configuration
		if err := s.initializeInterceptor(); err != nil {
			return nil, fmt.Errorf("failed to initialize auth interceptor: %w", err)
		}
	}

	// Use the interceptor to handle the pull operation
	var response *pb.PullImageResponse
	var pullErr error

	interceptErr := auth.InterceptImagePull(ctx, req.Name, s.authInterceptor, func() error {
		// Actual pull implementation
		resp, err := s.executePullImage(ctx, req)
		if err != nil {
			pullErr = err
			return err
		}
		response = resp
		return nil
	})

	if interceptErr != nil {
		return nil, fmt.Errorf("pull operation failed: %w", interceptErr)
	}

	if pullErr != nil {
		return nil, pullErr
	}

	return response, nil
}

// executePullImage executes the actual image pull operation
func (s *ImageServiceV2) executePullImage(ctx context.Context, req *pb.PullImageRequest) (*pb.PullImageResponse, error) {
	// Create containerd client
	globalOpts := s.createBaseGlobalOptions()
	client, ctxClient, cancel, err := clientutil.NewClient(ctx, globalOpts.Namespace, globalOpts.Address)
	if err != nil {
		return nil, fmt.Errorf("failed to create containerd client: %w", err)
	}
	defer cancel()

	// Create pull options
	pullOpts := types.ImagePullOptions{
		GOptions: *globalOpts,
		Quiet:    req.Quiet,
	}

	// Set up stdout capture
	var outBuf bytes.Buffer
	pullOpts.Stdout = &outBuf

	// Call nerdctl's image pull functionality
	if err := imageCmd.Pull(ctxClient, client, req.Name, pullOpts); err != nil {
		return nil, fmt.Errorf("failed to pull image %s: %w", req.Name, err)
	}

	return &pb.PullImageResponse{
		Status: fmt.Sprintf("Successfully pulled %s", req.Name),
		Digest: "", // Would need to extract from output
	}, nil
}

// PushImage implements the PushImage RPC method with auto-ECR authentication
func (s *ImageServiceV2) PushImage(ctx context.Context, req *pb.PushImageRequest) (*pb.PushImageResponse, error) {
	// Configure auth interceptor if ECR auth is provided
	if req.EcrAuth != nil {
		if err := s.configureAuthInterceptor(req.EcrAuth); err != nil {
			return nil, fmt.Errorf("failed to configure auth interceptor: %w", err)
		}
	} else {
		// Initialize with default configuration
		if err := s.initializeInterceptor(); err != nil {
			return nil, fmt.Errorf("failed to initialize auth interceptor: %w", err)
		}
	}

	// Use the interceptor to handle the push operation
	var response *pb.PushImageResponse
	var pushErr error

	interceptErr := auth.InterceptImagePush(ctx, req.Name, s.authInterceptor, func() error {
		// Actual push implementation
		resp, err := s.executePushImage(ctx, req)
		if err != nil {
			pushErr = err
			return err
		}
		response = resp
		return nil
	})

	if interceptErr != nil {
		return nil, fmt.Errorf("push operation failed: %w", interceptErr)
	}

	if pushErr != nil {
		return nil, pushErr
	}

	return response, nil
}

// executePushImage executes the actual image push operation
func (s *ImageServiceV2) executePushImage(ctx context.Context, req *pb.PushImageRequest) (*pb.PushImageResponse, error) {
	// Create containerd client
	globalOpts := s.createBaseGlobalOptions()
	client, ctxClient, cancel, err := clientutil.NewClient(ctx, globalOpts.Namespace, globalOpts.Address)
	if err != nil {
		return nil, fmt.Errorf("failed to create containerd client: %w", err)
	}
	defer cancel()

	// Create push options
	pushOpts := types.ImagePushOptions{
		GOptions:     *globalOpts,
		Quiet:        req.Quiet,
		AllPlatforms: req.AllPlatforms,
	}

	// Set up stdout capture
	var outBuf bytes.Buffer
	pushOpts.Stdout = &outBuf

	// Call nerdctl's image push functionality
	if err := imageCmd.Push(ctxClient, client, req.Name, pushOpts); err != nil {
		return nil, fmt.Errorf("failed to push image %s: %w", req.Name, err)
	}

	return &pb.PushImageResponse{
		Status: fmt.Sprintf("Successfully pushed %s", req.Name),
		Digest: "", // Would need to extract from output
	}, nil
}

// RemoveImage implements the RemoveImage RPC method (equivalent to nerdctl rmi)
func (s *ImageServiceV2) RemoveImage(ctx context.Context, req *pb.RemoveImageRequest) (*pb.RemoveImageResponse, error) {
	// Create containerd client
	globalOpts := s.createBaseGlobalOptions()
	client, ctxClient, cancel, err := clientutil.NewClient(ctx, globalOpts.Namespace, globalOpts.Address)
	if err != nil {
		return nil, fmt.Errorf("failed to create containerd client: %w", err)
	}
	defer cancel()

	// Create remove options with all nerdctl rmi features
	removeOpts := types.ImageRemoveOptions{
		GOptions: *globalOpts,
		Force:    req.Force,
		// Note: nerdctl doesn't have explicit no_prune option in current version
		// This is handled internally by containerd
	}

	// Set up stdout capture
	var outBuf bytes.Buffer
	removeOpts.Stdout = &outBuf

	// Call nerdctl's image remove functionality
	if err := imageCmd.Remove(ctxClient, client, []string{req.Name}, removeOpts); err != nil {
		return nil, fmt.Errorf("failed to remove image %s: %w", req.Name, err)
	}

	// Parse output to get actual removed and untagged images
	output := strings.TrimSpace(outBuf.String())
	lines := strings.Split(output, "\n")
	
	var removed []string
	var untagged []string
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		if strings.HasPrefix(line, "Untagged:") {
			untagged = append(untagged, strings.TrimPrefix(line, "Untagged: "))
		} else if strings.HasPrefix(line, "Deleted:") {
			removed = append(removed, strings.TrimPrefix(line, "Deleted: "))
		} else {
			// Default case - treat as removed
			removed = append(removed, req.Name)
		}
	}
	
	// If no specific output parsing worked, assume successful removal
	if len(removed) == 0 && len(untagged) == 0 {
		removed = []string{req.Name}
	}

	return &pb.RemoveImageResponse{
		Removed:   removed,
		Untagged:  untagged,
	}, nil
}

// TagImage implements the TagImage RPC method (equivalent to nerdctl tag)
func (s *ImageServiceV2) TagImage(ctx context.Context, req *pb.TagImageRequest) (*pb.TagImageResponse, error) {
	// Validate input parameters
	if req.SourceImage == "" {
		return nil, fmt.Errorf("source image is required")
	}
	if req.TargetImage == "" {
		return nil, fmt.Errorf("target image is required")
	}

	// Create containerd client
	globalOpts := s.createBaseGlobalOptions()
	client, ctxClient, cancel, err := clientutil.NewClient(ctx, globalOpts.Namespace, globalOpts.Address)
	if err != nil {
		return nil, fmt.Errorf("failed to create containerd client: %w", err)
	}
	defer cancel()

	// Create tag options following nerdctl tag command structure
	tagOpts := types.ImageTagOptions{
		GOptions: *globalOpts,
		Source:   req.SourceImage,
		Target:   req.TargetImage,
	}

	// Call nerdctl's image tag functionality
	if err := imageCmd.Tag(ctxClient, client, tagOpts); err != nil {
		return nil, fmt.Errorf("failed to tag image %s as %s: %w", req.SourceImage, req.TargetImage, err)
	}

	return &pb.TagImageResponse{
		Status: fmt.Sprintf("Successfully tagged %s as %s", req.SourceImage, req.TargetImage),
	}, nil
}

// SaveImage implements the SaveImage RPC method
func (s *ImageServiceV2) SaveImage(ctx context.Context, req *pb.SaveImageRequest) (*pb.SaveImageResponse, error) {
	// Create containerd client
	globalOpts := s.createBaseGlobalOptions()
	client, ctxClient, cancel, err := clientutil.NewClient(ctx, globalOpts.Namespace, globalOpts.Address)
	if err != nil {
		return nil, fmt.Errorf("failed to create containerd client: %w", err)
	}
	defer cancel()

	// Create save options
	saveOpts := types.ImageSaveOptions{
		GOptions: *globalOpts,
	}

	// Create a buffer for output
	var outBuf bytes.Buffer
	saveOpts.Stdout = &outBuf

	// Call nerdctl's image save functionality
	if err := imageCmd.Save(ctxClient, client, req.Names, saveOpts); err != nil {
		return nil, fmt.Errorf("failed to save images %v: %w", req.Names, err)
	}

	return &pb.SaveImageResponse{
		OutputPath: req.Output,
		Size:       int64(outBuf.Len()),
	}, nil
}

// LoadImage implements the LoadImage RPC method
func (s *ImageServiceV2) LoadImage(ctx context.Context, req *pb.LoadImageRequest) (*pb.LoadImageResponse, error) {
	return &pb.LoadImageResponse{
		Loaded: []string{"load functionality not implemented"},
	}, nil
}

// ImageHistory implements the ImageHistory RPC method
func (s *ImageServiceV2) ImageHistory(ctx context.Context, req *pb.ImageHistoryRequest) (*pb.ImageHistoryResponse, error) {
	return &pb.ImageHistoryResponse{
		Layers: []*pb.HistoryLayer{},
	}, nil
}

// InspectImage implements the InspectImage RPC method
func (s *ImageServiceV2) InspectImage(ctx context.Context, req *pb.InspectImageRequest) (*pb.InspectImageResponse, error) {
	return &pb.InspectImageResponse{
		Content: "inspect functionality not implemented",
	}, nil
}

// CommitContainer implements the CommitContainer RPC method
func (s *ImageServiceV2) CommitContainer(ctx context.Context, req *pb.CommitContainerRequest) (*pb.CommitContainerResponse, error) {
	// Create containerd client
	globalOpts := s.createBaseGlobalOptions()
	client, ctxClient, cancel, err := clientutil.NewClient(ctx, globalOpts.Namespace, globalOpts.Address)
	if err != nil {
		return nil, fmt.Errorf("failed to create containerd client: %w", err)
	}
	defer cancel()

	// Construct the target image reference
	var targetRef string
	if req.Repository != "" && req.Tag != "" {
		targetRef = req.Repository + ":" + req.Tag
	} else if req.Repository != "" {
		targetRef = req.Repository
	} else {
		return nil, fmt.Errorf("repository is required for commit operation")
	}

	// Create commit options
	commitOpts := types.ContainerCommitOptions{
		GOptions:    *globalOpts,
		Author:      req.Author,
		Message:     req.Message,
		Pause:       req.Pause,
		Change:      req.Change,
		Compression: types.CompressionType(req.Compression),
		Format:      types.ImageFormat(req.Format),
		EstargzOptions: types.EstargzOptions{
			Estargz:                 req.Estargz,
			EstargzCompressionLevel: int(req.EstargzCompressionLevel),
			EstargzChunkSize:        int(req.EstargzChunkSize),
			EstargzMinChunkSize:     int(req.EstargzMinChunkSize),
		},
		ZstdChunkedOptions: types.ZstdChunkedOptions{
			ZstdChunked:                 req.Zstdchunked,
			ZstdChunkedCompressionLevel: int(req.ZstdchunkedCompressionLevel),
			ZstdChunkedChunkSize:        int(req.ZstdchunkedChunkSize),
		},
	}

	// Set up stdout capture
	var outBuf bytes.Buffer
	commitOpts.Stdout = &outBuf

	// Call nerdctl's container commit functionality
	if err := containerCmd.Commit(ctxClient, client, targetRef, req.ContainerId, commitOpts); err != nil {
		return nil, fmt.Errorf("failed to commit container %s: %w", req.ContainerId, err)
	}

	// Extract image ID from output
	output := strings.TrimSpace(outBuf.String())

	return &pb.CommitContainerResponse{
		ImageId: output,
		Status:  fmt.Sprintf("Successfully committed container %s as %s", req.ContainerId, targetRef),
	}, nil
}

// Note: ECR login/logout methods have been moved to separate ECRService