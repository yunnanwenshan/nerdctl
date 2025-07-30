package server

import (
	// Removed helloworld import
	healthv1 "dsagent/api/health/v1"
	imagev1 "dsagent/api/image/v1"
	ecrv1 "dsagent/api/ecr/v1"
	containerv1 "dsagent/api/container/v1"
	"dsagent/internal/conf"
	"dsagent/internal/service"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/transport/http"
)

// NewHTTPServer new an HTTP server.
func NewHTTPServer(c *conf.Server, health *service.HealthService, imageService *service.ImageServiceV2, ecrService *service.ECRService, containerService *service.ContainerService, eventsService *service.EventsService, logger log.Logger) *http.Server {
	var opts = []http.ServerOption{
		http.Middleware(
			recovery.Recovery(),
		),
	}
	if c.Http.Network != "" {
		opts = append(opts, http.Network(c.Http.Network))
	}
	if c.Http.Addr != "" {
		opts = append(opts, http.Address(c.Http.Addr))
	}
	if c.Http.Timeout != nil {
		opts = append(opts, http.Timeout(c.Http.Timeout.AsDuration()))
	}
	srv := http.NewServer(opts...)
	// Removed greeter HTTP server registration
	healthv1.RegisterHealthHTTPServer(srv, health)
	imagev1.RegisterImageServiceHTTPServer(srv, imageService)
	ecrv1.RegisterECRServiceHTTPServer(srv, ecrService)
	containerv1.RegisterContainerServiceHTTPServer(srv, containerService)
	return srv
}
