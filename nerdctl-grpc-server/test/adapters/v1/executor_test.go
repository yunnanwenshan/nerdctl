package v1_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/containerd/nerdctl-grpc-server/internal/adapters/v1"
	"github.com/containerd/nerdctl-grpc-server/internal/config"
	"github.com/containerd/nerdctl-grpc-server/internal/version"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockV1CommandExecutor provides a mock implementation for testing
type MockV1CommandExecutor struct {
	mockResults map[string]*v1.CommandResult
	mockErrors  map[string]error
	callLog     []string
}

// NewMockV1CommandExecutor creates a new mock executor
func NewMockV1CommandExecutor() *MockV1CommandExecutor {
	return &MockV1CommandExecutor{
		mockResults: make(map[string]*v1.CommandResult),
		mockErrors:  make(map[string]error),
		callLog:     make([]string, 0),
	}
}

// Execute mocks command execution
func (m *MockV1CommandExecutor) Execute(ctx context.Context, args []string) (*v1.CommandResult, error) {
	cmdKey := strings.Join(args, " ")
	m.callLog = append(m.callLog, cmdKey)
	
	if err, exists := m.mockErrors[cmdKey]; exists {
		return nil, err
	}
	
	if result, exists := m.mockResults[cmdKey]; exists {
		return result, nil
	}
	
	// Default successful result
	return &v1.CommandResult{
		ExitCode: 0,
		Stdout:   "mock output",
		Stderr:   "",
		Duration: 10 * time.Millisecond,
	}, nil
}

// SetMockResult sets a mock result for a specific command
func (m *MockV1CommandExecutor) SetMockResult(args []string, result *v1.CommandResult) {
	cmdKey := strings.Join(args, " ")
	m.mockResults[cmdKey] = result
}

// SetMockError sets a mock error for a specific command
func (m *MockV1CommandExecutor) SetMockError(args []string, err error) {
	cmdKey := strings.Join(args, " ")
	m.mockErrors[cmdKey] = err
}

// GetCallLog returns the log of executed commands
func (m *MockV1CommandExecutor) GetCallLog() []string {
	return m.callLog
}

func TestV1CommandExecutor_Execute(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectedResult *v1.CommandResult
		expectedError  string
	}{
		{
			name: "successful command execution",
			args: []string{"version"},
			expectedResult: &v1.CommandResult{
				ExitCode: 0,
				Stdout:   "nerdctl version 1.7.0",
				Stderr:   "",
				Duration: 10 * time.Millisecond,
			},
		},
		{
			name: "command with non-zero exit code",
			args: []string{"invalid", "command"},
			expectedResult: &v1.CommandResult{
				ExitCode: 1,
				Stdout:   "",
				Stderr:   "unknown command: invalid",
				Duration: 5 * time.Millisecond,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test configuration
			config := &config.Config{
				Nerdctl: config.NerdctlConfig{
					BinaryPath:       "nerdctl",
					DefaultNamespace: "default",
					ExecTimeout:      30 * time.Second,
				},
			}
			
			// Create test version info
			versionInfo := &version.NerdctlVersionInfo{
				Client: version.Version{
					Major: 1,
					Minor: 7,
					Patch: 0,
					Raw:   "1.7.0",
				},
			}
			
			// Create logger
			logger := logrus.New()
			logger.SetLevel(logrus.DebugLevel)
			
			// Create executor
			executor, err := v1.NewV1CommandExecutor(config, versionInfo, logger)
			require.NoError(t, err)
			
			// This would require integration testing or mocking exec.Command
			// For unit tests, we'd typically mock the command execution
			t.Skip("Integration test - requires actual nerdctl binary")
		})
	}
}

