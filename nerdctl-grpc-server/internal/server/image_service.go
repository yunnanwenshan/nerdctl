package server

import (
	"context"
	"io"

	"github.com/containerd/nerdctl-grpc-server/internal/interfaces"
	"github.com/containerd/nerdctl-grpc-server/pkg/types"
	pb "github.com/containerd/nerdctl-grpc-server/api/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ImageServiceServer implements the gRPC ImageService using abstract interfaces
// This demonstrates the decoupled architecture for image operations
type ImageServiceServer struct {
	pb.UnimplementedImageServiceServer
	imageManager interfaces.ImageManager
}

// NewImageServiceServer creates a new ImageServiceServer
func NewImageServiceServer(imageManager interfaces.ImageManager) *ImageServiceServer {
	return &ImageServiceServer{
		imageManager: imageManager,
	}
}

// PullImage pulls an image from registry
func (s *ImageServiceServer) PullImage(ctx context.Context, req *pb.PullImageRequest) (*pb.PullImageResponse, error) {
	internalReq := &types.PullImageRequest{
		ImageName:  req.ImageName,
		Tag:        req.Tag,
		Platform:   req.Platform,
		AllTags:    req.AllTags,
		Namespace:  req.Namespace,
		Username:   req.Username,
		Password:   req.Password,
		Quiet:      req.Quiet,
	}

	resp, err := s.imageManager.PullImage(ctx, internalReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to pull image: %v", err)
	}

	return &pb.PullImageResponse{
		ImageId:   resp.ImageID,
		Digest:    resp.Digest,
		Size:      resp.Size,
		Warnings:  resp.Warnings,
	}, nil
}

// PushImage pushes an image to registry
func (s *ImageServiceServer) PushImage(ctx context.Context, req *pb.PushImageRequest) (*pb.PushImageResponse, error) {
	internalReq := &types.PushImageRequest{
		ImageName:  req.ImageName,
		Tag:        req.Tag,
		AllTags:    req.AllTags,
		Namespace:  req.Namespace,
		Username:   req.Username,
		Password:   req.Password,
		SignImage:  req.SignImage,
		Quiet:      req.Quiet,
	}

	resp, err := s.imageManager.PushImage(ctx, internalReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to push image: %v", err)
	}

	return &pb.PushImageResponse{
		ImageId:  resp.ImageID,
		Digest:   resp.Digest,
		Size:     resp.Size,
		Warnings: resp.Warnings,
	}, nil
}

// BuildImage builds an image from Dockerfile
func (s *ImageServiceServer) BuildImage(ctx context.Context, req *pb.BuildImageRequest) (*pb.BuildImageResponse, error) {
	internalReq := &types.BuildImageRequest{
		ContextDir:   req.ContextDir,
		Dockerfile:   req.Dockerfile,
		Tags:         req.Tags,
		BuildArgs:    req.BuildArgs,
		Labels:       req.Labels,
		Target:       req.Target,
		Platform:     req.Platform,
		Progress:     req.Progress,
		NoCache:      req.NoCache,
		Pull:         req.Pull,
		Namespace:    req.Namespace,
		Output:       req.Output,
		CacheFrom:    req.CacheFrom,
		CacheTo:      req.CacheTo,
		Secrets:      convertBuildSecrets(req.Secrets),
		SSH:          req.Ssh,
	}

	resp, err := s.imageManager.BuildImage(ctx, internalReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to build image: %v", err)
	}

	return &pb.BuildImageResponse{
		ImageId:   resp.ImageID,
		Warnings:  resp.Warnings,
		BuildLogs: resp.BuildLogs,
	}, nil
}

// ListImages lists available images
func (s *ImageServiceServer) ListImages(ctx context.Context, req *pb.ListImagesRequest) (*pb.ListImagesResponse, error) {
	internalReq := &types.ListImagesRequest{
		All:       req.All,
		Digests:   req.Digests,
		NoTrunc:   req.NoTrunc,
		Quiet:     req.Quiet,
		Namespace: req.Namespace,
		Filters:   req.Filters,
		Format:    req.Format,
	}

	resp, err := s.imageManager.ListImages(ctx, internalReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list images: %v", err)
	}

	// Convert internal response to gRPC format
	var images []*pb.ImageInfo
	for _, image := range resp.Images {
		images = append(images, convertImageInfo(&image))
	}

	return &pb.ListImagesResponse{
		Images: images,
	}, nil
}

// InspectImage inspects an image
func (s *ImageServiceServer) InspectImage(ctx context.Context, req *pb.InspectImageRequest) (*pb.InspectImageResponse, error) {
	internalReq := &types.InspectImageRequest{
		ImageName: req.ImageName,
		Namespace: req.Namespace,
	}

	resp, err := s.imageManager.InspectImage(ctx, internalReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to inspect image: %v", err)
	}

	return &pb.InspectImageResponse{
		Image: convertImageInfo(resp.Image),
	}, nil
}

// RemoveImage removes an image
func (s *ImageServiceServer) RemoveImage(ctx context.Context, req *pb.RemoveImageRequest) (*pb.RemoveImageResponse, error) {
	internalReq := &types.RemoveImageRequest{
		ImageName: req.ImageName,
		Force:     req.Force,
		NoPrune:   req.NoPrune,
		Namespace: req.Namespace,
	}

	resp, err := s.imageManager.RemoveImage(ctx, internalReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to remove image: %v", err)
	}

	return &pb.RemoveImageResponse{
		Deleted:    resp.Deleted,
		Untagged:   resp.Untagged,
		SpaceFreed: resp.SpaceFreed,
	}, nil
}

// TagImage creates a new tag for an image
func (s *ImageServiceServer) TagImage(ctx context.Context, req *pb.TagImageRequest) (*pb.TagImageResponse, error) {
	internalReq := &types.TagImageRequest{
		SourceImage: req.SourceImage,
		TargetImage: req.TargetImage,
		Namespace:   req.Namespace,
	}

	resp, err := s.imageManager.TagImage(ctx, internalReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to tag image: %v", err)
	}

	return &pb.TagImageResponse{
		ImageId: resp.ImageID,
	}, nil
}

// SaveImage saves image(s) to a tar file
func (s *ImageServiceServer) SaveImage(req *pb.SaveImageRequest, stream pb.ImageService_SaveImageServer) error {
	ctx := stream.Context()
	
	internalReq := &types.SaveImageRequest{
		ImageNames: req.ImageNames,
		Namespace:  req.Namespace,
		Output:     req.Output,
		Format:     req.Format,
	}

	reader, err := s.imageManager.SaveImage(ctx, internalReq)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to save image: %v", err)
	}
	defer reader.Close()

	// Stream the tar data to client
	buffer := make([]byte, 32*1024) // 32KB buffer
	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			chunk := &pb.SaveImageResponse{
				Data: buffer[:n],
			}
			if err := stream.Send(chunk); err != nil {
				return status.Errorf(codes.Internal, "failed to send image data: %v", err)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return status.Errorf(codes.Internal, "failed to read image data: %v", err)
		}
	}

	return nil
}

