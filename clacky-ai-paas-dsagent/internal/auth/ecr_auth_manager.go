package auth

import (
	"context"
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	ecrservice "github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// ECRAuthManager ECR认证管理器接口
type ECRAuthManager interface {
	// ECR自动登录
	AutoLogin(ctx context.Context, registryURL string) (*AuthResult, error)
	
	// 配置默认AWS凭证
	ConfigureDefaults(config map[string]interface{}) error
	
	// Token管理
	IsTokenValid(registryURL string) bool
	RefreshToken(ctx context.Context, registryURL string) error
	GetCachedToken(registryURL string) (*ECRToken, error)
	
	// 检测ECR registry
	IsECRRegistry(registryURL string) bool
	ParseECRInfo(registryURL string) (*ECRRegistryInfo, error)
}

// ECRRegistryInfo ECR仓库信息
type ECRRegistryInfo struct {
	AccountID string `json:"account_id"`
	Region    string `json:"region"`
	Domain    string `json:"domain"`
}

// ECRToken ECR认证token
type ECRToken struct {
	Username  string    `json:"username"`
	Password  string    `json:"password"`
	ExpiresAt time.Time `json:"expires_at"`
	Registry  string    `json:"registry"`
}

// ECRCredentialsConfig ECR凭证配置
type ECRCredentialsConfig struct {
	Region          string                 `json:"region"`
	AccessKeyID     string                 `json:"access_key_id"`
	SecretAccessKey string                 `json:"secret_access_key"`
	SessionToken    string                 `json:"session_token"`
	RoleArn         string                 `json:"role_arn"`
	UseInstanceRole bool                   `json:"use_instance_role"`
	UseECSTaskRole  bool                   `json:"use_ecs_task_role"`
	Extra           map[string]interface{} `json:"extra"`
}

// ecrAuthManager ECR认证管理器实现
type ecrAuthManager struct {
	mutex          sync.RWMutex
	tokenCache     map[string]*ECRToken
	defaultConfig  *ECRCredentialsConfig
	dockerManager  DockerConfigManager
	configManager  ECRConfigManager
	lastCacheSync  time.Time
}

// ECR registry URL正则表达式
var ecrRegex = regexp.MustCompile(`^(\d+)\.dkr\.ecr\.([a-z0-9-]+)\.amazonaws\.com`)

// NewECRAuthManager 创建ECR认证管理器
func NewECRAuthManager() (ECRAuthManager, error) {
	dockerManager, err := NewDockerConfigManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create docker config manager: %w", err)
	}

	configManager := NewECRConfigManager()
	
	manager := &ecrAuthManager{
		tokenCache:    make(map[string]*ECRToken),
		dockerManager: dockerManager,
		configManager: configManager,
		lastCacheSync: time.Now(),
	}
	
	// 尝试加载持久化的token缓存
	if err := manager.loadPersistedTokens(); err != nil {
		// 加载失败不是致命错误，记录日志即可
		// 这里可以添加日志记录
	}
	
	return manager, nil
}

// AutoLogin ECR自动登录
func (ecr *ecrAuthManager) AutoLogin(ctx context.Context, registryURL string) (*AuthResult, error) {
	if !ecr.IsECRRegistry(registryURL) {
		return nil, fmt.Errorf("not an ECR registry: %s", registryURL)
	}

	ecrInfo, err := ecr.ParseECRInfo(registryURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ECR info: %w", err)
	}

	// 检查token缓存
	if token := ecr.GetCachedTokenUnsafe(registryURL); token != nil {
		if ecr.IsTokenValidUnsafe(registryURL) {
			return &AuthResult{
				Success:     true,
				RegistryURL: registryURL,
				Username:    token.Username,
				Message:     "Using cached ECR token",
				TokenExpiry: token.ExpiresAt.Format(time.RFC3339),
			}, nil
		}
	}

	// 获取新的ECR token
	token, err := ecr.getECRToken(ctx, ecrInfo)
	if err != nil {
		return &AuthResult{
			Success:     false,
			RegistryURL: registryURL,
			Message:     fmt.Sprintf("Failed to get ECR token: %v", err),
		}, err
	}

	// 缓存token
	ecr.mutex.Lock()
	ecr.tokenCache[registryURL] = token
	ecr.mutex.Unlock()
	
	// 异步保存到持久化缓存
	go ecr.saveTokenCache()

	// 配置Docker认证
	err = ecr.dockerManager.SetRegistryAuth(registryURL, token.Username, token.Password)
	if err != nil {
		return &AuthResult{
			Success:     false,
			RegistryURL: registryURL,
			Message:     fmt.Sprintf("Failed to configure docker auth: %v", err),
		}, err
	}

	return &AuthResult{
		Success:     true,
		RegistryURL: registryURL,
		Username:    token.Username,
		Message:     "ECR login successful",
		TokenExpiry: token.ExpiresAt.Format(time.RFC3339),
	}, nil
}

