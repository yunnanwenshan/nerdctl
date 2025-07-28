package v1_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/containerd/nerdctl-grpc-server/internal/adapters/v1"
	"github.com/containerd/nerdctl-grpc-server/internal/config"
	"github.com/containerd/nerdctl-grpc-server/pkg/types"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestV1ContainerManager_CreateContainer(t *testing.T) {
	tests := []struct {
		name          string
		request       *types.CreateContainerRequest
		mockResult    *v1.CommandResult
		mockError     error
		expectedID    string
		expectedError bool
	}{
		{
			name: "successful container creation",
			request: &types.CreateContainerRequest{
				Image: "alpine:latest",
				Name:  "test-container",
				Command: []string{"echo", "hello"},
			},
			mockResult: &v1.CommandResult{
				ExitCode: 0,
				Stdout:   "container123abc\n",
				Stderr:   "",
				Duration: 100 * time.Millisecond,
			},
			expectedID: "container123abc",
			expectedError: false,
		},
		{
			name: "container creation with ports and environment",
			request: &types.CreateContainerRequest{
				Image: "nginx:latest",
				Name:  "web-server",
				Ports: []types.PortBinding{
					{HostPort: 8080, ContainerPort: 80, Protocol: "tcp"},
				},
				Environment: map[string]string{
					"ENV": "test",
					"DEBUG": "true",
				},
			},
			mockResult: &v1.CommandResult{
				ExitCode: 0,
				Stdout:   "nginx123def\n",
				Stderr:   "",
				Duration: 150 * time.Millisecond,
			},
			expectedID: "nginx123def",
			expectedError: false,
		},
		{
			name: "container creation with volume mounts",
			request: &types.CreateContainerRequest{
				Image: "ubuntu:latest",
				Name:  "data-container",
				Mounts: []types.MountPoint{
					{Source: "/host/data", Target: "/container/data", Type: "bind"},
					{Source: "volume-name", Target: "/app", Type: "volume"},
				},
			},
			mockResult: &v1.CommandResult{
				ExitCode: 0,
				Stdout:   "ubuntu123ghi\n",
				Stderr:   "",
				Duration: 120 * time.Millisecond,
			},
			expectedID: "ubuntu123ghi",
			expectedError: false,
		},
		{
			name: "container creation failure",
			request: &types.CreateContainerRequest{
				Image: "nonexistent:latest",
				Name:  "fail-container",
			},
			mockResult: &v1.CommandResult{
				ExitCode: 1,
				Stdout:   "",
				Stderr:   "image not found: nonexistent:latest",
				Duration: 50 * time.Millisecond,
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock executor
			mockExecutor := NewMockV1CommandExecutor()
			
			// Build expected command
			expectedArgs := []string{"container", "create"}
			if tt.request.Name != "" {
				expectedArgs = append(expectedArgs, "--name", tt.request.Name)
			}
			for _, port := range tt.request.Ports {
				portStr := fmt.Sprintf("%d:%d", port.HostPort, port.ContainerPort)
				if port.Protocol != "" {
					portStr += "/" + port.Protocol
				}
				expectedArgs = append(expectedArgs, "-p", portStr)
			}
			for key, value := range tt.request.Environment {
				expectedArgs = append(expectedArgs, "-e", fmt.Sprintf("%s=%s", key, value))
			}
			for _, mount := range tt.request.Mounts {
				mountStr := fmt.Sprintf("%s:%s", mount.Source, mount.Target)
				if mount.ReadOnly {
					mountStr += ":ro"
				}
				expectedArgs = append(expectedArgs, "-v", mountStr)
			}
			expectedArgs = append(expectedArgs, tt.request.Image)
			expectedArgs = append(expectedArgs, tt.request.Command...)
			
			// Set up mock response
			if tt.mockError != nil {
				mockExecutor.SetMockError(expectedArgs, tt.mockError)
			} else {
				mockExecutor.SetMockResult(expectedArgs, tt.mockResult)
			}
			
			// Create test configuration
			config := &config.Config{}
			logger := logrus.New()
			logger.SetLevel(logrus.WarnLevel) // Reduce test noise
			
			// Create container manager with mock executor
			manager := &v1.V1ContainerManager{
				Executor: mockExecutor,
				Config:   config,
				Logger:   logger.WithField("component", "test"),
			}
			
			// Execute create container
			ctx := context.Background()
			response, err := manager.CreateContainer(ctx, tt.request)
			
			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, response)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, response)
				assert.Equal(t, tt.expectedID, response.ID)
			}
		})
	}
}