// LoadImage loads image(s) from a tar file
func (s *ImageServiceServer) LoadImage(stream pb.ImageService_LoadImageServer) error {
	ctx := stream.Context()
	
	// Receive first message to get metadata
	req, err := stream.Recv()
	if err != nil {
		return status.Errorf(codes.Internal, "failed to receive load request: %v", err)
	}

	internalReq := &types.LoadImageRequest{
		Namespace: req.Namespace,
		Quiet:     req.Quiet,
	}

	// Create pipe for streaming data
	reader, writer := io.Pipe()

	// Start goroutine to receive streaming data
	go func() {
		defer writer.Close()
		
		// Write first chunk if it has data
		if len(req.Data) > 0 {
			if _, err := writer.Write(req.Data); err != nil {
				writer.CloseWithError(err)
				return
			}
		}

		// Continue receiving chunks
		for {
			req, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				writer.CloseWithError(err)
				return
			}
			
			if _, err := writer.Write(req.Data); err != nil {
				writer.CloseWithError(err)
				return
			}
		}
	}()

	// Load the image
	resp, err := s.imageManager.LoadImage(ctx, internalReq, reader)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to load image: %v", err)
	}

	// Send response
	return stream.SendAndClose(&pb.LoadImageResponse{
		LoadedImages: resp.LoadedImages,
		Warnings:     resp.Warnings,
	})
}

