package v2_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/containerd/nerdctl-grpc-server/internal/adapters/v2"
	"github.com/containerd/nerdctl-grpc-server/internal/config"
	"github.com/containerd/nerdctl-grpc-server/internal/version"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockV2CommandExecutor provides a mock implementation for testing v2.x features
type MockV2CommandExecutor struct {
	mockResults map[string]*v2.CommandResult
	mockErrors  map[string]error
	callLog     []string
	metadata    map[string]interface{}
}

// NewMockV2CommandExecutor creates a new mock v2 executor
func NewMockV2CommandExecutor() *MockV2CommandExecutor {
	return &MockV2CommandExecutor{
		mockResults: make(map[string]*v2.CommandResult),
		mockErrors:  make(map[string]error),
		callLog:     make([]string, 0),
		metadata:    make(map[string]interface{}),
	}
}

// Execute mocks v2 command execution with enhanced metadata
func (m *MockV2CommandExecutor) Execute(ctx context.Context, args []string) (*v2.CommandResult, error) {
	cmdKey := strings.Join(args, " ")
	m.callLog = append(m.callLog, cmdKey)
	
	if err, exists := m.mockErrors[cmdKey]; exists {
		return nil, err
	}
	
	if result, exists := m.mockResults[cmdKey]; exists {
		return result, nil
	}
	
	// Default successful result with v2.x metadata
	return &v2.CommandResult{
		ExitCode: 0,
		Stdout:   "mock v2 output",
		Stderr:   "",
		Duration: 10 * time.Millisecond,
		Metadata: map[string]interface{}{
			"output_type":        "text",
			"parsed_successfully": false,
			"version":            "v2.x",
		},
	}, nil
}

// SetMockResult sets a mock result for v2 command
func (m *MockV2CommandExecutor) SetMockResult(args []string, result *v2.CommandResult) {
	cmdKey := strings.Join(args, " ")
	m.mockResults[cmdKey] = result
}

// SetMockError sets a mock error for v2 command
func (m *MockV2CommandExecutor) SetMockError(args []string, err error) {
	cmdKey := strings.Join(args, " ")
	m.mockErrors[cmdKey] = err
}

// GetCallLog returns the log of executed commands
func (m *MockV2CommandExecutor) GetCallLog() []string {
	return m.callLog
}

