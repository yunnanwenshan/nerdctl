package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/containerd/nerdctl/v2/pkg/api/types"
	loginCmd "github.com/containerd/nerdctl/v2/pkg/cmd/login"
	pb "dsagent/api/image/v1"
)

// DockerConfigManager manages Docker registry authentication
type DockerConfigManager struct {
	ecrAuthService *ECRAuthService
}

// NewDockerConfigManager creates a new Docker config manager
func NewDockerConfigManager() *DockerConfigManager {
	return &DockerConfigManager{
		ecrAuthService: NewECRAuthService(),
	}
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

// LoginToECR performs ECR login and stores credentials
func (dcm *DockerConfigManager) LoginToECR(ctx context.Context, authConfig *pb.ECRAuthConfig, globalOpts *types.GlobalCommandOptions) error {
	// Validate ECR config
	if err := dcm.ecrAuthService.ValidateECRConfig(authConfig); err != nil {
		return fmt.Errorf("invalid ECR config: %w", err)
	}

	// Get ECR authorization token
	loginResp, err := dcm.ecrAuthService.GetECRAuthorizationToken(ctx, authConfig)
	if err != nil {
		return fmt.Errorf("failed to get ECR token: %w", err)
	}

	// Note: nerdctl login doesn't need containerd client directly

	// Use nerdctl's login functionality
	loginOpts := types.LoginCommandOptions{
		GOptions: *globalOpts,
		Username: loginResp.Username,
		Password: loginResp.Token,
		ServerAddress: loginResp.RegistryUrl,
	}

	// Create stdout buffer for login output
	var loginOutput bytes.Buffer

	// Perform the login
	if err := loginCmd.Login(ctx, loginOpts, &loginOutput); err != nil {
		return fmt.Errorf("failed to login to ECR via nerdctl: %w", err)
	}

	return nil
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

// AutoDetectECRAndLogin automatically detects if an image is from ECR and performs login
func (dcm *DockerConfigManager) AutoDetectECRAndLogin(ctx context.Context, imageName string, ecrAuth *pb.ECRAuthConfig, globalOpts *types.GlobalCommandOptions) error {
	if ecrAuth == nil {
		return nil // No ECR config provided
	}

	// Extract registry from image name
	registryUrl := dcm.ExtractRegistryFromImageName(imageName)
	if registryUrl == "" {
		return nil // Not a fully qualified image name
	}

	// Check if it's an ECR registry
	if !dcm.ecrAuthService.IsECRRegistry(registryUrl) {
		return nil // Not an ECR registry
	}

	// Check if already logged in
	loggedIn, err := dcm.IsLoggedInToRegistry(registryUrl)
	if err != nil {
		return fmt.Errorf("failed to check login status: %w", err)
	}

	if loggedIn && !ecrAuth.AutoRefresh {
		return nil // Already logged in and no auto-refresh requested
	}

	// Auto-detect region if not provided
	if ecrAuth.Region == "" {
		ecrAuth.Region = dcm.ecrAuthService.ExtractECRRegion(registryUrl)
	}

	// Auto-detect registry ID if not provided
	if ecrAuth.RegistryId == "" {
		ecrAuth.RegistryId = dcm.ecrAuthService.ExtractAccountIDFromRegistry(registryUrl)
	}

	// Perform ECR login
	return dcm.LoginToECR(ctx, ecrAuth, globalOpts)
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

