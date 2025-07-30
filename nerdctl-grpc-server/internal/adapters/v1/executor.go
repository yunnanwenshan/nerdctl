package v1

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/containerd/nerdctl-grpc-server/internal/config"
	"github.com/containerd/nerdctl-grpc-server/internal/version"
	"github.com/sirupsen/logrus"
)

// V1CommandExecutor executes nerdctl commands for v1.x versions
// This handles version-specific command execution and output parsing
type V1CommandExecutor struct {
	config      *config.Config
	versionInfo *version.NerdctlVersionInfo
	logger      *logrus.Logger
	
	// v1-specific settings
	binaryPath   string
	timeout      time.Duration
	namespace    string
	globalArgs   []string
}

// CommandResult represents the result of executing a nerdctl command
type CommandResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
	Duration time.Duration
}

// StreamResult represents streaming command output
type StreamResult struct {
	LinesChan <-chan string
	ErrorsChan <-chan error
	Cancel    context.CancelFunc
}

// NewV1CommandExecutor creates a new V1CommandExecutor
func NewV1CommandExecutor(config *config.Config, versionInfo *version.NerdctlVersionInfo, logger *logrus.Logger) (*V1CommandExecutor, error) {
	executor := &V1CommandExecutor{
		config:      config,
		versionInfo: versionInfo,
		logger:      logger.WithField("component", "v1-executor"),
		binaryPath:  config.Nerdctl.BinaryPath,
		timeout:     config.Nerdctl.ExecTimeout,
		namespace:   config.Nerdctl.DefaultNamespace,
	}
	
	// Build global arguments for v1.x
	executor.globalArgs = executor.buildGlobalArgs()
	
	executor.logger.WithFields(logrus.Fields{
		"binary_path": executor.binaryPath,
		"namespace":   executor.namespace,
		"timeout":     executor.timeout,
		"version":     versionInfo.Client.Raw,
	}).Debug("V1 command executor initialized")
	
	return executor, nil
}

// buildGlobalArgs builds the global arguments for all nerdctl commands
func (e *V1CommandExecutor) buildGlobalArgs() []string {
	var args []string
	
	// Add namespace if specified
	if e.namespace != "" && e.namespace != "default" {
		args = append(args, "--namespace", e.namespace)
	}
	
	// Add containerd address if specified
	if e.config.Containerd.Address != "" && e.config.Containerd.Address != "/run/containerd/containerd.sock" {
		args = append(args, "--address", e.config.Containerd.Address)
	}
	
	// Add data root if specified
	if e.config.Nerdctl.DataRoot != "" {
		args = append(args, "--data-root", e.config.Nerdctl.DataRoot)
	}
	
	// Add config path if specified
	if e.config.Nerdctl.ConfigPath != "" {
		args = append(args, "--config", e.config.Nerdctl.ConfigPath)
	}
	
	return args
}

// Execute executes a nerdctl command and returns the result
func (e *V1CommandExecutor) Execute(ctx context.Context, args []string) (*CommandResult, error) {
	return e.ExecuteWithInput(ctx, args, nil)
}

// ExecuteWithInput executes a nerdctl command with input and returns the result
func (e *V1CommandExecutor) ExecuteWithInput(ctx context.Context, args []string, input io.Reader) (*CommandResult, error) {
	start := time.Now()
	
	// Combine global args with command args
	fullArgs := append(e.globalArgs, args...)
	
	// Log the command being executed
	e.logger.WithFields(logrus.Fields{
		"command": e.binaryPath,
		"args":    strings.Join(fullArgs, " "),
	}).Debug("Executing nerdctl command")
	
	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()
	
	// Create command
	cmd := exec.CommandContext(ctx, e.binaryPath, fullArgs...)
	
	// Set up input/output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	
	if input != nil {
		cmd.Stdin = input
	}
	
	// Execute command
	err := cmd.Run()
	duration := time.Since(start)
	
	// Get exit code
	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			// Non-exit error (e.g., command not found, timeout)
			e.logger.WithFields(logrus.Fields{
				"command": e.binaryPath,
				"args":    strings.Join(fullArgs, " "),
				"error":   err,
				"duration": duration,
			}).Error("Command execution failed")
			return nil, fmt.Errorf("command execution failed: %w", err)
		}
	}
	
	result := &CommandResult{
		ExitCode: exitCode,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: duration,
	}
	
	// Log result
	level := logrus.DebugLevel
	if exitCode != 0 {
		level = logrus.WarnLevel
	}
	
	e.logger.WithFields(logrus.Fields{
		"command":   e.binaryPath,
		"args":      strings.Join(fullArgs, " "),
		"exit_code": exitCode,
		"duration":  duration,
		"stdout_len": len(result.Stdout),
		"stderr_len": len(result.Stderr),
	}).Log(level, "Command completed")
	
	if exitCode != 0 {
		e.logger.WithFields(logrus.Fields{
			"stderr": result.Stderr,
		}).Debug("Command stderr output")
	}
	
	return result, nil
}

