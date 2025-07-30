package integration

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/containerd/nerdctl-grpc-server/internal/adapters/factory"
	"github.com/containerd/nerdctl-grpc-server/internal/config"
	"github.com/containerd/nerdctl-grpc-server/internal/compatibility"
	"github.com/containerd/nerdctl-grpc-server/internal/interfaces"
)

// Integration tests that require actual nerdctl binaries
// These tests are skipped if NERDCTL_INTEGRATION_TESTS environment variable is not set

func TestIntegration_AdapterFactory_RealBinaries(t *testing.T) {
	if os.Getenv("NERDCTL_INTEGRATION_TESTS") == "" {
		t.Skip("Integration tests disabled. Set NERDCTL_INTEGRATION_TESTS=1 to run.")
	}

	tests := []struct {
		name           string
		binaryPath     string
		expectedType   string
		expectError    bool
	}{
		{
			name:         "nerdctl v2.x binary",
			binaryPath:   findNerdctlBinary("v2"),
			expectedType: "v2",
			expectError:  false,
		},
		{
			name:         "nerdctl v1.x binary", 
			binaryPath:   findNerdctlBinary("v1"),
			expectedType: "v1",
			expectError:  false,
		},
		{
			name:        "non-existent binary",
			binaryPath:  "/non/existent/path/nerdctl",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.binaryPath == "" && !tt.expectError {
				t.Skip("Required nerdctl binary not found")
			}

			cfg := &config.Config{
				NerdctlPath:     tt.binaryPath,
				AdapterStrategy: "auto",
				LogLevel:        "info",
			}

			detector := compatibility.NewDetector()
			factoryInstance := factory.NewAdapterFactory(detector, cfg)

			ctx := context.Background()
			adapter, err := factoryInstance.CreateAdapter(ctx)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, adapter)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, adapter)
				
				// Verify adapter type
				containerManager := adapter.ContainerManager()
				assert.NotNil(t, containerManager)
			}
		})
	}
}

func TestIntegration_ContainerLifecycle_V2(t *testing.T) {
	if os.Getenv("NERDCTL_INTEGRATION_TESTS") == "" {
		t.Skip("Integration tests disabled. Set NERDCTL_INTEGRATION_TESTS=1 to run.")
	}

	binaryPath := findNerdctlBinary("v2")
	if binaryPath == "" {
		t.Skip("nerdctl v2.x binary not found")
	}

	cfg := &config.Config{
		NerdctlPath:     binaryPath,
		AdapterStrategy: "auto",
		LogLevel:        "debug",
		V2Features: &config.V2Features{
			StructuredLogs:   true,
			JSONOutput:       true,
			MetadataStreaming: true,
		},
	}

	detector := compatibility.NewDetector()
	factoryInstance := factory.NewAdapterFactory(detector, cfg)

	ctx := context.Background()
	adapter, err := factoryInstance.CreateAdapter(ctx)
	require.NoError(t, err)
	require.NotNil(t, adapter)

	containerManager := adapter.ContainerManager()
	require.NotNil(t, containerManager)

	// Test container lifecycle
	containerName := "integration-test-" + generateRandomID()
	containerConfig := &interfaces.ContainerConfig{
		Name:    containerName,
		Image:   "alpine:latest",
		Command: []string{"echo", "hello integration test"},
		Environment: map[string]string{
			"TEST_ENV": "integration",
		},
		Labels: map[string]string{
			"test.type": "integration",
			"test.id":   containerName,
		},
	}

	t.Run("create_container", func(t *testing.T) {
		result, err := containerManager.CreateContainer(ctx, containerConfig)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, containerName, result.Name)
		assert.NotEmpty(t, result.ID)

		// Store container ID for cleanup
		containerConfig.ID = result.ID
	})

	t.Run("list_containers", func(t *testing.T) {
		containers, err := containerManager.ListContainers(ctx, true)
		require.NoError(t, err)
		
		// Find our test container
		found := false
		for _, container := range containers {
			if container.Name == containerName {
				found = true
				assert.NotNil(t, container.Metadata)
				break
			}
		}
		assert.True(t, found, "Created container should appear in list")
	})

	t.Run("inspect_container", func(t *testing.T) {
		if containerConfig.ID == "" {
			t.Skip("Container ID not available")
		}

		details, err := containerManager.InspectContainer(ctx, containerConfig.ID)
		require.NoError(t, err)
		require.NotNil(t, details)
		
		assert.Equal(t, containerConfig.ID, details.ID)
		assert.Equal(t, containerName, details.Name)
		assert.NotNil(t, details.Metadata)
		
		// Verify environment variables
		envFound := false
		for _, env := range details.Environment {
			if env == "TEST_ENV=integration" {
				envFound = true
				break
			}
		}
		assert.True(t, envFound, "Environment variable should be present")
	})

	// Cleanup
	t.Cleanup(func() {
		if containerConfig.ID != "" {
			_ = containerManager.RemoveContainer(context.Background(), containerConfig.ID, true)
		}
	})
}

