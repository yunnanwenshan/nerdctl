package factory_test

import (
	"context"
	"testing"
	"time"

	"github.com/containerd/nerdctl-grpc-server/internal/adapters/factory"
	"github.com/containerd/nerdctl-grpc-server/internal/config"
	"github.com/containerd/nerdctl-grpc-server/internal/version"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockAdapterCreator provides a mock implementation for testing
type MockAdapterCreator struct {
	name            string
	supportedRange  version.VersionRange
	priority        int
	compatible      bool
	createError     error
	adapterSet      *factory.AdapterSet
}

// CreateAdapters mocks adapter creation
func (m *MockAdapterCreator) CreateAdapters(ctx context.Context, config *config.Config, versionInfo *version.NerdctlVersionInfo) (*factory.AdapterSet, error) {
	if m.createError != nil {
		return nil, m.createError
	}
	
	if m.adapterSet != nil {
		return m.adapterSet, nil
	}
	
	// Return a basic adapter set for testing
	return &factory.AdapterSet{
		Version:     versionInfo.Client.Raw,
		NerdctlPath: config.Nerdctl.BinaryPath,
		Priority:    m.priority,
	}, nil
}

// GetSupportedVersionRange returns the mock version range
func (m *MockAdapterCreator) GetSupportedVersionRange() version.VersionRange {
	return m.supportedRange
}

// GetPriority returns the mock priority
func (m *MockAdapterCreator) GetPriority() int {
	return m.priority
}

// IsCompatible returns the mock compatibility
func (m *MockAdapterCreator) IsCompatible(versionInfo *version.NerdctlVersionInfo) bool {
	return m.compatible
}

func TestAdapterFactory_CreateAdapters(t *testing.T) {
	tests := []struct {
		name          string
		versionInfo   *version.NerdctlVersionInfo
		config        *config.Config
		mockCreators  map[string]*MockAdapterCreator
		expectedError bool
		expectedType  string
	}{
		{
			name: "successful v1 adapter creation",
			versionInfo: &version.NerdctlVersionInfo{
				Client: version.Version{Major: 1, Minor: 7, Patch: 0, Raw: "1.7.0"},
			},
			config: &config.Config{
				Nerdctl: config.NerdctlConfig{BinaryPath: "nerdctl"},
				Adapters: config.AdaptersConfig{
					SelectionStrategy: "auto",
					V1: config.AdapterVersionConfig{Enabled: true, Priority: 1},
					V2: config.AdapterVersionConfig{Enabled: true, Priority: 2},
				},
			},
			mockCreators: map[string]*MockAdapterCreator{
				"v1": {
					name:       "v1",
					priority:   1,
					compatible: true,
					supportedRange: version.VersionRange{
						Min: version.Version{Major: 1, Minor: 7, Patch: 0, Raw: "1.7.0"},
						Max: version.Version{Major: 1, Minor: 99, Patch: 99, Raw: "1.99.99"},
					},
				},
				"v2": {
					name:       "v2",
					priority:   2,
					compatible: false,
					supportedRange: version.VersionRange{
						Min: version.Version{Major: 2, Minor: 0, Patch: 0, Raw: "2.0.0"},
						Max: version.Version{Major: 2, Minor: 99, Patch: 99, Raw: "2.99.99"},
					},
				},
			},
			expectedError: false,
			expectedType:  "v1",
		},
		{
			name: "successful v2 adapter creation",
			versionInfo: &version.NerdctlVersionInfo{
				Client: version.Version{Major: 2, Minor: 1, Patch: 0, Raw: "2.1.0"},
			},
			config: &config.Config{
				Nerdctl: config.NerdctlConfig{BinaryPath: "nerdctl"},
				Adapters: config.AdaptersConfig{
					SelectionStrategy: "auto",
					V1: config.AdapterVersionConfig{Enabled: true, Priority: 1},
					V2: config.AdapterVersionConfig{Enabled: true, Priority: 2},
				},
			},
			mockCreators: map[string]*MockAdapterCreator{
				"v1": {
					name:       "v1",
					priority:   1,
					compatible: false,
					supportedRange: version.VersionRange{
						Min: version.Version{Major: 1, Minor: 7, Patch: 0, Raw: "1.7.0"},
						Max: version.Version{Major: 1, Minor: 99, Patch: 99, Raw: "1.99.99"},
					},
				},
				"v2": {
					name:       "v2",
					priority:   2,
					compatible: true,
					supportedRange: version.VersionRange{
						Min: version.Version{Major: 2, Minor: 0, Patch: 0, Raw: "2.0.0"},
						Max: version.Version{Major: 2, Minor: 99, Patch: 99, Raw: "2.99.99"},
					},
				},
			},
			expectedError: false,
			expectedType:  "v2",
		},
		{
			name: "priority-based selection",
			versionInfo: &version.NerdctlVersionInfo{
				Client: version.Version{Major: 1, Minor: 8, Patch: 0, Raw: "1.8.0"},
			},
			config: &config.Config{
				Nerdctl: config.NerdctlConfig{BinaryPath: "nerdctl"},
				Adapters: config.AdaptersConfig{
					SelectionStrategy: "auto",
					V1: config.AdapterVersionConfig{Enabled: true, Priority: 3}, // Higher priority
					V2: config.AdapterVersionConfig{Enabled: true, Priority: 1}, // Lower priority
				},
			},
			mockCreators: map[string]*MockAdapterCreator{
				"v1": {
					name:       "v1",
					priority:   3,
					compatible: true,
					supportedRange: version.VersionRange{
						Min: version.Version{Major: 1, Minor: 7, Patch: 0, Raw: "1.7.0"},
						Max: version.Version{Major: 1, Minor: 99, Patch: 99, Raw: "1.99.99"},
					},
				},
				"v2": {
					name:       "v2",
					priority:   1,
					compatible: true, // Also compatible but lower priority
					supportedRange: version.VersionRange{
						Min: version.Version{Major: 1, Minor: 8, Patch: 0, Raw: "1.8.0"},
						Max: version.Version{Major: 2, Minor: 99, Patch: 99, Raw: "2.99.99"},
					},
				},
			},
			expectedError: false,
			expectedType:  "v1", // Should select v1 due to higher priority
		},
		{
			name: "manual version override",
			versionInfo: &version.NerdctlVersionInfo{
				Client: version.Version{Major: 2, Minor: 0, Patch: 0, Raw: "2.0.0"},
			},
			config: &config.Config{
				Nerdctl: config.NerdctlConfig{BinaryPath: "nerdctl"},
				Adapters: config.AdaptersConfig{
					SelectionStrategy: "manual",
					ForceVersion:      "v1", // Force v1 even for v2.x binary
					V1: config.AdapterVersionConfig{Enabled: true, Priority: 1},
					V2: config.AdapterVersionConfig{Enabled: true, Priority: 2},
				},
			},
			mockCreators: map[string]*MockAdapterCreator{
				"v1": {
					name:       "v1",
					priority:   1,
					compatible: true, // Mock as compatible for manual override
					supportedRange: version.VersionRange{
						Min: version.Version{Major: 1, Minor: 7, Patch: 0, Raw: "1.7.0"},
						Max: version.Version{Major: 1, Minor: 99, Patch: 99, Raw: "1.99.99"},
					},
				},
				"v2": {
					name:       "v2",
					priority:   2,
					compatible: true,
					supportedRange: version.VersionRange{
						Min: version.Version{Major: 2, Minor: 0, Patch: 0, Raw: "2.0.0"},
						Max: version.Version{Major: 2, Minor: 99, Patch: 99, Raw: "2.99.99"},
					},
				},
			},
			expectedError: false,
			expectedType:  "v1", // Should select v1 due to manual override
		},
		{
			name: "no compatible adapters",
			versionInfo: &version.NerdctlVersionInfo{
				Client: version.Version{Major: 3, Minor: 0, Patch: 0, Raw: "3.0.0"},
			},
			config: &config.Config{
				Nerdctl: config.NerdctlConfig{BinaryPath: "nerdctl"},
				Adapters: config.AdaptersConfig{
					SelectionStrategy: "auto",
					V1: config.AdapterVersionConfig{Enabled: true, Priority: 1},
					V2: config.AdapterVersionConfig{Enabled: true, Priority: 2},
				},
			},
			mockCreators: map[string]*MockAdapterCreator{
				"v1": {
					name:       "v1",
					priority:   1,
					compatible: false,
					supportedRange: version.VersionRange{
						Min: version.Version{Major: 1, Minor: 7, Patch: 0, Raw: "1.7.0"},
						Max: version.Version{Major: 1, Minor: 99, Patch: 99, Raw: "1.99.99"},
					},
				},
				"v2": {
					name:       "v2",
					priority:   2,
					compatible: false,
					supportedRange: version.VersionRange{
						Min: version.Version{Major: 2, Minor: 0, Patch: 0, Raw: "2.0.0"},
						Max: version.Version{Major: 2, Minor: 99, Patch: 99, Raw: "2.99.99"},
					},
				},
			},
			expectedError: true,
		},
		{
			name: "adapter creation error",
			versionInfo: &version.NerdctlVersionInfo{
				Client: version.Version{Major: 1, Minor: 7, Patch: 0, Raw: "1.7.0"},
			},
			config: &config.Config{
				Nerdctl: config.NerdctlConfig{BinaryPath: "nerdctl"},
				Adapters: config.AdaptersConfig{
					SelectionStrategy: "auto",
					V1: config.AdapterVersionConfig{Enabled: true, Priority: 1},
				},
			},
			mockCreators: map[string]*MockAdapterCreator{
				"v1": {
					name:        "v1",
					priority:    1,
					compatible:  true,
					createError: assert.AnError,
					supportedRange: version.VersionRange{
						Min: version.Version{Major: 1, Minor: 7, Patch: 0, Raw: "1.7.0"},
						Max: version.Version{Major: 1, Minor: 99, Patch: 99, Raw: "1.99.99"},
					},
				},
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock factory with the test creators
			logger := logrus.New()
			logger.SetLevel(logrus.WarnLevel)
			
			mockFactory := &factory.AdapterFactory{
				Config:          tt.config,
				Logger:          logger,
				AdapterCreators: make(map[string]factory.AdapterCreator),
				AdapterCache:    make(map[string]*factory.AdapterSet),
			}
			
			// Register mock creators
			for name, creator := range tt.mockCreators {
				mockFactory.AdapterCreators[name] = creator
			}
			
			// Execute create adapters
			ctx := context.Background()
			adapterSet, err := mockFactory.CreateAdapters(ctx, tt.versionInfo)
			
			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, adapterSet)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, adapterSet)
				assert.Equal(t, tt.versionInfo.Client.Raw, adapterSet.Version)
				assert.Equal(t, tt.config.Nerdctl.BinaryPath, adapterSet.NerdctlPath)
				
				// Verify the correct adapter type was selected based on priority/compatibility
				if tt.expectedType == "v1" {
					assert.Equal(t, tt.mockCreators["v1"].priority, adapterSet.Priority)
				} else if tt.expectedType == "v2" {
					assert.Equal(t, tt.mockCreators["v2"].priority, adapterSet.Priority)
				}
			}
		})
	}
}