// ExecuteStream executes a command and returns streaming output
func (e *V1CommandExecutor) ExecuteStream(ctx context.Context, args []string) (*StreamResult, error) {
	// Combine global args with command args
	fullArgs := append(e.globalArgs, args...)
	
	e.logger.WithFields(logrus.Fields{
		"command": e.binaryPath,
		"args":    strings.Join(fullArgs, " "),
	}).Debug("Executing streaming nerdctl command")
	
	// Create command
	cmd := exec.CommandContext(ctx, e.binaryPath, fullArgs...)
	
	// Get stdout pipe
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	
	// Start command
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command: %w", err)
	}
	
	// Create channels for streaming
	linesChan := make(chan string)
	errorsChan := make(chan error, 1)
	
	// Create cancellable context
	streamCtx, cancel := context.WithCancel(ctx)
	
	// Start goroutine to read output
	go func() {
		defer close(linesChan)
		defer stdout.Close()
		
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			select {
			case <-streamCtx.Done():
				return
			case linesChan <- scanner.Text():
			}
		}
		
		if err := scanner.Err(); err != nil {
			select {
			case <-streamCtx.Done():
			case errorsChan <- fmt.Errorf("scanner error: %w", err):
			}
		}
		
		// Wait for command to finish
		if err := cmd.Wait(); err != nil {
			select {
			case <-streamCtx.Done():
			case errorsChan <- fmt.Errorf("command error: %w", err):
			}
		}
	}()
	
	return &StreamResult{
		LinesChan:  linesChan,
		ErrorsChan: errorsChan,
		Cancel:     cancel,
	}, nil
}

// ExecuteJSON executes a command that returns JSON and unmarshals the result
func (e *V1CommandExecutor) ExecuteJSON(ctx context.Context, args []string, result interface{}) error {
	// Add --format json to args if not already present
	jsonArgs := e.ensureJSONFormat(args)
	
	commandResult, err := e.Execute(ctx, jsonArgs)
	if err != nil {
		return fmt.Errorf("command execution failed: %w", err)
	}
	
	if commandResult.ExitCode != 0 {
		return fmt.Errorf("command failed with exit code %d: %s", commandResult.ExitCode, commandResult.Stderr)
	}
	
	if commandResult.Stdout == "" {
		return fmt.Errorf("no output received from command")
	}
	
	// Unmarshal JSON
	if err := json.Unmarshal([]byte(commandResult.Stdout), result); err != nil {
		e.logger.WithFields(logrus.Fields{
			"stdout": commandResult.Stdout,
			"error":  err,
		}).Error("Failed to unmarshal JSON response")
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}
	
	return nil
}

// ensureJSONFormat adds --format json to command args if not present
func (e *V1CommandExecutor) ensureJSONFormat(args []string) []string {
	// Check if --format is already specified
	for i, arg := range args {
		if arg == "--format" && i+1 < len(args) {
			// Format already specified, return as-is
			return args
		}
		if strings.HasPrefix(arg, "--format=") {
			// Format already specified inline, return as-is
			return args
		}
	}
	
	// Add --format json
	return append(args, "--format", "json")
}