// ConfigureDefaults 配置默认AWS凭证
func (ecr *ecrAuthManager) ConfigureDefaults(config map[string]interface{}) error {
	ecr.mutex.Lock()
	defer ecr.mutex.Unlock()

	credentials := &ECRCredentialsConfig{}
	
	if region, exists := config["region"]; exists {
		if regionStr, ok := region.(string); ok {
			credentials.Region = regionStr
		}
	}
	
	if accessKey, exists := config["access_key_id"]; exists {
		if accessKeyStr, ok := accessKey.(string); ok {
			credentials.AccessKeyID = accessKeyStr
		}
	}
	
	if secretKey, exists := config["secret_access_key"]; exists {
		if secretKeyStr, ok := secretKey.(string); ok {
			credentials.SecretAccessKey = secretKeyStr
		}
	}
	
	if roleArn, exists := config["role_arn"]; exists {
		if roleArnStr, ok := roleArn.(string); ok {
			credentials.RoleArn = roleArnStr
		}
	}
	
	if useInstanceRole, exists := config["use_instance_role"]; exists {
		if useInstanceRoleBool, ok := useInstanceRole.(bool); ok {
			credentials.UseInstanceRole = useInstanceRoleBool
		}
	}
	
	if useECSTaskRole, exists := config["use_ecs_task_role"]; exists {
		if useECSTaskRoleBool, ok := useECSTaskRole.(bool); ok {
			credentials.UseECSTaskRole = useECSTaskRoleBool
		}
	}

	ecr.defaultConfig = credentials
	return nil
}

// IsTokenValid 检查token是否有效
func (ecr *ecrAuthManager) IsTokenValid(registryURL string) bool {
	ecr.mutex.RLock()
	defer ecr.mutex.RUnlock()
	return ecr.IsTokenValidUnsafe(registryURL)
}

// IsTokenValidUnsafe 检查token是否有效（不加锁）
func (ecr *ecrAuthManager) IsTokenValidUnsafe(registryURL string) bool {
	token, exists := ecr.tokenCache[registryURL]
	if !exists {
		return false
	}
	
	// 提前5分钟过期，确保有足够时间使用
	return time.Now().Add(5 * time.Minute).Before(token.ExpiresAt)
}

// RefreshToken 刷新token
func (ecr *ecrAuthManager) RefreshToken(ctx context.Context, registryURL string) error {
	if !ecr.IsECRRegistry(registryURL) {
		return fmt.Errorf("not an ECR registry: %s", registryURL)
	}

	ecrInfo, err := ecr.ParseECRInfo(registryURL)
	if err != nil {
		return fmt.Errorf("failed to parse ECR info: %w", err)
	}

	token, err := ecr.getECRToken(ctx, ecrInfo)
	if err != nil {
		return fmt.Errorf("failed to refresh ECR token: %w", err)
	}

	ecr.mutex.Lock()
	ecr.tokenCache[registryURL] = token
	ecr.mutex.Unlock()
	
	// 异步保存到持久化缓存
	go ecr.saveTokenCache()

	return ecr.dockerManager.SetRegistryAuth(registryURL, token.Username, token.Password)
}

// GetCachedToken 获取缓存的token
func (ecr *ecrAuthManager) GetCachedToken(registryURL string) (*ECRToken, error) {
	ecr.mutex.RLock()
	defer ecr.mutex.RUnlock()
	return ecr.GetCachedTokenUnsafe(registryURL), nil
}

