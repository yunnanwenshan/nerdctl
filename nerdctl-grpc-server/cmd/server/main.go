package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/containerd/nerdctl-grpc-server/internal/config"
	"github.com/containerd/nerdctl-grpc-server/internal/server"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildTime = "unknown"
)

func main() {
	var (
		configPath    = flag.String("config", "", "Path to configuration file")
		devMode       = flag.Bool("dev", false, "Enable development mode")
		showVersion   = flag.Bool("version", false, "Show version information")
		logLevel      = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
		serverAddr    = flag.String("address", "", "Server address (overrides config)")
		serverPort    = flag.Int("port", 0, "Server port (overrides config)")
		nerdctlPath   = flag.String("nerdctl-path", "", "Path to nerdctl binary (overrides config)")
		enableTLS     = flag.Bool("enable-tls", false, "Enable TLS (overrides config)")
		tlsCertFile   = flag.String("tls-cert", "", "TLS certificate file (overrides config)")
		tlsKeyFile    = flag.String("tls-key", "", "TLS key file (overrides config)")
		enableReflection = flag.Bool("enable-reflection", false, "Enable gRPC reflection (overrides config)")
	)
	
	flag.Parse()

	// Show version information
	if *showVersion {
		fmt.Printf("nerdctl-grpc-server\n")
		fmt.Printf("Version: %s\n", version)
		fmt.Printf("Commit: %s\n", commit)
		fmt.Printf("Build Time: %s\n", buildTime)
		os.Exit(0)
	}

	// Setup logger
	logger := logrus.New()
	if level, err := logrus.ParseLevel(*logLevel); err == nil {
		logger.SetLevel(level)
	} else {
		logger.Warnf("Invalid log level %q, using info", *logLevel)
		logger.SetLevel(logrus.InfoLevel)
	}

	// Development mode setup
	if *devMode {
		logger.SetLevel(logrus.DebugLevel)
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
			ForceColors:   true,
		})
		logger.Info("Development mode enabled")
		
		// Override some flags for development
		if *enableReflection == false {
			*enableReflection = true
		}
	} else {
		logger.SetFormatter(&logrus.JSONFormatter{})
	}

	// Load configuration
	cfg, err := loadConfiguration(*configPath, *devMode)
	if err != nil {
		logger.Fatalf("Failed to load configuration: %v", err)
	}

	// Apply command line overrides
	if err := applyCommandLineOverrides(cfg, 
		*serverAddr, *serverPort, *nerdctlPath, *enableTLS, 
		*tlsCertFile, *tlsKeyFile, *enableReflection, *logLevel); err != nil {
		logger.Fatalf("Failed to apply command line overrides: %v", err)
	}

	// Validate final configuration
	if err := validateConfig(cfg); err != nil {
		logger.Fatalf("Configuration validation failed: %v", err)
	}

	logger.WithFields(logrus.Fields{
		"version":    version,
		"commit":     commit,
		"build_time": buildTime,
		"config":     *configPath,
		"dev_mode":   *devMode,
	}).Info("Starting nerdctl gRPC server")

	// Create context that listens for interrupt signals
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		logger.WithField("signal", sig).Info("Received shutdown signal")
		cancel()
	}()

	// Create fallback server if complex server fails
	srv, err := createServerWithFallback(cfg, logger)
	if err != nil {
		logger.Fatalf("Failed to create server: %v", err)
	}

	// Start server with graceful shutdown
	if err := runServerWithGracefulShutdown(ctx, srv, logger); err != nil {
		logger.Errorf("Server error: %v", err)
		os.Exit(1)
	}

	logger.Info("Server shutdown completed")
}

// loadConfiguration loads the configuration from file or creates default config
func loadConfiguration(configPath string, devMode bool) (*config.Config, error) {
	if configPath == "" {
		if devMode {
			// In dev mode, use default configuration
			return config.NewDefaultConfig(), nil
		}
		
		// Try to find config in standard locations
		possiblePaths := []string{
			"./config.yaml",
			"./nerdctl-grpc-server.yaml",
			"/etc/nerdctl-grpc-server/config.yaml",
			filepath.Join(os.Getenv("HOME"), ".config", "nerdctl-grpc-server", "config.yaml"),
		}
		
		for _, path := range possiblePaths {
			if _, err := os.Stat(path); err == nil {
				configPath = path
				break
			}
		}
		
		if configPath == "" {
			logrus.Info("No configuration file found, using defaults")
			return config.NewDefaultConfig(), nil
		}
	}

	// Load configuration from file
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config from %s: %w", configPath, err)
	}

	logrus.WithField("config_path", configPath).Info("Configuration loaded successfully")
	return cfg, nil
}

