package service

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/containerd/log"
)

// NydusifyExecutor handles the execution of nydusify commands
type NydusifyExecutor struct {
	// Add monitoring capabilities in the future
}

// NewNydusifyExecutor creates a new nydusify executor
func NewNydusifyExecutor() *NydusifyExecutor {
	return &NydusifyExecutor{}
}

// ExecuteCommit executes a nydusify commit command
func (e *NydusifyExecutor) ExecuteCommit(ctx context.Context, containerID, targetRepo string) (*TaskResult, error) {
	log.L.Infof("Executing nydusify commit: container=%s, target=%s", containerID, targetRepo)

	startTime := time.Now()

	// Build command
	cmd := exec.CommandContext(ctx, "nydusify", "commit",
		"--container", containerID,
		"--target", targetRepo,
		"--output-json") // Request JSON output for easier parsing

	// Create pipes for stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start command
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start nydusify command: %w", err)
	}

	// Collect output
	var stdoutBuf, stderrBuf strings.Builder
	
	// Create goroutines to read output
	done := make(chan error, 1)
	
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.L.Errorf("Panic in stdout reader: %v", r)
			}
		}()
		
		if err := e.readOutput(stdout, &stdoutBuf, "stdout"); err != nil {
			log.L.Warnf("Error reading stdout: %v", err)
		}
	}()
	
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.L.Errorf("Panic in stderr reader: %v", r)
			}
		}()
		
		if err := e.readOutput(stderr, &stderrBuf, "stderr"); err != nil {
			log.L.Warnf("Error reading stderr: %v", err)
		}
	}()

	// Wait for command to complete
	go func() {
		done <- cmd.Wait()
	}()

	// Wait for completion or context cancellation
	select {
	case err := <-done:
		// Command completed
		duration := time.Since(startTime)
		
		result := &TaskResult{
			ExitCode:    0,
			Stdout:      stdoutBuf.String(),
			Stderr:      stderrBuf.String(),
			TargetURL:   targetRepo,
		}

		if err != nil {
			if exitError, ok := err.(*exec.ExitError); ok {
				result.ExitCode = exitError.ExitCode()
			} else {
				result.ExitCode = 1
			}
		}

		// Parse output for additional information
		e.parseOutput(result)

		// Add execution metrics (basic implementation)
		result.Metrics = &ExecutionMetrics{
			// TODO: Implement actual resource monitoring
			CPUUsage:    0.0,  // Placeholder
			MemoryUsage: 0,    // Placeholder
		}

		log.L.Infof("nydusify command completed in %v with exit code %d", duration, result.ExitCode)

		if result.ExitCode != 0 {
			return result, fmt.Errorf("nydusify command failed with exit code %d", result.ExitCode)
		}

		return result, nil

	case <-ctx.Done():
		// Context cancelled - terminate command
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		return nil, fmt.Errorf("nydusify command cancelled: %w", ctx.Err())
	}
}

// readOutput reads output from a pipe and writes to buffer
func (e *NydusifyExecutor) readOutput(reader io.Reader, buffer *strings.Builder, streamName string) error {
	scanner := bufio.NewScanner(reader)
	
	for scanner.Scan() {
		line := scanner.Text()
		buffer.WriteString(line)
		buffer.WriteString("\n")
		log.L.Debugf("nydusify %s: %s", streamName, line)
	}
	
	return scanner.Err()
}

// parseOutput parses nydusify output for useful information
func (e *NydusifyExecutor) parseOutput(result *TaskResult) {
	// Try to extract information from stdout
	e.parseStdout(result)
	
	// Also check stderr for any useful information
	e.parseStderr(result)
}

// parseStdout parses stdout for useful information
func (e *NydusifyExecutor) parseStdout(result *TaskResult) {
	lines := strings.Split(result.Stdout, "\n")
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// Look for image digest
		if digest := e.extractImageDigest(line); digest != "" {
			result.ImageDigest = digest
		}
		
		// Look for size information
		if sizeInfo := e.extractSizeInfo(line); sizeInfo != nil {
			result.SizeInfo = sizeInfo
		}
	}
}

// parseStderr parses stderr for useful information or errors
func (e *NydusifyExecutor) parseStderr(result *TaskResult) {
	// stderr might contain progress information or warnings
	// For now, just log it
	if result.Stderr != "" {
		log.L.Debugf("nydusify stderr: %s", result.Stderr)
	}
}