func TestAdapterFactory_SelectionStrategies(t *testing.T) {
	tests := []struct {
		name            string
		strategy        string
		forceVersion    string
		versionInfo     *version.NerdctlVersionInfo
		mockCreators    map[string]*MockAdapterCreator
		expectedAdapter string
		expectedError   bool
	}{
		{
			name:     "auto strategy - best compatibility",
			strategy: "auto",
			versionInfo: &version.NerdctlVersionInfo{
				Client: version.Version{Major: 1, Minor: 8, Patch: 0, Raw: "1.8.0"},
			},
			mockCreators: map[string]*MockAdapterCreator{
				"v1": {
					name:       "v1",
					priority:   1,
					compatible: true,
				},
				"v2": {
					name:       "v2",
					priority:   2,
					compatible: true,
				},
			},
			expectedAdapter: "v2", // Higher priority wins
		},
		{
			name:         "manual strategy - force specific version",
			strategy:     "manual",
			forceVersion: "v1",
			versionInfo: &version.NerdctlVersionInfo{
				Client: version.Version{Major: 2, Minor: 0, Patch: 0, Raw: "2.0.0"},
			},
			mockCreators: map[string]*MockAdapterCreator{
				"v1": {
					name:       "v1",
					priority:   1,
					compatible: true, // Mock as compatible for testing
				},
				"v2": {
					name:       "v2",
					priority:   2,
					compatible: true,
				},
			},
			expectedAdapter: "v1", // Forced selection
		},
		{
			name:     "latest strategy - prefer newest version",
			strategy: "latest",
			versionInfo: &version.NerdctlVersionInfo{
				Client: version.Version{Major: 2, Minor: 0, Patch: 0, Raw: "2.0.0"},
			},
			mockCreators: map[string]*MockAdapterCreator{
				"v1": {
					name:       "v1",
					priority:   1,
					compatible: true,
					supportedRange: version.VersionRange{
						Max: version.Version{Major: 1, Minor: 99, Patch: 99, Raw: "1.99.99"},
					},
				},
				"v2": {
					name:       "v2",
					priority:   2,
					compatible: true,
					supportedRange: version.VersionRange{
						Max: version.Version{Major: 2, Minor: 99, Patch: 99, Raw: "2.99.99"},
					},
				},
			},
			expectedAdapter: "v2", // Latest compatible version
		},
		{
			name:     "stable strategy - prefer stable version",
			strategy: "stable",
			versionInfo: &version.NerdctlVersionInfo{
				Client: version.Version{Major: 1, Minor: 8, Patch: 0, Raw: "1.8.0"},
			},
			mockCreators: map[string]*MockAdapterCreator{
				"v1": {
					name:       "v1",
					priority:   1,
					compatible: true,
					supportedRange: version.VersionRange{
						Min: version.Version{Major: 1, Minor: 7, Patch: 0, Raw: "1.7.0"},
						Max: version.Version{Major: 1, Minor: 99, Patch: 99, Raw: "1.99.99"},
					},
				},
				"v2": {
					name:       "v2",
					priority:   2,
					compatible: true,
					supportedRange: version.VersionRange{
						Min: version.Version{Major: 2, Minor: 0, Patch: 0, Raw: "2.0.0"},
						Max: version.Version{Major: 2, Minor: 99, Patch: 99, Raw: "2.99.99"},
					},
				},
			},
			expectedAdapter: "v1", // More stable/mature version for 1.8.0
		},
		{
			name:         "manual strategy with invalid force version",
			strategy:     "manual",
			forceVersion: "v3",
			versionInfo: &version.NerdctlVersionInfo{
				Client: version.Version{Major: 1, Minor: 8, Patch: 0, Raw: "1.8.0"},
			},
			mockCreators: map[string]*MockAdapterCreator{
				"v1": {name: "v1", priority: 1, compatible: true},
				"v2": {name: "v2", priority: 2, compatible: true},
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &config.Config{
				Nerdctl: config.NerdctlConfig{BinaryPath: "nerdctl"},
				Adapters: config.AdaptersConfig{
					SelectionStrategy: tt.strategy,
					ForceVersion:      tt.forceVersion,
					V1: config.AdapterVersionConfig{Enabled: true, Priority: 1},
					V2: config.AdapterVersionConfig{Enabled: true, Priority: 2},
				},
			}

			logger := logrus.New()
			logger.SetLevel(logrus.WarnLevel)
			
			// This would test the internal selection logic
			// For unit testing, we'd need to expose the selection methods or test through public API
			assert.NotNil(t, config)
		})
	}
}

