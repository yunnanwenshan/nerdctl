package service

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/containerd/nerdctl/v2/pkg/api/types"
	"github.com/containerd/nerdctl/v2/pkg/clientutil"
	imageCmd "github.com/containerd/nerdctl/v2/pkg/cmd/image"
	pb "dsagent/api/image/v1"
	"github.com/spf13/cobra"
)

// ImageService implements the ImageService gRPC interface
type ImageService struct {
	pb.UnimplementedImageServiceServer
	ecrAuthService    *ECRAuthService
	dockerConfigMgr   *DockerConfigManager
}

// NewImageService creates a new ImageService instance
func NewImageService() *ImageService {
	return &ImageService{
		ecrAuthService:  NewECRAuthService(),
		dockerConfigMgr: NewDockerConfigManager(),
	}
}

// createBaseGlobalOptions creates base global options for nerdctl operations
func (s *ImageService) createBaseGlobalOptions() *types.GlobalCommandOptions {
	opts := &types.GlobalCommandOptions{
		Namespace:     "default",
		Address:       "/run/containerd/containerd.sock",
		Snapshotter:   "overlayfs",
		DebugFull:     false,
		Debug:         false,
	}
	return opts
}

// createMockCommand creates a mock cobra command for testing/wrapping purposes
func (s *ImageService) createMockCommand(name string, args []string) *cobra.Command {
	cmd := &cobra.Command{
		Use: name,
		Run: func(cmd *cobra.Command, args []string) {},
	}
	// Set the args
	cmd.SetArgs(args)
	return cmd
}