func TestIntegration_ImageOperations_V2(t *testing.T) {
	if os.Getenv("NERDCTL_INTEGRATION_TESTS") == "" {
		t.Skip("Integration tests disabled. Set NERDCTL_INTEGRATION_TESTS=1 to run.")
	}

	binaryPath := findNerdctlBinary("v2")
	if binaryPath == "" {
		t.Skip("nerdctl v2.x binary not found")
	}

	cfg := &config.Config{
		NerdctlPath:     binaryPath,
		AdapterStrategy: "auto",
		V2Features: &config.V2Features{
			JSONOutput: true,
			MetadataStreaming: true,
		},
	}

	detector := compatibility.NewDetector()
	factoryInstance := factory.NewAdapterFactory(detector, cfg)

	ctx := context.Background()
	adapter, err := factoryInstance.CreateAdapter(ctx)
	require.NoError(t, err)

	imageManager := adapter.ImageManager()
	require.NotNil(t, imageManager)

	testImage := "alpine:3.18"

	t.Run("pull_image", func(t *testing.T) {
		err := imageManager.PullImage(ctx, testImage, &interfaces.PullOptions{})
		assert.NoError(t, err)
	})

	t.Run("list_images", func(t *testing.T) {
		images, err := imageManager.ListImages(ctx)
		require.NoError(t, err)
		
		// Find our pulled image
		found := false
		for _, image := range images {
			if image.Repository == "alpine" && image.Tag == "3.18" {
				found = true
				assert.NotNil(t, image.Metadata)
				break
			}
		}
		assert.True(t, found, "Pulled image should appear in list")
	})

	t.Run("inspect_image", func(t *testing.T) {
		details, err := imageManager.InspectImage(ctx, testImage)
		require.NoError(t, err)
		require.NotNil(t, details)
		
		assert.Contains(t, details.RepoTags, testImage)
		assert.NotNil(t, details.Metadata)
	})
}

func TestIntegration_NetworkOperations_V2(t *testing.T) {
	if os.Getenv("NERDCTL_INTEGRATION_TESTS") == "" {
		t.Skip("Integration tests disabled. Set NERDCTL_INTEGRATION_TESTS=1 to run.")
	}

	binaryPath := findNerdctlBinary("v2")
	if binaryPath == "" {
		t.Skip("nerdctl v2.x binary not found")
	}

	cfg := &config.Config{
		NerdctlPath:     binaryPath,
		AdapterStrategy: "auto",
	}

	detector := compatibility.NewDetector()
	factoryInstance := factory.NewAdapterFactory(detector, cfg)

	ctx := context.Background()
	adapter, err := factoryInstance.CreateAdapter(ctx)
	require.NoError(t, err)

	networkManager := adapter.NetworkManager()
	require.NotNil(t, networkManager)

	networkName := "integration-test-net-" + generateRandomID()
	networkConfig := &interfaces.NetworkConfig{
		Name:   networkName,
		Driver: "bridge",
		Options: map[string]string{
			"subnet": "172.30.0.0/16",
		},
	}

	t.Run("create_network", func(t *testing.T) {
		result, err := networkManager.CreateNetwork(ctx, networkConfig)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, networkName, result.Name)
		assert.NotEmpty(t, result.ID)
	})

	t.Run("list_networks", func(t *testing.T) {
		networks, err := networkManager.ListNetworks(ctx)
		require.NoError(t, err)
		
		// Find our test network
		found := false
		for _, network := range networks {
			if network.Name == networkName {
				found = true
				break
			}
		}
		assert.True(t, found, "Created network should appear in list")
	})

	t.Run("inspect_network", func(t *testing.T) {
		details, err := networkManager.InspectNetwork(ctx, networkName)
		require.NoError(t, err)
		require.NotNil(t, details)
		assert.Equal(t, networkName, details.Name)
	})

	// Cleanup
	t.Cleanup(func() {
		_ = networkManager.RemoveNetwork(context.Background(), networkName, false)
	})
}

