/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"time"

	_ "github.com/containerd/containerd/api/events" // Register grpc event types
	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/events"
	"github.com/containerd/log"
	"github.com/containerd/typeurl/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/containerd/nerdctl-grpc-server/api/proto"
	"github.com/containerd/nerdctl-grpc-server/internal/adapters/factory"
	"github.com/containerd/nerdctl-grpc-server/pkg/types"
)

// SystemServiceServer implements the SystemService gRPC interface
type SystemServiceServer struct {
	pb.UnimplementedSystemServiceServer
	adapterFactory *factory.AdapterFactory
}

// NewSystemServiceServer creates a new SystemServiceServer instance
func NewSystemServiceServer(factory *factory.AdapterFactory) *SystemServiceServer {
	return &SystemServiceServer{
		adapterFactory: factory,
	}
}

// GetSystemInfo retrieves system information
func (s *SystemServiceServer) GetSystemInfo(ctx context.Context, req *pb.GetSystemInfoRequest) (*pb.GetSystemInfoResponse, error) {
	adapter := s.adapterFactory.CreateV2Adapter()
	
	// Get containerd client
	client, err := adapter.GetContainerdClient(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get containerd client: %v", err)
	}

	// Get system info
	info, err := s.collectSystemInfo(ctx, client)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to collect system info: %v", err)
	}

	return &pb.GetSystemInfoResponse{
		Info: info,
	}, nil
}

// GetSystemEvents streams real-time events from the server
func (s *SystemServiceServer) GetSystemEvents(req *pb.GetSystemEventsRequest, stream pb.SystemService_GetSystemEventsServer) error {
	ctx := stream.Context()
	adapter := s.adapterFactory.CreateV2Adapter()
	
	// Get containerd client
	client, err := adapter.GetContainerdClient(ctx)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to get containerd client: %v", err)
	}

	// Create event service
	eventsClient := client.EventService()
	eventsCh, errCh := eventsClient.Subscribe(ctx)

	// Parse filters
	filterMap, err := s.parseEventFilters(req.Filters)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "invalid filter: %v", err)
	}

	// Stream events
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errCh:
			if err != nil {
				return status.Errorf(codes.Internal, "event stream error: %v", err)
			}
		case envelope := <-eventsCh:
			if envelope != nil {
				// Convert containerd event to our gRPC event format
				pbEvent, err := s.convertEventEnvelopeToProto(envelope)
				if err != nil {
					log.G(ctx).WithError(err).Warn("failed to convert event")
					continue
				}

				// Apply filters
				if s.eventMatchesFilters(pbEvent, filterMap) {
					if err := stream.Send(pbEvent); err != nil {
						return status.Errorf(codes.Internal, "failed to send event: %v", err)
					}
				}
			}
		}
	}
}

// SystemPrune cleans up unused resources
func (s *SystemServiceServer) SystemPrune(ctx context.Context, req *pb.SystemPruneRequest) (*pb.SystemPruneResponse, error) {
	adapter := s.adapterFactory.CreateV2Adapter()
	
	response := &pb.SystemPruneResponse{
		Summary: &pb.SystemPruneSummary{},
	}

	var totalSpaceReclaimed int64

	// Prune containers if requested
	if req.PruneContainers {
		deletedContainers, err := s.pruneContainers(ctx, adapter, req.Force)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to prune containers: %v", err)
		}
		response.ContainersDeleted = deletedContainers
		response.Summary.ContainersCount = int32(len(deletedContainers))
	}

	// Prune images if requested
	if req.PruneImages || req.All {
		deletedImages, spaceReclaimed, err := s.pruneImages(ctx, adapter, req.Force)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to prune images: %v", err)
		}
		response.ImagesDeleted = deletedImages
		response.Summary.ImagesCount = int32(len(deletedImages))
		totalSpaceReclaimed += spaceReclaimed
	}

	// Prune networks if requested
	if req.PruneNetworks || req.All {
		deletedNetworks, err := s.pruneNetworks(ctx, adapter, req.Force)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to prune networks: %v", err)
		}
		response.NetworksDeleted = deletedNetworks
		response.Summary.NetworksCount = int32(len(deletedNetworks))
	}

	response.SpaceReclaimed = totalSpaceReclaimed
	response.Summary.TotalSizeBytes = totalSpaceReclaimed

	return response, nil
}

// collectSystemInfo gathers system information
func (s *SystemServiceServer) collectSystemInfo(ctx context.Context, client *containerd.Client) (*pb.SystemInfo, error) {
	info := &pb.SystemInfo{
		Os:           runtime.GOOS,
		Architecture: runtime.GOARCH,
	}

	// Get version info
	if version := client.Version(ctx); version != nil {
		info.ContainerdVersion = version.Version
	}

	// Count containers by status
	containers, err := client.Containers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	var running, paused, stopped int64
	for _, container := range containers {
		task, err := container.Task(ctx, nil)
		if err != nil {
			stopped++
			continue
		}
		
		status, err := task.Status(ctx)
		if err != nil {
			stopped++
			continue
		}

		switch status.Status {
		case containerd.Running:
			running++
		case containerd.Paused:
			paused++
		default:
			stopped++
		}
	}

	info.ContainersRunning = running
	info.ContainersPaused = paused
	info.ContainersStopped = stopped

	// Count images
	images, err := client.ImageService().List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list images: %w", err)
	}
	info.ImagesCount = int64(len(images))

	return info, nil
}

