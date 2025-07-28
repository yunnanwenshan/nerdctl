package services

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/containerd/nerdctl-grpc-server/api/proto"
	"github.com/containerd/nerdctl-grpc-server/internal/interfaces"
	"github.com/containerd/nerdctl-grpc-server/pkg/types"
)

// ImageServiceImpl implements the comprehensive image service
type ImageServiceImpl struct {
	pb.UnimplementedImageServiceExtendedServer
	imageManager interfaces.ImageManager
	eventStreams map[string]chan *pb.ImageEvent
	eventStreamsMu sync.RWMutex
}

// NewImageService creates a new image service implementation
func NewImageService(imageManager interfaces.ImageManager) *ImageServiceImpl {
	return &ImageServiceImpl{
		imageManager: imageManager,
		eventStreams: make(map[string]chan *pb.ImageEvent),
	}
}

// PullImage pulls an image from registry with streaming progress
func (s *ImageServiceImpl) PullImage(req *pb.PullImageRequest, stream pb.ImageServiceExtended_PullImageServer) error {
	if req.Image == "" {
		return status.Error(codes.InvalidArgument, "image is required")
	}

	ctx := stream.Context()
	
	// Convert request to internal type
	pullOpts := &types.PullImageOptions{
		Image:     req.Image,
		Platform:  req.Platform,
		AllTags:   req.AllTags,
		Quiet:     req.Quiet,
		Namespace: req.Namespace,
		Verify:    req.Verify,
		Unpack:    req.Unpack,
		AuthConfig: req.AuthConfig,
	}

	// Pull image with streaming
	progressChan, errChan := s.imageManager.PullImageStream(ctx, pullOpts)

	for {
		select {
		case progress, ok := <-progressChan:
			if !ok {
				// Stream completed successfully
				return stream.Send(&pb.PullImageResponse{
					Status:   "completed",
					Complete: true,
				})
			}

			response := &pb.PullImageResponse{
				Status:   progress.Status,
				Progress: progress.Progress,
				LayerId:  progress.LayerID,
				Current:  progress.Current,
				Total:    progress.Total,
				Complete: progress.Complete,
			}

			if err := stream.Send(response); err != nil {
				return status.Errorf(codes.Internal, "failed to send pull progress: %v", err)
			}

		case err := <-errChan:
			if err != nil {
				return status.Errorf(codes.Internal, "pull failed: %v", err)
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// PushImage pushes an image to registry with streaming progress
func (s *ImageServiceImpl) PushImage(req *pb.PushImageRequest, stream pb.ImageServiceExtended_PushImageServer) error {
	if req.Image == "" {
		return status.Error(codes.InvalidArgument, "image is required")
	}

	ctx := stream.Context()

	pushOpts := &types.PushImageOptions{
		Image:      req.Image,
		AllTags:    req.AllTags,
		Quiet:      req.Quiet,
		Namespace:  req.Namespace,
		Sign:       req.Sign,
		Platform:   req.Platform,
		AuthConfig: req.AuthConfig,
	}

	progressChan, errChan := s.imageManager.PushImageStream(ctx, pushOpts)

	for {
		select {
		case progress, ok := <-progressChan:
			if !ok {
				// Emit image pushed event
				s.emitImageEvent("push", req.Image, "", "")
				
				return stream.Send(&pb.PushImageResponse{
					Status:   "completed",
					Complete: true,
				})
			}

			response := &pb.PushImageResponse{
				Status:   progress.Status,
				Progress: progress.Progress,
				LayerId:  progress.LayerID,
				Current:  progress.Current,
				Total:    progress.Total,
				Complete: progress.Complete,
			}

			if err := stream.Send(response); err != nil {
				return status.Errorf(codes.Internal, "failed to send push progress: %v", err)
			}

		case err := <-errChan:
			if err != nil {
				return status.Errorf(codes.Internal, "push failed: %v", err)
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// ListImages lists images
func (s *ImageServiceImpl) ListImages(ctx context.Context, req *pb.ListImagesRequest) (*pb.ListImagesResponse, error) {
	listOpts := &types.ListImagesOptions{
		Namespace: req.Namespace,
		All:       req.All,
		Quiet:     req.Quiet,
		NoTrunc:   req.NoTrunc,
		Digests:   req.Digests,
		Format:    req.Format,
		Filters:   req.Filters,
	}

	images, err := s.imageManager.ListImages(ctx, listOpts)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list images: %v", err)
	}

	// Convert to response
	response := &pb.ListImagesResponse{
		Images:     make([]*pb.ImageInfo, len(images)),
		TotalCount: int32(len(images)),
	}

	for i, image := range images {
		response.Images[i] = s.convertToImageInfo(&image)
	}

	return response, nil
}

// InspectImage inspects an image
func (s *ImageServiceImpl) InspectImage(ctx context.Context, req *pb.InspectImageRequest) (*pb.InspectImageResponse, error) {
	if req.Image == "" {
		return nil, status.Error(codes.InvalidArgument, "image is required")
	}

	inspectOpts := &types.InspectImageOptions{
		Image:     req.Image,
		Namespace: req.Namespace,
		Format:    req.Format,
		Platform:  req.Platform,
	}

	result, err := s.imageManager.InspectImage(ctx, inspectOpts)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to inspect image: %v", err)
	}

	return &pb.InspectImageResponse{
		ImageInfo: s.convertToImageInfo(&result.Image),
		RawData:   result.RawData,
	}, nil
}

// RemoveImage removes one or more images
func (s *ImageServiceImpl) RemoveImage(ctx context.Context, req *pb.RemoveImageRequest) (*pb.RemoveImageResponse, error) {
	if len(req.Images) == 0 {
		return nil, status.Error(codes.InvalidArgument, "at least one image is required")
	}

	removeOpts := &types.RemoveImageOptions{
		Images:    req.Images,
		Namespace: req.Namespace,
		Force:     req.Force,
		NoPrune:   req.NoPrune,
	}

	result, err := s.imageManager.RemoveImage(ctx, removeOpts)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to remove images: %v", err)
	}

	// Convert result
	removed := make([]*pb.RemovedImage, len(result.Removed))
	for i, r := range result.Removed {
		removed[i] = &pb.RemovedImage{
			Image:     r.Image,
			Untagged:  r.Untagged,
			Deleted:   r.Deleted,
			Error:     r.Error,
		}

		// Emit image removed event for successfully removed images
		if r.Deleted && r.Error == "" {
			s.emitImageEvent("remove", r.Image, "", "")
		}
	}

	return &pb.RemoveImageResponse{
		Removed:        removed,
		SpaceReclaimed: result.SpaceReclaimed,
	}, nil
}

// TagImage tags an image
func (s *ImageServiceImpl) TagImage(ctx context.Context, req *pb.TagImageRequest) (*pb.TagImageResponse, error) {
	if req.SourceImage == "" || req.TargetImage == "" {
		return nil, status.Error(codes.InvalidArgument, "source and target images are required")
	}

	tagOpts := &types.TagImageOptions{
		SourceImage: req.SourceImage,
		TargetImage: req.TargetImage,
		Namespace:   req.Namespace,
	}

	err := s.imageManager.TagImage(ctx, tagOpts)
	if err != nil {
		return &pb.TagImageResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	// Emit image tagged event
	s.emitImageEvent("tag", req.TargetImage, "", "")

	return &pb.TagImageResponse{
		Success: true,
		Message: "Image tagged successfully",
	}, nil
}

// UntagImage untags an image
func (s *ImageServiceImpl) UntagImage(ctx context.Context, req *pb.UntagImageRequest) (*pb.UntagImageResponse, error) {
	if req.Image == "" {
		return nil, status.Error(codes.InvalidArgument, "image is required")
	}

	untagOpts := &types.UntagImageOptions{
		Image:     req.Image,
		Namespace: req.Namespace,
	}

	err := s.imageManager.UntagImage(ctx, untagOpts)
	if err != nil {
		return &pb.UntagImageResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	// Emit image untagged event
	s.emitImageEvent("untag", req.Image, "", "")

	return &pb.UntagImageResponse{
		Success: true,
		Message: "Image untagged successfully",
	}, nil
}

// BuildImage builds an image from streaming context
func (s *ImageServiceImpl) BuildImage(stream pb.ImageServiceExtended_BuildImageServer) error {
	ctx := stream.Context()
	var buildConfig *pb.BuildConfig
	var contextData []byte

	// Receive build configuration and context
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return status.Errorf(codes.Internal, "failed to receive build request: %v", err)
		}

		switch r := req.Request.(type) {
		case *pb.BuildImageRequest_Config:
			buildConfig = r.Config
		case *pb.BuildImageRequest_Chunk:
			contextData = append(contextData, r.Chunk...)
		}
	}

	if buildConfig == nil {
		return status.Error(codes.InvalidArgument, "build config is required")
	}

	// Convert to internal type
	buildOpts := &types.BuildImageOptions{
		Dockerfile: buildConfig.Dockerfile,
		Context:    buildConfig.Context,
		Tags:       buildConfig.Tags,
		BuildArgs:  buildConfig.BuildArgs,
		Target:     buildConfig.Target,
		Platform:   buildConfig.Platform,
		NoCache:    buildConfig.NoCache,
		Rm:         buildConfig.Rm,
		ForceRm:    buildConfig.ForceRm,
		Pull:       buildConfig.Pull,
		Quiet:      buildConfig.Quiet,
		Output:     buildConfig.Output,
		Secrets:    buildConfig.Secrets,
		SSH:        buildConfig.Ssh,
		Progress:   buildConfig.Progress,
		Labels:     buildConfig.Labels,
		Namespace:  buildConfig.Namespace,
		ContextData: contextData,
	}

	// Build image with streaming
	progressChan, errChan := s.imageManager.BuildImageStream(ctx, buildOpts)

	for {
		select {
		case progress, ok := <-progressChan:
			if !ok {
				return nil
			}

			response := &pb.BuildImageResponse{
				Status:   progress.Status,
				Stream:   progress.Stream,
				Progress: progress.Progress,
				ImageId:  progress.ImageID,
				Complete: progress.Complete,
			}

			if progress.Error != "" {
				response.Error = progress.Error
			}

			if err := stream.Send(response); err != nil {
				return status.Errorf(codes.Internal, "failed to send build progress: %v", err)
			}

			// Emit image built event when complete
			if progress.Complete && progress.ImageID != "" {
				for _, tag := range buildConfig.Tags {
					s.emitImageEvent("build", tag, "", "")
				}
			}

		case err := <-errChan:
			if err != nil {
				return status.Errorf(codes.Internal, "build failed: %v", err)
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// BuildImageFromContext builds an image from a context path
func (s *ImageServiceImpl) BuildImageFromContext(req *pb.BuildImageFromContextRequest, stream pb.ImageServiceExtended_BuildImageFromContextServer) error {
	if req.Config == nil {
		return status.Error(codes.InvalidArgument, "build config is required")
	}

	ctx := stream.Context()

	buildOpts := &types.BuildImageOptions{
		Dockerfile:  req.Config.Dockerfile,
		Context:     req.ContextPath,
		Tags:        req.Config.Tags,
		BuildArgs:   req.Config.BuildArgs,
		Target:      req.Config.Target,
		Platform:    req.Config.Platform,
		NoCache:     req.Config.NoCache,
		Rm:          req.Config.Rm,
		ForceRm:     req.Config.ForceRm,
		Pull:        req.Config.Pull,
		Quiet:       req.Config.Quiet,
		Output:      req.Config.Output,
		Secrets:     req.Config.Secrets,
		SSH:         req.Config.Ssh,
		Progress:    req.Config.Progress,
		Labels:      req.Config.Labels,
		Namespace:   req.Config.Namespace,
	}

	progressChan, errChan := s.imageManager.BuildImageStream(ctx, buildOpts)

	for {
		select {
		case progress, ok := <-progressChan:
			if !ok {
				return nil
			}

			response := &pb.BuildImageResponse{
				Status:   progress.Status,
				Stream:   progress.Stream,
				Progress: progress.Progress,
				ImageId:  progress.ImageID,
				Complete: progress.Complete,
			}

			if progress.Error != "" {
				response.Error = progress.Error
			}

			if err := stream.Send(response); err != nil {
				return status.Errorf(codes.Internal, "failed to send build progress: %v", err)
			}

		case err := <-errChan:
			if err != nil {
				return status.Errorf(codes.Internal, "build failed: %v", err)
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// SaveImage saves images to tar archive with streaming
func (s *ImageServiceImpl) SaveImage(req *pb.SaveImageRequest, stream pb.ImageServiceExtended_SaveImageServer) error {
	if len(req.Images) == 0 {
		return status.Error(codes.InvalidArgument, "at least one image is required")
	}

	ctx := stream.Context()

	saveOpts := &types.SaveImageOptions{
		Images:    req.Images,
		Namespace: req.Namespace,
		Output:    req.Output,
		AllTags:   req.AllTags,
	}

	// Save image with streaming
	dataChan, errChan := s.imageManager.SaveImageStream(ctx, saveOpts)

	for {
		select {
		case data, ok := <-dataChan:
			if !ok {
				// Stream completed
				return stream.Send(&pb.SaveImageResponse{
					Complete: true,
				})
			}

			response := &pb.SaveImageResponse{
				Chunk: data.Chunk,
			}

			if err := stream.Send(response); err != nil {
				return status.Errorf(codes.Internal, "failed to send save data: %v", err)
			}

		case err := <-errChan:
			if err != nil {
				return status.Errorf(codes.Internal, "save failed: %v", err)
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// LoadImage loads images from tar archive with streaming
func (s *ImageServiceImpl) LoadImage(stream pb.ImageServiceExtended_LoadImageServer) error {
	ctx := stream.Context()
	var loadOpts *types.LoadImageOptions
	var imageData []byte

	// Receive image data
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return status.Errorf(codes.Internal, "failed to receive load data: %v", err)
		}

		if loadOpts == nil {
			loadOpts = &types.LoadImageOptions{
				Namespace: req.Namespace,
				Quiet:     req.Quiet,
			}
		}

		imageData = append(imageData, req.Chunk...)

		if req.Complete {
			break
		}
	}

	if loadOpts == nil {
		return status.Error(codes.InvalidArgument, "load options are required")
	}

	loadOpts.ImageData = imageData

	// Load images
	result, err := s.imageManager.LoadImage(ctx, loadOpts)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to load images: %v", err)
	}

	// Emit image loaded events
	for _, image := range result.LoadedImages {
		s.emitImageEvent("load", image, "", "")
	}

	return stream.SendAndClose(&pb.LoadImageResponse{
		LoadedImages: result.LoadedImages,
		Message:      result.Message,
	})
}

// GetImageHistory gets image history
func (s *ImageServiceImpl) GetImageHistory(ctx context.Context, req *pb.GetImageHistoryRequest) (*pb.GetImageHistoryResponse, error) {
	if req.Image == "" {
		return nil, status.Error(codes.InvalidArgument, "image is required")
	}

	historyOpts := &types.GetImageHistoryOptions{
		Image:     req.Image,
		Namespace: req.Namespace,
		NoTrunc:   req.NoTrunc,
		Quiet:     req.Quiet,
		Format:    req.Format,
	}

	history, err := s.imageManager.GetImageHistory(ctx, historyOpts)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get image history: %v", err)
	}

	// Convert to response
	historyEntries := make([]*pb.HistoryEntry, len(history))
	for i, entry := range history {
		historyEntries[i] = &pb.HistoryEntry{
			Id:         entry.ID,
			Created:    timestamppb.New(entry.Created),
			CreatedBy:  entry.CreatedBy,
			Comment:    entry.Comment,
			Size:       entry.Size,
			EmptyLayer: entry.EmptyLayer,
			Labels:     entry.Labels,
		}
	}

	return &pb.GetImageHistoryResponse{
		History: historyEntries,
	}, nil
}

// PruneImages prunes unused images
func (s *ImageServiceImpl) PruneImages(ctx context.Context, req *pb.PruneImagesRequest) (*pb.PruneImagesResponse, error) {
	pruneOpts := &types.PruneImagesOptions{
		Namespace: req.Namespace,
		All:       req.All,
		Filters:   req.Filters,
	}

	result, err := s.imageManager.PruneImages(ctx, pruneOpts)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to prune images: %v", err)
	}

	return &pb.PruneImagesResponse{
		DeletedImages:  result.DeletedImages,
		SpaceReclaimed: result.SpaceReclaimed,
	}, nil
}

// BatchImageOperation performs batch operations on images
func (s *ImageServiceImpl) BatchImageOperation(ctx context.Context, req *pb.BatchImageRequest) (*pb.BatchImageResponse, error) {
	if len(req.Images) == 0 {
		return nil, status.Error(codes.InvalidArgument, "images are required")
	}

	results := make([]*pb.BatchImageResult, len(req.Images))
	successCount := int32(0)
	errorCount := int32(0)

	for i, image := range req.Images {
		result := &pb.BatchImageResult{
			Image:   image,
			Success: false,
		}

		var err error
		switch req.Operation {
		case "pull":
			pullOpts := &types.PullImageOptions{
				Image:     image,
				Namespace: req.Namespace,
			}
			_, err = s.imageManager.PullImage(ctx, pullOpts)
		case "remove":
			removeOpts := &types.RemoveImageOptions{
				Images:    []string{image},
				Namespace: req.Namespace,
			}
			_, err = s.imageManager.RemoveImage(ctx, removeOpts)
		default:
			err = fmt.Errorf("unsupported operation: %s", req.Operation)
		}

		if err != nil {
			result.Error = err.Error()
			errorCount++
			if !req.ContinueOnError {
				break
			}
		} else {
			result.Success = true
			successCount++
		}

		results[i] = result
	}

	return &pb.BatchImageResponse{
		Results:      results,
		SuccessCount: successCount,
		ErrorCount:   errorCount,
	}, nil
}

// MonitorImageEvents monitors image events
func (s *ImageServiceImpl) MonitorImageEvents(req *pb.MonitorImageEventsRequest, stream pb.ImageServiceExtended_MonitorImageEventsServer) error {
	ctx := stream.Context()
	streamID := fmt.Sprintf("stream_%d", time.Now().UnixNano())

	// Create event channel
	eventChan := make(chan *pb.ImageEvent, 100)
	
	s.eventStreamsMu.Lock()
	s.eventStreams[streamID] = eventChan
	s.eventStreamsMu.Unlock()

	defer func() {
		s.eventStreamsMu.Lock()
		delete(s.eventStreams, streamID)
		close(eventChan)
		s.eventStreamsMu.Unlock()
	}()

	// Send events
	for {
		select {
		case event := <-eventChan:
			if s.shouldSendImageEvent(event, req) {
				if err := stream.Send(event); err != nil {
					return status.Errorf(codes.Internal, "failed to send event: %v", err)
				}
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Helper methods

func (s *ImageServiceImpl) convertToImageInfo(image *types.Image) *pb.ImageInfo {
	return &pb.ImageInfo{
		Id:           image.ID,
		Digest:       image.Digest,
		RepoTags:     image.RepoTags,
		RepoDigests:  image.RepoDigests,
		ParentId:     image.ParentID,
		Comment:      image.Comment,
		Created:      timestamppb.New(image.Created),
		Container:    image.Container,
		Author:       image.Author,
		Architecture: image.Architecture,
		Os:           image.OS,
		OsVersion:    image.OSVersion,
		Size:         image.Size,
		VirtualSize:  image.VirtualSize,
		SharedSize:   image.SharedSize,
		Labels:       image.Labels,
	}
}

func (s *ImageServiceImpl) emitImageEvent(eventType, image, tag, digest string) {
	event := &pb.ImageEvent{
		Type:      eventType,
		Image:     image,
		Tag:       tag,
		Digest:    digest,
		Timestamp: timestamppb.Now(),
		Attributes: make(map[string]string),
	}

	s.eventStreamsMu.RLock()
	for _, eventChan := range s.eventStreams {
		select {
		case eventChan <- event:
		default:
			// Channel is full, skip
		}
	}
	s.eventStreamsMu.RUnlock()
}

func (s *ImageServiceImpl) shouldSendImageEvent(event *pb.ImageEvent, req *pb.MonitorImageEventsRequest) bool {
	// Filter by images
	if len(req.Images) > 0 {
		found := false
		for _, img := range req.Images {
			if strings.Contains(event.Image, img) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Filter by event types
	if len(req.EventTypes) > 0 {
		found := false
		for _, eventType := range req.EventTypes {
			if event.Type == eventType {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Filter by time range
	if req.Since != nil && event.Timestamp.AsTime().Before(req.Since.AsTime()) {
		return false
	}

	if req.Until != nil && event.Timestamp.AsTime().After(req.Until.AsTime()) {
		return false
	}

	return true
}

// Additional methods for comprehensive image management...

func (s *ImageServiceImpl) ConvertImage(req *pb.ConvertImageRequest, stream pb.ImageServiceExtended_ConvertImageServer) error {
	if req.SourceImage == "" || req.TargetImage == "" {
		return status.Error(codes.InvalidArgument, "source and target images are required")
	}

	ctx := stream.Context()

	convertOpts := &types.ConvertImageOptions{
		SourceImage: req.SourceImage,
		TargetImage: req.TargetImage,
		Format:      req.Format,
		Namespace:   req.Namespace,
		Platform:    req.Platform,
		Options:     req.Options,
	}

	progressChan, errChan := s.imageManager.ConvertImageStream(ctx, convertOpts)

	for {
		select {
		case progress, ok := <-progressChan:
			if !ok {
				return stream.Send(&pb.ConvertImageResponse{
					Status:   "completed",
					Complete: true,
				})
			}

			response := &pb.ConvertImageResponse{
				Status:   progress.Status,
				Progress: progress.Progress,
				Complete: progress.Complete,
			}

			if progress.Error != "" {
				response.Error = progress.Error
			}

			if err := stream.Send(response); err != nil {
				return status.Errorf(codes.Internal, "failed to send conversion progress: %v", err)
			}

		case err := <-errChan:
			if err != nil {
				return status.Errorf(codes.Internal, "conversion failed: %v", err)
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (s *ImageServiceImpl) SearchRegistry(ctx context.Context, req *pb.SearchRegistryRequest) (*pb.SearchRegistryResponse, error) {
	if req.Term == "" {
		return nil, status.Error(codes.InvalidArgument, "search term is required")
	}

	searchOpts := &types.SearchRegistryOptions{
		Term:    req.Term,
		Limit:   int(req.Limit),
		Filters: req.Filters,
		NoTrunc: req.NoTrunc,
	}

	results, err := s.imageManager.SearchRegistry(ctx, searchOpts)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "search failed: %v", err)
	}

	// Convert results
	searchResults := make([]*pb.SearchResult, len(results))
	for i, result := range results {
		searchResults[i] = &pb.SearchResult{
			Name:        result.Name,
			Description: result.Description,
			StarCount:   int32(result.StarCount),
			IsOfficial:  result.IsOfficial,
			IsAutomated: result.IsAutomated,
		}
	}

	return &pb.SearchRegistryResponse{
		Results: searchResults,
	}, nil
}