// ListImages implements the ListImages RPC method
func (s *ImageService) ListImages(ctx context.Context, req *pb.ListImagesRequest) (*pb.ListImagesResponse, error) {
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
		GOptions: *globalOpts, // Pass by value, not pointer
		Quiet:    req.Quiet,
		NoTrunc:  req.NoTrunc,
		All:      req.All,
		Filters:  filters, // Use Filters (plural), not Filter
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
		// This is a simplified parser - you'd want to implement proper parsing
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

// PullImage implements the PullImage RPC method
func (s *ImageService) PullImage(ctx context.Context, req *pb.PullImageRequest) (*pb.PullImageResponse, error) {
	// Create containerd client
	globalOpts := s.createBaseGlobalOptions()
	client, ctxClient, cancel, err := clientutil.NewClient(ctx, globalOpts.Namespace, globalOpts.Address)
	if err != nil {
		return nil, fmt.Errorf("failed to create containerd client: %w", err)
	}
	defer cancel()

	// Auto-detect ECR and login if needed
	if req.EcrAuth != nil {
		if err := s.dockerConfigMgr.AutoDetectECRAndLogin(ctx, req.Name, req.EcrAuth, globalOpts); err != nil {
			return nil, fmt.Errorf("ECR authentication failed: %w", err)
		}
	}

	// Check the actual ImagePullOptions struct for valid fields
	// For now, let's create a basic pull options struct
	pullOpts := types.ImagePullOptions{
		GOptions: *globalOpts, // Pass by value
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

// PushImage implements the PushImage RPC method
func (s *ImageService) PushImage(ctx context.Context, req *pb.PushImageRequest) (*pb.PushImageResponse, error) {
	// Create containerd client
	globalOpts := s.createBaseGlobalOptions()
	client, ctxClient, cancel, err := clientutil.NewClient(ctx, globalOpts.Namespace, globalOpts.Address)
	if err != nil {
		return nil, fmt.Errorf("failed to create containerd client: %w", err)
	}
	defer cancel()

	// Auto-detect ECR and login if needed
	if req.EcrAuth != nil {
		if err := s.dockerConfigMgr.AutoDetectECRAndLogin(ctx, req.Name, req.EcrAuth, globalOpts); err != nil {
			return nil, fmt.Errorf("ECR authentication failed: %w", err)
		}
	}

	// Create basic push options
	pushOpts := types.ImagePushOptions{
		GOptions:     *globalOpts, // Pass by value
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

// RemoveImage implements the RemoveImage RPC method  
func (s *ImageService) RemoveImage(ctx context.Context, req *pb.RemoveImageRequest) (*pb.RemoveImageResponse, error) {
	// Create containerd client
	globalOpts := s.createBaseGlobalOptions()
	client, ctxClient, cancel, err := clientutil.NewClient(ctx, globalOpts.Namespace, globalOpts.Address)
	if err != nil {
		return nil, fmt.Errorf("failed to create containerd client: %w", err)
	}
	defer cancel()

	// Create basic remove options
	removeOpts := types.ImageRemoveOptions{
		GOptions: *globalOpts, // Pass by value
		Force:    req.Force,
	}

	// Set up stdout capture
	var outBuf bytes.Buffer
	removeOpts.Stdout = &outBuf

	// Call nerdctl's image remove functionality
	if err := imageCmd.Remove(ctxClient, client, []string{req.Name}, removeOpts); err != nil {
		return nil, fmt.Errorf("failed to remove image %s: %w", req.Name, err)
	}

	return &pb.RemoveImageResponse{
		Removed: []string{req.Name},
	}, nil
}

// TagImage implements the TagImage RPC method
func (s *ImageService) TagImage(ctx context.Context, req *pb.TagImageRequest) (*pb.TagImageResponse, error) {
	// Create containerd client
	globalOpts := s.createBaseGlobalOptions()
	client, ctxClient, cancel, err := clientutil.NewClient(ctx, globalOpts.Namespace, globalOpts.Address)
	if err != nil {
		return nil, fmt.Errorf("failed to create containerd client: %w", err)
	}
	defer cancel()

	// Create tag options
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
func (s *ImageService) SaveImage(ctx context.Context, req *pb.SaveImageRequest) (*pb.SaveImageResponse, error) {
	// Create containerd client
	globalOpts := s.createBaseGlobalOptions()
	client, ctxClient, cancel, err := clientutil.NewClient(ctx, globalOpts.Namespace, globalOpts.Address)
	if err != nil {
		return nil, fmt.Errorf("failed to create containerd client: %w", err)
	}
	defer cancel()

	// Create save options
	saveOpts := types.ImageSaveOptions{
		GOptions: *globalOpts, // Pass by value
	}

	// Create a buffer or file writer for output
	var outBuf bytes.Buffer
	if req.Output == "" {
		saveOpts.Stdout = &outBuf
	} else {
		// For now, we'll just use stdout - in a real implementation,
		// you'd want to handle file output properly
		saveOpts.Stdout = &outBuf
	}

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
func (s *ImageService) LoadImage(ctx context.Context, req *pb.LoadImageRequest) (*pb.LoadImageResponse, error) {
	// For now, return basic response
	return &pb.LoadImageResponse{
		Loaded: []string{"load functionality not implemented"},
	}, nil
}

// ImageHistory implements the ImageHistory RPC method
func (s *ImageService) ImageHistory(ctx context.Context, req *pb.ImageHistoryRequest) (*pb.ImageHistoryResponse, error) {
	// For now, return basic response with empty layers
	return &pb.ImageHistoryResponse{
		Layers: []*pb.HistoryLayer{},
	}, nil
}

// InspectImage implements the InspectImage RPC method
func (s *ImageService) InspectImage(ctx context.Context, req *pb.InspectImageRequest) (*pb.InspectImageResponse, error) {
	// For now, return basic response
	return &pb.InspectImageResponse{
		Content: fmt.Sprintf(`{"name": "%s", "status": "not implemented"}`, req.Name),
	}, nil
}

// ECRLogin implements the ECRLogin RPC method
func (s *ImageService) ECRLogin(ctx context.Context, req *pb.ECRLoginRequest) (*pb.ECRLoginResponse, error) {
	// Validate ECR config
	if req.EcrAuth == nil {
		return nil, fmt.Errorf("ECR authentication config is required")
	}

	// Get ECR authorization token
	loginResp, err := s.ecrAuthService.GetECRAuthorizationToken(ctx, req.EcrAuth)
	if err != nil {
		return nil, fmt.Errorf("failed to get ECR authorization token: %w", err)
	}

	// Perform Docker login using nerdctl
	globalOpts := s.createBaseGlobalOptions()
	if err := s.dockerConfigMgr.LoginToECR(ctx, req.EcrAuth, globalOpts); err != nil {
		return nil, fmt.Errorf("failed to login to ECR: %w", err)
	}

	return loginResp, nil
}

// ECRLogout implements the ECRLogout RPC method
func (s *ImageService) ECRLogout(ctx context.Context, req *pb.ECRLogoutRequest) (*pb.ECRLogoutResponse, error) {
	// Validate registry URL
	if req.RegistryUrl == "" {
		return nil, fmt.Errorf("registry URL is required")
	}

	// Check if it's an ECR registry
	if !s.ecrAuthService.IsECRRegistry(req.RegistryUrl) {
		return nil, fmt.Errorf("not an ECR registry: %s", req.RegistryUrl)
	}

	// Remove registry authentication
	if err := s.dockerConfigMgr.RemoveRegistryAuth(req.RegistryUrl); err != nil {
		return nil, fmt.Errorf("failed to logout from ECR: %w", err)
	}

	return &pb.ECRLogoutResponse{
		Status: fmt.Sprintf("Successfully logged out from %s", req.RegistryUrl),
	}, nil
}