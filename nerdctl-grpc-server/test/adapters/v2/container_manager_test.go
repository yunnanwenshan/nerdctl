package v2

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/containerd/nerdctl-grpc-server/internal/adapters/v2"
	"github.com/containerd/nerdctl-grpc-server/internal/interfaces"
	"github.com/containerd/nerdctl-grpc-server/test/mocks"
)

func TestV2ContainerManager_CreateContainer(t *testing.T) {
	tests := []struct {
		name           string
		config         *interfaces.ContainerConfig
		mockOutput     string
		mockError      error
		expectedError  bool
	}{
		{
			name: "successful container creation with metadata",
			config: &interfaces.ContainerConfig{
				Name:  "test-container",
				Image: "alpine:latest",
				Command: []string{"echo", "hello"},
				Ports: map[string]string{"8080": "80"},
				Environment: map[string]string{"ENV": "test"},
			},
			mockOutput: `{
				"id": "container-id-123",
				"name": "test-container",
				"status": "Created",
				"metadata": {
					"created_at": "2024-01-01T00:00:00Z",
					"image_digest": "sha256:abc123"
				}
			}`,
			mockError:     nil,
			expectedError: false,
		},
		{
			name: "container creation with resource limits",
			config: &interfaces.ContainerConfig{
				Name:  "resource-limited",
				Image: "nginx:alpine",
				Resources: &interfaces.ResourceConfig{
					CPULimit:    "0.5",
					MemoryLimit: "512m",
					GPULimit:    "1",
				},
			},
			mockOutput: `{
				"id": "container-id-456",
				"name": "resource-limited",
				"status": "Created",
				"metadata": {
					"cpu_limit": "0.5",
					"memory_limit": "512m",
					"gpu_count": "1"
				}
			}`,
			mockError:     nil,
			expectedError: false,
		},
		{
			name: "container creation failure",
			config: &interfaces.ContainerConfig{
				Name:  "failing-container",
				Image: "non-existent:tag",
			},
			mockOutput:    "",
			mockError:     assert.AnError,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExecutor := &mocks.MockCommandExecutor{}
			manager := &v2.V2ContainerManager{Executor: mockExecutor}

			// Setup mock expectation
			mockExecutor.On("ExecuteCommand", mock.Anything, mock.MatchedBy(func(args []string) bool {
				return len(args) > 0 && args[0] == "run"
			})).Return(tt.mockOutput, tt.mockError)

			ctx := context.Background()
			result, err := manager.CreateContainer(ctx, tt.config)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.NotEmpty(t, result.ID)
				assert.Equal(t, tt.config.Name, result.Name)
			}

			mockExecutor.AssertExpectations(t)
		})
	}
}

func TestV2ContainerManager_StartContainer(t *testing.T) {
	tests := []struct {
		name          string
		containerID   string
		mockOutput    string
		mockError     error
		expectedError bool
	}{
		{
			name:        "successful start with detailed output",
			containerID: "container-123",
			mockOutput: `{
				"id": "container-123",
				"status": "Running",
				"metadata": {
					"started_at": "2024-01-01T00:00:01Z",
					"pid": "12345",
					"ports": ["0.0.0.0:8080->80/tcp"]
				}
			}`,
			mockError:     nil,
			expectedError: false,
		},
		{
			name:          "start non-existent container",
			containerID:   "non-existent",
			mockOutput:    "",
			mockError:     assert.AnError,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExecutor := &mocks.MockCommandExecutor{}
			manager := &v2.V2ContainerManager{Executor: mockExecutor}

			mockExecutor.On("ExecuteCommand", mock.Anything, []string{"start", tt.containerID}).
				Return(tt.mockOutput, tt.mockError)

			ctx := context.Background()
			err := manager.StartContainer(ctx, tt.containerID)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			mockExecutor.AssertExpectations(t)
		})
	}
}

func TestV2ContainerManager_StopContainer(t *testing.T) {
	tests := []struct {
		name          string
		containerID   string
		timeout       *time.Duration
		mockOutput    string
		mockError     error
		expectedError bool
	}{
		{
			name:        "graceful stop with timeout",
			containerID: "container-123",
			timeout:     func() *time.Duration { d := 30 * time.Second; return &d }(),
			mockOutput: `{
				"id": "container-123",
				"status": "Exited",
				"metadata": {
					"stopped_at": "2024-01-01T00:00:30Z",
					"exit_code": "0"
				}
			}`,
			mockError:     nil,
			expectedError: false,
		},
		{
			name:        "immediate stop",
			containerID: "container-456",
			timeout:     nil,
			mockOutput: `{
				"id": "container-456",
				"status": "Killed",
				"metadata": {
					"stopped_at": "2024-01-01T00:00:01Z",
					"exit_code": "137"
				}
			}`,
			mockError:     nil,
			expectedError: false,
		},
		{
			name:          "stop non-existent container",
			containerID:   "non-existent",
			timeout:       nil,
			mockOutput:    "",
			mockError:     assert.AnError,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExecutor := &mocks.MockCommandExecutor{}
			manager := &v2.V2ContainerManager{Executor: mockExecutor}

			expectedArgs := []string{"stop"}
			if tt.timeout != nil {
				expectedArgs = append(expectedArgs, "-t", "30")
			}
			expectedArgs = append(expectedArgs, tt.containerID)

			mockExecutor.On("ExecuteCommand", mock.Anything, expectedArgs).
				Return(tt.mockOutput, tt.mockError)

			ctx := context.Background()
			err := manager.StopContainer(ctx, tt.containerID, tt.timeout)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			mockExecutor.AssertExpectations(t)
		})
	}
}

