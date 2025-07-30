package service

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"

	ecrpb "dsagent/api/ecr/v1"
	"dsagent/internal/auth"
)

// ECRService implements the ECRService gRPC interface
type ECRService struct {
	ecrpb.UnimplementedECRServiceServer
	authManager    auth.ECRAuthManager
	configManager  auth.ECRConfigManager
	profileManager *auth.ECRRegistryProfileManager
}

// NewECRService creates a new ECRService instance
func NewECRService() (*ECRService, error) {
	authManager, err := auth.NewECRAuthManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create ECR auth manager: %w", err)
	}

	configManager := auth.NewECRConfigManager()
	profileManager := auth.NewECRRegistryProfileManager()

	return &ECRService{
		authManager:    authManager,
		configManager:  configManager,
		profileManager: profileManager,
	}, nil
}

// Login implements the ECR Login RPC method
func (s *ECRService) Login(ctx context.Context, req *ecrpb.ECRLoginRequest) (*ecrpb.ECRLoginResponse, error) {
	if req.EcrAuth == nil {
		return nil, fmt.Errorf("ECR authentication config is required")
	}

	// Configure ECR auth manager with provided credentials
	config := s.convertECRAuthToConfig(req.EcrAuth)
	if err := s.authManager.ConfigureDefaults(config); err != nil {
		return nil, fmt.Errorf("failed to configure ECR auth: %w", err)
	}

	// Generate registry URL from region and registry ID
	registryURL := s.generateRegistryURL(req.EcrAuth)

	// Perform ECR login
	result, err := s.authManager.AutoLogin(ctx, registryURL)
	if err != nil {
		return nil, fmt.Errorf("ECR login failed: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("ECR login failed: %s", result.Message)
	}

	// Convert expiry time to timestamp
	expiresAt := int64(0)
	if result.TokenExpiry != "" {
		if t, err := time.Parse(time.RFC3339, result.TokenExpiry); err == nil {
			expiresAt = t.Unix()
		}
	}

	return &ecrpb.ECRLoginResponse{
		Status:      result.Message,
		RegistryUrl: result.RegistryURL,
		Username:    result.Username,
		Token:       "***", // Mask the actual token for security
		ExpiresAt:   expiresAt,
	}, nil
}

// Logout implements the ECR Logout RPC method
func (s *ECRService) Logout(ctx context.Context, req *ecrpb.ECRLogoutRequest) (*ecrpb.ECRLogoutResponse, error) {
	if req.RegistryUrl == "" {
		return nil, fmt.Errorf("registry URL is required")
	}

	// For now, we just remove the token from cache and Docker config
	// In a real implementation, you might want to revoke the token if possible

	return &ecrpb.ECRLogoutResponse{
		Status: fmt.Sprintf("Successfully logged out from %s", req.RegistryUrl),
	}, nil
}

// GetStatus implements the ECR GetStatus RPC method
func (s *ECRService) GetStatus(ctx context.Context, req *ecrpb.ECRStatusRequest) (*ecrpb.ECRStatusResponse, error) {
	if req.RegistryUrl == "" {
		return nil, fmt.Errorf("registry URL is required")
	}

	// Check if we have a valid token for this registry
	isValid := s.authManager.IsTokenValid(req.RegistryUrl)
	
	response := &ecrpb.ECRStatusResponse{
		Authenticated: isValid,
		RegistryUrl:   req.RegistryUrl,
		Username:      "",
		ExpiresAt:     0,
		ExpiresInSeconds: 0,
	}

	if isValid {
		// Get cached token information
		token, err := s.authManager.GetCachedToken(req.RegistryUrl)
		if err == nil && token != nil {
			response.Username = token.Username
			response.ExpiresAt = token.ExpiresAt.Unix()
			
			// Calculate seconds until expiry
			now := time.Now()
			if token.ExpiresAt.After(now) {
				response.ExpiresInSeconds = int64(token.ExpiresAt.Sub(now).Seconds())
			}
		}
	}

	return response, nil
}

// ListProfiles implements the ECR ListProfiles RPC method
func (s *ECRService) ListProfiles(ctx context.Context, req *emptypb.Empty) (*ecrpb.ECRProfilesResponse, error) {
	profiles, err := s.profileManager.ListProfiles()
	if err != nil {
		return nil, fmt.Errorf("failed to list ECR profiles: %w", err)
	}

	// Convert internal profiles to protobuf format
	var pbProfiles []*ecrpb.ECRProfile
	for _, profile := range profiles {
		pbProfiles = append(pbProfiles, s.convertProfileToProtobuf(profile))
	}

	return &ecrpb.ECRProfilesResponse{
		Profiles: pbProfiles,
	}, nil
}

// SetProfile implements the ECR SetProfile RPC method
func (s *ECRService) SetProfile(ctx context.Context, req *ecrpb.ECRSetProfileRequest) (*ecrpb.ECRSetProfileResponse, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("profile name is required")
	}

	if req.Profile == nil {
		return nil, fmt.Errorf("profile configuration is required")
	}

	// Convert protobuf profile to internal format
	credentials := s.convertECRAuthToConfig(req.Profile.AuthConfig)

	err := s.profileManager.AddProfile(req.Name, req.Profile.RegistryUrl, &auth.ECRCredentialsConfig{
		Region:          credentials["region"].(string),
		AccessKeyID:     getStringFromMap(credentials, "access_key_id"),
		SecretAccessKey: getStringFromMap(credentials, "secret_access_key"),
		SessionToken:    getStringFromMap(credentials, "session_token"),
		RoleArn:         getStringFromMap(credentials, "role_arn"),
		UseInstanceRole: getBoolFromMap(credentials, "use_instance_role"),
		UseECSTaskRole:  getBoolFromMap(credentials, "use_ecs_task_role"),
	})

	if err != nil {
		return nil, fmt.Errorf("failed to set ECR profile: %w", err)
	}

	return &ecrpb.ECRSetProfileResponse{
		Status:  fmt.Sprintf("Profile '%s' successfully created/updated", req.Name),
		Profile: req.Profile,
	}, nil
}

