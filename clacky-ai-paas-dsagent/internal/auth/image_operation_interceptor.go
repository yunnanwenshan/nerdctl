package auth

import (
	"context"
	"fmt"
)

// ImageOperation 镜像操作类型
type ImageOperation string

const (
	OperationPull ImageOperation = "pull"
	OperationPush ImageOperation = "push"
)

// ImageOperationInterceptor 镜像操作拦截器接口
type ImageOperationInterceptor interface {
	// 在镜像操作前执行拦截逻辑
	BeforeOperation(ctx context.Context, operation ImageOperation, imageURL string) error
	
	// 在镜像操作后执行清理逻辑
	AfterOperation(ctx context.Context, operation ImageOperation, imageURL string, err error) error
	
	// 配置拦截器
	Configure(config map[string]interface{}) error
}

// imageOperationInterceptor 镜像操作拦截器实现
type imageOperationInterceptor struct {
	registryAuthManager RegistryAuthManager
	urlParser           *ImageURLParser
	enabled             bool
	autoLoginResults    map[string]*AuthResult // 缓存本次操作的登录结果
}

// NewImageOperationInterceptor 创建镜像操作拦截器
func NewImageOperationInterceptor() (ImageOperationInterceptor, error) {
	registryAuthManager, err := NewRegistryAuthManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create registry auth manager: %w", err)
	}

	return &imageOperationInterceptor{
		registryAuthManager: registryAuthManager,
		urlParser:           NewImageURLParser(),
		enabled:             true,
		autoLoginResults:    make(map[string]*AuthResult),
	}, nil
}

// BeforeOperation 在镜像操作前执行拦截逻辑
func (ioi *imageOperationInterceptor) BeforeOperation(ctx context.Context, operation ImageOperation, imageURL string) error {
	if !ioi.enabled {
		return nil
	}

	// 解析镜像URL
	imageInfo, err := ioi.urlParser.Parse(imageURL)
	if err != nil {
		return fmt.Errorf("failed to parse image URL %s: %w", imageURL, err)
	}

	registryURL := imageInfo.GetRegistryURL()
	
	// 检查是否需要认证
	if !ioi.registryAuthManager.IsSupported(registryURL) {
		// 不是支持的registry类型，跳过认证
		return nil
	}

	// 执行自动认证
	err = ioi.registryAuthManager.AutoAuthenticate(ctx, registryURL, string(operation))
	if err != nil {
		return fmt.Errorf("auto authentication failed for registry %s: %w", registryURL, err)
	}

	// 记录认证结果（便于后续清理或审计）
	ioi.autoLoginResults[registryURL] = &AuthResult{
		Success:     true,
		RegistryURL: registryURL,
		Message:     fmt.Sprintf("Auto login for %s operation", operation),
	}

	return nil
}

// AfterOperation 在镜像操作后执行清理逻辑
func (ioi *imageOperationInterceptor) AfterOperation(ctx context.Context, operation ImageOperation, imageURL string, operationError error) error {
	if !ioi.enabled {
		return nil
	}

	// 这里可以添加操作后的清理逻辑
	// 例如：记录审计日志、清理临时认证等

	// 清理本次操作的登录结果缓存
	defer func() {
		ioi.autoLoginResults = make(map[string]*AuthResult)
	}()

	// 如果操作失败，可以在这里记录或处理
	if operationError != nil {
		imageInfo, parseErr := ioi.urlParser.Parse(imageURL)
		if parseErr == nil {
			registryURL := imageInfo.GetRegistryURL()
			if result, exists := ioi.autoLoginResults[registryURL]; exists {
				// 可以在这里添加失败处理逻辑
				_ = result
				// 例如：记录到日志，发送告警等
			}
		}
	}

	return nil
}

// Configure 配置拦截器
func (ioi *imageOperationInterceptor) Configure(config map[string]interface{}) error {
	// 配置是否启用拦截器
	if enabled, exists := config["enabled"]; exists {
		if enabledBool, ok := enabled.(bool); ok {
			ioi.enabled = enabledBool
		}
	}

	// 配置registry认证管理器的默认凭证
	if defaults, exists := config["defaults"]; exists {
		if defaultsMap, ok := defaults.(map[string]interface{}); ok {
			err := ioi.registryAuthManager.ConfigureDefaults(defaultsMap)
			if err != nil {
				return fmt.Errorf("failed to configure registry auth defaults: %w", err)
			}
		}
	}

	return nil
}

// GetAutoLoginResults 获取自动登录结果（用于调试和审计）
func (ioi *imageOperationInterceptor) GetAutoLoginResults() map[string]*AuthResult {
	results := make(map[string]*AuthResult)
	for k, v := range ioi.autoLoginResults {
		results[k] = v
	}
	return results
}

// InterceptImagePull 拦截镜像Pull操作的便捷方法
func InterceptImagePull(ctx context.Context, imageURL string, interceptor ImageOperationInterceptor, actualPullFunc func() error) error {
	// 执行前置拦截
	err := interceptor.BeforeOperation(ctx, OperationPull, imageURL)
	if err != nil {
		return fmt.Errorf("pull operation intercepted: %w", err)
	}

	// 执行实际的Pull操作
	pullErr := actualPullFunc()

	// 执行后置处理
	afterErr := interceptor.AfterOperation(ctx, OperationPull, imageURL, pullErr)
	if afterErr != nil {
		// 后置处理错误不应该覆盖原始操作错误
		if pullErr != nil {
			return fmt.Errorf("pull failed: %v, cleanup failed: %w", pullErr, afterErr)
		}
		return fmt.Errorf("pull succeeded but cleanup failed: %w", afterErr)
	}

	return pullErr
}

// InterceptImagePush 拦截镜像Push操作的便捷方法
func InterceptImagePush(ctx context.Context, imageURL string, interceptor ImageOperationInterceptor, actualPushFunc func() error) error {
	// 执行前置拦截
	err := interceptor.BeforeOperation(ctx, OperationPush, imageURL)
	if err != nil {
		return fmt.Errorf("push operation intercepted: %w", err)
	}

	// 执行实际的Push操作
	pushErr := actualPushFunc()

	// 执行后置处理
	afterErr := interceptor.AfterOperation(ctx, OperationPush, imageURL, pushErr)
	if afterErr != nil {
		// 后置处理错误不应该覆盖原始操作错误
		if pushErr != nil {
			return fmt.Errorf("push failed: %v, cleanup failed: %w", pushErr, afterErr)
		}
		return fmt.Errorf("push succeeded but cleanup failed: %w", afterErr)
	}

	return pushErr
}