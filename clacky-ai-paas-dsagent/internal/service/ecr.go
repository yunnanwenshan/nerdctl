package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	pb "dsagent/api/image/v1"
)

// ECRAuthService handles ECR authentication logic
type ECRAuthService struct {
}

// NewECRAuthService creates a new ECR authentication service
func NewECRAuthService() *ECRAuthService {
	return &ECRAuthService{}
}

// CreateAWSConfig creates an AWS configuration from the provided credentials
func (s *ECRAuthService) CreateAWSConfig(ctx context.Context, authConfig *pb.ECRAuthConfig) (aws.Config, error) {
	var cfg aws.Config
	var err error

	region := authConfig.Region
	if region == "" {
		region = "us-east-1" // Default region
	}

	awsCreds := authConfig.AwsCredentials
	if awsCreds == nil {
		// Use default credential chain
		cfg, err = config.LoadDefaultConfig(ctx, config.WithRegion(region))
		if err != nil {
			return cfg, fmt.Errorf("failed to load default AWS config: %w", err)
		}
		return cfg, nil
	}

	switch cred := awsCreds.CredentialType.(type) {
	case *pb.AWSCredentials_AccessKey:
		// Static credentials
		staticCreds := credentials.NewStaticCredentialsProvider(
			cred.AccessKey.AccessKeyId,
			cred.AccessKey.SecretAccessKey,
			cred.AccessKey.SessionToken,
		)
		cfg, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(region),
			config.WithCredentialsProvider(staticCreds),
		)

	case *pb.AWSCredentials_InstanceProfile:
		// EC2 Instance Profile
		cfg, err = config.LoadDefaultConfig(ctx, config.WithRegion(region))

	case *pb.AWSCredentials_EcsTaskRole:
		// ECS Task Role
		cfg, err = config.LoadDefaultConfig(ctx, config.WithRegion(region))

	case *pb.AWSCredentials_AssumeRole:
		// STS Assume Role
		// First create config with source credentials
		sourceCfg, err := s.CreateAWSConfig(ctx, &pb.ECRAuthConfig{
			Region:         region,
			AwsCredentials: cred.AssumeRole.SourceCredentials,
		})
		if err != nil {
			return cfg, fmt.Errorf("failed to create source credentials config: %w", err)
		}

		// Create STS client to assume role
		stsClient := sts.NewFromConfig(sourceCfg)
		assumeRoleInput := &sts.AssumeRoleInput{
			RoleArn:         aws.String(cred.AssumeRole.RoleArn),
			RoleSessionName: aws.String(cred.AssumeRole.SessionName),
		}
		
		if cred.AssumeRole.ExternalId != "" {
			assumeRoleInput.ExternalId = aws.String(cred.AssumeRole.ExternalId)
		}
		
		if cred.AssumeRole.DurationSeconds > 0 {
			assumeRoleInput.DurationSeconds = aws.Int32(cred.AssumeRole.DurationSeconds)
		}

		assumeRoleOutput, err := stsClient.AssumeRole(ctx, assumeRoleInput)
		if err != nil {
			return cfg, fmt.Errorf("failed to assume role: %w", err)
		}

		// Create config with assumed role credentials
		assumedCreds := credentials.NewStaticCredentialsProvider(
			*assumeRoleOutput.Credentials.AccessKeyId,
			*assumeRoleOutput.Credentials.SecretAccessKey,
			*assumeRoleOutput.Credentials.SessionToken,
		)
		cfg, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(region),
			config.WithCredentialsProvider(assumedCreds),
		)

	default:
		return cfg, fmt.Errorf("unsupported credential type")
	}

	if err != nil {
		return cfg, fmt.Errorf("failed to create AWS config: %w", err)
	}

	return cfg, nil
}

