package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// DockerConfigManager Docker配置管理器接口
type DockerConfigManager interface {
	// 设置registry认证信息
	SetRegistryAuth(registry, username, password string) error
	
	// 获取registry认证信息
	GetRegistryAuth(registry string) (*DockerAuthInfo, error)
	
	// 移除registry认证信息
	RemoveRegistryAuth(registry string) error
	
	// 获取Docker配置文件路径
	GetConfigPath() string
}

// DockerAuthInfo Docker认证信息
type DockerAuthInfo struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Auth     string `json:"auth"`
}

// DockerConfig Docker配置文件格式
type DockerConfig struct {
	Auths map[string]*DockerAuthInfo `json:"auths"`
}

// dockerConfigManager Docker配置管理器实现
type dockerConfigManager struct {
	configPath string
}

// NewDockerConfigManager 创建Docker配置管理器
func NewDockerConfigManager() (DockerConfigManager, error) {
	configPath := getDockerConfigPath()
	
	// 确保配置目录存在
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create docker config directory: %w", err)
	}

	return &dockerConfigManager{
		configPath: configPath,
	}, nil
}

// SetRegistryAuth 设置registry认证信息
func (dcm *dockerConfigManager) SetRegistryAuth(registry, username, password string) error {
	config, err := dcm.loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load docker config: %w", err)
	}

	if config.Auths == nil {
		config.Auths = make(map[string]*DockerAuthInfo)
	}

	// 生成auth字段（base64编码的username:password）
	auth := generateDockerAuth(username, password)

	config.Auths[registry] = &DockerAuthInfo{
		Username: username,
		Password: password,
		Auth:     auth,
	}

	err = dcm.saveConfig(config)
	if err != nil {
		return fmt.Errorf("failed to save docker config: %w", err)
	}

	return nil
}

// GetRegistryAuth 获取registry认证信息
func (dcm *dockerConfigManager) GetRegistryAuth(registry string) (*DockerAuthInfo, error) {
	config, err := dcm.loadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load docker config: %w", err)
	}

	auth, exists := config.Auths[registry]
	if !exists {
		return nil, fmt.Errorf("no auth found for registry: %s", registry)
	}

	return auth, nil
}

// RemoveRegistryAuth 移除registry认证信息
func (dcm *dockerConfigManager) RemoveRegistryAuth(registry string) error {
	config, err := dcm.loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load docker config: %w", err)
	}

	if config.Auths == nil {
		return nil // 没有认证信息，无需删除
	}

	delete(config.Auths, registry)

	err = dcm.saveConfig(config)
	if err != nil {
		return fmt.Errorf("failed to save docker config: %w", err)
	}

	return nil
}

// GetConfigPath 获取Docker配置文件路径
func (dcm *dockerConfigManager) GetConfigPath() string {
	return dcm.configPath
}

// loadConfig 加载Docker配置
func (dcm *dockerConfigManager) loadConfig() (*DockerConfig, error) {
	config := &DockerConfig{
		Auths: make(map[string]*DockerAuthInfo),
	}

	if _, err := os.Stat(dcm.configPath); os.IsNotExist(err) {
		// 配置文件不存在，返回空配置
		return config, nil
	}

	data, err := os.ReadFile(dcm.configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if len(data) == 0 {
		// 空文件，返回空配置
		return config, nil
	}

	err = json.Unmarshal(data, config)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if config.Auths == nil {
		config.Auths = make(map[string]*DockerAuthInfo)
	}

	return config, nil
}

// saveConfig 保存Docker配置
func (dcm *dockerConfigManager) saveConfig(config *DockerConfig) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	err = os.WriteFile(dcm.configPath, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// getDockerConfigPath 获取Docker配置文件路径
func getDockerConfigPath() string {
	// 首先检查DOCKER_CONFIG环境变量
	if dockerConfig := os.Getenv("DOCKER_CONFIG"); dockerConfig != "" {
		return filepath.Join(dockerConfig, "config.json")
	}

	// 使用默认路径
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// 降级到当前目录
		return filepath.Join(".", ".docker", "config.json")
	}

	return filepath.Join(homeDir, ".docker", "config.json")
}

// generateDockerAuth 生成Docker auth字段
func generateDockerAuth(username, password string) string {
	auth := fmt.Sprintf("%s:%s", username, password)
	return base64.StdEncoding.EncodeToString([]byte(auth))
}