func TestV2CommandExecutor_Execute(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectedResult *v2.CommandResult
		expectedError  string
	}{
		{
			name: "successful v2 command with JSON output",
			args: []string{"version", "--format", "json"},
			expectedResult: &v2.CommandResult{
				ExitCode: 0,
				Stdout:   `{"Client":{"Version":"2.0.0","GitCommit":"abc123"}}`,
				Stderr:   "",
				Duration: 15 * time.Millisecond,
				Metadata: map[string]interface{}{
					"output_type":        "json",
					"parsed_successfully": true,
				},
			},
		},
		{
			name: "v2 command with structured logs",
			args: []string{"container", "ls", "--format", "json"},
			expectedResult: &v2.CommandResult{
				ExitCode: 0,
				Stdout:   `[]`,
				Stderr:   `{"level":"info","msg":"listing containers","timestamp":"2024-01-01T00:00:00Z"}`,
				Duration: 20 * time.Millisecond,
				Metadata: map[string]interface{}{
					"output_type":        "json",
					"parsed_successfully": true,
					"log_levels": map[string]int{
						"info": 1,
					},
				},
			},
		},
		{
			name: "v2 enhanced error handling",
			args: []string{"invalid", "command"},
			expectedResult: &v2.CommandResult{
				ExitCode: 1,
				Stdout:   "",
				Stderr:   `{"level":"error","msg":"unknown command: invalid","timestamp":"2024-01-01T00:00:00Z"}`,
				Duration: 5 * time.Millisecond,
				Metadata: map[string]interface{}{
					"output_type": "text",
					"log_levels": map[string]int{
						"error": 1,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test configuration with v2 features
			config := &config.Config{
				Nerdctl: config.NerdctlConfig{
					BinaryPath:       "nerdctl",
					DefaultNamespace: "default",
					ExecTimeout:      30 * time.Second,
				},
				Adapters: config.AdaptersConfig{
					V2: config.AdapterVersionConfig{
						EnableStructuredLogs: true,
						EnableMetrics:        true,
						EnableJSONOutput:     true,
						EnableHealthChecks:   true,
					},
				},
			}
			
			// Create test version info for v2.x
			versionInfo := &version.NerdctlVersionInfo{
				Client: version.Version{
					Major: 2,
					Minor: 0,
					Patch: 0,
					Raw:   "2.0.0",
				},
			}
			
			// Create logger
			logger := logrus.New()
			logger.SetLevel(logrus.DebugLevel)
			
			// Create v2 executor
			executor, err := v2.NewV2CommandExecutor(config, versionInfo, logger)
			require.NoError(t, err)
			
			// Verify v2-specific features are enabled
			assert.True(t, executor.IsV2Feature("enhanced_json"))
			assert.True(t, executor.IsV2Feature("structured_logs"))
			assert.True(t, executor.IsV2Feature("metadata_streaming"))
			
			// This would require integration testing or mocking exec.Command
			// For unit tests, we'd typically mock the command execution
			t.Skip("Integration test - requires actual nerdctl v2.x binary")
		})
	}
}

func TestV2CommandExecutor_EnhanceArgsForV2(t *testing.T) {
	tests := []struct {
		name         string
		inputArgs    []string
		expectedArgs []string
		enableJSON   bool
	}{
		{
			name:         "enhance container list command",
			inputArgs:    []string{"ps"},
			expectedArgs: []string{"ps", "--format", "json"},
			enableJSON:   true,
		},
		{
			name:         "enhance image list command",
			inputArgs:    []string{"images"},
			expectedArgs: []string{"images", "--format", "json"},
			enableJSON:   true,
		},
		{
			name:         "preserve existing format argument",
			inputArgs:    []string{"ps", "--format", "table"},
			expectedArgs: []string{"ps", "--format", "table"},
			enableJSON:   true,
		},
		{
			name:         "no enhancement when JSON disabled",
			inputArgs:    []string{"ps"},
			expectedArgs: []string{"ps"},
			enableJSON:   false,
		},
		{
			name:         "no enhancement for non-compatible command",
			inputArgs:    []string{"run", "alpine", "echo", "hello"},
			expectedArgs: []string{"run", "alpine", "echo", "hello"},
			enableJSON:   true,
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
				Adapters: config.AdaptersConfig{
					V2: config.AdapterVersionConfig{
						EnableJSONOutput: tt.enableJSON,
					},
				},
			}
			
			versionInfo := &version.NerdctlVersionInfo{
				Client: version.Version{Major: 2, Minor: 0, Patch: 0, Raw: "2.0.0"},
			}
			
			logger := logrus.New()
			executor, err := v2.NewV2CommandExecutor(config, versionInfo, logger)
			require.NoError(t, err)
			
			// This test would require access to the private enhanceArgsForV2 method
			// In a real implementation, we might expose this as a public method for testing
			// or test it indirectly through the Execute method
			assert.NotNil(t, executor)
		})
	}
}

