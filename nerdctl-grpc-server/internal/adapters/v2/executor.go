package v2

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

// V2CommandExecutor executes nerdctl commands for v2.x versions
// This handles version-specific command execution with v2.x enhancements
type V2CommandExecutor struct {
	config      *config.Config
	versionInfo *version.NerdctlVersionInfo
	logger      *logrus.Entry
	
	// v2-specific settings
	binaryPath   string
	timeout      time.Duration
	namespace    string
	globalArgs   []string
	
	// v2-specific features
	enableJSONOutput      bool
	enableStructuredLogs  bool
	enableMetrics         bool
}

// CommandResult represents the result of executing a nerdctl v2.x command
type CommandResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
	Duration time.Duration
	
	// v2-specific metadata
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// StreamResult represents streaming command output with v2 enhancements
type StreamResult struct {
	LinesChan   <-chan string
	ErrorsChan  <-chan error
	MetadataChan <-chan map[string]interface{} // v2 metadata stream
	Cancel      context.CancelFunc
}

// NewV2CommandExecutor creates a new V2CommandExecutor
func NewV2CommandExecutor(config *config.Config, versionInfo *version.NerdctlVersionInfo, logger *logrus.Logger) (*V2CommandExecutor, error) {
	executor := &V2CommandExecutor{
		config:      config,
		versionInfo: versionInfo,
		logger:      logger.WithField("component", "v2-executor"),
		binaryPath:  config.Nerdctl.BinaryPath,
		timeout:     config.Nerdctl.ExecTimeout,
		namespace:   config.Nerdctl.DefaultNamespace,
		
		// Enable v2-specific features
		enableJSONOutput:     true,
		enableStructuredLogs: config.Adapters.V2.EnableStructuredLogs,
		enableMetrics:        config.Adapters.V2.EnableMetrics,
	}
	
	// Build global arguments for v2.x with enhanced features
	executor.globalArgs = executor.buildGlobalArgs()
	
	executor.logger.WithFields(logrus.Fields{
		"binary_path":         executor.binaryPath,
		"namespace":           executor.namespace,
		"timeout":             executor.timeout,
		"version":             versionInfo.Client.Raw,
		"json_output":         executor.enableJSONOutput,
		"structured_logs":     executor.enableStructuredLogs,
		"metrics":             executor.enableMetrics,
	}).Debug("V2 command executor initialized")
	
	return executor, nil
}

// buildGlobalArgs builds the global arguments for all nerdctl v2.x commands
func (e *V2CommandExecutor) buildGlobalArgs() []string {
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
	
	// v2-specific: Add structured logging if enabled
	if e.enableStructuredLogs {
		args = append(args, "--log-format", "json")
	}
	
	// v2-specific: Add debug mode if configured
	if e.config.Logging.Level == "debug" {
		args = append(args, "--debug")
	}
	
	return args
}

// Execute executes a nerdctl v2.x command and returns the result
func (e *V2CommandExecutor) Execute(ctx context.Context, args []string) (*CommandResult, error) {
	return e.ExecuteWithInput(ctx, args, nil)
}

// ExecuteWithInput executes a nerdctl v2.x command with input and returns the result
func (e *V2CommandExecutor) ExecuteWithInput(ctx context.Context, args []string, input io.Reader) (*CommandResult, error) {
	start := time.Now()
	
	// Add JSON output for compatible commands in v2.x
	enhancedArgs := e.enhanceArgsForV2(args)
	
	// Combine global args with command args
	fullArgs := append(e.globalArgs, enhancedArgs...)
	
	// Log the command being executed
	e.logger.WithFields(logrus.Fields{
		"command": e.binaryPath,
		"args":    strings.Join(fullArgs, " "),
	}).Debug("Executing nerdctl v2.x command")
	
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
			}).Error("V2 command execution failed")
			return nil, fmt.Errorf("v2 command execution failed: %w", err)
		}
	}
	
	// Parse v2-specific metadata from output
	result := &CommandResult{
		ExitCode: exitCode,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: duration,
		Metadata: e.parseV2Metadata(stdout.String(), stderr.String()),
	}
	
	// Log execution result with v2 enhancements
	e.logger.WithFields(logrus.Fields{
		"command":   e.binaryPath,
		"args":      strings.Join(fullArgs, " "),
		"exit_code": exitCode,
		"duration":  duration,
		"stdout_size": len(result.Stdout),
		"stderr_size": len(result.Stderr),
		"has_metadata": len(result.Metadata) > 0,
	}).Debug("V2 command execution completed")
	
	return result, nil
}

// enhanceArgsForV2 adds v2-specific arguments to improve output quality
func (e *V2CommandExecutor) enhanceArgsForV2(args []string) []string {
	if len(args) == 0 {
		return args
	}
	
	command := args[0]
	enhancedArgs := make([]string, len(args))
	copy(enhancedArgs, args)
	
	// Add JSON output for list/inspect commands in v2.x
	if e.enableJSONOutput && e.shouldUseJSONOutput(command) {
		// Check if --format or -f is already specified
		hasFormat := false
		for i, arg := range args {
			if arg == "--format" || arg == "-f" {
				hasFormat = true
				break
			}
			if strings.HasPrefix(arg, "--format=") || strings.HasPrefix(arg, "-f=") {
				hasFormat = true
				break
			}
		}
		
		// Add JSON format if not already specified
		if !hasFormat {
			enhancedArgs = append(enhancedArgs, "--format", "json")
		}
	}
	
	return enhancedArgs
}

// shouldUseJSONOutput determines if JSON output should be used for a command
func (e *V2CommandExecutor) shouldUseJSONOutput(command string) bool {
	jsonCompatibleCommands := []string{
		"ps", "images", "volume", "network", "inspect",
		"container", "image", "system", "info", "version",
	}
	
	for _, cmd := range jsonCompatibleCommands {
		if command == cmd {
			return true
		}
	}
	
	return false
}