func TestV1ContainerManager_ListContainers(t *testing.T) {
	tests := []struct {
		name           string
		request        *types.ListContainersRequest
		mockResult     *v1.CommandResult
		expectedCount  int
		expectedError  bool
	}{
		{
			name: "list all containers",
			request: &types.ListContainersRequest{
				All: true,
			},
			mockResult: &v1.CommandResult{
				ExitCode: 0,
				Stdout: `[
					{
						"Id": "container123",
						"Names": ["/test-container"],
						"Image": "alpine:latest",
						"Status": "running",
						"Command": "echo hello"
					},
					{
						"Id": "container456",
						"Names": ["/web-server"],
						"Image": "nginx:latest",
						"Status": "exited",
						"Command": "nginx -g daemon off;"
					}
				]`,
				Stderr:   "",
				Duration: 200 * time.Millisecond,
			},
			expectedCount: 2,
			expectedError: false,
		},
		{
			name: "list running containers only",
			request: &types.ListContainersRequest{
				All: false,
			},
			mockResult: &v1.CommandResult{
				ExitCode: 0,
				Stdout: `[
					{
						"Id": "container123",
						"Names": ["/test-container"],
						"Image": "alpine:latest",
						"Status": "running",
						"Command": "echo hello"
					}
				]`,
				Stderr:   "",
				Duration: 150 * time.Millisecond,
			},
			expectedCount: 1,
			expectedError: false,
		},
		{
			name: "list with filters",
			request: &types.ListContainersRequest{
				All: true,
				Filters: map[string][]string{
					"status": {"running"},
					"label":  {"env=test"},
				},
			},
			mockResult: &v1.CommandResult{
				ExitCode: 0,
				Stdout:   `[]`,
				Stderr:   "",
				Duration: 100 * time.Millisecond,
			},
			expectedCount: 0,
			expectedError: false,
		},
		{
			name: "command failure",
			request: &types.ListContainersRequest{
				All: true,
			},
			mockResult: &v1.CommandResult{
				ExitCode: 1,
				Stdout:   "",
				Stderr:   "permission denied",
				Duration: 50 * time.Millisecond,
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock executor
			mockExecutor := NewMockV1CommandExecutor()
			
			// Build expected command
			expectedArgs := []string{"container", "ls"}
			if tt.request.All {
				expectedArgs = append(expectedArgs, "--all")
			}
			if tt.request.Size {
				expectedArgs = append(expectedArgs, "--size")
			}
			for key, values := range tt.request.Filters {
				for _, value := range values {
					expectedArgs = append(expectedArgs, "--filter", fmt.Sprintf("%s=%s", key, value))
				}
			}
			expectedArgs = append(expectedArgs, "--format", "json")
			
			// Set up mock response
			mockExecutor.SetMockResult(expectedArgs, tt.mockResult)
			
			// Create container manager
			config := &config.Config{}
			logger := logrus.New()
			logger.SetLevel(logrus.WarnLevel)
			
			manager := &v1.V1ContainerManager{
				Executor: mockExecutor,
				Config:   config,
				Logger:   logger.WithField("component", "test"),
			}
			
			// Execute list containers
			ctx := context.Background()
			response, err := manager.ListContainers(ctx, tt.request)
			
			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, response)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, response)
				assert.Len(t, response.Containers, tt.expectedCount)
			}
		})
	}
}

func TestV1ContainerManager_StartContainer(t *testing.T) {
	tests := []struct {
		name          string
		request       *types.StartContainerRequest
		mockResult    *v1.CommandResult
		expectedError bool
	}{
		{
			name: "successful container start",
			request: &types.StartContainerRequest{
				ID: "container123",
			},
			mockResult: &v1.CommandResult{
				ExitCode: 0,
				Stdout:   "",
				Stderr:   "",
				Duration: 100 * time.Millisecond,
			},
			expectedError: false,
		},
		{
			name: "start with attach options",
			request: &types.StartContainerRequest{
				ID:           "container456",
				AttachStdout: true,
				AttachStderr: true,
			},
			mockResult: &v1.CommandResult{
				ExitCode: 0,
				Stdout:   "",
				Stderr:   "",
				Duration: 150 * time.Millisecond,
			},
			expectedError: false,
		},
		{
			name: "container not found",
			request: &types.StartContainerRequest{
				ID: "nonexistent",
			},
			mockResult: &v1.CommandResult{
				ExitCode: 1,
				Stdout:   "",
				Stderr:   "No such container: nonexistent",
				Duration: 50 * time.Millisecond,
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock executor
			mockExecutor := NewMockV1CommandExecutor()
			
			// Build expected command
			expectedArgs := []string{"container", "start"}
			if tt.request.AttachStdout || tt.request.AttachStderr {
				expectedArgs = append(expectedArgs, "--attach")
			}
			if tt.request.AttachStdin {
				expectedArgs = append(expectedArgs, "--interactive")
			}
			expectedArgs = append(expectedArgs, tt.request.ID)
			
			// Set up mock response
			mockExecutor.SetMockResult(expectedArgs, tt.mockResult)
			
			// Create container manager
			config := &config.Config{}
			logger := logrus.New()
			logger.SetLevel(logrus.WarnLevel)
			
			manager := &v1.V1ContainerManager{
				Executor: mockExecutor,
				Config:   config,
				Logger:   logger.WithField("component", "test"),
			}
			
			// Execute start container
			ctx := context.Background()
			response, err := manager.StartContainer(ctx, tt.request)
			
			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, response)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, response)
			}
		})
	}
}

