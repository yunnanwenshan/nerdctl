package container

import (
	// Removed unused biz import
	"dsagent/internal/conf"
	"dsagent/internal/data"
	"dsagent/internal/server"
	"dsagent/internal/service"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-kratos/kratos/v2/transport/http"
	"go.uber.org/dig"
)

// Container wraps the dig container
type Container struct {
	container *dig.Container
}

// New creates a new DI container with all providers registered
func New() *Container {
	container := dig.New()
	c := &Container{container: container}
	
	// Register all providers
	c.registerDataProviders()
	c.registerBizProviders()
	c.registerServiceProviders()
	c.registerServerProviders()
	c.registerAppProvider()
	
	return c
}

// registerDataProviders registers all data layer providers
func (c *Container) registerDataProviders() {
	// Register data providers
	c.container.Provide(data.NewData)
	// Removed greeter repo provider
}

// registerBizProviders registers all business logic providers
func (c *Container) registerBizProviders() {
	// Register biz providers
	// Removed greeter usecase provider
}

// registerServiceProviders registers all service layer providers
func (c *Container) registerServiceProviders() {
	// Register service providers
	// Removed greeter service provider
	c.container.Provide(service.NewHealthService)
	// Use the new ImageServiceV2 with auto-ECR authentication
	c.container.Provide(service.NewImageServiceV2)
	// Register independent ECR authentication service
	c.container.Provide(service.NewECRService)
	// Register container service
	c.container.Provide(service.NewContainerService)
	// Register events service
	c.container.Provide(service.NewEventsService)
}

// registerServerProviders registers all server providers
func (c *Container) registerServerProviders() {
	// Register server providers
	c.container.Provide(server.NewGRPCServer)
	c.container.Provide(server.NewHTTPServer)
}

// registerAppProvider registers the main app provider
func (c *Container) registerAppProvider() {
	c.container.Provide(func(logger log.Logger, grpcServer *grpc.Server, httpServer *http.Server) *kratos.App {
		return kratos.New(
			kratos.ID("dsagent"),
			kratos.Name("dsagent"),
			kratos.Logger(logger),
			kratos.Server(
				grpcServer,
				httpServer,
			),
		)
	})
}

// BuildApp builds the complete application with all dependencies
func (c *Container) BuildApp(serverConf *conf.Server, dataConf *conf.Data, logger log.Logger) (*kratos.App, func(), error) {
	// Provide external dependencies
	err := c.container.Provide(func() *conf.Server { return serverConf })
	if err != nil {
		return nil, nil, err
	}
	
	err = c.container.Provide(func() *conf.Data { return dataConf })
	if err != nil {
		return nil, nil, err
	}
	
	err = c.container.Provide(func() log.Logger { return logger })
	if err != nil {
		return nil, nil, err
	}

	// Invoke to build the app
	var app *kratos.App
	var cleanup func()
	
	err = c.container.Invoke(func(a *kratos.App, dataCleanup func()) {
		app = a
		cleanup = dataCleanup
	})
	
	if err != nil {
		return nil, nil, err
	}
	
	return app, cleanup, nil
}

// GetContainer returns the underlying dig container for advanced usage
func (c *Container) GetContainer() *dig.Container {
	return c.container
}