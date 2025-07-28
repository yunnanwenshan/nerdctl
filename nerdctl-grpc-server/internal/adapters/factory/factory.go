package factory

import (
	"context"
	"fmt"
	"strings"

	"github.com/containerd/nerdctl-grpc-server/internal/config"
	"github.com/containerd/nerdctl-grpc-server/internal/interfaces"
	"github.com/containerd/nerdctl-grpc-server/internal/version"
	
	// Version-specific adapters will be registered via dependency injection
	
	"github.com/sirupsen/logrus"
)

// AdapterSet contains all manager interfaces for a specific nerdctl version
// This represents the complete set of adapters for one version
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

// AdapterFactory manages the creation and selection of version-specific adapters
// This is the core component that implements the adapter pattern for version compatibility
type AdapterFactory struct {
	config  *config.Config
	logger  *logrus.Logger
	
	// Registry of available adapter creators
	adapterCreators map[string]AdapterCreator
	
	// Cache of created adapter sets
	adapterCache map[string]*AdapterSet
}

// AdapterCreator defines the interface for creating version-specific adapters
type AdapterCreator interface {
	// CreateAdapters creates a complete set of adapters for this version
	CreateAdapters(ctx context.Context, config *config.Config, versionInfo *version.NerdctlVersionInfo) (*AdapterSet, error)
	
	// GetSupportedVersionRange returns the range of versions this creator supports
	GetSupportedVersionRange() version.VersionRange
	
	// GetPriority returns the priority of this adapter creator (higher = more preferred)
	GetPriority() int
	
	// IsCompatible checks if this creator is compatible with the given version
	IsCompatible(versionInfo *version.NerdctlVersionInfo) bool
}

// NewAdapterFactory creates a new adapter factory
func NewAdapterFactory(config *config.Config, logger *logrus.Logger) (*AdapterFactory, error) {
	factory := &AdapterFactory{
		config:          config,
		logger:          logger,
		adapterCreators: make(map[string]AdapterCreator),
		adapterCache:    make(map[string]*AdapterSet),
	}
	
	// Register all available adapter creators
	if err := factory.registerAdapterCreators(); err != nil {
		return nil, fmt.Errorf("failed to register adapter creators: %w", err)
	}
	
	return factory, nil
}

// registerAdapterCreators registers all available version-specific adapter creators
func (f *AdapterFactory) registerAdapterCreators() error {
	f.logger.Info("Registering adapter creators...")
	
	// Note: Adapter creators will be registered via RegisterAdapterCreator method
	// This allows breaking circular dependencies
	
	f.logger.Info("Adapter factory initialized, awaiting adapter creator registration")
	return nil
}

// RegisterAdapterCreator registers an adapter creator with the factory
func (f *AdapterFactory) RegisterAdapterCreator(name string, creator AdapterCreator) error {
	f.adapterCreators[name] = creator
	f.logger.WithField("name", name).Info("Registered adapter creator")
	return nil
}

// CreateAdapters creates the appropriate adapter set for the detected nerdctl version
// This is the main entry point for the adapter selection logic
func (f *AdapterFactory) CreateAdapters(ctx context.Context, versionInfo *version.NerdctlVersionInfo) (*AdapterSet, error) {
	f.logger.WithField("version", versionInfo.Client.Raw).Info("Creating adapters for nerdctl version")
	
	// Check cache first
	cacheKey := f.getCacheKey(versionInfo)
	if cached, exists := f.adapterCache[cacheKey]; exists {
		f.logger.WithField("version", versionInfo.Client.Raw).Debug("Using cached adapter set")
		return cached, nil
	}
	
	// Select appropriate adapter creator based on configuration strategy
	creator, err := f.selectAdapterCreator(versionInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to select adapter creator: %w", err)
	}
	
	// Create adapter set
	adapterSet, err := creator.CreateAdapters(ctx, f.config, versionInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to create adapter set: %w", err)
	}
	
	// Cache the adapter set
	f.adapterCache[cacheKey] = adapterSet
	
	f.logger.WithFields(logrus.Fields{
		"version":      versionInfo.Client.Raw,
		"adapter_type": f.getAdapterTypeFromVersion(versionInfo.Client.Raw),
		"priority":     adapterSet.Priority,
	}).Info("Successfully created and cached adapter set")
	
	return adapterSet, nil
}