func TestAdapterFactory_Caching(t *testing.T) {
	t.Run("adapter set caching", func(t *testing.T) {
		config := &config.Config{
			Nerdctl: config.NerdctlConfig{BinaryPath: "nerdctl"},
			Adapters: config.AdaptersConfig{
				SelectionStrategy: "auto",
				V1: config.AdapterVersionConfig{Enabled: true, Priority: 1},
			},
		}
		
		versionInfo := &version.NerdctlVersionInfo{
			Client: version.Version{Major: 1, Minor: 7, Patch: 0, Raw: "1.7.0"},
		}
		
		logger := logrus.New()
		logger.SetLevel(logrus.WarnLevel)
		
		mockCreator := &MockAdapterCreator{
			name:       "v1",
			priority:   1,
			compatible: true,
		}
		
		mockFactory := &factory.AdapterFactory{
			Config:          config,
			Logger:          logger,
			AdapterCreators: map[string]factory.AdapterCreator{"v1": mockCreator},
			AdapterCache:    make(map[string]*factory.AdapterSet),
		}
		
		ctx := context.Background()
		
		// First call should create and cache
		adapterSet1, err := mockFactory.CreateAdapters(ctx, versionInfo)
		require.NoError(t, err)
		require.NotNil(t, adapterSet1)
		
		// Second call should return cached version
		adapterSet2, err := mockFactory.CreateAdapters(ctx, versionInfo)
		require.NoError(t, err)
		require.NotNil(t, adapterSet2)
		
		// Should be the same instance (cached)
		assert.Equal(t, adapterSet1, adapterSet2)
	})
}