// ImportImage imports an image from a tarball
func (s *ImageServiceServer) ImportImage(stream pb.ImageService_ImportImageServer) error {
	ctx := stream.Context()
	
	// Receive first message to get metadata
	req, err := stream.Recv()
	if err != nil {
		return status.Errorf(codes.Internal, "failed to receive import request: %v", err)
	}

	internalReq := &types.ImportImageRequest{
		ImageName: req.ImageName,
		Tag:       req.Tag,
		Message:   req.Message,
		Changes:   req.Changes,
		Namespace: req.Namespace,
	}

	// Create pipe for streaming data
	reader, writer := io.Pipe()

	// Start goroutine to receive streaming data
	go func() {
		defer writer.Close()
		
		// Write first chunk if it has data
		if len(req.Data) > 0 {
			if _, err := writer.Write(req.Data); err != nil {
				writer.CloseWithError(err)
				return
			}
		}

		// Continue receiving chunks
		for {
			req, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				writer.CloseWithError(err)
				return
			}
			
			if _, err := writer.Write(req.Data); err != nil {
				writer.CloseWithError(err)
				return
			}
		}
	}()

	// Import the image
	resp, err := s.imageManager.ImportImage(ctx, internalReq, reader)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to import image: %v", err)
	}

	// Send response
	return stream.SendAndClose(&pb.ImportImageResponse{
		ImageId:  resp.ImageID,
		Warnings: resp.Warnings,
	})
}

// GetImageHistory gets image history
func (s *ImageServiceServer) GetImageHistory(ctx context.Context, req *pb.GetImageHistoryRequest) (*pb.GetImageHistoryResponse, error) {
	internalReq := &types.GetImageHistoryRequest{
		ImageName: req.ImageName,
		Namespace: req.Namespace,
		NoTrunc:   req.NoTrunc,
		Human:     req.Human,
		Quiet:     req.Quiet,
	}

	resp, err := s.imageManager.GetImageHistory(ctx, internalReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get image history: %v", err)
	}

	// Convert history entries
	var historyEntries []*pb.ImageHistoryEntry
	for _, entry := range resp.History {
		historyEntries = append(historyEntries, &pb.ImageHistoryEntry{
			Id:           entry.ID,
			Created:      timestamppb.New(entry.Created),
			CreatedBy:    entry.CreatedBy,
			Size:         entry.Size,
			Comment:      entry.Comment,
		})
	}

	return &pb.GetImageHistoryResponse{
		History: historyEntries,
	}, nil
}

// PruneImages removes unused images
func (s *ImageServiceServer) PruneImages(ctx context.Context, req *pb.PruneImagesRequest) (*pb.PruneImagesResponse, error) {
	internalReq := &types.PruneImagesRequest{
		All:       req.All,
		Filters:   req.Filters,
		Namespace: req.Namespace,
	}

	resp, err := s.imageManager.PruneImages(ctx, internalReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to prune images: %v", err)
	}

	return &pb.PruneImagesResponse{
		DeletedImages: resp.DeletedImages,
		SpaceReclaimed: resp.SpaceReclaimed,
	}, nil
}

// Helper functions to convert between gRPC and internal types

func convertBuildSecrets(secrets []*pb.BuildSecret) []types.BuildSecret {
	var result []types.BuildSecret
	for _, secret := range secrets {
		result = append(result, types.BuildSecret{
			ID:       secret.Id,
			Source:   secret.Source,
			Target:   secret.Target,
			Type:     secret.Type,
			Mode:     secret.Mode,
		})
	}
	return result
}

func convertImageInfo(image *types.ImageInfo) *pb.ImageInfo {
	var repoTags []*pb.RepoTag
	for _, tag := range image.RepoTags {
		repoTags = append(repoTags, &pb.RepoTag{
			Repository: tag.Repository,
			Tag:        tag.Tag,
		})
	}

	var repoDigests []*pb.RepoDigest
	for _, digest := range image.RepoDigests {
		repoDigests = append(repoDigests, &pb.RepoDigest{
			Repository: digest.Repository,
			Digest:     digest.Digest,
		})
	}

	return &pb.ImageInfo{
		Id:          image.ID,
		ShortId:     image.ShortID,
		RepoTags:    repoTags,
		RepoDigests: repoDigests,
		Size:        image.Size,
		VirtualSize: image.VirtualSize,
		SharedSize:  image.SharedSize,
		UniqueSize:  image.UniqueSize,
		Created:     timestamppb.New(image.Created),
		Labels:      image.Labels,
		Platform:    image.Platform,
		Architecture: image.Architecture,
		Os:          image.OS,
		OsVersion:   image.OSVersion,
	}
}