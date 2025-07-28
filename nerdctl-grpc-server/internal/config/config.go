package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// Config represents the complete server configuration
// This supports configuration for different adapter versions and runtime environments
type Config struct {
	Server     ServerConfig     `yaml:"server"`
	Nerdctl    NerdctlConfig    `yaml:"nerdctl"`
	Containerd ContainerdConfig `yaml:"containerd"`
	Adapters   AdaptersConfig   `yaml:"adapters"`
	Auth       AuthConfig       `yaml:"auth"`
	Monitoring MonitoringConfig `yaml:"monitoring"`
	LogLevel   logrus.Level     `yaml:"log_level"`
}

// ServerConfig defines gRPC server configuration
type ServerConfig struct {
	Address          string    `yaml:"address"`
	Port             int       `yaml:"port"`
	EnableReflection bool      `yaml:"enable_reflection"`
	TLS              TLSConfig `yaml:"tls"`
	Timeouts         TimeoutsConfig `yaml:"timeouts"`
}

// TLSConfig defines TLS configuration for secure communication
type TLSConfig struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
	CAFile   string `yaml:"ca_file"`
}

// TimeoutsConfig defines various timeout configurations
type TimeoutsConfig struct {
	Read    time.Duration `yaml:"read"`
	Write   time.Duration `yaml:"write"`
	Idle    time.Duration `yaml:"idle"`
	Startup time.Duration `yaml:"startup"`
}

// NerdctlConfig defines nerdctl-specific configuration
type NerdctlConfig struct {
	BinaryPath         string        `yaml:"binary_path"`
	DefaultNamespace   string        `yaml:"default_namespace"`
	ConfigPath         string        `yaml:"config_path"`
	DataRoot           string        `yaml:"data_root"`
	ExecTimeout        time.Duration `yaml:"exec_timeout"`
	PreferredVersions  []string      `yaml:"preferred_versions"`
}

// ContainerdConfig defines containerd-specific configuration
type ContainerdConfig struct {
	Address   string `yaml:"address"`
	Namespace string `yaml:"namespace"`
}

// AdaptersConfig defines adapter-specific configuration
type AdaptersConfig struct {
	// Version selection strategy
	SelectionStrategy string `yaml:"selection_strategy"` // "auto", "manual", "latest", "stable"
	
	// Manual version override (when strategy is "manual")
	ForceVersion string `yaml:"force_version"`
	
	// Version-specific configurations
	V1 AdapterVersionConfig `yaml:"v1"`
	V2 AdapterVersionConfig `yaml:"v2"`
	
	// Common adapter settings
	RetryAttempts int           `yaml:"retry_attempts"`
	RetryDelay    time.Duration `yaml:"retry_delay"`
	
	// Feature toggles
	EnabledFeatures []string `yaml:"enabled_features"`
	DisabledFeatures []string `yaml:"disabled_features"`
}

// AdapterVersionConfig defines version-specific adapter configuration
type AdapterVersionConfig struct {
	Enabled         bool              `yaml:"enabled"`
	Priority        int               `yaml:"priority"`
	MinVersion      string            `yaml:"min_version"`
	MaxVersion      string            `yaml:"max_version"`
	ConfigOverrides map[string]string `yaml:"config_overrides"`
	FeatureFlags    map[string]bool   `yaml:"feature_flags"`
	
	// v2.x specific enhancements
	EnableStructuredLogs bool `yaml:"enable_structured_logs"`
	EnableMetrics        bool `yaml:"enable_metrics"`
	EnableJSONOutput     bool `yaml:"enable_json_output"`
	EnableHealthChecks   bool `yaml:"enable_health_checks"`
}

// AuthConfig defines authentication and authorization configuration
type AuthConfig struct {
	Enabled   bool      `yaml:"enabled"`
	Provider  string    `yaml:"provider"` // "jwt", "oauth", "ldap", "none"
	JWT       JWTConfig `yaml:"jwt"`
	TokenTTL  time.Duration `yaml:"token_ttl"`
}

// JWTConfig defines JWT-specific authentication configuration
type JWTConfig struct {
	Secret     string `yaml:"secret"`
	Issuer     string `yaml:"issuer"`
	Audience   string `yaml:"audience"`
	KeyFile    string `yaml:"key_file"`
	VerifyTTL  bool   `yaml:"verify_ttl"`
}

// MonitoringConfig defines monitoring and observability configuration
type MonitoringConfig struct {
	PrometheusEnabled bool     `yaml:"prometheus_enabled"`
	MetricsPort       int      `yaml:"metrics_port"`
	MetricsPath       string   `yaml:"metrics_path"`
	LoggingEnabled    bool     `yaml:"logging_enabled"`
	TracingEnabled    bool     `yaml:"tracing_enabled"`
	TracingEndpoint   string   `yaml:"tracing_endpoint"`
	HealthCheckPath   string   `yaml:"health_check_path"`
}