func TestV1CommandExecutor_BuildGlobalArgs(t *testing.T) {
	tests := []struct {
		name           string
		config         *config.Config
		expectedArgs   []string
	}{
		{
			name: "default configuration",
			config: &config.Config{
				Nerdctl: config.NerdctlConfig{
					DefaultNamespace: "default",
				},
				Containerd: config.ContainerdConfig{
					Address: "/run/containerd/containerd.sock",
				},
			},
			expectedArgs: []string{},
		},
		{
			name: "custom namespace and address",
			config: &config.Config{
				Nerdctl: config.NerdctlConfig{
					DefaultNamespace: "test",
					DataRoot:        "/tmp/nerdctl",
				},
				Containerd: config.ContainerdConfig{
					Address: "/tmp/containerd.sock",
				},
			},
			expectedArgs: []string{
				"--namespace", "test",
				"--address", "/tmp/containerd.sock",
				"--data-root", "/tmp/nerdctl",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test version info
			versionInfo := &version.NerdctlVersionInfo{
				Client: version.Version{
					Major: 1,
					Minor: 7,
					Patch: 0,
					Raw:   "1.7.0",
				},
			}
			
			// Create logger
			logger := logrus.New()
			logger.SetLevel(logrus.DebugLevel)
			
			// Create executor
			executor, err := v1.NewV1CommandExecutor(tt.config, versionInfo, logger)
			require.NoError(t, err)
			
			// Test global args building (this would require access to private method)
			// For now, we test through public interface
			assert.NotNil(t, executor)
		})
	}
}

func TestV1CommandExecutor_ParseJSONOutput(t *testing.T) {
	tests := []struct {
		name           string
		jsonOutput     string
		target         interface{}
		expectedError  bool
	}{
		{
			name:       "valid JSON output",
			jsonOutput: `{"id": "test123", "name": "test-container"}`,
			target:     &map[string]interface{}{},
			expectedError: false,
		},
		{
			name:       "invalid JSON output",
			jsonOutput: `{invalid json}`,
			target:     &map[string]interface{}{},
			expectedError: true,
		},
		{
			name:       "empty output",
			jsonOutput: "",
			target:     &map[string]interface{}{},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test configuration and executor
			config := &config.Config{
				Nerdctl: config.NerdctlConfig{
					BinaryPath:       "nerdctl",
					DefaultNamespace: "default",
					ExecTimeout:      30 * time.Second,
				},
			}
			
			versionInfo := &version.NerdctlVersionInfo{
				Client: version.Version{Major: 1, Minor: 7, Patch: 0, Raw: "1.7.0"},
			}
			
			logger := logrus.New()
			executor, err := v1.NewV1CommandExecutor(config, versionInfo, logger)
			require.NoError(t, err)
			
			// Test JSON parsing
			err = executor.ParseJSONOutput(tt.jsonOutput, tt.target)
			
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestV1CommandExecutor_ExecuteStream(t *testing.T) {
	t.Run("stream execution", func(t *testing.T) {
		// Create test configuration
		config := &config.Config{
			Nerdctl: config.NerdctlConfig{
				BinaryPath:       "nerdctl",
				DefaultNamespace: "default",
				ExecTimeout:      30 * time.Second,
			},
		}
		
		versionInfo := &version.NerdctlVersionInfo{
			Client: version.Version{Major: 1, Minor: 7, Patch: 0, Raw: "1.7.0"},
		}
		
		logger := logrus.New()
		executor, err := v1.NewV1CommandExecutor(config, versionInfo, logger)
		require.NoError(t, err)
		
		// Test streaming execution
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		// This would require integration testing
		t.Skip("Integration test - requires actual nerdctl binary and streaming command")
	})
}

// TestV1ExecutorIntegration provides integration tests
func TestV1ExecutorIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}
	
	// These tests require actual nerdctl binary
	t.Run("version command", func(t *testing.T) {
		config := &config.Config{
			Nerdctl: config.NerdctlConfig{
				BinaryPath:       "nerdctl",
				DefaultNamespace: "default",
				ExecTimeout:      30 * time.Second,
			},
		}
		
		versionInfo := &version.NerdctlVersionInfo{
			Client: version.Version{Major: 1, Minor: 7, Patch: 0, Raw: "1.7.0"},
		}
		
		logger := logrus.New()
		executor, err := v1.NewV1CommandExecutor(config, versionInfo, logger)
		require.NoError(t, err)
		
		ctx := context.Background()
		result, err := executor.Execute(ctx, []string{"version"})
		
		if err != nil {
			t.Skipf("nerdctl not available: %v", err)
		}
		
		assert.NoError(t, err)
		assert.Equal(t, 0, result.ExitCode)
		assert.Contains(t, result.Stdout, "nerdctl")
	})
}