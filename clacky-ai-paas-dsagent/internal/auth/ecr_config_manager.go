package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ECRConfigManager ECR配置管理器接口
type ECRConfigManager interface {
	// 全局配置管理
	LoadGlobalConfig() (*ECRGlobalConfig, error)
	SaveGlobalConfig(config *ECRGlobalConfig) error
	
	// Token缓存持久化
	LoadTokenCache() (map[string]*ECRToken, error)
	SaveTokenCache(tokens map[string]*ECRToken) error
	
	// 配置路径管理
	GetConfigDir() string
	SetConfigDir(dir string) error
	
	// 清理过期token
	CleanExpiredTokens(tokens map[string]*ECRToken) map[string]*ECRToken
}

// ECRGlobalConfig ECR全局配置
type ECRGlobalConfig struct {
	DefaultRegion      string                    `json:"default_region"`
	DefaultCredentials *ECRCredentialsConfig     `json:"default_credentials"`
	RegistryProfiles   map[string]*ECRProfile    `json:"registry_profiles"`
	CacheSettings      *CacheSettings            `json:"cache_settings"`
	UpdatedAt          time.Time                 `json:"updated_at"`
}

// ECRProfile ECR registry配置profile
type ECRProfile struct {
	Name        string                 `json:"name"`
	RegistryURL string                 `json:"registry_url"`
	Credentials *ECRCredentialsConfig  `json:"credentials"`
	Enabled     bool                   `json:"enabled"`
}

// CacheSettings Token缓存设置
type CacheSettings struct {
	EnablePersistence bool          `json:"enable_persistence"`
	TokenExpireBuffer time.Duration `json:"token_expire_buffer"` // 提前过期时间
	CacheCleanupTTL   time.Duration `json:"cache_cleanup_ttl"`   // 清理间隔
}

// ecrConfigManager ECR配置管理器实现
type ecrConfigManager struct {
	configDir string
	mutex     sync.RWMutex
}

// NewECRConfigManager 创建ECR配置管理器
func NewECRConfigManager() ECRConfigManager {
	return &ecrConfigManager{
		configDir: getDefaultECRConfigDir(),
	}
}

// LoadGlobalConfig 加载全局配置
func (ecm *ecrConfigManager) LoadGlobalConfig() (*ECRGlobalConfig, error) {
	ecm.mutex.RLock()
	defer ecm.mutex.RUnlock()
	
	configPath := filepath.Join(ecm.configDir, "ecr-config.json")
	
	// 如果配置文件不存在，返回默认配置
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return ecm.getDefaultGlobalConfig(), nil
	}
	
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read ECR config file: %w", err)
	}
	
	var config ECRGlobalConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ECR config: %w", err)
	}
	
	// 确保必要字段有默认值
	if config.RegistryProfiles == nil {
		config.RegistryProfiles = make(map[string]*ECRProfile)
	}
	
	if config.CacheSettings == nil {
		config.CacheSettings = &CacheSettings{
			EnablePersistence: true,
			TokenExpireBuffer: 5 * time.Minute,
			CacheCleanupTTL:   1 * time.Hour,
		}
	}
	
	return &config, nil
}

// SaveGlobalConfig 保存全局配置
func (ecm *ecrConfigManager) SaveGlobalConfig(config *ECRGlobalConfig) error {
	ecm.mutex.Lock()
	defer ecm.mutex.Unlock()
	
	// 确保配置目录存在
	if err := os.MkdirAll(ecm.configDir, 0755); err != nil {
		return fmt.Errorf("failed to create ECR config directory: %w", err)
	}
	
	// 更新时间戳
	config.UpdatedAt = time.Now()
	
	configPath := filepath.Join(ecm.configDir, "ecr-config.json")
	
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal ECR config: %w", err)
	}
	
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write ECR config file: %w", err)
	}
	
	return nil
}

// LoadTokenCache 加载token缓存
func (ecm *ecrConfigManager) LoadTokenCache() (map[string]*ECRToken, error) {
	ecm.mutex.RLock()
	defer ecm.mutex.RUnlock()
	
	cachePath := filepath.Join(ecm.configDir, "ecr-token-cache.json")
	
	// 如果缓存文件不存在，返回空缓存
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		return make(map[string]*ECRToken), nil
	}
	
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read ECR token cache file: %w", err)
	}
	
	var tokens map[string]*ECRToken
	if err := json.Unmarshal(data, &tokens); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ECR token cache: %w", err)
	}
	
	if tokens == nil {
		tokens = make(map[string]*ECRToken)
	}
	
	// 清理过期token
	tokens = ecm.CleanExpiredTokens(tokens)
	
	return tokens, nil
}

