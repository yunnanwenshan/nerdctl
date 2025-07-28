package version

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// NerdctlVersionInfo contains version information about nerdctl and its components
type NerdctlVersionInfo struct {
	Client     VersionInfo            `json:"client"`
	Components map[string]VersionInfo `json:"components"`
}

// VersionInfo represents version information for a component
type VersionInfo struct {
	Version string `json:"version"`
	Raw     string `json:"raw"`
	GitCommit string `json:"git_commit,omitempty"`
}

// VersionRange represents a version range for compatibility checking
type VersionRange struct {
	MinVersion string
	MaxVersion string
}

// Detector detects nerdctl version and component information
type Detector struct {
	binaryPath string
	timeout    time.Duration
	logger     *logrus.Logger
}

// NewDetector creates a new version detector
func NewDetector(binaryPath string) *Detector {
	return &Detector{
		binaryPath: binaryPath,
		timeout:    10 * time.Second,
		logger:     logrus.New(),
	}
}

// DetectVersion detects the nerdctl version and component versions
func (d *Detector) DetectVersion(ctx context.Context) (*NerdctlVersionInfo, error) {
	d.logger.WithField("binary_path", d.binaryPath).Debug("Detecting nerdctl version")
	
	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()
	
	// Execute nerdctl version command
	cmd := exec.CommandContext(timeoutCtx, d.binaryPath, "version")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute nerdctl version: %w", err)
	}
	
	// Parse version output
	versionInfo, err := d.parseVersionOutput(string(output))
	if err != nil {
		return nil, fmt.Errorf("failed to parse version output: %w", err)
	}
	
	d.logger.WithFields(logrus.Fields{
		"nerdctl_version":    versionInfo.Client.Version,
		"containerd_version": versionInfo.Components["containerd"].Version,
	}).Info("Successfully detected nerdctl version")
	
	return versionInfo, nil
}

// parseVersionOutput parses the output of 'nerdctl version' command
func (d *Detector) parseVersionOutput(output string) (*NerdctlVersionInfo, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 0 {
		return nil, fmt.Errorf("empty version output")
	}
	
	versionInfo := &NerdctlVersionInfo{
		Components: make(map[string]VersionInfo),
	}
	
	// Regular expressions for parsing version information
	clientVersionRegex := regexp.MustCompile(`Client:\s*Version:\s*v?([^\s]+)`)
	componentRegex := regexp.MustCompile(`^\s*([^:]+):\s*v?([^\s]+)`)
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		// Parse client version
		if matches := clientVersionRegex.FindStringSubmatch(line); len(matches) > 1 {
			versionInfo.Client = VersionInfo{
				Version: matches[1],
				Raw:     matches[1],
			}
			continue
		}
		
		// Parse component versions
		if matches := componentRegex.FindStringSubmatch(line); len(matches) > 2 {
			componentName := strings.ToLower(strings.TrimSpace(matches[1]))
			componentVersion := strings.TrimSpace(matches[2])
			
			versionInfo.Components[componentName] = VersionInfo{
				Version: componentVersion,
				Raw:     componentVersion,
			}
		}
	}
	
	// If we couldn't parse the client version from the structured output,
	// try to extract it from the first line
	if versionInfo.Client.Version == "" {
		if len(lines) > 0 {
			firstLine := lines[0]
			// Try various patterns for version extraction
			patterns := []string{
				`nerdctl version ([^\s,]+)`,
				`version ([^\s,]+)`,
				`v?([0-9]+\.[0-9]+\.[0-9]+[^\s]*)`,
			}
			
			for _, pattern := range patterns {
				regex := regexp.MustCompile(pattern)
				if matches := regex.FindStringSubmatch(firstLine); len(matches) > 1 {
					versionInfo.Client = VersionInfo{
						Version: matches[1],
						Raw:     matches[1],
					}
					break
				}
			}
		}
	}
	
	// Ensure we have at least a client version
	if versionInfo.Client.Version == "" {
		return nil, fmt.Errorf("could not extract nerdctl client version from output")
	}
	
	// Set default component versions if not detected
	if _, exists := versionInfo.Components["containerd"]; !exists {
		versionInfo.Components["containerd"] = VersionInfo{
			Version: "unknown",
			Raw:     "unknown",
		}
	}
	
	if _, exists := versionInfo.Components["runc"]; !exists {
		versionInfo.Components["runc"] = VersionInfo{
			Version: "unknown", 
			Raw:     "unknown",
		}
	}
	
	return versionInfo, nil
}

// GetMajorMinorVersion extracts the major.minor version from a version string
func GetMajorMinorVersion(version string) string {
	// Remove 'v' prefix if present
	version = strings.TrimPrefix(version, "v")
	
	// Extract major.minor version using regex
	regex := regexp.MustCompile(`^(\d+\.\d+)`)
	matches := regex.FindStringSubmatch(version)
	if len(matches) > 1 {
		return matches[1]
	}
	
	return version
}

// IsVersionInRange checks if a version falls within the specified range
func IsVersionInRange(version string, versionRange VersionRange) bool {
	// This is a simplified version comparison
	// In a production system, you might want to use a proper semantic versioning library
	
	version = strings.TrimPrefix(version, "v")
	minVersion := strings.TrimPrefix(versionRange.MinVersion, "v")
	maxVersion := strings.TrimPrefix(versionRange.MaxVersion, "v")
	
	// Simple string comparison for now
	// This works for versions like "1.7.0", "2.0.0" etc.
	return version >= minVersion && version <= maxVersion
}