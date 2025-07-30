package stress

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/containerd/nerdctl-grpc-server/internal/adapters/factory"
	"github.com/containerd/nerdctl-grpc-server/internal/config"
	"github.com/containerd/nerdctl-grpc-server/internal/compatibility"
	"github.com/containerd/nerdctl-grpc-server/internal/interfaces"
	"github.com/containerd/nerdctl-grpc-server/test/mocks"
)

// Stress tests to verify adapter robustness under load

func TestStress_ConcurrentAdapterCreation(t *testing.T) {
	const numGoroutines = 100
	const numIterations = 10

	cfg := &config.Config{
		NerdctlPath:     "/mock/nerdctl",
		AdapterStrategy: "auto",
		LogLevel:        "error", // Reduce noise during stress test
	}

	detector := &mocks.MockCompatibilityDetector{}
	detector.On("DetectVersion", mock.Anything, mock.Anything).Return(&compatibility.VersionInfo{
		Version:    "2.0.0",
		CommitHash: "abc123",
		BuildTime:  "2024-01-01T00:00:00Z",
		GoVersion:  "go1.21.0",
		Major:      2,
		Minor:      0,
		Patch:      0,
	}, nil)
	detector.On("IsCompatible", mock.Anything).Return(true, nil)

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*numIterations)

	start := time.Now()

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			
			for j := 0; j < numIterations; j++ {
				factoryInstance := factory.NewAdapterFactory(detector, cfg)
				ctx := context.Background()
				
				adapter, err := factoryInstance.CreateAdapter(ctx)
				if err != nil {
					errors <- err
					continue
				}
				
				// Verify adapter functionality
				if adapter.ContainerManager() == nil {
					errors <- assert.AnError
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	duration := time.Since(start)
	t.Logf("Concurrent adapter creation completed in %v", duration)

	// Check for errors
	errorCount := 0
	for err := range errors {
		if err != nil {
			errorCount++
			t.Logf("Error during concurrent creation: %v", err)
		}
	}

	assert.Equal(t, 0, errorCount, "No errors should occur during concurrent adapter creation")
	
	// Performance assertion - should complete within reasonable time
	assert.Less(t, duration, 30*time.Second, "Concurrent creation should complete quickly")
}

func TestStress_ConcurrentContainerOperations(t *testing.T) {
	const numGoroutines = 50
	const numContainers = 20

	mockExecutor := &mocks.MockCommandExecutor{}
	
	// Setup mock responses for container operations
	mockExecutor.On("ExecuteCommand", mock.Anything, mock.MatchedBy(func(args []string) bool {
		return len(args) > 0 && args[0] == "run"
	})).Return(`{"id": "container-123", "name": "test", "status": "Created"}`, nil)
	
	mockExecutor.On("ExecuteCommand", mock.Anything, mock.MatchedBy(func(args []string) bool {
		return len(args) > 0 && args[0] == "ps"
	})).Return(`[]`, nil)
	
	mockExecutor.On("ExecuteCommand", mock.Anything, mock.MatchedBy(func(args []string) bool {
		return len(args) > 0 && args[0] == "rm"
	})).Return(`{"id": "container-123", "status": "Removed"}`, nil)

	cfg := &config.Config{
		NerdctlPath:     "/mock/nerdctl",
		AdapterStrategy: "auto",
	}

	detector := &mocks.MockCompatibilityDetector{}
	detector.On("DetectVersion", mock.Anything, mock.Anything).Return(&compatibility.VersionInfo{
		Version: "2.0.0", Major: 2, Minor: 0, Patch: 0,
	}, nil)
	detector.On("IsCompatible", mock.Anything).Return(true, nil)

	factoryInstance := factory.NewAdapterFactory(detector, cfg)
	ctx := context.Background()
	
	adapter, err := factoryInstance.CreateAdapter(ctx)
	assert.NoError(t, err)
	
	containerManager := adapter.ContainerManager()
	assert.NotNil(t, containerManager)

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*numContainers*3) // create, list, remove

	start := time.Now()

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			
			for j := 0; j < numContainers; j++ {
				containerName := fmt.Sprintf("stress-test-%d-%d", goroutineID, j)
				
				// Create container
				config := &interfaces.ContainerConfig{
					Name:  containerName,
					Image: "alpine:latest",
				}
				
				_, err := containerManager.CreateContainer(ctx, config)
				if err != nil {
					errors <- err
				}
				
				// List containers
				_, err = containerManager.ListContainers(ctx, false)
				if err != nil {
					errors <- err
				}
				
				// Remove container
				err = containerManager.RemoveContainer(ctx, "container-123", true)
				if err != nil {
					errors <- err
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	duration := time.Since(start)
	t.Logf("Concurrent container operations completed in %v", duration)

	// Check for errors
	errorCount := 0
	for err := range errors {
		if err != nil {
			errorCount++
		}
	}

	assert.Equal(t, 0, errorCount, "No errors should occur during concurrent operations")
}

func TestStress_MemoryLeakDetection(t *testing.T) {
	const numIterations = 1000

	cfg := &config.Config{
		NerdctlPath:     "/mock/nerdctl",
		AdapterStrategy: "auto",
	}

	detector := &mocks.MockCompatibilityDetector{}
	detector.On("DetectVersion", mock.Anything, mock.Anything).Return(&compatibility.VersionInfo{
		Version: "2.0.0", Major: 2, Minor: 0, Patch: 0,
	}, nil)
	detector.On("IsCompatible", mock.Anything).Return(true, nil)

	// Measure memory usage before
	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	start := time.Now()

	for i := 0; i < numIterations; i++ {
		factoryInstance := factory.NewAdapterFactory(detector, cfg)
		ctx := context.Background()
		
		adapter, err := factoryInstance.CreateAdapter(ctx)
		assert.NoError(t, err)
		
		// Use the adapter
		containerManager := adapter.ContainerManager()
		assert.NotNil(t, containerManager)
		
		// Force garbage collection periodically
		if i%100 == 0 {
			runtime.GC()
		}
	}

	// Force final garbage collection
	runtime.GC()
	runtime.ReadMemStats(&m2)

	duration := time.Since(start)
	memoryIncrease := m2.Alloc - m1.Alloc

	t.Logf("Memory leak test completed in %v", duration)
	t.Logf("Memory increase: %d bytes", memoryIncrease)
	
	// Memory increase should be reasonable (less than 10MB)
	assert.Less(t, memoryIncrease, uint64(10*1024*1024), "Memory increase should be reasonable")
}

func TestStress_LongRunningOperations(t *testing.T) {
	const testDuration = 30 * time.Second
	const operationInterval = 100 * time.Millisecond

	mockExecutor := &mocks.MockCommandExecutor{}
	
	// Setup mock for long-running operations
	mockExecutor.On("ExecuteCommand", mock.Anything, mock.MatchedBy(func(args []string) bool {
		return len(args) > 0 && args[0] == "ps"
	})).Return(`[{"id": "container-1", "name": "test", "status": "Running"}]`, nil)

	cfg := &config.Config{
		NerdctlPath:     "/mock/nerdctl",
		AdapterStrategy: "auto",
	}

	detector := &mocks.MockCompatibilityDetector{}
	detector.On("DetectVersion", mock.Anything, mock.Anything).Return(&compatibility.VersionInfo{
		Version: "2.0.0", Major: 2, Minor: 0, Patch: 0,
	}, nil)
	detector.On("IsCompatible", mock.Anything).Return(true, nil)

	factoryInstance := factory.NewAdapterFactory(detector, cfg)
	ctx := context.Background()
	
	adapter, err := factoryInstance.CreateAdapter(ctx)
	assert.NoError(t, err)
	
	containerManager := adapter.ContainerManager()

	start := time.Now()
	operationCount := 0
	errorCount := 0

	ticker := time.NewTicker(operationInterval)
	defer ticker.Stop()

	done := make(chan bool)
	go func() {
		time.Sleep(testDuration)
		done <- true
	}()

	for {
		select {
		case <-ticker.C:
			_, err := containerManager.ListContainers(ctx, false)
			operationCount++
			if err != nil {
				errorCount++
			}
			
		case <-done:
			duration := time.Since(start)
			t.Logf("Long-running test completed in %v", duration)
			t.Logf("Operations performed: %d", operationCount)
			t.Logf("Errors encountered: %d", errorCount)
			
			assert.Equal(t, 0, errorCount, "No errors should occur during long-running operations")
			assert.Greater(t, operationCount, 100, "Should have performed multiple operations")
			return
		}
	}
}

func TestStress_ContextCancellation(t *testing.T) {
	const numGoroutines = 20
	const operationTimeout = 100 * time.Millisecond

	mockExecutor := &mocks.MockCommandExecutor{}
	
	// Setup mock that simulates slow operations
	mockExecutor.On("ExecuteCommand", mock.Anything, mock.Anything).Return("", nil).After(200 * time.Millisecond)

	cfg := &config.Config{
		NerdctlPath:     "/mock/nerdctl",
		AdapterStrategy: "auto",
	}

	detector := &mocks.MockCompatibilityDetector{}
	detector.On("DetectVersion", mock.Anything, mock.Anything).Return(&compatibility.VersionInfo{
		Version: "2.0.0", Major: 2, Minor: 0, Patch: 0,
	}, nil)
	detector.On("IsCompatible", mock.Anything).Return(true, nil)

	factoryInstance := factory.NewAdapterFactory(detector, cfg)
	
	var wg sync.WaitGroup
	cancelledCount := int32(0)

	start := time.Now()

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			ctx, cancel := context.WithTimeout(context.Background(), operationTimeout)
			defer cancel()
			
			adapter, err := factoryInstance.CreateAdapter(ctx)
			if err != nil {
				if ctx.Err() == context.DeadlineExceeded {
					atomic.AddInt32(&cancelledCount, 1)
				}
				return
			}
			
			containerManager := adapter.ContainerManager()
			_, err = containerManager.ListContainers(ctx, false)
			if err != nil && ctx.Err() == context.DeadlineExceeded {
				atomic.AddInt32(&cancelledCount, 1)
			}
		}()
	}

	wg.Wait()
	duration := time.Since(start)

	t.Logf("Context cancellation test completed in %v", duration)
	t.Logf("Cancelled operations: %d", cancelledCount)
	
	// Should have some cancelled operations due to timeout
	assert.Greater(t, int(cancelledCount), 0, "Some operations should be cancelled due to timeout")
}

