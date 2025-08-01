package server

import (
	// Removed helloworld import
	healthv1 "dsagent/api/health/v1"
	imagev1 "dsagent/api/image/v1"
	ecrv1 "dsagent/api/ecr/v1"
	containerv1 "dsagent/api/container/v1"
	eventsv1 "dsagent/api/events/v1"
	nydusifyv1 "dsagent/api/nydusify/v1"
	"dsagent/internal/conf"
	"dsagent/internal/service"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/transport/grpc"
)

// NewGRPCServer new a gRPC server.
func NewGRPCServer(c *conf.Server, health *service.HealthService, imageService *service.ImageServiceV2, ecrService *service.ECRService, containerService *service.ContainerService, eventsService *service.EventsService, nydusifyService *service.NydusifyService, logger log.Logger) *grpc.Server {
	var opts = []grpc.ServerOption{
		grpc.Middleware(
			recovery.Recovery(),
		),
	}
	if c.Grpc.Network != "" {
		opts = append(opts, grpc.Network(c.Grpc.Network))
	}
	if c.Grpc.Addr != "" {
		opts = append(opts, grpc.Address(c.Grpc.Addr))
	}
	if c.Grpc.Timeout != nil {
		opts = append(opts, grpc.Timeout(c.Grpc.Timeout.AsDuration()))
	}
	srv := grpc.NewServer(opts...)
	// Removed greeter server registration
	healthv1.RegisterHealthServer(srv, health)
	imagev1.RegisterImageServiceServer(srv, imageService)
	ecrv1.RegisterECRServiceServer(srv, ecrService)
	containerv1.RegisterContainerServiceServer(srv, containerService)
	eventsv1.RegisterEventsServiceServer(srv, eventsService)
	nydusifyv1.RegisterNydusifyServiceServer(srv, nydusifyService)
	return srv
}