// applyCommandLineOverrides applies command line flag overrides to the configuration
func applyCommandLineOverrides(cfg *config.Config, 
	addr string, port int, nerdctlPath string, enableTLS bool,
	tlsCertFile, tlsKeyFile string, enableReflection bool, logLevel string) error {
	
	// Server address and port
	if addr != "" {
		cfg.Server.Address = addr
	}
	if port != 0 {
		cfg.Server.Port = port
	}
	
	// Nerdctl binary path
	if nerdctlPath != "" {
		cfg.Nerdctl.BinaryPath = nerdctlPath
	}
	
	// TLS configuration
	if enableTLS {
		cfg.Server.TLS.Enabled = true
		if tlsCertFile != "" {
			cfg.Server.TLS.CertFile = tlsCertFile
		}
		if tlsKeyFile != "" {
			cfg.Server.TLS.KeyFile = tlsKeyFile
		}
	}
	
	// gRPC reflection
	cfg.Server.EnableReflection = enableReflection
	
	// Log level
	if level, err := logrus.ParseLevel(logLevel); err == nil {
		cfg.LogLevel = level
	}

	return nil
}

// validateConfig validates the server configuration
func validateConfig(cfg *config.Config) error {
	if cfg.Server.Port <= 0 || cfg.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d (must be 1-65535)", cfg.Server.Port)
	}

	if cfg.Server.TLS.Enabled {
		if cfg.Server.TLS.CertFile == "" {
			return fmt.Errorf("TLS is enabled but cert file is not specified")
		}
		if cfg.Server.TLS.KeyFile == "" {
			return fmt.Errorf("TLS is enabled but key file is not specified")
		}
		
		// Check if certificate files exist
		if _, err := os.Stat(cfg.Server.TLS.CertFile); os.IsNotExist(err) {
			return fmt.Errorf("TLS cert file not found: %s", cfg.Server.TLS.CertFile)
		}
		if _, err := os.Stat(cfg.Server.TLS.KeyFile); os.IsNotExist(err) {
			return fmt.Errorf("TLS key file not found: %s", cfg.Server.TLS.KeyFile)
		}
	}

	// Check if nerdctl binary exists
	if cfg.Nerdctl.BinaryPath != "nerdctl" { // Skip check if using PATH
		if _, err := os.Stat(cfg.Nerdctl.BinaryPath); os.IsNotExist(err) {
			return fmt.Errorf("nerdctl binary not found: %s", cfg.Nerdctl.BinaryPath)
		}
	}

	return nil
}

// createServerWithFallback attempts to create the complex server, falls back to simple server if needed
func createServerWithFallback(cfg *config.Config, logger *logrus.Logger) (ServerInterface, error) {
	// Try to create the complex server first
	if srv, err := server.NewServer(cfg); err == nil {
		logger.Info("Using enhanced server with full feature set")
		return &ComplexServerWrapper{server: srv, config: cfg, logger: logger}, nil
	} else {
		logger.WithError(err).Warn("Failed to create enhanced server, falling back to simple server")
	}

	// Fallback to simple server
	logger.Info("Using simple gRPC server fallback")
	return &SimpleServerWrapper{config: cfg, logger: logger}, nil
}

// ServerInterface defines common server interface for both complex and simple servers
type ServerInterface interface {
	Initialize(ctx context.Context) error
	Start(ctx context.Context) error
	Stop() error
	HealthCheck() error
}

// ComplexServerWrapper wraps the complex server
type ComplexServerWrapper struct {
	server *server.Server
	config *config.Config
	logger *logrus.Logger
}

func (w *ComplexServerWrapper) Initialize(ctx context.Context) error {
	return w.server.Initialize(ctx)
}

func (w *ComplexServerWrapper) Start(ctx context.Context) error {
	return w.server.Start(ctx)
}

