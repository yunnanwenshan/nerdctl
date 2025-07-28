package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"time"

	"github.com/containerd/nerdctl-grpc-server/internal/adapters/factory"
	"github.com/containerd/nerdctl-grpc-server/internal/config"
	"github.com/containerd/nerdctl-grpc-server/internal/interfaces"
	"github.com/containerd/nerdctl-grpc-server/internal/version"
	pb "github.com/containerd/nerdctl-grpc-server/api/proto"
	
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"github.com/sirupsen/logrus"
)

// Server represents the main gRPC server
// This demonstrates the complete decoupled architecture:
// - Server doesn't know about specific nerdctl versions
// - Uses adapter factory to get appropriate managers
// - All operations go through abstract interfaces
type Server struct {
	config   *config.Config
	grpcServer *grpc.Server
	listener   net.Listener
	
	// Version detector for nerdctl compatibility
	versionDetector *version.Detector
	
	// Adapter factory for creating version-specific adapters
	adapterFactory *factory.AdapterFactory
	
	// Abstract interface managers (version-agnostic)
	containerManager interfaces.ContainerManager
	imageManager     interfaces.ImageManager
	networkManager   interfaces.NetworkManager
	volumeManager    interfaces.VolumeManager
	composeManager   interfaces.ComposeManager
	systemManager    interfaces.SystemManager
	
	// gRPC service servers
	containerService *ContainerServiceServer
	imageService     *ImageServiceServer
	networkService   *NetworkServiceServer
	healthService    *health.Server
	
	// Server state
	running bool
	logger  *logrus.Logger
}

// NewServer creates a new gRPC server instance
func NewServer(cfg *config.Config) (*Server, error) {
	logger := logrus.New()
	logger.SetLevel(cfg.LogLevel)

	// Create version detector
	versionDetector := version.NewDetector(cfg.Nerdctl.BinaryPath)
	
	// Create adapter factory
	adapterFactory, err := factory.NewAdapterFactory(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create adapter factory: %w", err)
	}

	server := &Server{
		config:          cfg,
		logger:          logger,
		versionDetector: versionDetector,
		adapterFactory:  adapterFactory,
		running:         false,
	}

	return server, nil
}

// Initialize initializes the server components
func (s *Server) Initialize(ctx context.Context) error {
	s.logger.Info("Initializing nerdctl gRPC server...")

	// Detect nerdctl version
	versionInfo, err := s.versionDetector.DetectVersion(ctx)
	if err != nil {
		return fmt.Errorf("failed to detect nerdctl version: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"nerdctl_version":    versionInfo.Client.Raw,
		"containerd_version": versionInfo.Components.Containerd,
		"runc_version":       versionInfo.Components.Runc,
	}).Info("Detected nerdctl and runtime versions")

	// Create version-appropriate adapters using the factory
	adapters, err := s.adapterFactory.CreateAdapters(ctx, versionInfo)
	if err != nil {
		return fmt.Errorf("failed to create adapters: %w", err)
	}

	// Assign the abstract interface managers
	// The server doesn't know what version-specific adapter is being used
	s.containerManager = adapters.ContainerManager
	s.imageManager = adapters.ImageManager
	s.networkManager = adapters.NetworkManager
	s.volumeManager = adapters.VolumeManager
	s.composeManager = adapters.ComposeManager
	s.systemManager = adapters.SystemManager

	// Create gRPC service servers using the abstract interfaces
	s.containerService = NewContainerServiceServer(s.containerManager)
	s.imageService = NewImageServiceServer(s.imageManager)
	s.networkService = NewNetworkServiceServer(s.networkManager)
	s.healthService = health.NewServer()

	// Initialize gRPC server
	if err := s.initializeGRPCServer(); err != nil {
		return fmt.Errorf("failed to initialize gRPC server: %w", err)
	}

	s.logger.Info("Server initialization completed")
	return nil
}

// initializeGRPCServer initializes the gRPC server with all services
func (s *Server) initializeGRPCServer() error {
	// Create gRPC server options
	var opts []grpc.ServerOption

	// Add TLS if enabled
	if s.config.Server.TLS.Enabled {
		cert, err := tls.LoadX509KeyPair(s.config.Server.TLS.CertFile, s.config.Server.TLS.KeyFile)
		if err != nil {
			return fmt.Errorf("failed to load TLS certificates: %w", err)
		}
		
		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
			ClientAuth:   tls.RequireAndVerifyClientCert,
		}
		
		creds := credentials.NewTLS(tlsConfig)
		opts = append(opts, grpc.Creds(creds))
		s.logger.Info("TLS encryption enabled")
	}

	// Add interceptors for logging, auth, metrics, etc.
	opts = append(opts, 
		grpc.UnaryInterceptor(s.unaryInterceptor),
		grpc.StreamInterceptor(s.streamInterceptor),
	)

	// Create gRPC server
	s.grpcServer = grpc.NewServer(opts...)

	// Register all services
	pb.RegisterContainerServiceServer(s.grpcServer, s.containerService)
	pb.RegisterImageServiceServer(s.grpcServer, s.imageService)
	pb.RegisterNetworkServiceServer(s.grpcServer, s.networkService)
	grpc_health_v1.RegisterHealthServer(s.grpcServer, s.healthService)

	// Enable reflection for development
	if s.config.Server.EnableReflection {
		reflection.Register(s.grpcServer)
		s.logger.Info("gRPC reflection enabled")
	}

	// Create listener
	addr := fmt.Sprintf("%s:%d", s.config.Server.Address, s.config.Server.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}
	s.listener = listener

	s.logger.WithField("address", addr).Info("gRPC server configured")
	return nil
}

