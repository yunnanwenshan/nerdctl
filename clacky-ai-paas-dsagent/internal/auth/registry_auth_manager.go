package auth

import (
	"context"
	"fmt"
	"strings"
)

// RegistryAuthManager 镜像仓库认证管理器接口
type RegistryAuthManager interface {
	// 自动认证 - 检测registry类型并执行相应认证
	AutoAuthenticate(ctx context.Context, registryURL, operation string) error
	
	// 检查是否为支持的registry
	IsSupported(registryURL string) bool
	
	// 获取支持的registry类型
	GetSupportedTypes() []string
	
	// 配置默认凭证
	ConfigureDefaults(config map[string]interface{}) error
}

// RegistryType 表示镜像仓库类型
type RegistryType string

const (
	RegistryTypeECR        RegistryType = "ECR"
	RegistryTypeDockerHub  RegistryType = "DockerHub"
	RegistryTypePrivate    RegistryType = "Private"
)

// AuthResult 认证结果
type AuthResult struct {
	Success     bool     `json:"success"`
	RegistryURL string   `json:"registry_url"`
	Username    string   `json:"username,omitempty"`
	Message     string   `json:"message"`
	TokenExpiry string   `json:"token_expiry,omitempty"`
}

// registryAuthManager 镜像仓库认证管理器实现
type registryAuthManager struct {
	ecrManager ECRAuthManager
}

// NewRegistryAuthManager 创建镜像仓库认证管理器
func NewRegistryAuthManager() (RegistryAuthManager, error) {
	ecrManager, err := NewECRAuthManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create ECR auth manager: %w", err)
	}

	return &registryAuthManager{
		ecrManager: ecrManager,
	}, nil
}

// AutoAuthenticate 自动认证
func (rm *registryAuthManager) AutoAuthenticate(ctx context.Context, registryURL, operation string) error {
	if registryURL == "" {
		return nil // 对于Docker Hub等默认registry，不需要特殊认证
	}

	registryType := rm.detectRegistryType(registryURL)
	
	switch registryType {
	case RegistryTypeECR:
		return rm.authenticateECR(ctx, registryURL, operation)
	case RegistryTypeDockerHub:
		// Docker Hub认证逻辑（如果需要的话）
		return nil
	case RegistryTypePrivate:
		// 私有registry认证逻辑（如果需要的话）
		return nil
	default:
		// 未知registry类型，不处理
		return nil
	}
}

// IsSupported 检查是否为支持的registry
func (rm *registryAuthManager) IsSupported(registryURL string) bool {
	registryType := rm.detectRegistryType(registryURL)
	return registryType != ""
}

// GetSupportedTypes 获取支持的registry类型
func (rm *registryAuthManager) GetSupportedTypes() []string {
	return []string{string(RegistryTypeECR), string(RegistryTypeDockerHub), string(RegistryTypePrivate)}
}

// ConfigureDefaults 配置默认凭证
func (rm *registryAuthManager) ConfigureDefaults(config map[string]interface{}) error {
	// 配置ECR默认凭证
	if ecrConfig, exists := config["ecr"]; exists {
		if ecrConfigMap, ok := ecrConfig.(map[string]interface{}); ok {
			return rm.ecrManager.ConfigureDefaults(ecrConfigMap)
		}
	}
	return nil
}

// detectRegistryType 检测registry类型
func (rm *registryAuthManager) detectRegistryType(registryURL string) RegistryType {
	if rm.ecrManager.IsECRRegistry(registryURL) {
		return RegistryTypeECR
	}
	
	// Docker Hub检测
	if registryURL == "" || 
	   registryURL == "docker.io" || 
	   registryURL == "registry-1.docker.io" ||
	   strings.HasPrefix(registryURL, "index.docker.io/") {
		return RegistryTypeDockerHub
	}
	
	// 其他私有registry
	if strings.Contains(registryURL, ".") {
		return RegistryTypePrivate
	}
	
	return ""
}

// authenticateECR 执行ECR认证
func (rm *registryAuthManager) authenticateECR(ctx context.Context, registryURL, operation string) error {
	result, err := rm.ecrManager.AutoLogin(ctx, registryURL)
	if err != nil {
		return fmt.Errorf("ECR authentication failed for %s: %w", registryURL, err)
	}
	
	if !result.Success {
		return fmt.Errorf("ECR authentication failed for %s: %s", registryURL, result.Message)
	}
	
	return nil
}