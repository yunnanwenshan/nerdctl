package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/events"
	"github.com/containerd/log"
	"github.com/containerd/typeurl/v2"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "dsagent/api/events/v1"
	"github.com/containerd/nerdctl/v2/pkg/api/types"
	"github.com/containerd/nerdctl/v2/pkg/clientutil"
	"github.com/containerd/nerdctl/v2/pkg/cmd/system"
)

// EventsService implements the gRPC events service interface
type EventsService struct {
	pb.UnimplementedEventsServiceServer
}

// NewEventsService creates a new events service instance
func NewEventsService() *EventsService {
	return &EventsService{}
}

// GetSystemEvents streams real-time system events - equivalent to `nerdctl events`
func (s *EventsService) GetSystemEvents(req *pb.GetSystemEventsRequest, stream pb.EventsService_GetSystemEventsServer) error {
	log.L.Debug("Starting system events stream")

	// Convert request to nerdctl options
	options, err := s.convertSystemEventsRequest(req)
	if err != nil {
		return err
	}

	// Create containerd client
	client, _, cancel, err := clientutil.NewClient(context.Background(), options.GOptions.Namespace, options.GOptions.Address)
	if err != nil {
		return err
	}
	defer cancel()

	// Stream events using our custom implementation
	return s.streamEvents(stream.Context(), client, options, stream)
}

// GetFilteredSystemEvents streams system events with filters
func (s *EventsService) GetFilteredSystemEvents(req *pb.GetFilteredSystemEventsRequest, stream pb.EventsService_GetFilteredSystemEventsServer) error {
	log.L.Debug("Starting filtered system events stream")

	// Convert request to nerdctl options
	options, err := s.convertFilteredSystemEventsRequest(req)
	if err != nil {
		return err
	}

	// Create containerd client
	client, _, cancel, err := clientutil.NewClient(context.Background(), options.GOptions.Namespace, options.GOptions.Address)
	if err != nil {
		return err
	}
	defer cancel()

	// Stream events using our custom implementation
	return s.streamEvents(stream.Context(), client, options, stream)
}

// streamEvents is a custom implementation that streams containerd events to gRPC stream
// Based on nerdctl's pkg/cmd/system/events.go but adapted for streaming to gRPC
func (s *EventsService) streamEvents(ctx context.Context, client *containerd.Client, options types.SystemEventsOptions, stream interface{}) error {
	eventsClient := client.EventService()
	eventsCh, errCh := eventsClient.Subscribe(ctx)

	// Generate event filters
	filterMap, err := s.generateEventFilters(options.Filters)
	if err != nil {
		return err
	}

	// Stream events until context is cancelled
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errCh:
			log.L.WithError(err).Error("Error in event subscription")
			return err
		case envelope := <-eventsCh:
			if envelope != nil {
				// Convert containerd event to our protobuf event
				pbEvent, err := s.convertEventEnvelopeToProto(envelope)
				if err != nil {
					log.L.WithError(err).Warn("Failed to convert event to protobuf")
					continue
				}

				// Apply filters
				eventOut := s.convertProtoToEventOut(pbEvent)
				if !s.applyFilters(eventOut, filterMap) {
					continue // Skip event if it doesn't match filters
				}

				// Send event to stream - both interfaces are identical, so we can use interface{} cast
				if sender, ok := stream.(interface{ Send(*pb.SystemEvent) error }); ok {
					if err := sender.Send(pbEvent); err != nil {
						log.L.WithError(err).Error("Failed to send event to stream")
						return err
					}
				} else {
					log.L.Error("Stream does not implement expected interface")
					return fmt.Errorf("invalid stream type")
				}
			}
		}
	}
}

// convertEventEnvelopeToProto converts containerd event envelope to protobuf SystemEvent
func (s *EventsService) convertEventEnvelopeToProto(envelope *events.Envelope) (*pb.SystemEvent, error) {
	pbEvent := &pb.SystemEvent{
		Timestamp: timestamppb.New(envelope.Timestamp),
		Namespace: envelope.Namespace,
		Topic:     envelope.Topic,
		Status:    s.topicToStatus(envelope.Topic),
	}

	// Extract container ID and event data
	if envelope.Event != nil {
		v, err := typeurl.UnmarshalAny(envelope.Event)
		if err != nil {
			log.L.WithError(err).Warn("cannot unmarshal an event from Any")
			return pbEvent, nil // Return basic event without detailed data
		}

		// Marshal to JSON for event data
		out, err := json.Marshal(v)
		if err != nil {
			log.L.WithError(err).Warn("cannot marshal Any into JSON")
			return pbEvent, nil // Return basic event without detailed data
		}
		pbEvent.EventData = string(out)

		// Parse event data to extract additional info
		var data map[string]interface{}
		if err := json.Unmarshal(out, &data); err == nil {
			// Extract container ID if present
			if containerID, ok := data["container_id"].(string); ok {
				pbEvent.ContainerId = containerID
			}

			// Populate attributes map
			pbEvent.Attributes = make(map[string]string)
			for k, v := range data {
				if str, ok := v.(string); ok {
					pbEvent.Attributes[k] = str
				}
			}
		}
	}

	// Determine event type and action from topic
	pbEvent.EventType, pbEvent.Action = s.parseTopicForTypeAndAction(envelope.Topic)

	return pbEvent, nil
}

// EventOut represents an event for filtering (copied from nerdctl)
type EventOut struct {
	Timestamp time.Time
	ID        string
	Namespace string
	Topic     string
	Status    string
	Event     string
}

// EventFilter for filtering events
type EventFilter func(*EventOut) bool