func TestV2ContainerManager_RemoveContainer(t *testing.T) {
	tests := []struct {
		name          string
		containerID   string
		force         bool
		mockOutput    string
		mockError     error
		expectedError bool
	}{
		{
			name:        "normal removal",
			containerID: "container-123",
			force:       false,
			mockOutput: `{
				"id": "container-123",
				"status": "Removed",
				"metadata": {
					"removed_at": "2024-01-01T00:01:00Z"
				}
			}`,
			mockError:     nil,
			expectedError: false,
		},
		{
			name:        "force removal",
			containerID: "running-container",
			force:       true,
			mockOutput: `{
				"id": "running-container",
				"status": "Removed",
				"metadata": {
					"removed_at": "2024-01-01T00:01:00Z",
					"forced": true
				}
			}`,
			mockError:     nil,
			expectedError: false,
		},
		{
			name:          "removal failure",
			containerID:   "protected-container",
			force:         false,
			mockOutput:    "",
			mockError:     assert.AnError,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExecutor := &mocks.MockCommandExecutor{}
			manager := &v2.V2ContainerManager{Executor: mockExecutor}

			expectedArgs := []string{"rm"}
			if tt.force {
				expectedArgs = append(expectedArgs, "-f")
			}
			expectedArgs = append(expectedArgs, tt.containerID)

			mockExecutor.On("ExecuteCommand", mock.Anything, expectedArgs).
				Return(tt.mockOutput, tt.mockError)

			ctx := context.Background()
			err := manager.RemoveContainer(ctx, tt.containerID, tt.force)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			mockExecutor.AssertExpectations(t)
		})
	}
}

func TestV2ContainerManager_ListContainers(t *testing.T) {
	tests := []struct {
		name          string
		all           bool
		mockOutput    string
		mockError     error
		expectedCount int
		expectedError bool
	}{
		{
			name: "list running containers with metadata",
			all:  false,
			mockOutput: `[
				{
					"id": "container-1",
					"name": "web-server",
					"status": "Running",
					"image": "nginx:alpine",
					"metadata": {
						"created_at": "2024-01-01T00:00:00Z",
						"started_at": "2024-01-01T00:00:01Z",
						"ports": ["0.0.0.0:8080->80/tcp"],
						"health": "healthy"
					}
				},
				{
					"id": "container-2",
					"name": "database",
					"status": "Running",
					"image": "postgres:13",
					"metadata": {
						"created_at": "2024-01-01T00:00:00Z",
						"started_at": "2024-01-01T00:00:02Z",
						"volumes": ["/data:/var/lib/postgresql/data"],
						"health": "healthy"
					}
				}
			]`,
			mockError:     nil,
			expectedCount: 2,
			expectedError: false,
		},
		{
			name: "list all containers including stopped",
			all:  true,
			mockOutput: `[
				{
					"id": "container-1",
					"name": "web-server",
					"status": "Running",
					"image": "nginx:alpine",
					"metadata": {
						"health": "healthy"
					}
				},
				{
					"id": "container-3",
					"name": "old-service",
					"status": "Exited",
					"image": "alpine:latest",
					"metadata": {
						"exit_code": "0",
						"finished_at": "2024-01-01T00:05:00Z"
					}
				}
			]`,
			mockError:     nil,
			expectedCount: 2,
			expectedError: false,
		},
		{
			name:          "list containers failure",
			all:           false,
			mockOutput:    "",
			mockError:     assert.AnError,
			expectedCount: 0,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExecutor := &mocks.MockCommandExecutor{}
			manager := &v2.V2ContainerManager{Executor: mockExecutor}

			expectedArgs := []string{"ps", "--format", "json"}
			if tt.all {
				expectedArgs = append(expectedArgs, "-a")
			}

			mockExecutor.On("ExecuteCommand", mock.Anything, expectedArgs).
				Return(tt.mockOutput, tt.mockError)

			ctx := context.Background()
			containers, err := manager.ListContainers(ctx, tt.all)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, containers)
			} else {
				assert.NoError(t, err)
				assert.Len(t, containers, tt.expectedCount)
				
				// Verify metadata is properly parsed
				if len(containers) > 0 {
					assert.NotNil(t, containers[0].Metadata)
				}
			}

			mockExecutor.AssertExpectations(t)
		})
	}
}