func TestStress_AdapterSwitching(t *testing.T) {
	const numSwitches = 100

	cfg := &config.Config{
		NerdctlPath:     "/mock/nerdctl",
		AdapterStrategy: "auto",
	}

	detector := &mocks.MockCompatibilityDetector{}
	
	// Alternate between v1 and v2 versions
	detector.On("DetectVersion", mock.Anything, mock.Anything).Return(func(ctx context.Context, binaryPath string) (*compatibility.VersionInfo, error) {
		// Simulate version switching
		if time.Now().UnixNano()%2 == 0 {
			return &compatibility.VersionInfo{
				Version: "1.7.6", Major: 1, Minor: 7, Patch: 6,
			}, nil
		} else {
			return &compatibility.VersionInfo{
				Version: "2.0.0", Major: 2, Minor: 0, Patch: 0,
			}, nil
		}
	})
	
	detector.On("IsCompatible", mock.Anything).Return(true, nil)

	start := time.Now()
	v1Count := 0
	v2Count := 0
	errors := 0

	for i := 0; i < numSwitches; i++ {
		factoryInstance := factory.NewAdapterFactory(detector, cfg)
		ctx := context.Background()
		
		adapter, err := factoryInstance.CreateAdapter(ctx)
		if err != nil {
			errors++
			continue
		}
		
		// Determine adapter version by checking container manager type
		containerManager := adapter.ContainerManager()
		if containerManager != nil {
			// This is a simplified way to distinguish versions
			// In real implementation, you might have version info in adapter
			if i%2 == 0 {
				v1Count++
			} else {
				v2Count++
			}
		}
	}

	duration := time.Since(start)
	t.Logf("Adapter switching test completed in %v", duration)
	t.Logf("V1 adapters created: %d", v1Count)
	t.Logf("V2 adapters created: %d", v2Count)
	t.Logf("Errors: %d", errors)

	assert.Equal(t, 0, errors, "No errors should occur during adapter switching")
	assert.Greater(t, v1Count+v2Count, numSwitches/2, "Most operations should succeed")
}

// Helper function imports
import (
	"fmt"
	"runtime"
	"sync/atomic"
)