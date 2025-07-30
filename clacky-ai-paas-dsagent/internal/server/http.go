package server

import (
	v1 "dsagent/api/helloworld/v1"
	healthv1 "dsagent/api/health/v1"
	imagev1 "dsagent/api/image/v1"
	"dsagent/internal/conf"
	"dsagent/internal/service"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/transport/http"
)

// NewHTTPServer new an HTTP server.
func NewHTTPServer(c *conf.Server, greeter *service.GreeterService, health *service.HealthService, imageService *service.ImageServiceV2, logger log.Logger) *http.Server {
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
	v1.RegisterGreeterHTTPServer(srv, greeter)
	healthv1.RegisterHealthHTTPServer(srv, health)
	imagev1.RegisterImageServiceHTTPServer(srv, imageService)
	return srv
}