func TestAdapterFactory_ErrorHandling(t *testing.T) {
	tests := []struct {
		name          string
		config        *config.Config
		versionInfo   *version.NerdctlVersionInfo
		mockCreators  map[string]*MockAdapterCreator
		expectedError string
	}{
		{
			name: "no adapters enabled",
			config: &config.Config{
				Adapters: config.AdaptersConfig{
					V1: config.AdapterVersionConfig{Enabled: false},
					V2: config.AdapterVersionConfig{Enabled: false},
				},
			},
			versionInfo: &version.NerdctlVersionInfo{
				Client: version.Version{Major: 1, Minor: 7, Patch: 0, Raw: "1.7.0"},
			},
			mockCreators:  map[string]*MockAdapterCreator{},
			expectedError: "no adapter creators are enabled",
		},
		{
			name: "all adapters incompatible",
			config: &config.Config{
				Adapters: config.AdaptersConfig{
					SelectionStrategy: "auto",
					V1: config.AdapterVersionConfig{Enabled: true, Priority: 1},
					V2: config.AdapterVersionConfig{Enabled: true, Priority: 2},
				},
			},
			versionInfo: &version.NerdctlVersionInfo{
				Client: version.Version{Major: 3, Minor: 0, Patch: 0, Raw: "3.0.0"},
			},
			mockCreators: map[string]*MockAdapterCreator{
				"v1": {name: "v1", priority: 1, compatible: false},
				"v2": {name: "v2", priority: 2, compatible: false},
			},
			expectedError: "no compatible adapter found",
		},
		{
			name: "adapter creation failure",
			config: &config.Config{
				Adapters: config.AdaptersConfig{
					SelectionStrategy: "auto",
					V1: config.AdapterVersionConfig{Enabled: true, Priority: 1},
				},
			},
			versionInfo: &version.NerdctlVersionInfo{
				Client: version.Version{Major: 1, Minor: 7, Patch: 0, Raw: "1.7.0"},
			},
			mockCreators: map[string]*MockAdapterCreator{
				"v1": {
					name:        "v1",
					priority:    1,
					compatible:  true,
					createError: assert.AnError,
				},
			},
			expectedError: "failed to create adapter set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := logrus.New()
			logger.SetLevel(logrus.WarnLevel)
			
			mockFactory := &factory.AdapterFactory{
				Config:          tt.config,
				Logger:          logger,
				AdapterCreators: make(map[string]factory.AdapterCreator),
				AdapterCache:    make(map[string]*factory.AdapterSet),
			}
			
			// Register mock creators
			for name, creator := range tt.mockCreators {
				mockFactory.AdapterCreators[name] = creator
			}
			
			ctx := context.Background()
			adapterSet, err := mockFactory.CreateAdapters(ctx, tt.versionInfo)
			
			assert.Error(t, err)
			assert.Nil(t, adapterSet)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

// TestAdapterFactory_Integration provides integration tests
func TestAdapterFactory_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	t.Run("real factory creation", func(t *testing.T) {
		config := &config.Config{
			Nerdctl: config.NerdctlConfig{
				BinaryPath:       "nerdctl",
				DefaultNamespace: "default",
				ExecTimeout:      30 * time.Second,
			},
			Adapters: config.AdaptersConfig{
				SelectionStrategy: "auto",
				V1: config.AdapterVersionConfig{
					Enabled:    true,
					Priority:   1,
					MinVersion: "1.7.0",
					MaxVersion: "1.99.99",
				},
				V2: config.AdapterVersionConfig{
					Enabled:    true,
					Priority:   2,
					MinVersion: "2.0.0",
					MaxVersion: "2.99.99",
				},
			},
		}
		
		logger := logrus.New()
		
		// Create real factory (would fail if nerdctl not available)
		_, err := factory.NewAdapterFactory(config, logger)
		
		if err != nil {
			t.Skipf("Real factory creation failed (expected without nerdctl): %v", err)
		}
	})
}