// selectAdapterCreator selects the most appropriate adapter creator for the given version
func (f *AdapterFactory) selectAdapterCreator(versionInfo *version.NerdctlVersionInfo) (AdapterCreator, error) {
	versionStr := versionInfo.Client.Raw
	
	// Handle manual version override
	if f.config.Adapters.SelectionStrategy == "manual" {
		return f.selectManualAdapter(versionInfo)
	}
	
	// Find all compatible adapters
	var compatibleCreators []AdapterCreator
	for name, creator := range f.adapterCreators {
		if creator.IsCompatible(versionInfo) {
			compatibleCreators = append(compatibleCreators, creator)
			f.logger.WithFields(logrus.Fields{
				"adapter": name,
				"version": versionStr,
			}).Debug("Found compatible adapter")
		}
	}
	
	if len(compatibleCreators) == 0 {
		return nil, fmt.Errorf("no compatible adapters found for nerdctl version %s", versionStr)
	}
	
	// Apply selection strategy
	switch f.config.Adapters.SelectionStrategy {
	case "auto":
		return f.selectAutoAdapter(compatibleCreators, versionInfo)
	case "latest":
		return f.selectLatestAdapter(compatibleCreators)
	case "stable":
		return f.selectStableAdapter(compatibleCreators, versionInfo)
	default:
		return nil, fmt.Errorf("unsupported selection strategy: %s", f.config.Adapters.SelectionStrategy)
	}
}

// selectManualAdapter selects adapter based on manual configuration
func (f *AdapterFactory) selectManualAdapter(versionInfo *version.NerdctlVersionInfo) (AdapterCreator, error) {
	forceVersion := f.config.Adapters.ForceVersion
	adapterType := f.getAdapterTypeFromVersion(forceVersion)
	
	creator, exists := f.adapterCreators[adapterType]
	if !exists {
		return nil, fmt.Errorf("forced adapter version %s not available", forceVersion)
	}
	
	f.logger.WithFields(logrus.Fields{
		"forced_version": forceVersion,
		"adapter_type":   adapterType,
		"actual_version": versionInfo.Client.Raw,
	}).Info("Using manually configured adapter")
	
	return creator, nil
}

// selectAutoAdapter automatically selects the best adapter based on version and priority
func (f *AdapterFactory) selectAutoAdapter(creators []AdapterCreator, versionInfo *version.NerdctlVersionInfo) (AdapterCreator, error) {
	// Sort by priority (highest first)
	var bestCreator AdapterCreator
	highestPriority := -1
	
	for _, creator := range creators {
		priority := creator.GetPriority()
		if priority > highestPriority {
			highestPriority = priority
			bestCreator = creator
		}
	}
	
	if bestCreator == nil {
		return nil, fmt.Errorf("no suitable adapter found")
	}
	
	f.logger.WithFields(logrus.Fields{
		"version":  versionInfo.Client.Raw,
		"priority": highestPriority,
	}).Info("Auto-selected adapter based on priority")
	
	return bestCreator, nil
}

// selectLatestAdapter selects the adapter that supports the highest version range
func (f *AdapterFactory) selectLatestAdapter(creators []AdapterCreator) (AdapterCreator, error) {
	var latestCreator AdapterCreator
	var highestMaxVersion version.Version
	
	for _, creator := range creators {
		versionRange := creator.GetSupportedVersionRange()
		if compareVersions(versionRange.Max, highestMaxVersion) > 0 {
			highestMaxVersion = versionRange.Max
			latestCreator = creator
		}
	}
	
	if latestCreator == nil {
		return nil, fmt.Errorf("no suitable adapter found")
	}
	
	f.logger.WithField("max_version", highestMaxVersion.Raw).Info("Selected latest adapter")
	return latestCreator, nil
}