func TestV2ContainerManager_InspectContainer(t *testing.T) {
	tests := []struct {
		name          string
		containerID   string
		mockOutput    string
		mockError     error
		expectedError bool
	}{
		{
			name:        "inspect container with full details",
			containerID: "container-123",
			mockOutput: `{
				"id": "container-123",
				"name": "test-container",
				"status": "Running",
				"image": "nginx:alpine",
				"command": ["nginx", "-g", "daemon off;"],
				"ports": {
					"80/tcp": [{"HostIp": "0.0.0.0", "HostPort": "8080"}]
				},
				"environment": ["ENV=production", "PORT=80"],
				"volumes": ["/data:/var/lib/data:rw"],
				"metadata": {
					"created_at": "2024-01-01T00:00:00Z",
					"started_at": "2024-01-01T00:00:01Z",
					"pid": "12345",
					"platform": "linux/amd64",
					"runtime": "runc",
					"network_mode": "bridge",
					"restart_policy": "unless-stopped",
					"health": {
						"status": "healthy",
						"failing_streak": 0,
						"log": ["Health check passed"]
					}
				}
			}`,
			mockError:     nil,
			expectedError: false,
		},
		{
			name:          "inspect non-existent container",
			containerID:   "non-existent",
			mockOutput:    "",
			mockError:     assert.AnError,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExecutor := &mocks.MockCommandExecutor{}
			manager := &v2.V2ContainerManager{Executor: mockExecutor}

			mockExecutor.On("ExecuteCommand", mock.Anything, []string{"inspect", "--format", "json", tt.containerID}).
				Return(tt.mockOutput, tt.mockError)

			ctx := context.Background()
			details, err := manager.InspectContainer(ctx, tt.containerID)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, details)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, details)
				assert.Equal(t, tt.containerID, details.ID)
				assert.NotNil(t, details.Metadata)
			}

			mockExecutor.AssertExpectations(t)
		})
	}
}

func TestV2ContainerManager_GetContainerLogs(t *testing.T) {
	tests := []struct {
		name          string
		containerID   string
		options       *interfaces.LogOptions
		mockOutput    string
		mockError     error
		expectedError bool
	}{
		{
			name:        "get logs with metadata timestamps",
			containerID: "container-123",
			options: &interfaces.LogOptions{
				Follow:     false,
				Tail:       "100",
				Since:      "2024-01-01T00:00:00Z",
				Until:      "2024-01-01T00:10:00Z",
				Timestamps: true,
			},
			mockOutput: `2024-01-01T00:00:01.123456789Z stdout: Starting server
2024-01-01T00:00:02.234567890Z stdout: Server listening on port 80
2024-01-01T00:00:03.345678901Z stderr: Warning: deprecated config option`,
			mockError:     nil,
			expectedError: false,
		},
		{
			name:        "get logs following",
			containerID: "container-456",
			options: &interfaces.LogOptions{
				Follow:     true,
				Timestamps: false,
			},
			mockOutput: `Starting application
Application ready`,
			mockError:     nil,
			expectedError: false,
		},
		{
			name:          "get logs from non-existent container",
			containerID:   "non-existent",
			options:       &interfaces.LogOptions{},
			mockOutput:    "",
			mockError:     assert.AnError,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExecutor := &mocks.MockCommandExecutor{}
			manager := &v2.V2ContainerManager{Executor: mockExecutor}

			// Build expected args based on options
			expectedArgs := []string{"logs"}
			if tt.options.Follow {
				expectedArgs = append(expectedArgs, "-f")
			}
			if tt.options.Timestamps {
				expectedArgs = append(expectedArgs, "-t")
			}
			if tt.options.Tail != "" {
				expectedArgs = append(expectedArgs, "--tail", tt.options.Tail)
			}
			if tt.options.Since != "" {
				expectedArgs = append(expectedArgs, "--since", tt.options.Since)
			}
			if tt.options.Until != "" {
				expectedArgs = append(expectedArgs, "--until", tt.options.Until)
			}
			expectedArgs = append(expectedArgs, tt.containerID)

			mockExecutor.On("ExecuteCommand", mock.Anything, expectedArgs).
				Return(tt.mockOutput, tt.mockError)

			ctx := context.Background()
			logs, err := manager.GetContainerLogs(ctx, tt.containerID, tt.options)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Empty(t, logs)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, logs)
			}

			mockExecutor.AssertExpectations(t)
		})
	}
}