// parseV2Metadata extracts v2-specific metadata from command output
func (e *V2CommandExecutor) parseV2Metadata(stdout, stderr string) map[string]interface{} {
	metadata := make(map[string]interface{})
	
	// Parse structured logs from stderr if enabled
	if e.enableStructuredLogs {
		lines := strings.Split(stderr, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			
			// Try to parse as JSON log entry
			var logEntry map[string]interface{}
			if err := json.Unmarshal([]byte(line), &logEntry); err == nil {
				if level, ok := logEntry["level"]; ok {
					if metadata["log_levels"] == nil {
						metadata["log_levels"] = make(map[string]int)
					}
					levels := metadata["log_levels"].(map[string]int)
					levels[fmt.Sprintf("%v", level)]++
				}
			}
		}
	}
	
	// Parse JSON output for metadata extraction
	if e.enableJSONOutput && stdout != "" {
		// Try to parse stdout as JSON to extract metadata
		var jsonOutput interface{}
		if err := json.Unmarshal([]byte(stdout), &jsonOutput); err == nil {
			metadata["output_type"] = "json"
			metadata["parsed_successfully"] = true
		} else {
			metadata["output_type"] = "text"
			metadata["parsed_successfully"] = false
		}
	}
	
	return metadata
}

// ExecuteStream executes a command and streams the output with v2 enhancements
func (e *V2CommandExecutor) ExecuteStream(ctx context.Context, args []string) (*StreamResult, error) {
	// Create cancellable context
	ctx, cancel := context.WithCancel(ctx)
	
	// Add v2 enhancements
	enhancedArgs := e.enhanceArgsForV2(args)
	fullArgs := append(e.globalArgs, enhancedArgs...)
	
	e.logger.WithFields(logrus.Fields{
		"command": e.binaryPath,
		"args":    strings.Join(fullArgs, " "),
	}).Debug("Starting v2 streaming command execution")
	
	// Create command
	cmd := exec.CommandContext(ctx, e.binaryPath, fullArgs...)
	
	// Get stdout pipe
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	
	// Get stderr pipe for metadata
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}
	
	// Start command
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to start v2 streaming command: %w", err)
	}
	
	// Create channels
	linesChan := make(chan string, 100)
	errorsChan := make(chan error, 10)
	metadataChan := make(chan map[string]interface{}, 10)
	
	// Start goroutine to read stdout
	go func() {
		defer close(linesChan)
		
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			select {
			case linesChan <- scanner.Text():
			case <-ctx.Done():
				return
			}
		}
		
		if err := scanner.Err(); err != nil {
			select {
			case errorsChan <- fmt.Errorf("stdout scan error: %w", err):
			case <-ctx.Done():
			}
		}
	}()
	
	// Start goroutine to read stderr and extract v2 metadata
	go func() {
		defer close(metadataChan)
		
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			
			// Try to parse as JSON metadata in v2
			if e.enableStructuredLogs {
				var logEntry map[string]interface{}
				if err := json.Unmarshal([]byte(line), &logEntry); err == nil {
					select {
					case metadataChan <- logEntry:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()
	
	// Start goroutine to wait for command completion
	go func() {
		defer close(errorsChan)
		
		if err := cmd.Wait(); err != nil {
			select {
			case errorsChan <- fmt.Errorf("v2 streaming command failed: %w", err):
			case <-ctx.Done():
			}
		}
		
		e.logger.Debug("V2 streaming command completed")
	}()
	
	return &StreamResult{
		LinesChan:    linesChan,
		ErrorsChan:   errorsChan,
		MetadataChan: metadataChan,
		Cancel:       cancel,
	}, nil
}

// ParseJSONOutput parses JSON output with v2-specific enhancements
func (e *V2CommandExecutor) ParseJSONOutput(output string, target interface{}) error {
	if output == "" {
		return fmt.Errorf("empty output cannot be parsed as JSON")
	}
	
	// Clean output for v2 parsing
	output = e.cleanV2JSONOutput(output)
	
	err := json.Unmarshal([]byte(output), target)
	if err != nil {
		e.logger.WithFields(logrus.Fields{
			"output_preview": e.truncateString(output, 200),
			"error":          err,
		}).Debug("Failed to parse v2 JSON output")
		return fmt.Errorf("failed to parse v2 JSON output: %w", err)
	}
	
	return nil
}

// cleanV2JSONOutput cleans the JSON output for v2-specific parsing
func (e *V2CommandExecutor) cleanV2JSONOutput(output string) string {
	lines := strings.Split(output, "\n")
	var jsonLines []string
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		// Skip non-JSON lines (like log messages)
		if strings.HasPrefix(line, "{") || strings.HasPrefix(line, "[") {
			jsonLines = append(jsonLines, line)
		}
	}
	
	return strings.Join(jsonLines, "\n")
}

// truncateString truncates a string to the specified length
func (e *V2CommandExecutor) truncateString(s string, length int) string {
	if len(s) <= length {
		return s
	}
	return s[:length] + "..."
}

// GetVersionInfo returns the version information
func (e *V2CommandExecutor) GetVersionInfo() *version.NerdctlVersionInfo {
	return e.versionInfo
}

// IsV2Feature checks if a feature is available in this v2 version
func (e *V2CommandExecutor) IsV2Feature(feature string) bool {
	v2Features := map[string]bool{
		"enhanced_json":           true,
		"structured_logs":         e.enableStructuredLogs,
		"metrics":                 e.enableMetrics,
		"improved_error_handling": true,
		"metadata_streaming":      true,
	}
	
	return v2Features[feature]
}