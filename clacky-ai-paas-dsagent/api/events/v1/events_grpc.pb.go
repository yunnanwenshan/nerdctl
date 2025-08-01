// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.5.1
// - protoc             v3.12.4
// source: events/v1/events.proto

package v1

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.64.0 or later.
const _ = grpc.SupportPackageIsVersion9

const (
	EventsService_GetSystemEvents_FullMethodName         = "/events.v1.EventsService/GetSystemEvents"
	EventsService_GetFilteredSystemEvents_FullMethodName = "/events.v1.EventsService/GetFilteredSystemEvents"
)

// EventsServiceClient is the client API for EventsService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
//
// EventsService provides methods for monitoring container and system events.
type EventsServiceClient interface {
	// Get real-time system events stream - equivalent to `nerdctl events`
	GetSystemEvents(ctx context.Context, in *GetSystemEventsRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[SystemEvent], error)
	// Get system events with filters
	GetFilteredSystemEvents(ctx context.Context, in *GetFilteredSystemEventsRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[SystemEvent], error)
}

type eventsServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewEventsServiceClient(cc grpc.ClientConnInterface) EventsServiceClient {
	return &eventsServiceClient{cc}
}

func (c *eventsServiceClient) GetSystemEvents(ctx context.Context, in *GetSystemEventsRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[SystemEvent], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &EventsService_ServiceDesc.Streams[0], EventsService_GetSystemEvents_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[GetSystemEventsRequest, SystemEvent]{ClientStream: stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type EventsService_GetSystemEventsClient = grpc.ServerStreamingClient[SystemEvent]

func (c *eventsServiceClient) GetFilteredSystemEvents(ctx context.Context, in *GetFilteredSystemEventsRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[SystemEvent], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &EventsService_ServiceDesc.Streams[1], EventsService_GetFilteredSystemEvents_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[GetFilteredSystemEventsRequest, SystemEvent]{ClientStream: stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type EventsService_GetFilteredSystemEventsClient = grpc.ServerStreamingClient[SystemEvent]

// EventsServiceServer is the server API for EventsService service.
// All implementations must embed UnimplementedEventsServiceServer
// for forward compatibility.
//
// EventsService provides methods for monitoring container and system events.
type EventsServiceServer interface {
	// Get real-time system events stream - equivalent to `nerdctl events`
	GetSystemEvents(*GetSystemEventsRequest, grpc.ServerStreamingServer[SystemEvent]) error
	// Get system events with filters
	GetFilteredSystemEvents(*GetFilteredSystemEventsRequest, grpc.ServerStreamingServer[SystemEvent]) error
	mustEmbedUnimplementedEventsServiceServer()
}

// UnimplementedEventsServiceServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedEventsServiceServer struct{}

func (UnimplementedEventsServiceServer) GetSystemEvents(*GetSystemEventsRequest, grpc.ServerStreamingServer[SystemEvent]) error {
	return status.Errorf(codes.Unimplemented, "method GetSystemEvents not implemented")
}
func (UnimplementedEventsServiceServer) GetFilteredSystemEvents(*GetFilteredSystemEventsRequest, grpc.ServerStreamingServer[SystemEvent]) error {
	return status.Errorf(codes.Unimplemented, "method GetFilteredSystemEvents not implemented")
}
func (UnimplementedEventsServiceServer) mustEmbedUnimplementedEventsServiceServer() {}
func (UnimplementedEventsServiceServer) testEmbeddedByValue()                       {}

// UnsafeEventsServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to EventsServiceServer will
// result in compilation errors.
type UnsafeEventsServiceServer interface {
	mustEmbedUnimplementedEventsServiceServer()
}

func RegisterEventsServiceServer(s grpc.ServiceRegistrar, srv EventsServiceServer) {
	// If the following call pancis, it indicates UnimplementedEventsServiceServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&EventsService_ServiceDesc, srv)
}

func _EventsService_GetSystemEvents_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(GetSystemEventsRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(EventsServiceServer).GetSystemEvents(m, &grpc.GenericServerStream[GetSystemEventsRequest, SystemEvent]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type EventsService_GetSystemEventsServer = grpc.ServerStreamingServer[SystemEvent]

func _EventsService_GetFilteredSystemEvents_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(GetFilteredSystemEventsRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(EventsServiceServer).GetFilteredSystemEvents(m, &grpc.GenericServerStream[GetFilteredSystemEventsRequest, SystemEvent]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type EventsService_GetFilteredSystemEventsServer = grpc.ServerStreamingServer[SystemEvent]

// EventsService_ServiceDesc is the grpc.ServiceDesc for EventsService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var EventsService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "events.v1.EventsService",
	HandlerType: (*EventsServiceServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "GetSystemEvents",
			Handler:       _EventsService_GetSystemEvents_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "GetFilteredSystemEvents",
			Handler:       _EventsService_GetFilteredSystemEvents_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "events/v1/events.proto",
}
