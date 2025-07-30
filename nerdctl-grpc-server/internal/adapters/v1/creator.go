package v1

import (
	"context"
	"fmt"

	"github.com/containerd/nerdctl-grpc-server/internal/config"
	"github.com/containerd/nerdctl-grpc-server/internal/interfaces"
	"github.com/containerd/nerdctl-grpc-server/internal/version"
	"github.com/sirupsen/logrus"
)

// AdapterSet contains all manager interfaces for a specific nerdctl version
type AdapterSet struct {
	ContainerManager interfaces.ContainerManager
	ImageManager     interfaces.ImageManager
	NetworkManager   interfaces.NetworkManager
	VolumeManager    interfaces.VolumeManager
	ComposeManager   interfaces.ComposeManager
	SystemManager    interfaces.SystemManager
	
	// Metadata about the adapter set
	Version     string
	NerdctlPath string
	Priority    int
}

// V1AdapterCreator creates adapters for nerdctl v1.x versions (1.7.0+)
// This implements version-specific adapter creation for nerdctl v1.x
type V1AdapterCreator struct {
	config          *config.Config
	logger          *logrus.Logger
	supportedRange  version.VersionRange
	priority        int
}

// NewV1AdapterCreator creates a new V1AdapterCreator
func NewV1AdapterCreator(config *config.Config, logger *logrus.Logger) (*V1AdapterCreator, error) {
	// Parse version range from configuration
	minVer, err := parseVersion(config.Adapters.V1.MinVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid min version for v1 adapter: %w", err)
	}
	
	maxVer, err := parseVersion(config.Adapters.V1.MaxVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid max version for v1 adapter: %w", err)
	}
	
	return &V1AdapterCreator{
		config: config,
		logger: logger.WithField("adapter", "v1"),
		supportedRange: version.VersionRange{
			Min: *minVer,
			Max: *maxVer,
		},
		priority: config.Adapters.V1.Priority,
	}, nil
}

// CreateAdapters creates a complete set of v1 adapters
func (c *V1AdapterCreator) CreateAdapters(ctx context.Context, config *config.Config, versionInfo *version.NerdctlVersionInfo) (*AdapterSet, error) {
	c.logger.WithField("version", versionInfo.Client.Raw).Info("Creating v1 adapters")
	
	// Validate version compatibility
	if !c.IsCompatible(versionInfo) {
		return nil, fmt.Errorf("nerdctl version %s is not compatible with v1 adapter", versionInfo.Client.Raw)
	}
	
	// Create nerdctl command executor with v1-specific configuration
	executor, err := NewV1CommandExecutor(config, versionInfo, c.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create v1 command executor: %w", err)
	}
	
	// Create all manager interfaces using the v1 executor
	containerManager, err := NewV1ContainerManager(executor, config, c.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create v1 container manager: %w", err)
	}
	
	imageManager, err := NewV1ImageManager(executor, config, c.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create v1 image manager: %w", err)
	}
	
	networkManager, err := NewV1NetworkManager(executor, config, c.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create v1 network manager: %w", err)
	}
	
	volumeManager, err := NewV1VolumeManager(executor, config, c.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create v1 volume manager: %w", err)
	}
	
	composeManager, err := NewV1ComposeManager(executor, config, c.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create v1 compose manager: %w", err)
	}
	
	systemManager, err := NewV1SystemManager(executor, config, c.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create v1 system manager: %w", err)
	}
	
	adapterSet := &AdapterSet{
		ContainerManager: containerManager,
		ImageManager:     imageManager,
		NetworkManager:   networkManager,
		VolumeManager:    volumeManager,
		ComposeManager:   composeManager,
		SystemManager:    systemManager,
		Version:          versionInfo.Client.Raw,
		NerdctlPath:      config.Nerdctl.BinaryPath,
		Priority:         c.priority,
	}
	
	c.logger.WithFields(logrus.Fields{
		"version":      versionInfo.Client.Raw,
		"priority":     c.priority,
		"nerdctl_path": config.Nerdctl.BinaryPath,
	}).Info("Successfully created v1 adapter set")
	
	return adapterSet, nil
}

// GetSupportedVersionRange returns the range of nerdctl versions this adapter supports
func (c *V1AdapterCreator) GetSupportedVersionRange() version.VersionRange {
	return c.supportedRange
}

// GetPriority returns the priority of this adapter creator
func (c *V1AdapterCreator) GetPriority() int {
	return c.priority
}

// IsCompatible checks if the given nerdctl version is compatible with v1 adapters
func (c *V1AdapterCreator) IsCompatible(versionInfo *version.NerdctlVersionInfo) bool {
	clientVersion := versionInfo.Client
	
	// Check if version is within supported range
	if !isVersionInRange(clientVersion, c.supportedRange) {
		c.logger.WithFields(logrus.Fields{
			"version":    clientVersion.Raw,
			"min_supported": c.supportedRange.Min.Raw,
			"max_supported": c.supportedRange.Max.Raw,
		}).Debug("Version is outside supported range")
		return false
	}
	
	// Additional v1-specific compatibility checks
	if !c.checkV1SpecificFeatures(versionInfo) {
		c.logger.WithField("version", clientVersion.Raw).Debug("V1-specific feature check failed")
		return false
	}
	
	c.logger.WithField("version", clientVersion.Raw).Debug("Version is compatible with v1 adapter")
	return true
}