// GetCachedTokenUnsafe 获取缓存的token（不加锁）
func (ecr *ecrAuthManager) GetCachedTokenUnsafe(registryURL string) *ECRToken {
	token, exists := ecr.tokenCache[registryURL]
	if !exists {
		return nil
	}
	return token
}

// IsECRRegistry 检查是否为ECR registry
func (ecr *ecrAuthManager) IsECRRegistry(registryURL string) bool {
	return ecrRegex.MatchString(registryURL)
}

// ParseECRInfo 解析ECR信息
func (ecr *ecrAuthManager) ParseECRInfo(registryURL string) (*ECRRegistryInfo, error) {
	matches := ecrRegex.FindStringSubmatch(registryURL)
	if len(matches) != 3 {
		return nil, fmt.Errorf("invalid ECR registry URL: %s", registryURL)
	}

	return &ECRRegistryInfo{
		AccountID: matches[1],
		Region:    matches[2],
		Domain:    registryURL,
	}, nil
}

// getECRToken 获取ECR认证token
func (ecr *ecrAuthManager) getECRToken(ctx context.Context, ecrInfo *ECRRegistryInfo) (*ECRToken, error) {
	cfg, err := ecr.buildAWSConfig(ctx, ecrInfo.Region)
	if err != nil {
		return nil, fmt.Errorf("failed to build AWS config: %w", err)
	}

	ecrClient := ecrservice.NewFromConfig(*cfg, func(o *ecrservice.Options) {
		o.Region = ecrInfo.Region
	})

	input := &ecrservice.GetAuthorizationTokenInput{}
	if ecrInfo.AccountID != "" {
		input.RegistryIds = []string{ecrInfo.AccountID}
	}

	result, err := ecrClient.GetAuthorizationToken(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get ECR authorization token: %w", err)
	}

	if len(result.AuthorizationData) == 0 {
		return nil, fmt.Errorf("no authorization data returned from ECR")
	}

	authData := result.AuthorizationData[0]
	if authData.AuthorizationToken == nil {
		return nil, fmt.Errorf("no authorization token returned from ECR")
	}

	// 解码认证token
	username, password, err := ecr.decodeAuthToken(*authData.AuthorizationToken)
	if err != nil {
		return nil, fmt.Errorf("failed to decode auth token: %w", err)
	}

	expiresAt := time.Now().Add(12 * time.Hour) // 默认12小时过期
	if authData.ExpiresAt != nil {
		expiresAt = *authData.ExpiresAt
	}

	return &ECRToken{
		Username:  username,
		Password:  password,
		ExpiresAt: expiresAt,
		Registry:  ecrInfo.Domain,
	}, nil
}

// buildAWSConfig 构建AWS配置
func (ecr *ecrAuthManager) buildAWSConfig(ctx context.Context, region string) (*aws.Config, error) {
	ecr.mutex.RLock()
	defaultConfig := ecr.defaultConfig
	ecr.mutex.RUnlock()

	// 优先使用默认配置
	if defaultConfig != nil {
		if defaultConfig.UseECSTaskRole {
			// 使用默认配置加载ECS task role
			cfg, err := config.LoadDefaultConfig(ctx,
				config.WithRegion(region),
			)
			return &cfg, err
		}

		if defaultConfig.UseInstanceRole {
			cfg, err := config.LoadDefaultConfig(ctx,
				config.WithRegion(region),
				config.WithCredentialsProvider(ec2rolecreds.New()),
			)
			return &cfg, err
		}

		if defaultConfig.AccessKeyID != "" && defaultConfig.SecretAccessKey != "" {
			creds := credentials.NewStaticCredentialsProvider(
				defaultConfig.AccessKeyID,
				defaultConfig.SecretAccessKey,
				defaultConfig.SessionToken,
			)

			cfg, err := config.LoadDefaultConfig(ctx,
				config.WithRegion(region),
				config.WithCredentialsProvider(creds),
			)
			
			if err != nil {
				return nil, err
			}

			// 如果配置了Role ARN，使用STS assume role
			if defaultConfig.RoleArn != "" {
				stsClient := sts.NewFromConfig(cfg)
				assumeRoleCreds := stscreds.NewAssumeRoleProvider(stsClient, defaultConfig.RoleArn)
				cfg, err = config.LoadDefaultConfig(ctx,
					config.WithRegion(region),
					config.WithCredentialsProvider(assumeRoleCreds),
				)
			}

			return &cfg, err
		}
	}

	// 使用默认AWS配置
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	return &cfg, err
}