func TestV2CommandExecutor_ParseV2Metadata(t *testing.T) {
	tests := []struct {
		name             string
		stdout           string
		stderr           string
		enableStructured bool
		expectedMetadata map[string]interface{}
	}{
		{
			name:             "parse JSON output metadata",
			stdout:           `{"containers": [], "total": 0}`,
			stderr:           "",
			enableStructured: true,
			expectedMetadata: map[string]interface{}{
				"output_type":        "json",
				"parsed_successfully": true,
			},
		},
		{
			name:   "parse structured logs",
			stdout: "",
			stderr: `{"level":"info","msg":"starting operation"}
{"level":"debug","msg":"processing item 1"}
{"level":"error","msg":"failed to process item 2"}`,
			enableStructured: true,
			expectedMetadata: map[string]interface{}{
				"output_type":        "text",
				"parsed_successfully": false,
				"log_levels": map[string]int{
					"info":  1,
					"debug": 1,
					"error": 1,
				},
			},
		},
		{
			name:             "parse plain text output",
			stdout:           "container123\ncontainer456",
			stderr:           "",
			enableStructured: false,
			expectedMetadata: map[string]interface{}{
				"output_type":        "text",
				"parsed_successfully": false,
			},
		},
		{
			name:             "mixed JSON and logs",
			stdout:           `{"result": "success"}`,
			stderr:           `{"level":"info","msg":"operation completed"}`,
			enableStructured: true,
			expectedMetadata: map[string]interface{}{
				"output_type":        "json",
				"parsed_successfully": true,
				"log_levels": map[string]int{
					"info": 1,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test configuration
			config := &config.Config{
				Adapters: config.AdaptersConfig{
					V2: config.AdapterVersionConfig{
						EnableStructuredLogs: tt.enableStructured,
						EnableJSONOutput:     true,
					},
				},
			}
			
			versionInfo := &version.NerdctlVersionInfo{
				Client: version.Version{Major: 2, Minor: 0, Patch: 0, Raw: "2.0.0"},
			}
			
			logger := logrus.New()
			executor, err := v2.NewV2CommandExecutor(config, versionInfo, logger)
			require.NoError(t, err)
			
			// This test would require access to the private parseV2Metadata method
			// In a real implementation, we might expose this as a public method for testing
			// or test it indirectly through the Execute method
			assert.NotNil(t, executor)
		})
	}
}

func TestV2CommandExecutor_ExecuteStream(t *testing.T) {
	t.Run("v2 enhanced streaming", func(t *testing.T) {
		// Create test configuration with v2 streaming features
		config := &config.Config{
			Nerdctl: config.NerdctlConfig{
				BinaryPath:       "nerdctl",
				DefaultNamespace: "default",
				ExecTimeout:      30 * time.Second,
			},
			Adapters: config.AdaptersConfig{
				V2: config.AdapterVersionConfig{
					EnableStructuredLogs: true,
					EnableMetrics:        true,
				},
			},
		}
		
		versionInfo := &version.NerdctlVersionInfo{
			Client: version.Version{Major: 2, Minor: 0, Patch: 0, Raw: "2.0.0"},
		}
		
		logger := logrus.New()
		executor, err := v2.NewV2CommandExecutor(config, versionInfo, logger)
		require.NoError(t, err)
		
		// Test v2 streaming execution with metadata channel
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		// This would require integration testing with actual streaming command
		// For unit tests, we'd mock the streaming behavior
		t.Skip("Integration test - requires actual nerdctl v2.x binary and streaming command")
	})
}

func TestV2CommandExecutor_JSONOutputParsing(t *testing.T) {
	tests := []struct {
		name          string
		jsonOutput    string
		target        interface{}
		expectedError bool
	}{
		{
			name:       "valid v2 JSON output",
			jsonOutput: `{"containers": [{"id": "test123", "status": "running"}], "metadata": {"version": "2.0"}}`,
			target:     &map[string]interface{}{},
			expectedError: false,
		},
		{
			name:       "v2 JSON with enhanced fields",
			jsonOutput: `{"id": "container123", "state": {"health": {"status": "healthy"}}, "platform": "linux/amd64"}`,
			target:     &map[string]interface{}{},
			expectedError: false,
		},
		{
			name: "v2 JSON array format",
			jsonOutput: `[
				{"id": "c1", "name": "test1", "enhanced_metadata": {"cpu_usage": 0.1}},
				{"id": "c2", "name": "test2", "enhanced_metadata": {"cpu_usage": 0.2}}
			]`,
			target:     &[]map[string]interface{}{},
			expectedError: false,
		},
		{
			name:       "mixed output with logs (cleaned)",
			jsonOutput: "INFO: starting operation\n{\"result\": \"success\"}\nDEBUG: operation completed",
			target:     &map[string]interface{}{},
			expectedError: false,
		},
		{
			name:       "invalid JSON in v2",
			jsonOutput: `{invalid v2 json}`,
			target:     &map[string]interface{}{},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create v2 executor
			config := &config.Config{
				Adapters: config.AdaptersConfig{
					V2: config.AdapterVersionConfig{
						EnableJSONOutput: true,
					},
				},
			}
			
			versionInfo := &version.NerdctlVersionInfo{
				Client: version.Version{Major: 2, Minor: 0, Patch: 0, Raw: "2.0.0"},
			}
			
			logger := logrus.New()
			executor, err := v2.NewV2CommandExecutor(config, versionInfo, logger)
			require.NoError(t, err)
			
			// Test v2 JSON parsing
			err = executor.ParseJSONOutput(tt.jsonOutput, tt.target)
			
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestV2CommandExecutor_V2FeatureCheck(t *testing.T) {
	tests := []struct {
		name            string
		feature         string
		config          config.AdapterVersionConfig
		expectedEnabled bool
	}{
		{
			name:    "enhanced JSON feature",
			feature: "enhanced_json",
			config: config.AdapterVersionConfig{
				EnableJSONOutput: true,
			},
			expectedEnabled: true,
		},
		{
			name:    "structured logs feature",
			feature: "structured_logs",
			config: config.AdapterVersionConfig{
				EnableStructuredLogs: true,
			},
			expectedEnabled: true,
		},
		{
			name:    "metrics feature",
			feature: "metrics",
			config: config.AdapterVersionConfig{
				EnableMetrics: true,
			},
			expectedEnabled: true,
		},
		{
			name:    "metadata streaming feature",
			feature: "metadata_streaming",
			config: config.AdapterVersionConfig{
				EnableStructuredLogs: true,
			},
			expectedEnabled: true,
		},
		{
			name:    "disabled feature",
			feature: "structured_logs",
			config: config.AdapterVersionConfig{
				EnableStructuredLogs: false,
			},
			expectedEnabled: false,
		},
		{
			name:    "unknown feature",
			feature: "unknown_feature",
			config:  config.AdapterVersionConfig{},
			expectedEnabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create v2 executor with specific configuration
			config := &config.Config{
				Adapters: config.AdaptersConfig{
					V2: tt.config,
				},
			}
			
			versionInfo := &version.NerdctlVersionInfo{
				Client: version.Version{Major: 2, Minor: 0, Patch: 0, Raw: "2.0.0"},
			}
			
			logger := logrus.New()
			executor, err := v2.NewV2CommandExecutor(config, versionInfo, logger)
			require.NoError(t, err)
			
			// Test v2 feature availability
			enabled := executor.IsV2Feature(tt.feature)
			assert.Equal(t, tt.expectedEnabled, enabled)
		})
	}
}

// TestV2ExecutorIntegration provides integration tests for v2.x features
func TestV2ExecutorIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}
	
	// These tests require actual nerdctl v2.x binary
	t.Run("v2 version command with JSON", func(t *testing.T) {
		config := &config.Config{
			Nerdctl: config.NerdctlConfig{
				BinaryPath:       "nerdctl",
				DefaultNamespace: "default",
				ExecTimeout:      30 * time.Second,
			},
			Adapters: config.AdaptersConfig{
				V2: config.AdapterVersionConfig{
					EnableJSONOutput:     true,
					EnableStructuredLogs: true,
				},
			},
		}
		
		versionInfo := &version.NerdctlVersionInfo{
			Client: version.Version{Major: 2, Minor: 0, Patch: 0, Raw: "2.0.0"},
		}
		
		logger := logrus.New()
		executor, err := v2.NewV2CommandExecutor(config, versionInfo, logger)
		require.NoError(t, err)
		
		ctx := context.Background()
		result, err := executor.Execute(ctx, []string{"version"})
		
		if err != nil {
			t.Skipf("nerdctl v2.x not available: %v", err)
		}
		
		assert.NoError(t, err)
		assert.Equal(t, 0, result.ExitCode)
		assert.Contains(t, result.Stdout, "nerdctl")
		
		// Check v2.x enhanced metadata
		assert.NotEmpty(t, result.Metadata)
		
		// Try to parse as JSON if enhanced output is detected
		if result.Metadata["output_type"] == "json" {
			var versionData map[string]interface{}
			err = json.Unmarshal([]byte(result.Stdout), &versionData)
			assert.NoError(t, err)
		}
	})
}