// convertProtoToEventOut converts protobuf SystemEvent to EventOut for filtering compatibility
func (s *EventsService) convertProtoToEventOut(pbEvent *pb.SystemEvent) *EventOut {
	return &EventOut{
		Timestamp: pbEvent.Timestamp.AsTime(),
		ID:        pbEvent.ContainerId,
		Namespace: pbEvent.Namespace,
		Topic:     pbEvent.Topic,
		Status:    pbEvent.Status,
		Event:     pbEvent.EventData,
	}
}

// topicToStatus converts event topic to status string (similar to nerdctl's TopicToStatus)
func (s *EventsService) topicToStatus(topic string) string {
	return string(system.TopicToStatus(topic))
}

// parseTopicForTypeAndAction extracts event type and action from topic
// e.g., "/containers/create" -> type: "container", action: "create"
func (s *EventsService) parseTopicForTypeAndAction(topic string) (string, string) {
	// Simple parsing logic - can be enhanced
	if len(topic) > 0 && topic[0] == '/' {
		topic = topic[1:] // Remove leading slash
	}

	parts := make([]string, 0)
	for _, part := range []string{topic} {
		if part != "" {
			parts = append(parts, part)
		}
	}

	if len(parts) == 0 {
		return "unknown", "unknown"
	}

	// Split by slash to get type and action
	eventType := "unknown"
	action := "unknown"

	// Look for common patterns
	if len(topic) > 0 {
		if len(parts) >= 1 {
			firstPart := parts[0]
			if firstPart == "containers" || firstPart == "container" {
				eventType = "container"
			} else if firstPart == "images" || firstPart == "image" {
				eventType = "image"
			} else if firstPart == "volumes" || firstPart == "volume" {
				eventType = "volume"
			} else if firstPart == "networks" || firstPart == "network" {
				eventType = "network"
			} else {
				eventType = "system"
			}
		}

		// Extract action from topic - look for common action keywords
		if topic != "" {
			if containsString(topic, "create") {
				action = "create"
			} else if containsString(topic, "start") {
				action = "start"
			} else if containsString(topic, "stop") {
				action = "stop"
			} else if containsString(topic, "delete") || containsString(topic, "remove") {
				action = "delete"
			} else if containsString(topic, "pause") {
				action = "pause"
			} else if containsString(topic, "unpause") {
				action = "unpause"
			} else if containsString(topic, "kill") {
				action = "kill"
			} else if containsString(topic, "restart") {
				action = "restart"
			}
		}
	}

	return eventType, action
}

// containsString checks if a string contains another string (case insensitive)
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || 
		(len(s) > len(substr) && 
		 (s[len(s)-len(substr):] == substr || 
		  s[:len(substr)] == substr ||
		  findInString(s, substr))))
}

func findInString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Helper conversion functions

func (s *EventsService) convertSystemEventsRequest(req *pb.GetSystemEventsRequest) (types.SystemEventsOptions, error) {
	return types.SystemEventsOptions{
		Stdout:   io.Discard, // We don't use stdout for gRPC streaming
		GOptions: types.GlobalCommandOptions{
			Address:   "/run/containerd/containerd.sock",
			Namespace: "default",
			DataRoot:  "/var/lib/nerdctl",
		},
		Format:  req.Format,
		Filters: []string{}, // No filters for simple request
	}, nil
}

func (s *EventsService) convertFilteredSystemEventsRequest(req *pb.GetFilteredSystemEventsRequest) (types.SystemEventsOptions, error) {
	return types.SystemEventsOptions{
		Stdout:   io.Discard, // We don't use stdout for gRPC streaming
		GOptions: types.GlobalCommandOptions{
			Address:   "/run/containerd/containerd.sock",
			Namespace: "default",
			DataRoot:  "/var/lib/nerdctl",
		},
		Format:  req.Format,
		Filters: req.Filters,
	}, nil
}

// Event filtering functions (copied and adapted from nerdctl)

// isStatus checks if a string is a valid status
func (s *EventsService) isStatus(status string) bool {
	status = strings.ToLower(status)
	statuses := []string{"start", "unknown"}
	for _, supportedStatus := range statuses {
		if supportedStatus == status {
			return true
		}
	}
	return false
}

// generateEventFilter generates a filter function for events
func (s *EventsService) generateEventFilter(filter, filterValue string) (EventFilter, error) {
	switch strings.ToUpper(filter) {
	case "EVENT", "STATUS":
		return func(e *EventOut) bool {
			if !s.isStatus(e.Status) {
				return false
			}
			return strings.EqualFold(e.Status, filterValue)
		}, nil
	}
	return nil, fmt.Errorf("%s is an invalid or unsupported filter", filter)
}

// parseFilter parses a filter string into key and value
func (s *EventsService) parseFilter(filter string) (string, string, error) {
	filterSplit := strings.SplitN(filter, "=", 2)
	if len(filterSplit) != 2 {
		return "", "", fmt.Errorf("%s is an invalid filter", filter)
	}
	return filterSplit[0], filterSplit[1], nil
}

// applyFilters applies filters to an event
func (s *EventsService) applyFilters(event *EventOut, filterMap map[string][]EventFilter) bool {
	for _, filters := range filterMap {
		match := false
		for _, filter := range filters {
			if filter(event) {
				match = true
				break
			}
		}
		if !match {
			return false
		}
	}
	return true
}

// generateEventFilters generates a map of event filters
func (s *EventsService) generateEventFilters(filters []string) (map[string][]EventFilter, error) {
	filterMap := make(map[string][]EventFilter)
	for _, filter := range filters {
		key, val, err := s.parseFilter(filter)
		if err != nil {
			return nil, err
		}
		filterFunc, err := s.generateEventFilter(key, val)
		if err != nil {
			return nil, err
		}
		filterSlice := filterMap[key]
		filterSlice = append(filterSlice, filterFunc)
		filterMap[key] = filterSlice
	}
	return filterMap, nil
}