func (w *ComplexServerWrapper) Stop() error {
	return w.server.Stop()
}

func (w *ComplexServerWrapper) HealthCheck() error {
	return w.server.HealthCheck()
}

// SimpleServerWrapper implements a fallback simple server
type SimpleServerWrapper struct {
	config     *config.Config
	logger     *logrus.Logger
	grpcServer *grpc.Server
	listener   net.Listener
	healthSrv  *health.Server
	running    bool
	mu         sync.RWMutex
}

func (w *SimpleServerWrapper) Initialize(ctx context.Context) error {
	w.logger.Info("Initializing simple gRPC server...")
	
	// Create gRPC server
	w.grpcServer = grpc.NewServer()
	w.healthSrv = health.NewServer()
	
	// Register health service
	grpc_health_v1.RegisterHealthServer(w.grpcServer, w.healthSrv)
	
	// Enable reflection if configured
	if w.config.Server.EnableReflection {
		reflection.Register(w.grpcServer)
		w.logger.Info("gRPC reflection enabled")
	}
	
	// Create listener
	addr := fmt.Sprintf("%s:%d", w.config.Server.Address, w.config.Server.Port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}
	w.listener = lis
	
	w.logger.WithField("address", addr).Info("Simple gRPC server initialized")
	return nil
}

func (w *SimpleServerWrapper) Start(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	
	if w.running {
		return fmt.Errorf("server is already running")
	}
	
	w.logger.WithField("address", w.listener.Addr()).Info("Starting simple gRPC server")
	
	// Set health status to serving
	w.healthSrv.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	w.running = true
	
	// Start server in goroutine
	errorCh := make(chan error, 1)
	go func() {
		if err := w.grpcServer.Serve(w.listener); err != nil {
			errorCh <- fmt.Errorf("gRPC server failed: %w", err)
		}
	}()
	
	// Wait for context cancellation or server error
	select {
	case <-ctx.Done():
		w.logger.Info("Received shutdown signal")
		return w.Stop()
	case err := <-errorCh:
		w.logger.WithError(err).Error("Server error")
		return err
	}
}

func (w *SimpleServerWrapper) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	
	if !w.running {
		return nil
	}
	
	w.logger.Info("Stopping simple gRPC server")
	
	// Set health status to not serving
	w.healthSrv.SetServingStatus("", grpc_health_v1.HealthCheckResponse_NOT_SERVING)
	
	// Gracefully stop the gRPC server
	w.grpcServer.GracefulStop()
	w.running = false
	
	w.logger.Info("Simple gRPC server stopped")
	return nil
}

func (w *SimpleServerWrapper) HealthCheck() error {
	w.mu.RLock()
	defer w.mu.RUnlock()
	
	if !w.running {
		return fmt.Errorf("server is not running")
	}
	return nil
}

// runServerWithGracefulShutdown runs the server with proper graceful shutdown handling
func runServerWithGracefulShutdown(ctx context.Context, srv ServerInterface, logger *logrus.Logger) error {
	// Initialize server components
	logger.Info("Initializing server components...")
	if err := srv.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize server: %w", err)
	}

	// Create a context for graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Start server in goroutine
	errorCh := make(chan error, 1)
	go func() {
		logger.Info("Starting gRPC server...")
		if err := srv.Start(ctx); err != nil {
			if ctx.Err() != context.Canceled {
				errorCh <- err
			}
		}
	}()

	// Wait for shutdown signal or error
	select {
	case <-ctx.Done():
		logger.Info("Received shutdown signal, gracefully stopping server...")
		
		// Perform graceful shutdown within timeout
		shutdownDone := make(chan error, 1)
		go func() {
			shutdownDone <- srv.Stop()
		}()
		
		select {
		case err := <-shutdownDone:
			if err != nil {
				logger.WithError(err).Error("Error during graceful shutdown")
				return err
			}
			logger.Info("Server shutdown completed successfully")
			return nil
		case <-shutdownCtx.Done():
			logger.Warn("Graceful shutdown timeout exceeded, forcing shutdown")
			return fmt.Errorf("graceful shutdown timeout")
		}
		
	case err := <-errorCh:
		logger.WithError(err).Error("Server startup failed")
		return err
	}
}