// selectStableAdapter selects the most stable adapter (typically v1 for proven stability)
func (f *AdapterFactory) selectStableAdapter(creators []AdapterCreator, versionInfo *version.NerdctlVersionInfo) (AdapterCreator, error) {
	// Prefer v1 adapters for stability if compatible
	for adapterType, creator := range f.adapterCreators {
		if adapterType == "v1" && creator.IsCompatible(versionInfo) {
			f.logger.Info("Selected v1 adapter for stability")
			return creator, nil
		}
	}
	
	// Fall back to auto selection if v1 is not compatible
	return f.selectAutoAdapter(creators, versionInfo)
}

// getAdapterTypeFromVersion determines adapter type from version string
func (f *AdapterFactory) getAdapterTypeFromVersion(versionStr string) string {
	if strings.HasPrefix(versionStr, "1.") {
		return "v1"
	} else if strings.HasPrefix(versionStr, "2.") {
		return "v2"
	}
	return "unknown"
}

// getCacheKey generates a cache key for adapter sets
func (f *AdapterFactory) getCacheKey(versionInfo *version.NerdctlVersionInfo) string {
	return fmt.Sprintf("%s_%s_%s", 
		versionInfo.Client.Raw,
		f.config.Adapters.SelectionStrategy,
		f.config.Adapters.ForceVersion)
}

// compareVersions compares two versions (-1: v1 < v2, 0: v1 == v2, 1: v1 > v2)
func compareVersions(v1, v2 version.Version) int {
	if v1.Major != v2.Major {
		return compareInts(v1.Major, v2.Major)
	}
	if v1.Minor != v2.Minor {
		return compareInts(v1.Minor, v2.Minor)
	}
	return compareInts(v1.Patch, v2.Patch)
}

// compareInts compares two integers
func compareInts(a, b int) int {
	if a < b {
		return -1
	} else if a > b {
		return 1
	}
	return 0
}

// GetAvailableAdapters returns information about all available adapters
func (f *AdapterFactory) GetAvailableAdapters() map[string]AdapterInfo {
	info := make(map[string]AdapterInfo)
	
	for name, creator := range f.adapterCreators {
		versionRange := creator.GetSupportedVersionRange()
		info[name] = AdapterInfo{
			Name:         name,
			MinVersion:   versionRange.Min.Raw,
			MaxVersion:   versionRange.Max.Raw,
			Priority:     creator.GetPriority(),
			Enabled:      true,
		}
	}
	
	return info
}

// AdapterInfo contains metadata about an adapter
type AdapterInfo struct {
	Name       string `json:"name"`
	MinVersion string `json:"min_version"`
	MaxVersion string `json:"max_version"`
	Priority   int    `json:"priority"`
	Enabled    bool   `json:"enabled"`
}

// ClearCache clears the adapter cache
func (f *AdapterFactory) ClearCache() {
	f.adapterCache = make(map[string]*AdapterSet)
	f.logger.Info("Adapter cache cleared")
}

// ValidateConfiguration validates the adapter factory configuration
func (f *AdapterFactory) ValidateConfiguration() error {
	if len(f.adapterCreators) == 0 {
		return fmt.Errorf("no adapter creators are available")
	}
	
	// Check for version range overlaps
	var ranges []version.VersionRange
	for _, creator := range f.adapterCreators {
		ranges = append(ranges, creator.GetSupportedVersionRange())
	}
	
	// Additional validation logic can be added here
	
	return nil
}

// Shutdown performs cleanup when the factory is no longer needed
func (f *AdapterFactory) Shutdown(ctx context.Context) error {
	f.logger.Info("Shutting down adapter factory")
	
	// Clean up cached adapter sets
	for key, adapterSet := range f.adapterCache {
		if closer, ok := adapterSet.ContainerManager.(interface{ Close() error }); ok {
			if err := closer.Close(); err != nil {
				f.logger.WithField("cache_key", key).WithError(err).Warn("Failed to close container manager")
			}
		}
		// Similar cleanup for other managers...
	}
	
	f.ClearCache()
	f.logger.Info("Adapter factory shutdown completed")
	return nil
}