// GetVersion returns the nerdctl version information
func (e *V1CommandExecutor) GetVersion() *version.NerdctlVersionInfo {
	return e.versionInfo
}

// GetBinaryPath returns the path to the nerdctl binary
func (e *V1CommandExecutor) GetBinaryPath() string {
	return e.binaryPath
}

// GetNamespace returns the default namespace
func (e *V1CommandExecutor) GetNamespace() string {
	return e.namespace
}

// SetNamespace changes the default namespace for subsequent commands
func (e *V1CommandExecutor) SetNamespace(namespace string) {
	e.namespace = namespace
	e.globalArgs = e.buildGlobalArgs()
	e.logger.WithField("namespace", namespace).Debug("Namespace changed")
}

// Ping tests connectivity to nerdctl/containerd
func (e *V1CommandExecutor) Ping(ctx context.Context) error {
	result, err := e.Execute(ctx, []string{"version"})
	if err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}
	
	if result.ExitCode != 0 {
		return fmt.Errorf("ping failed with exit code %d: %s", result.ExitCode, result.Stderr)
	}
	
	return nil
}

// BuildImageStreamArgs builds arguments for image streaming operations
func (e *V1CommandExecutor) BuildImageStreamArgs(operation string, imageName string, additionalArgs ...string) []string {
	args := []string{operation}
	
	if imageName != "" {
		args = append(args, imageName)
	}
	
	args = append(args, additionalArgs...)
	return args
}

// BuildContainerStreamArgs builds arguments for container streaming operations  
func (e *V1CommandExecutor) BuildContainerStreamArgs(operation string, containerID string, additionalArgs ...string) []string {
	args := []string{operation}
	
	if containerID != "" {
		args = append(args, containerID)
	}
	
	args = append(args, additionalArgs...)
	return args
}

// ParseContainerList parses container list output for v1.x format
func (e *V1CommandExecutor) ParseContainerList(output string) ([]map[string]interface{}, error) {
	if output == "" {
		return []map[string]interface{}{}, nil
	}
	
	var containers []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &containers); err != nil {
		// Try parsing as newline-delimited JSON
		lines := strings.Split(strings.TrimSpace(output), "\n")
		for _, line := range lines {
			if line == "" {
				continue
			}
			
			var container map[string]interface{}
			if err := json.Unmarshal([]byte(line), &container); err != nil {
				return nil, fmt.Errorf("failed to parse container JSON: %w", err)
			}
			containers = append(containers, container)
		}
	}
	
	return containers, nil
}

// ParseImageList parses image list output for v1.x format
func (e *V1CommandExecutor) ParseImageList(output string) ([]map[string]interface{}, error) {
	if output == "" {
		return []map[string]interface{}{}, nil
	}
	
	var images []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &images); err != nil {
		// Try parsing as newline-delimited JSON
		lines := strings.Split(strings.TrimSpace(output), "\n")
		for _, line := range lines {
			if line == "" {
				continue
			}
			
			var image map[string]interface{}
			if err := json.Unmarshal([]byte(line), &image); err != nil {
				return nil, fmt.Errorf("failed to parse image JSON: %w", err)
			}
			images = append(images, image)
		}
	}
	
	return images, nil
}

// IsV1FeatureSupported checks if a specific feature is supported in the current v1.x version
func (e *V1CommandExecutor) IsV1FeatureSupported(feature string) bool {
	clientVersion := e.versionInfo.Client
	
	// Define feature support matrix for v1.x versions
	switch feature {
	case "compose":
		return clientVersion.Minor >= 7
	case "build":
		return clientVersion.Minor >= 7
	case "ipfs":
		return clientVersion.Minor >= 8 // IPFS support might be limited in v1.x
	case "rootless":
		return clientVersion.Minor >= 7
	case "gpu":
		return clientVersion.Minor >= 8
	default:
		// Assume supported for basic features
		return true
	}
}

// Close cleans up resources
func (e *V1CommandExecutor) Close() error {
	e.logger.Debug("V1 command executor closed")
	return nil
}