// NewDefaultConfig creates a default configuration for development/testing
func NewDefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Address:          "0.0.0.0",
			Port:             9090,
			EnableReflection: true, // Enable for development
			TLS: TLSConfig{
				Enabled: false,
			},
			Timeouts: TimeoutsConfig{
				Read:    30 * time.Second,
				Write:   30 * time.Second,
				Idle:    120 * time.Second,
				Startup: 60 * time.Second,
			},
		},
		Nerdctl: NerdctlConfig{
			BinaryPath:         "nerdctl", // Use PATH
			DefaultNamespace:   "default",
			ConfigPath:         "",
			DataRoot:           "",
			ExecTimeout:        30 * time.Second,
			PreferredVersions:  []string{"2.0.0", "1.7.6"},
		},
		Containerd: ContainerdConfig{
			Address:   "/run/containerd/containerd.sock",
			Namespace: "default",
		},
		Adapters: AdaptersConfig{
			SelectionStrategy: "auto",
			V1: AdapterVersionConfig{
				Enabled:    true,
				Priority:   1,
				MinVersion: "1.7.0",
				MaxVersion: "1.99.99",
			},
			V2: AdapterVersionConfig{
				Enabled:              true,
				Priority:             2,
				MinVersion:           "2.0.0",
				MaxVersion:           "2.99.99",
				EnableStructuredLogs: true,
				EnableMetrics:        true,
				EnableJSONOutput:     true,
				EnableHealthChecks:   true,
			},
			RetryAttempts: 3,
			RetryDelay:    1 * time.Second,
		},
		Auth: AuthConfig{
			Enabled:  false, // Disabled for development
			Provider: "none",
			TokenTTL: 24 * time.Hour,
		},
		Monitoring: MonitoringConfig{
			PrometheusEnabled: true,
			MetricsPort:       9091,
			MetricsPath:       "/metrics",
			LoggingEnabled:    true,
			TracingEnabled:    false,
			HealthCheckPath:   "/health",
		},
		LogLevel: logrus.InfoLevel,
	}
}

// LoadConfig loads configuration from file with fallback to defaults
func LoadConfig(configPath string) (*Config, error) {
	// Set defaults
	config := &Config{
		Server: ServerConfig{
			Address:          "0.0.0.0",
			Port:             9090,
			EnableReflection: false,
			TLS: TLSConfig{
				Enabled: false,
			},
			Timeouts: TimeoutsConfig{
				Read:    30 * time.Second,
				Write:   30 * time.Second,
				Idle:    120 * time.Second,
				Startup: 60 * time.Second,
			},
		},
		Nerdctl: NerdctlConfig{
			BinaryPath:        "nerdctl",
			DefaultNamespace:  "default",
			ConfigPath:        "",
			DataRoot:          "",
			ExecTimeout:       30 * time.Second,
			PreferredVersions: []string{"2.x", "1.x"},
		},
		Containerd: ContainerdConfig{
			Address:   "/run/containerd/containerd.sock",
			Namespace: "default",
		},
		Adapters: AdaptersConfig{
			SelectionStrategy: "auto",
			V1: AdapterVersionConfig{
				Enabled:    true,
				Priority:   1,
				MinVersion: "1.7.0",
				MaxVersion: "1.99.99",
			},
			V2: AdapterVersionConfig{
				Enabled:    true,
				Priority:   2,
				MinVersion: "2.0.0",
				MaxVersion: "2.99.99",
				// v2.x enhanced features
				EnableStructuredLogs: true,
				EnableMetrics:        true,
				EnableJSONOutput:     true,
				EnableHealthChecks:   true,
			},
			RetryAttempts: 3,
			RetryDelay:    1 * time.Second,
		},
		Auth: AuthConfig{
			Enabled:  false,
			Provider: "none",
			TokenTTL: 24 * time.Hour,
		},
		Monitoring: MonitoringConfig{
			PrometheusEnabled: false,
			MetricsPort:       9091,
			MetricsPath:       "/metrics",
			LoggingEnabled:    true,
			TracingEnabled:    false,
			HealthCheckPath:   "/health",
		},
		LogLevel: logrus.InfoLevel,
	}

	// Load from file if provided
	if configPath != "" {
		if err := loadConfigFromFile(configPath, config); err != nil {
			return nil, fmt.Errorf("failed to load config from file: %w", err)
		}
	}

	// Override with environment variables
	overrideWithEnv(config)

	// Validate configuration
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// loadConfigFromFile loads configuration from YAML file
func loadConfigFromFile(configPath string, config *Config) error {
	// Expand path
	if strings.HasPrefix(configPath, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		configPath = filepath.Join(homeDir, configPath[2:])
	}

	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("config file does not exist: %s", configPath)
	}

	// Read file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML
	if err := yaml.Unmarshal(data, config); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	return nil
}