// SaveTokenCache 保存token缓存
func (ecm *ecrConfigManager) SaveTokenCache(tokens map[string]*ECRToken) error {
	ecm.mutex.Lock()
	defer ecm.mutex.Unlock()
	
	// 确保配置目录存在
	if err := os.MkdirAll(ecm.configDir, 0755); err != nil {
		return fmt.Errorf("failed to create ECR config directory: %w", err)
	}
	
	// 清理过期token before saving
	cleanTokens := ecm.CleanExpiredTokens(tokens)
	
	cachePath := filepath.Join(ecm.configDir, "ecr-token-cache.json")
	
	data, err := json.MarshalIndent(cleanTokens, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal ECR token cache: %w", err)
	}
	
	if err := os.WriteFile(cachePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write ECR token cache file: %w", err)
	}
	
	return nil
}

// GetConfigDir 获取配置目录
func (ecm *ecrConfigManager) GetConfigDir() string {
	ecm.mutex.RLock()
	defer ecm.mutex.RUnlock()
	return ecm.configDir
}

// SetConfigDir 设置配置目录
func (ecm *ecrConfigManager) SetConfigDir(dir string) error {
	ecm.mutex.Lock()
	defer ecm.mutex.Unlock()
	
	// 验证目录是否可写
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create or access directory %s: %w", dir, err)
	}
	
	ecm.configDir = dir
	return nil
}

// CleanExpiredTokens 清理过期token
func (ecm *ecrConfigManager) CleanExpiredTokens(tokens map[string]*ECRToken) map[string]*ECRToken {
	now := time.Now()
	cleanTokens := make(map[string]*ECRToken)
	
	for registry, token := range tokens {
		// 提前5分钟过期，确保有足够时间使用
		if token != nil && now.Add(5*time.Minute).Before(token.ExpiresAt) {
			cleanTokens[registry] = token
		}
		// 过期的token被自动丢弃
	}
	
	return cleanTokens
}

// getDefaultECRConfigDir 获取默认ECR配置目录
func getDefaultECRConfigDir() string {
	// 尝试使用XDG配置目录
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return filepath.Join(xdgConfig, "dsagent", "ecr")
	}
	
	// 使用用户主目录
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// 降级到当前目录
		return filepath.Join(".", ".dsagent", "ecr")
	}
	
	return filepath.Join(homeDir, ".dsagent", "ecr")
}

// getDefaultGlobalConfig 获取默认全局配置
func (ecm *ecrConfigManager) getDefaultGlobalConfig() *ECRGlobalConfig {
	return &ECRGlobalConfig{
		DefaultRegion:    "us-east-1", // AWS 默认region
		RegistryProfiles: make(map[string]*ECRProfile),
		CacheSettings: &CacheSettings{
			EnablePersistence: true,
			TokenExpireBuffer: 5 * time.Minute,
			CacheCleanupTTL:   1 * time.Hour,
		},
		UpdatedAt: time.Now(),
	}
}

// ECRRegistryProfileManager 管理ECR registry profiles
type ECRRegistryProfileManager struct {
	configMgr ECRConfigManager
}

// NewECRRegistryProfileManager 创建ECR registry profile管理器
func NewECRRegistryProfileManager() *ECRRegistryProfileManager {
	return &ECRRegistryProfileManager{
		configMgr: NewECRConfigManager(),
	}
}

// AddProfile 添加registry profile
func (rpm *ECRRegistryProfileManager) AddProfile(name, registryURL string, credentials *ECRCredentialsConfig) error {
	config, err := rpm.configMgr.LoadGlobalConfig()
	if err != nil {
		return fmt.Errorf("failed to load global config: %w", err)
	}
	
	if config.RegistryProfiles == nil {
		config.RegistryProfiles = make(map[string]*ECRProfile)
	}
	
	config.RegistryProfiles[name] = &ECRProfile{
		Name:        name,
		RegistryURL: registryURL,
		Credentials: credentials,
		Enabled:     true,
	}
	
	return rpm.configMgr.SaveGlobalConfig(config)
}

// GetProfile 获取registry profile
func (rpm *ECRRegistryProfileManager) GetProfile(name string) (*ECRProfile, error) {
	config, err := rpm.configMgr.LoadGlobalConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load global config: %w", err)
	}
	
	profile, exists := config.RegistryProfiles[name]
	if !exists {
		return nil, fmt.Errorf("profile %s not found", name)
	}
	
	return profile, nil
}

// ListProfiles 列出所有profiles
func (rpm *ECRRegistryProfileManager) ListProfiles() (map[string]*ECRProfile, error) {
	config, err := rpm.configMgr.LoadGlobalConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load global config: %w", err)
	}
	
	return config.RegistryProfiles, nil
}

// RemoveProfile 移除registry profile
func (rpm *ECRRegistryProfileManager) RemoveProfile(name string) error {
	config, err := rpm.configMgr.LoadGlobalConfig()
	if err != nil {
		return fmt.Errorf("failed to load global config: %w", err)
	}
	
	if config.RegistryProfiles == nil {
		return nil // 没有profiles，无需删除
	}
	
	delete(config.RegistryProfiles, name)
	
	return rpm.configMgr.SaveGlobalConfig(config)
}