// convertEventEnvelopeToProto converts containerd event to protobuf format
func (s *SystemServiceServer) convertEventEnvelopeToProto(envelope *events.Envelope) (*pb.SystemEvent, error) {
	pbEvent := &pb.SystemEvent{
		Timestamp: envelope.Timestamp.UnixNano(),
		Namespace: envelope.Namespace,
		Topic:     envelope.Topic,
		Status:    s.topicToStatus(envelope.Topic),
		Metadata:  make(map[string]string),
	}

	// Determine event type from topic
	pbEvent.Type = s.topicToEventType(envelope.Topic)

	// Extract event data
	if envelope.Event != nil {
		v, err := typeurl.UnmarshalAny(envelope.Event)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal event: %w", err)
		}

		eventData, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal event data: %w", err)
		}
		pbEvent.EventData = string(eventData)

		// Extract container ID if present
		var data map[string]interface{}
		if err := json.Unmarshal(eventData, &data); err == nil {
			if containerID, ok := data["container_id"].(string); ok {
				pbEvent.ContainerId = containerID
			}
		}
	}

	return pbEvent, nil
}

// topicToEventType converts topic to EventType enum
func (s *SystemServiceServer) topicToEventType(topic string) pb.EventType {
	topic = strings.ToLower(topic)
	
	if strings.Contains(topic, "container") {
		return pb.EventType_CONTAINER_EVENT
	}
	if strings.Contains(topic, "image") {
		return pb.EventType_IMAGE_EVENT
	}
	if strings.Contains(topic, "network") {
		return pb.EventType_NETWORK_EVENT
	}
	if strings.Contains(topic, "volume") {
		return pb.EventType_VOLUME_EVENT
	}
	if strings.Contains(topic, "task") {
		return pb.EventType_TASK_EVENT
	}
	
	return pb.EventType_UNKNOWN_EVENT
}

// topicToStatus converts topic to status string
func (s *SystemServiceServer) topicToStatus(topic string) string {
	if strings.Contains(strings.ToLower(topic), "start") {
		return "start"
	}
	if strings.Contains(strings.ToLower(topic), "stop") {
		return "stop"
	}
	if strings.Contains(strings.ToLower(topic), "delete") {
		return "delete"
	}
	if strings.Contains(strings.ToLower(topic), "create") {
		return "create"
	}
	return "unknown"
}

// parseEventFilters parses filter strings into a filter map
func (s *SystemServiceServer) parseEventFilters(filters []string) (map[string][]string, error) {
	filterMap := make(map[string][]string)
	
	for _, filter := range filters {
		parts := strings.SplitN(filter, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid filter format: %s", filter)
		}
		
		key := strings.ToLower(parts[0])
		value := parts[1]
		
		filterMap[key] = append(filterMap[key], value)
	}
	
	return filterMap, nil
}

// eventMatchesFilters checks if event matches the given filters
func (s *SystemServiceServer) eventMatchesFilters(event *pb.SystemEvent, filterMap map[string][]string) bool {
	if len(filterMap) == 0 {
		return true
	}

	for filterKey, filterValues := range filterMap {
		match := false
		
		for _, filterValue := range filterValues {
			switch filterKey {
			case "event", "status":
				if strings.EqualFold(event.Status, filterValue) {
					match = true
				}
			case "container":
				if strings.Contains(event.ContainerId, filterValue) {
					match = true
				}
			case "type":
				if strings.EqualFold(event.Type.String(), filterValue) {
					match = true
				}
			}
			
			if match {
				break
			}
		}
		
		if !match {
			return false
		}
	}
	
	return true
}

// pruneContainers removes stopped containers
func (s *SystemServiceServer) pruneContainers(ctx context.Context, adapter types.ContainerManager, force bool) ([]string, error) {
	// Implementation would depend on the adapter interface
	// This is a placeholder for now
	return []string{}, nil
}

// pruneImages removes unused images
func (s *SystemServiceServer) pruneImages(ctx context.Context, adapter types.ContainerManager, force bool) ([]string, int64, error) {
	// Implementation would depend on the adapter interface
	// This is a placeholder for now
	return []string{}, 0, nil
}

// pruneNetworks removes unused networks
func (s *SystemServiceServer) pruneNetworks(ctx context.Context, adapter types.ContainerManager, force bool) ([]string, error) {
	// Implementation would depend on the adapter interface
	// This is a placeholder for now
	return []string{}, nil
}