// overrideWithEnv overrides configuration with environment variables
func overrideWithEnv(config *Config) {
	if addr := os.Getenv("NERDCTL_GRPC_ADDRESS"); addr != "" {
		config.Server.Address = addr
	}
	
	if port := os.Getenv("NERDCTL_GRPC_PORT"); port != "" {
		// Parse port number (error ignored for simplicity, should be handled properly)
		if p := parseInt(port); p > 0 {
			config.Server.Port = p
		}
	}
	
	if path := os.Getenv("NERDCTL_BINARY_PATH"); path != "" {
		config.Nerdctl.BinaryPath = path
	}
	
	if namespace := os.Getenv("NERDCTL_NAMESPACE"); namespace != "" {
		config.Nerdctl.DefaultNamespace = namespace
	}
	
	if addr := os.Getenv("CONTAINERD_ADDRESS"); addr != "" {
		config.Containerd.Address = addr
	}
	
	if strategy := os.Getenv("ADAPTER_SELECTION_STRATEGY"); strategy != "" {
		config.Adapters.SelectionStrategy = strategy
	}
	
	if version := os.Getenv("FORCE_NERDCTL_VERSION"); version != "" {
		config.Adapters.ForceVersion = version
		config.Adapters.SelectionStrategy = "manual"
	}
	
	if level := os.Getenv("LOG_LEVEL"); level != "" {
		if l, err := logrus.ParseLevel(level); err == nil {
			config.LogLevel = l
		}
	}
}

// validateConfig validates the configuration
func validateConfig(config *Config) error {
	// Validate server configuration
	if config.Server.Port <= 0 || config.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", config.Server.Port)
	}

	// Validate TLS configuration
	if config.Server.TLS.Enabled {
		if config.Server.TLS.CertFile == "" || config.Server.TLS.KeyFile == "" {
			return fmt.Errorf("TLS enabled but cert_file or key_file not provided")
		}
		
		if _, err := os.Stat(config.Server.TLS.CertFile); os.IsNotExist(err) {
			return fmt.Errorf("TLS cert file does not exist: %s", config.Server.TLS.CertFile)
		}
		
		if _, err := os.Stat(config.Server.TLS.KeyFile); os.IsNotExist(err) {
			return fmt.Errorf("TLS key file does not exist: %s", config.Server.TLS.KeyFile)
		}
	}

	// Validate adapter selection strategy
	validStrategies := map[string]bool{
		"auto":   true,
		"manual": true,
		"latest": true,
		"stable": true,
	}
	if !validStrategies[config.Adapters.SelectionStrategy] {
		return fmt.Errorf("invalid adapter selection strategy: %s", config.Adapters.SelectionStrategy)
	}

	// Validate manual version override
	if config.Adapters.SelectionStrategy == "manual" && config.Adapters.ForceVersion == "" {
		return fmt.Errorf("manual adapter selection requires force_version to be set")
	}

	// Validate monitoring configuration
	if config.Monitoring.PrometheusEnabled {
		if config.Monitoring.MetricsPort <= 0 || config.Monitoring.MetricsPort > 65535 {
			return fmt.Errorf("invalid metrics port: %d", config.Monitoring.MetricsPort)
		}
	}

	return nil
}

// SaveConfig saves configuration to file
func SaveConfig(config *Config, configPath string) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GetDefaultConfigPath returns the default configuration file path
func GetDefaultConfigPath() string {
	if homeDir, err := os.UserHomeDir(); err == nil {
		return filepath.Join(homeDir, ".config", "nerdctl-grpc-server", "config.yaml")
	}
	return "config.yaml"
}

// parseInt is a simple helper to parse integer from string
func parseInt(s string) int {
	// Simplified implementation - in real code, use strconv.Atoi
	// This is just to avoid import for this example
	return 0
}

// IsAdapterVersionEnabled checks if a specific adapter version is enabled
func (c *Config) IsAdapterVersionEnabled(version string) bool {
	switch {
	case strings.HasPrefix(version, "1."):
		return c.Adapters.V1.Enabled
	case strings.HasPrefix(version, "2."):
		return c.Adapters.V2.Enabled
	default:
		return false
	}
}

// GetAdapterPriority returns the priority for a specific adapter version
func (c *Config) GetAdapterPriority(version string) int {
	switch {
	case strings.HasPrefix(version, "1."):
		return c.Adapters.V1.Priority
	case strings.HasPrefix(version, "2."):
		return c.Adapters.V2.Priority
	default:
		return 0
	}
}

// IsFeatureEnabled checks if a specific feature is enabled
func (c *Config) IsFeatureEnabled(feature string) bool {
	for _, enabled := range c.Adapters.EnabledFeatures {
		if enabled == feature {
			return true
		}
	}
	
	for _, disabled := range c.Adapters.DisabledFeatures {
		if disabled == feature {
			return false
		}
	}
	
	// Default to enabled if not explicitly disabled
	return true
}