// extractImageDigest extracts image digest from output line
func (e *NydusifyExecutor) extractImageDigest(line string) string {
	// Look for patterns like "sha256:abcd1234..."
	digestRegex := regexp.MustCompile(`sha256:[a-f0-9]{64}`)
	matches := digestRegex.FindStringSubmatch(line)
	
	if len(matches) > 0 {
		return matches[0]
	}
	
	// Also look for patterns like "digest: sha256:..."
	if strings.Contains(strings.ToLower(line), "digest") {
		parts := strings.Split(line, ":")
		if len(parts) >= 3 {
			digest := strings.TrimSpace(parts[1] + ":" + parts[2])
			if digestRegex.MatchString(digest) {
				return digest
			}
		}
	}
	
	return ""
}

// extractSizeInfo extracts size information from output line
func (e *NydusifyExecutor) extractSizeInfo(line string) *ImageSizeInfo {
	// Look for size information patterns
	// Examples: "Original size: 100MB", "Compressed size: 50MB", "Compression ratio: 50%"
	
	line = strings.ToLower(line)
	
	sizeInfo := &ImageSizeInfo{}
	updated := false
	
	// Extract original size
	if originalSize := e.extractSize(line, []string{"original size", "source size", "input size"}); originalSize > 0 {
		sizeInfo.OriginalSize = originalSize
		updated = true
	}
	
	// Extract nydus size
	if nydusSize := e.extractSize(line, []string{"compressed size", "output size", "nydus size"}); nydusSize > 0 {
		sizeInfo.NydusSize = nydusSize
		updated = true
	}
	
	// Extract compression ratio
	if ratio := e.extractPercentage(line, []string{"compression ratio", "reduction ratio"}); ratio > 0 {
		sizeInfo.CompressionRatio = ratio
		updated = true
	}
	
	if updated {
		// Calculate size reduction if we have both sizes
		if sizeInfo.OriginalSize > 0 && sizeInfo.NydusSize > 0 {
			sizeInfo.SizeReduction = sizeInfo.OriginalSize - sizeInfo.NydusSize
			if sizeInfo.CompressionRatio == 0 {
				sizeInfo.CompressionRatio = float32(sizeInfo.SizeReduction) / float32(sizeInfo.OriginalSize) * 100
			}
		}
		return sizeInfo
	}
	
	return nil
}

// extractSize extracts size in bytes from a line
func (e *NydusifyExecutor) extractSize(line string, keywords []string) int64 {
	for _, keyword := range keywords {
		if strings.Contains(line, keyword) {
			// Look for size after the keyword
			parts := strings.Split(line, keyword)
			if len(parts) > 1 {
				sizeStr := strings.TrimSpace(parts[1])
				if size := e.parseSize(sizeStr); size > 0 {
					return size
				}
			}
		}
	}
	return 0
}

// parseSize parses size string like "100MB", "1.5GB", etc. and returns bytes
func (e *NydusifyExecutor) parseSize(sizeStr string) int64 {
	// Remove common prefixes like ":", "="
	sizeStr = strings.TrimLeft(sizeStr, ":= ")
	
	// Extract number and unit
	sizeRegex := regexp.MustCompile(`^(\d+(?:\.\d+)?)\s*([KMGT]?B?)`)
	matches := sizeRegex.FindStringSubmatch(sizeStr)
	
	if len(matches) < 2 {
		return 0
	}
	
	value, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0
	}
	
	unit := strings.ToUpper(matches[2])
	switch unit {
	case "B", "":
		return int64(value)
	case "KB", "K":
		return int64(value * 1024)
	case "MB", "M":
		return int64(value * 1024 * 1024)
	case "GB", "G":
		return int64(value * 1024 * 1024 * 1024)
	case "TB", "T":
		return int64(value * 1024 * 1024 * 1024 * 1024)
	default:
		return int64(value) // Assume bytes
	}
}

// extractPercentage extracts percentage from a line
func (e *NydusifyExecutor) extractPercentage(line string, keywords []string) float32 {
	for _, keyword := range keywords {
		if strings.Contains(line, keyword) {
			// Look for percentage after the keyword
			parts := strings.Split(line, keyword)
			if len(parts) > 1 {
				percentStr := strings.TrimSpace(parts[1])
				if percent := e.parsePercentage(percentStr); percent >= 0 {
					return percent
				}
			}
		}
	}
	return 0
}

// parsePercentage parses percentage string like "50%", "75.5%" and returns float32
func (e *NydusifyExecutor) parsePercentage(percentStr string) float32 {
	// Remove common prefixes like ":", "="
	percentStr = strings.TrimLeft(percentStr, ":= ")
	
	// Extract number before %
	percentRegex := regexp.MustCompile(`^(\d+(?:\.\d+)?)\s*%`)
	matches := percentRegex.FindStringSubmatch(percentStr)
	
	if len(matches) < 2 {
		return -1
	}
	
	value, err := strconv.ParseFloat(matches[1], 32)
	if err != nil {
		return -1
	}
	
	return float32(value)
}