// Start starts the gRPC server
func (s *Server) Start(ctx context.Context) error {
	if s.running {
		return fmt.Errorf("server is already running")
	}

	s.logger.Info("Starting nerdctl gRPC server...")

	// Set health status to serving
	s.healthService.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := s.grpcServer.Serve(s.listener); err != nil {
			errChan <- err
		}
	}()

	s.running = true
	s.logger.WithField("address", s.listener.Addr().String()).Info("nerdctl gRPC server started successfully")

	// Wait for context cancellation or server error
	select {
	case <-ctx.Done():
		s.logger.Info("Shutdown signal received")
		return s.Stop()
	case err := <-errChan:
		s.running = false
		return fmt.Errorf("server error: %w", err)
	}
}

// Stop stops the gRPC server gracefully
func (s *Server) Stop() error {
	if !s.running {
		return nil
	}

	s.logger.Info("Stopping nerdctl gRPC server...")

	// Set health status to not serving
	s.healthService.SetServingStatus("", grpc_health_v1.HealthCheckResponse_NOT_SERVING)

	// Graceful shutdown with timeout
	done := make(chan struct{})
	go func() {
		s.grpcServer.GracefulStop()
		close(done)
	}()

	// Wait for graceful shutdown or force stop after timeout
	timer := time.NewTimer(30 * time.Second)
	select {
	case <-done:
		timer.Stop()
		s.logger.Info("Server stopped gracefully")
	case <-timer.C:
		s.logger.Warn("Graceful shutdown timeout, forcing stop")
		s.grpcServer.Stop()
	}

	s.running = false
	return nil
}

// IsRunning returns whether the server is running
func (s *Server) IsRunning() bool {
	return s.running
}

// GetAddress returns the server listening address
func (s *Server) GetAddress() string {
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return ""
}

// GetVersionInfo returns detected version information
func (s *Server) GetVersionInfo(ctx context.Context) (*version.NerdctlVersionInfo, error) {
	return s.versionDetector.DetectVersion(ctx)
}

// unaryInterceptor provides logging and metrics for unary RPCs
func (s *Server) unaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	start := time.Now()
	
	s.logger.WithFields(logrus.Fields{
		"method":    info.FullMethod,
		"request":   fmt.Sprintf("%T", req),
	}).Debug("Processing unary RPC")

	resp, err := handler(ctx, req)
	
	duration := time.Since(start)
	level := logrus.InfoLevel
	if err != nil {
		level = logrus.ErrorLevel
	}
	
	s.logger.WithFields(logrus.Fields{
		"method":   info.FullMethod,
		"duration": duration,
		"error":    err,
	}).Log(level, "Completed unary RPC")

	return resp, err
}

// streamInterceptor provides logging and metrics for streaming RPCs
func (s *Server) streamInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	start := time.Now()
	
	s.logger.WithFields(logrus.Fields{
		"method":        info.FullMethod,
		"is_client_stream": info.IsClientStream,
		"is_server_stream": info.IsServerStream,
	}).Debug("Processing streaming RPC")

	err := handler(srv, ss)
	
	duration := time.Since(start)
	level := logrus.InfoLevel
	if err != nil {
		level = logrus.ErrorLevel
	}
	
	s.logger.WithFields(logrus.Fields{
		"method":   info.FullMethod,
		"duration": duration,
		"error":    err,
	}).Log(level, "Completed streaming RPC")

	return err
}

// HealthCheck implements basic health checking
func (s *Server) HealthCheck(ctx context.Context) error {
	// Verify that adapters are still working
	if s.containerManager != nil {
		if _, err := s.containerManager.GetVersion(ctx); err != nil {
			return fmt.Errorf("container manager health check failed: %w", err)
		}
	}

	if s.imageManager != nil {
		if _, err := s.imageManager.GetVersion(ctx); err != nil {
			return fmt.Errorf("image manager health check failed: %w", err)
		}
	}

	return nil
}

// GetMetrics returns server metrics (for Prometheus integration)
func (s *Server) GetMetrics() map[string]interface{} {
	return map[string]interface{}{
		"server_running": s.running,
		"server_address": s.GetAddress(),
		"uptime":         time.Since(time.Now()).Seconds(), // This should be tracked properly
	}
}