// decodeAuthToken 解码ECR认证token
func (ecr *ecrAuthManager) decodeAuthToken(token string) (username, password string, err error) {
	// ECR认证token是base64编码的"username:password"格式
	decodedBytes, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return "", "", fmt.Errorf("failed to decode base64 token: %w", err)
	}

	decoded := string(decodedBytes)
	parts := strings.SplitN(decoded, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid token format")
	}

	return parts[0], parts[1], nil
}

// loadPersistedTokens 加载持久化缓存的tokens
func (ecr *ecrAuthManager) loadPersistedTokens() error {
	persistedTokens, err := ecr.configManager.LoadTokenCache()
	if err != nil {
		return fmt.Errorf("failed to load persisted token cache: %w", err)
	}
	
	ecr.mutex.Lock()
	defer ecr.mutex.Unlock()
	
	// 合并持久化的tokens到内存缓存
	for registryURL, token := range persistedTokens {
		if ecr.isTokenValidUnsafe(registryURL, token) {
			ecr.tokenCache[registryURL] = token
			// 配置Docker认证
			go func(regURL, username, password string) {
				ecr.dockerManager.SetRegistryAuth(regURL, username, password)
			}(registryURL, token.Username, token.Password)
		}
	}
	
	ecr.lastCacheSync = time.Now()
	return nil
}

// saveTokenCache 保存token缓存到持久化存储
func (ecr *ecrAuthManager) saveTokenCache() {
	// 避免频繁保存，最多每分30秒保存一次
	if time.Since(ecr.lastCacheSync) < 30*time.Second {
		return
	}
	
	ecr.mutex.RLock()
	tokensCopy := make(map[string]*ECRToken)
	for k, v := range ecr.tokenCache {
		tokensCopy[k] = v
	}
	ecr.mutex.RUnlock()
	
	if err := ecr.configManager.SaveTokenCache(tokensCopy); err != nil {
		// 保存失败不影响主流程，记录日志即可
		// 这里可以添加日志记录
		return
	}
	
	ecr.mutex.Lock()
	ecr.lastCacheSync = time.Now()
	ecr.mutex.Unlock()
}

// isTokenValidUnsafe 检查token是否有效（不加锁版本，支持传入token）
func (ecr *ecrAuthManager) isTokenValidUnsafe(registryURL string, token *ECRToken) bool {
	if token == nil {
		return false
	}
	
	// 提前5分钟过期，确保有足够时间使用
	return time.Now().Add(5 * time.Minute).Before(token.ExpiresAt)
}

// ConfigureFromGlobal 从全局配置加载默认设置
func (ecr *ecrAuthManager) ConfigureFromGlobal() error {
	globalConfig, err := ecr.configManager.LoadGlobalConfig()
	if err != nil {
		return fmt.Errorf("failed to load global config: %w", err)
	}
	
	ecr.mutex.Lock()
	defer ecr.mutex.Unlock()
	
	if globalConfig.DefaultCredentials != nil {
		ecr.defaultConfig = globalConfig.DefaultCredentials
	}
	
	return nil
}

// GetCacheStats 获取缓存统计信息
func (ecr *ecrAuthManager) GetCacheStats() map[string]interface{} {
	ecr.mutex.RLock()
	defer ecr.mutex.RUnlock()
	
	validTokens := 0
	expiredTokens := 0
	
	for _, token := range ecr.tokenCache {
		if ecr.isTokenValidUnsafe("", token) {
			validTokens++
		} else {
			expiredTokens++
		}
	}
	
	return map[string]interface{}{
		"total_tokens":     len(ecr.tokenCache),
		"valid_tokens":     validTokens,
		"expired_tokens":   expiredTokens,
		"last_cache_sync":  ecr.lastCacheSync,
	}
}