// checkV1SpecificFeatures performs v1-specific compatibility checks
func (c *V1AdapterCreator) checkV1SpecificFeatures(versionInfo *version.NerdctlVersionInfo) bool {
	// Check for required v1 features
	clientVersion := versionInfo.Client
	
	// Ensure minimum version for required features
	// For example, certain features were introduced in specific v1.x versions
	if clientVersion.Major == 1 {
		// Check for minimum minor version requirements
		if clientVersion.Minor < 7 {
			c.logger.WithFields(logrus.Fields{
				"version":     clientVersion.Raw,
				"min_minor":   7,
			}).Debug("V1 adapter requires nerdctl 1.7.0 or higher")
			return false
		}
		
		// Check for deprecated features in later v1 versions
		if clientVersion.Minor >= 20 {
			c.logger.WithField("version", clientVersion.Raw).Warn("Using v1 adapter with newer nerdctl version may lack some features")
		}
	}
	
	// Check for required containerd version compatibility with v1
	if versionInfo.Components.Containerd != "" {
		if !isContainerdCompatibleWithV1(versionInfo.Components.Containerd) {
			c.logger.WithField("containerd_version", versionInfo.Components.Containerd).Debug("Containerd version not compatible with v1 adapter")
			return false
		}
	}
	
	return true
}

// isVersionInRange checks if version is within the specified range
func isVersionInRange(ver version.Version, range_ version.VersionRange) bool {
	return compareVersion(ver, range_.Min) >= 0 && compareVersion(ver, range_.Max) <= 0
}

// compareVersion compares two versions (-1: a < b, 0: a == b, 1: a > b)
func compareVersion(a, b version.Version) int {
	if a.Major != b.Major {
		if a.Major < b.Major {
			return -1
		}
		return 1
	}
	if a.Minor != b.Minor {
		if a.Minor < b.Minor {
			return -1
		}
		return 1
	}
	if a.Patch != b.Patch {
		if a.Patch < b.Patch {
			return -1
		}
		return 1
	}
	return 0
}

// isContainerdCompatibleWithV1 checks containerd version compatibility
func isContainerdCompatibleWithV1(containerdVersion string) bool {
	// Parse containerd version and check compatibility
	// This is a simplified check - in real implementation, would parse semantic version
	
	// v1 adapters work well with containerd 1.6.x and 1.7.x
	if containerdVersion == "" {
		return true // Assume compatible if version unknown
	}
	
	// Basic compatibility check
	if len(containerdVersion) > 0 && (containerdVersion[0] == '1' || containerdVersion[0] == '2') {
		return true
	}
	
	return false
}

// parseVersion parses a version string into a Version struct
func parseVersion(versionStr string) (*version.Version, error) {
	if versionStr == "" {
		return nil, fmt.Errorf("empty version string")
	}
	
	// This is a simplified version parser
	// In real implementation, would use a proper semver parser
	
	// For now, create a basic version struct
	ver := &version.Version{
		Raw: versionStr,
	}
	
	// Parse major.minor.patch
	// Simplified parsing - real implementation would be more robust
	if len(versionStr) >= 5 && versionStr[0] >= '1' && versionStr[0] <= '9' {
		ver.Major = int(versionStr[0] - '0')
		if len(versionStr) > 2 && versionStr[2] >= '0' && versionStr[2] <= '9' {
			ver.Minor = int(versionStr[2] - '0')
		}
		if len(versionStr) > 4 && versionStr[4] >= '0' && versionStr[4] <= '9' {
			ver.Patch = int(versionStr[4] - '0')
		}
	}
	
	return ver, nil
}

// GetCapabilities returns the capabilities supported by v1 adapters
func (c *V1AdapterCreator) GetCapabilities() []string {
	return []string{
		"container_lifecycle",
		"image_management",
		"network_management", 
		"volume_management",
		"compose_basic",
		"system_info",
		"logging",
		"exec",
		"cp",
		"build_basic",
		// Note: v1 may not support all latest features
	}
}

// GetLimitations returns known limitations of v1 adapters
func (c *V1AdapterCreator) GetLimitations() []string {
	return []string{
		"limited_compose_features", // Some advanced compose features may not be available
		"no_ipfs_support",          // IPFS features might be limited
		"basic_buildkit_support",   // Advanced BuildKit features may not work
		"legacy_networking",        // Some newer networking features unavailable
	}
}

// ValidateEnvironment validates the environment for v1 adapter compatibility
func (c *V1AdapterCreator) ValidateEnvironment(ctx context.Context, versionInfo *version.NerdctlVersionInfo) error {
	c.logger.WithField("version", versionInfo.Client.Raw).Debug("Validating environment for v1 adapter")
	
	// Check nerdctl binary accessibility
	if c.config.Nerdctl.BinaryPath == "" {
		return fmt.Errorf("nerdctl binary path not configured")
	}
	
	// Check containerd accessibility
	if c.config.Containerd.Address == "" {
		return fmt.Errorf("containerd address not configured")
	}
	
	// Additional v1-specific environment checks
	// For example, check for required CNI plugins, specific file permissions, etc.
	
	c.logger.WithField("version", versionInfo.Client.Raw).Debug("Environment validation passed for v1 adapter")
	return nil
}