func TestIntegration_VersionCompatibility(t *testing.T) {
	if os.Getenv("NERDCTL_INTEGRATION_TESTS") == "" {
		t.Skip("Integration tests disabled. Set NERDCTL_INTEGRATION_TESTS=1 to run.")
	}

	tests := []struct {
		name           string
		strategy       string
		binaryPath     string
		expectAdapter  bool
	}{
		{
			name:          "auto strategy with v2 binary",
			strategy:      "auto",
			binaryPath:    findNerdctlBinary("v2"),
			expectAdapter: true,
		},
		{
			name:          "latest strategy",
			strategy:      "latest",
			binaryPath:    findNerdctlBinary("latest"),
			expectAdapter: true,
		},
		{
			name:          "stable strategy",
			strategy:      "stable",
			binaryPath:    findNerdctlBinary("stable"),
			expectAdapter: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.binaryPath == "" {
				t.Skip("Required nerdctl binary not found")
			}

			cfg := &config.Config{
				NerdctlPath:     tt.binaryPath,
				AdapterStrategy: tt.strategy,
			}

			detector := compatibility.NewDetector()
			factoryInstance := factory.NewAdapterFactory(detector, cfg)

			ctx := context.Background()
			adapter, err := factoryInstance.CreateAdapter(ctx)

			if tt.expectAdapter {
				assert.NoError(t, err)
				assert.NotNil(t, adapter)
				
				// Test basic functionality
				containerManager := adapter.ContainerManager()
				assert.NotNil(t, containerManager)
				
				// Test list operation
				_, err = containerManager.ListContainers(ctx, false)
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

// Helper functions

func findNerdctlBinary(version string) string {
	// Common paths where nerdctl might be installed
	searchPaths := []string{
		"/usr/local/bin/nerdctl",
		"/usr/bin/nerdctl",
		"/home/runner/app/_output/nerdctl", // Our built binary
		"/opt/nerdctl/bin/nerdctl",
	}

	for _, path := range searchPaths {
		if _, err := os.Stat(path); err == nil {
			// Check if binary works and get version
			if isValidNerdctl(path) {
				detectedVersion := detectBinaryVersion(path)
				if version == "latest" || version == "stable" || matchesVersion(detectedVersion, version) {
					return path
				}
			}
		}
	}

	return ""
}

func isValidNerdctl(binaryPath string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath, "--version")
	return cmd.Run() == nil
}

func detectBinaryVersion(binaryPath string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath, "--version")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	// Parse version from output like "nerdctl version 2.0.0"
	version := string(output)
	if len(version) > 15 { // "nerdctl version "
		return version[15:]
	}
	return ""
}

func matchesVersion(detected, expected string) bool {
	if expected == "v1" {
		return detected != "" && detected[0] == '1'
	}
	if expected == "v2" {
		return detected != "" && detected[0] == '2'
	}
	return detected == expected
}

func generateRandomID() string {
	return time.Now().Format("20060102-150405")
}

// Benchmark tests for performance validation

func BenchmarkIntegration_AdapterCreation(b *testing.B) {
	if os.Getenv("NERDCTL_INTEGRATION_TESTS") == "" {
		b.Skip("Integration tests disabled. Set NERDCTL_INTEGRATION_TESTS=1 to run.")
	}

	binaryPath := findNerdctlBinary("v2")
	if binaryPath == "" {
		b.Skip("nerdctl v2.x binary not found")
	}

	cfg := &config.Config{
		NerdctlPath:     binaryPath,
		AdapterStrategy: "auto",
	}

	detector := compatibility.NewDetector()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			factoryInstance := factory.NewAdapterFactory(detector, cfg)
			ctx := context.Background()
			
			adapter, err := factoryInstance.CreateAdapter(ctx)
			if err != nil {
				b.Fatalf("Failed to create adapter: %v", err)
			}
			
			// Verify adapter is functional
			if adapter.ContainerManager() == nil {
				b.Fatal("Container manager is nil")
			}
		}
	})
}

func BenchmarkIntegration_ContainerList(b *testing.B) {
	if os.Getenv("NERDCTL_INTEGRATION_TESTS") == "" {
		b.Skip("Integration tests disabled. Set NERDCTL_INTEGRATION_TESTS=1 to run.")
	}

	binaryPath := findNerdctlBinary("v2")
	if binaryPath == "" {
		b.Skip("nerdctl v2.x binary not found")
	}

	cfg := &config.Config{
		NerdctlPath:     binaryPath,
		AdapterStrategy: "auto",
	}

	detector := compatibility.NewDetector()
	factoryInstance := factory.NewAdapterFactory(detector, cfg)

	ctx := context.Background()
	adapter, err := factoryInstance.CreateAdapter(ctx)
	if err != nil {
		b.Fatalf("Failed to create adapter: %v", err)
	}

	containerManager := adapter.ContainerManager()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := containerManager.ListContainers(ctx, false)
			if err != nil {
				b.Fatalf("Failed to list containers: %v", err)
			}
		}
	})
}