func TestV1ContainerManager_StopContainer(t *testing.T) {
	tests := []struct {
		name          string
		request       *types.StopContainerRequest
		mockResult    *v1.CommandResult
		expectedError bool
	}{
		{
			name: "successful container stop",
			request: &types.StopContainerRequest{
				ID: "container123",
			},
			mockResult: &v1.CommandResult{
				ExitCode: 0,
				Stdout:   "",
				Stderr:   "",
				Duration: 200 * time.Millisecond,
			},
			expectedError: false,
		},
		{
			name: "stop with timeout",
			request: &types.StopContainerRequest{
				ID:      "container456",
				Timeout: 10,
			},
			mockResult: &v1.CommandResult{
				ExitCode: 0,
				Stdout:   "",
				Stderr:   "",
				Duration: 300 * time.Millisecond,
			},
			expectedError: false,
		},
		{
			name: "container not running",
			request: &types.StopContainerRequest{
				ID: "stopped-container",
			},
			mockResult: &v1.CommandResult{
				ExitCode: 1,
				Stdout:   "",
				Stderr:   "Container stopped-container is not running",
				Duration: 50 * time.Millisecond,
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock executor
			mockExecutor := NewMockV1CommandExecutor()
			
			// Build expected command
			expectedArgs := []string{"container", "stop"}
			if tt.request.Timeout > 0 {
				expectedArgs = append(expectedArgs, "--time", fmt.Sprintf("%d", tt.request.Timeout))
			}
			expectedArgs = append(expectedArgs, tt.request.ID)
			
			// Set up mock response
			mockExecutor.SetMockResult(expectedArgs, tt.mockResult)
			
			// Create container manager
			config := &config.Config{}
			logger := logrus.New()
			logger.SetLevel(logrus.WarnLevel)
			
			manager := &v1.V1ContainerManager{
				Executor: mockExecutor,
				Config:   config,
				Logger:   logger.WithField("component", "test"),
			}
			
			// Execute stop container
			ctx := context.Background()
			response, err := manager.StopContainer(ctx, tt.request)
			
			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, response)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, response)
			}
		})
	}
}

func TestV1ContainerManager_RemoveContainer(t *testing.T) {
	tests := []struct {
		name          string
		request       *types.RemoveContainerRequest
		mockResult    *v1.CommandResult
		expectedError bool
	}{
		{
			name: "successful container removal",
			request: &types.RemoveContainerRequest{
				ID: "container123",
			},
			mockResult: &v1.CommandResult{
				ExitCode: 0,
				Stdout:   "container123",
				Stderr:   "",
				Duration: 150 * time.Millisecond,
			},
			expectedError: false,
		},
		{
			name: "force remove running container",
			request: &types.RemoveContainerRequest{
				ID:    "running-container",
				Force: true,
			},
			mockResult: &v1.CommandResult{
				ExitCode: 0,
				Stdout:   "running-container",
				Stderr:   "",
				Duration: 250 * time.Millisecond,
			},
			expectedError: false,
		},
		{
			name: "remove with volumes",
			request: &types.RemoveContainerRequest{
				ID:            "data-container",
				RemoveVolumes: true,
			},
			mockResult: &v1.CommandResult{
				ExitCode: 0,
				Stdout:   "data-container",
				Stderr:   "",
				Duration: 200 * time.Millisecond,
			},
			expectedError: false,
		},
		{
			name: "container in use",
			request: &types.RemoveContainerRequest{
				ID: "busy-container",
			},
			mockResult: &v1.CommandResult{
				ExitCode: 1,
				Stdout:   "",
				Stderr:   "cannot remove running container busy-container",
				Duration: 50 * time.Millisecond,
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock executor
			mockExecutor := NewMockV1CommandExecutor()
			
			// Build expected command
			expectedArgs := []string{"container", "rm"}
			if tt.request.Force {
				expectedArgs = append(expectedArgs, "--force")
			}
			if tt.request.RemoveVolumes {
				expectedArgs = append(expectedArgs, "--volumes")
			}
			if tt.request.RemoveLinks {
				expectedArgs = append(expectedArgs, "--link")
			}
			expectedArgs = append(expectedArgs, tt.request.ID)
			
			// Set up mock response
			mockExecutor.SetMockResult(expectedArgs, tt.mockResult)
			
			// Create container manager
			config := &config.Config{}
			logger := logrus.New()
			logger.SetLevel(logrus.WarnLevel)
			
			manager := &v1.V1ContainerManager{
				Executor: mockExecutor,
				Config:   config,
				Logger:   logger.WithField("component", "test"),
			}
			
			// Execute remove container
			ctx := context.Background()
			response, err := manager.RemoveContainer(ctx, tt.request)
			
			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, response)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, response)
			}
		})
	}
}