package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/containerd/nerdctl/v2/pkg/api/types"
	pb "dsagent/api/image/v1"
)

// DockerConfigManager manages Docker registry authentication
type DockerConfigManager struct {
	// Removed ECRAuthService reference since ECR is now a separate service
}

// NewDockerConfigManager creates a new Docker config manager
func NewDockerConfigManager() *DockerConfigManager {
	return &DockerConfigManager{}
}

// DockerAuthConfig represents the Docker config.json structure
type DockerAuthConfig struct {
	Auths map[string]DockerAuthEntry `json:"auths"`
}

// DockerAuthEntry represents an auth entry in Docker config
type DockerAuthEntry struct {
	Auth     string `json:"auth,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// LoginToECR has been moved to the dedicated ECR service
// This method is deprecated and should not be used
func (dcm *DockerConfigManager) LoginToECR(ctx context.Context, authConfig *pb.ECRAuthConfig, globalOpts *types.GlobalCommandOptions) error {
	// ECR functionality has been moved to the dedicated ECR service
	// Use the ECRService for ECR-related operations
	return fmt.Errorf("ECR functionality has been moved to dedicated ECR service. Use ECRService.Login instead")
}

// GetDockerConfigPath returns the Docker config file path
func (dcm *DockerConfigManager) GetDockerConfigPath() string {
	dockerConfig := os.Getenv("DOCKER_CONFIG")
	if dockerConfig == "" {
		homeDir, _ := os.UserHomeDir()
		dockerConfig = filepath.Join(homeDir, ".docker")
	}
	return filepath.Join(dockerConfig, "config.json")
}

// IsLoggedInToRegistry checks if we're already logged in to a registry
func (dcm *DockerConfigManager) IsLoggedInToRegistry(registryUrl string) (bool, error) {
	configPath := dcm.GetDockerConfigPath()
	
	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return false, nil
	}

	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return false, fmt.Errorf("failed to read Docker config: %w", err)
	}

	var config DockerAuthConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return false, fmt.Errorf("failed to parse Docker config: %w", err)
	}

	// Check if registry exists in auths
	_, exists := config.Auths[registryUrl]
	return exists, nil
}

// RemoveRegistryAuth removes authentication for a specific registry
func (dcm *DockerConfigManager) RemoveRegistryAuth(registryUrl string) error {
	configPath := dcm.GetDockerConfigPath()
	
	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil // Nothing to remove
	}

	// Read current config
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read Docker config: %w", err)
	}

	var config DockerAuthConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse Docker config: %w", err)
	}

	// Remove the registry
	if config.Auths != nil {
		delete(config.Auths, registryUrl)
	}

	// Write back to file
	newData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal Docker config: %w", err)
	}

	if err := os.WriteFile(configPath, newData, 0600); err != nil {
		return fmt.Errorf("failed to write Docker config: %w", err)
	}

	return nil
}

// AutoDetectECRAndLogin has been moved to the dedicated ECR service
// This method is deprecated and should not be used
func (dcm *DockerConfigManager) AutoDetectECRAndLogin(ctx context.Context, imageName string, ecrAuth *pb.ECRAuthConfig, globalOpts *types.GlobalCommandOptions) error {
	// ECR functionality has been moved to the dedicated ECR service
	// Use the ECRService for ECR-related operations
	return fmt.Errorf("ECR functionality has been moved to dedicated ECR service. Use ECRService.Login instead")
}

// ExtractRegistryFromImageName extracts the registry URL from an image name
func (dcm *DockerConfigManager) ExtractRegistryFromImageName(imageName string) string {
	// Handle different image name formats:
	// 1. registry.com/namespace/image:tag
	// 2. registry.com:port/namespace/image:tag
	// 3. namespace/image:tag (Docker Hub)
	// 4. image:tag (Docker Hub)

	// Remove tag if present
	if colonIndex := strings.LastIndex(imageName, ":"); colonIndex != -1 {
		// Check if it's a tag (not a port)
		after := imageName[colonIndex+1:]
		if !strings.Contains(after, "/") {
			imageName = imageName[:colonIndex]
		}
	}

	// Split by '/' to get parts
	parts := strings.Split(imageName, "/")

	// If there are multiple parts, check if the first part looks like a registry
	if len(parts) > 1 {
		firstPart := parts[0]
		// Registry usually contains '.' or ':'
		if strings.Contains(firstPart, ".") || strings.Contains(firstPart, ":") {
			return firstPart
		}
	}

	return ""
}