// GetECRAuthorizationToken gets an ECR authorization token
func (s *ECRAuthService) GetECRAuthorizationToken(ctx context.Context, authConfig *pb.ECRAuthConfig) (*pb.ECRLoginResponse, error) {
	// Create AWS config
	cfg, err := s.CreateAWSConfig(ctx, authConfig)
	if err != nil {
		return nil, err
	}

	// Create ECR client
	ecrClient := ecr.NewFromConfig(cfg)

	// Get authorization token
	input := &ecr.GetAuthorizationTokenInput{}
	if authConfig.RegistryId != "" {
		input.RegistryIds = []string{authConfig.RegistryId}
	}

	result, err := ecrClient.GetAuthorizationToken(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get ECR authorization token: %w", err)
	}

	if len(result.AuthorizationData) == 0 {
		return nil, fmt.Errorf("no authorization data received from ECR")
	}

	authData := result.AuthorizationData[0]
	if authData.AuthorizationToken == nil || authData.ProxyEndpoint == nil {
		return nil, fmt.Errorf("invalid authorization data received from ECR")
	}

	// Decode the authorization token
	tokenBytes, err := base64.StdEncoding.DecodeString(*authData.AuthorizationToken)
	if err != nil {
		return nil, fmt.Errorf("failed to decode authorization token: %w", err)
	}

	// The token is in format "username:password"
	tokenStr := string(tokenBytes)
	parts := strings.SplitN(tokenStr, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid token format")
	}

	username := parts[0]
	token := parts[1]
	registryUrl := *authData.ProxyEndpoint

	// Calculate expiration timestamp
	var expiresAt int64
	if authData.ExpiresAt != nil {
		expiresAt = authData.ExpiresAt.Unix()
	}

	return &pb.ECRLoginResponse{
		Status:      "Login successful",
		RegistryUrl: registryUrl,
		Username:    username,
		Token:       token,
		ExpiresAt:   expiresAt,
	}, nil
}

// ValidateECRConfig validates the ECR configuration
func (s *ECRAuthService) ValidateECRConfig(authConfig *pb.ECRAuthConfig) error {
	if authConfig == nil {
		return fmt.Errorf("ECR auth config is required")
	}

	if authConfig.Region == "" {
		return fmt.Errorf("AWS region is required")
	}

	awsCreds := authConfig.AwsCredentials
	if awsCreds == nil {
		// Using default credentials is ok
		return nil
	}

	switch cred := awsCreds.CredentialType.(type) {
	case *pb.AWSCredentials_AccessKey:
		if cred.AccessKey.AccessKeyId == "" || cred.AccessKey.SecretAccessKey == "" {
			return fmt.Errorf("access key ID and secret access key are required")
		}
	case *pb.AWSCredentials_AssumeRole:
		if cred.AssumeRole.RoleArn == "" || cred.AssumeRole.SessionName == "" {
			return fmt.Errorf("role ARN and session name are required for assume role")
		}
		if cred.AssumeRole.SourceCredentials == nil {
			return fmt.Errorf("source credentials are required for assume role")
		}
		// Recursively validate source credentials
		return s.ValidateECRConfig(&pb.ECRAuthConfig{
			Region:         authConfig.Region,
			AwsCredentials: cred.AssumeRole.SourceCredentials,
		})
	case *pb.AWSCredentials_InstanceProfile:
		// Instance profile doesn't need additional validation
		return nil
	case *pb.AWSCredentials_EcsTaskRole:
		// ECS task role doesn't need additional validation
		return nil
	default:
		return fmt.Errorf("unsupported credential type")
	}

	return nil
}

// IsECRRegistry checks if the given registry URL is an ECR registry
func (s *ECRAuthService) IsECRRegistry(registryUrl string) bool {
	// ECR registry URLs follow the pattern: ACCOUNT_ID.dkr.ecr.REGION.amazonaws.com
	return strings.Contains(registryUrl, ".dkr.ecr.") && strings.Contains(registryUrl, ".amazonaws.com")
}

// ExtractECRRegion extracts the AWS region from an ECR registry URL
func (s *ECRAuthService) ExtractECRRegion(registryUrl string) string {
	// ECR registry URLs: ACCOUNT_ID.dkr.ecr.REGION.amazonaws.com
	parts := strings.Split(registryUrl, ".")
	if len(parts) >= 4 {
		for i, part := range parts {
			if part == "ecr" && i+1 < len(parts) {
				return parts[i+1]
			}
		}
	}
	return ""
}

// ExtractAccountIDFromRegistry extracts the AWS account ID from an ECR registry URL
func (s *ECRAuthService) ExtractAccountIDFromRegistry(registryUrl string) string {
	// ECR registry URLs: ACCOUNT_ID.dkr.ecr.REGION.amazonaws.com
	parts := strings.Split(registryUrl, ".")
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}