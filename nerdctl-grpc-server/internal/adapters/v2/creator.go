package v2

import (
	"context"
	"fmt"
	"strconv"
	"strings"

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

// V2AdapterCreator creates adapters for nerdctl v2.x versions (2.0.0+)
// This implements version-specific adapter creation for nerdctl v2.x with enhanced features
type V2AdapterCreator struct {
	config          *config.Config
	logger          *logrus.Entry
	supportedRange  version.VersionRange
	priority        int
}

// NewV2AdapterCreator creates a new V2AdapterCreator
func NewV2AdapterCreator(config *config.Config, logger *logrus.Logger) (*V2AdapterCreator, error) {
	// Parse version range from configuration
	minVer, err := parseVersion(config.Adapters.V2.MinVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid min version for v2 adapter: %w", err)
	}
	
	maxVer, err := parseVersion(config.Adapters.V2.MaxVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid max version for v2 adapter: %w", err)
	}
	
	return &V2AdapterCreator{
		config: config,
		logger: logger.WithField("adapter", "v2"),
		supportedRange: version.VersionRange{
			Min: *minVer,
			Max: *maxVer,
		},
		priority: config.Adapters.V2.Priority,
	}, nil
}

// CreateAdapters creates a complete set of v2 adapters
func (c *V2AdapterCreator) CreateAdapters(ctx context.Context, config *config.Config, versionInfo *version.NerdctlVersionInfo) (*AdapterSet, error) {
	c.logger.WithField("version", versionInfo.Client.Raw).Info("Creating v2 adapters")
	
	// Validate version compatibility
	if !c.IsCompatible(versionInfo) {
		return nil, fmt.Errorf("nerdctl version %s is not compatible with v2 adapter", versionInfo.Client.Raw)
	}
	
	// Create nerdctl command executor with v2-specific configuration
	executor, err := NewV2CommandExecutor(config, versionInfo, c.logger.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create v2 command executor: %w", err)
	}
	
	// Create all manager interfaces using the v2 executor
	containerManager, err := NewV2ContainerManager(executor, config, c.logger.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create v2 container manager: %w", err)
	}
	
	imageManager, err := NewV2ImageManager(executor, config, c.logger.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create v2 image manager: %w", err)
	}
	
	networkManager, err := NewV2NetworkManager(executor, config, c.logger.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create v2 network manager: %w", err)
	}
	
	volumeManager, err := NewV2VolumeManager(executor, config, c.logger.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create v2 volume manager: %w", err)
	}
	
	composeManager, err := NewV2ComposeManager(executor, config, c.logger.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create v2 compose manager: %w", err)
	}
	
	systemManager, err := NewV2SystemManager(executor, config, c.logger.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create v2 system manager: %w", err)
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
	}).Info("Successfully created v2 adapter set")
	
	return adapterSet, nil
}

// GetSupportedVersionRange returns the range of nerdctl versions this adapter supports
func (c *V2AdapterCreator) GetSupportedVersionRange() version.VersionRange {
	return c.supportedRange
}

// GetPriority returns the priority of this adapter creator
func (c *V2AdapterCreator) GetPriority() int {
	return c.priority
}

// IsCompatible checks if the given nerdctl version is compatible with v2 adapters
func (c *V2AdapterCreator) IsCompatible(versionInfo *version.NerdctlVersionInfo) bool {
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
	
	// Additional v2-specific compatibility checks
	if !c.checkV2SpecificFeatures(versionInfo) {
		c.logger.WithField("version", clientVersion.Raw).Debug("V2-specific feature check failed")
		return false
	}
	
	c.logger.WithField("version", clientVersion.Raw).Debug("Version is compatible with v2 adapter")
	return true
}

// checkV2SpecificFeatures performs v2-specific compatibility checks
func (c *V2AdapterCreator) checkV2SpecificFeatures(versionInfo *version.NerdctlVersionInfo) bool {
	// Check for required v2 features
	clientVersion := versionInfo.Client
	
	// v2.x introduced enhanced security features, improved CLI output formats, etc.
	// These are the key differentiators from v1.x
	
	// Check minimum v2 version (2.0.0+)
	if clientVersion.Major < 2 {
		c.logger.WithField("version", clientVersion.Raw).Debug("Version is not v2.x")
		return false
	}
	
	// Check for v2-specific features like enhanced JSON output
	if !c.hasEnhancedJSONOutput(versionInfo) {
		c.logger.WithField("version", clientVersion.Raw).Debug("Enhanced JSON output not available")
		return false
	}
	
	// Check for v2-specific container lifecycle management features
	if !c.hasEnhancedContainerLifecycle(versionInfo) {
		c.logger.WithField("version", clientVersion.Raw).Debug("Enhanced container lifecycle features not available")
		return false
	}
	
	return true
}

// hasEnhancedJSONOutput checks if the version supports v2's enhanced JSON output format
func (c *V2AdapterCreator) hasEnhancedJSONOutput(versionInfo *version.NerdctlVersionInfo) bool {
	// v2.0.0+ introduced structured JSON output with additional metadata
	return versionInfo.Client.Major >= 2
}

// hasEnhancedContainerLifecycle checks if the version supports v2's container lifecycle features
func (c *V2AdapterCreator) hasEnhancedContainerLifecycle(versionInfo *version.NerdctlVersionInfo) bool {
	// v2.0.0+ introduced improved container state management and health checks
	return versionInfo.Client.Major >= 2
}

// parseVersion parses a version string into a version.Version struct
func parseVersion(versionStr string) (*version.Version, error) {
	// Remove 'v' prefix if present
	versionStr = strings.TrimPrefix(versionStr, "v")
	
	// Split by dots
	parts := strings.Split(versionStr, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid version format: %s (expected major.minor.patch)", versionStr)
	}
	
	// Parse major, minor, patch
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid major version: %s", parts[0])
	}
	
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid minor version: %s", parts[1])
	}
	
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return nil, fmt.Errorf("invalid patch version: %s", parts[2])
	}
	
	return &version.Version{
		Major: major,
		Minor: minor,
		Patch: patch,
		Raw:   versionStr,
	}, nil
}

// isVersionInRange checks if a version is within the specified range
func isVersionInRange(version version.Version, versionRange version.VersionRange) bool {
	// Check if version >= min
	if compareVersions(version, versionRange.Min) < 0 {
		return false
	}
	
	// Check if version <= max
	if compareVersions(version, versionRange.Max) > 0 {
		return false
	}
	
	return true
}

// compareVersions compares two versions and returns:
// -1 if a < b, 0 if a == b, 1 if a > b
func compareVersions(a, b version.Version) int {
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