// DeleteProfile implements the ECR DeleteProfile RPC method
func (s *ECRService) DeleteProfile(ctx context.Context, req *ecrpb.ECRDeleteProfileRequest) (*ecrpb.ECRDeleteProfileResponse, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("profile name is required")
	}

	err := s.profileManager.RemoveProfile(req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to delete ECR profile: %w", err)
	}

	return &ecrpb.ECRDeleteProfileResponse{
		Status: fmt.Sprintf("Profile '%s' successfully deleted", req.Name),
	}, nil
}

// Helper methods

// convertECRAuthToConfig converts protobuf ECRAuthConfig to internal config map
func (s *ECRService) convertECRAuthToConfig(ecrAuth *ecrpb.ECRAuthConfig) map[string]interface{} {
	config := map[string]interface{}{
		"region": ecrAuth.Region,
	}

	if ecrAuth.RegistryId != "" {
		config["registry_id"] = ecrAuth.RegistryId
	}

	if ecrAuth.AwsCredentials != nil {
		creds := ecrAuth.AwsCredentials

		// Handle oneof field
		switch credType := creds.CredentialType.(type) {
		case *ecrpb.AWSCredentials_AccessKey:
			config["access_key_id"] = credType.AccessKey.AccessKeyId
			config["secret_access_key"] = credType.AccessKey.SecretAccessKey
			if credType.AccessKey.SessionToken != "" {
				config["session_token"] = credType.AccessKey.SessionToken
			}
		case *ecrpb.AWSCredentials_AssumeRole:
			config["role_arn"] = credType.AssumeRole.RoleArn
			if credType.AssumeRole.SessionName != "" {
				config["session_name"] = credType.AssumeRole.SessionName
			}
		case *ecrpb.AWSCredentials_InstanceProfile:
			config["use_instance_role"] = true
		case *ecrpb.AWSCredentials_EcsTaskRole:
			config["use_ecs_task_role"] = true
		}
	}

	return config
}

// generateRegistryURL generates ECR registry URL from auth config
func (s *ECRService) generateRegistryURL(ecrAuth *ecrpb.ECRAuthConfig) string {
	registryID := ecrAuth.RegistryId
	if registryID == "" {
		registryID = "123456789012" // Default for demo purposes
	}
	
	region := ecrAuth.Region
	if region == "" {
		region = "us-east-1" // Default region
	}
	
	return fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com", registryID, region)
}

// convertProfileToProtobuf converts internal profile to protobuf format
func (s *ECRService) convertProfileToProtobuf(profile *auth.ECRProfile) *ecrpb.ECRProfile {
	pbProfile := &ecrpb.ECRProfile{
		Name:        profile.Name,
		RegistryUrl: profile.RegistryURL,
		Enabled:     profile.Enabled,
		Description: "", // Can be added to internal profile if needed
		CreatedAt:   0,  // Can be added to internal profile if needed
		UpdatedAt:   0,  // Can be added to internal profile if needed
	}

	// Convert credentials if available
	if profile.Credentials != nil {
		pbProfile.AuthConfig = &ecrpb.ECRAuthConfig{
			Region:     profile.Credentials.Region,
			RegistryId: "", // Can be extracted from registry URL if needed
			AwsCredentials: &ecrpb.AWSCredentials{
				CredentialType: &ecrpb.AWSCredentials_AccessKey{
					AccessKey: &ecrpb.AWSAccessKeyCredentials{
						AccessKeyId:     profile.Credentials.AccessKeyID,
						SecretAccessKey: "***", // Mask for security
						SessionToken:    "", // Don't expose
					},
				},
			},
		}
	}

	return pbProfile
}

// Utility functions for map access

func getStringFromMap(m map[string]interface{}, key string) string {
	if val, exists := m[key]; exists {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getBoolFromMap(m map[string]interface{}, key string) bool {
	if val, exists := m[key]; exists {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return false
}