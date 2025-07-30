package auth

import (
	"fmt"
	"strings"
)

// ImageURLInfo 镜像URL信息
type ImageURLInfo struct {
	Registry   string `json:"registry"`
	Namespace  string `json:"namespace"`
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
	Digest     string `json:"digest"`
	FullURL    string `json:"full_url"`
}

// ImageURLParser 镜像URL解析器
type ImageURLParser struct{}

// NewImageURLParser 创建镜像URL解析器
func NewImageURLParser() *ImageURLParser {
	return &ImageURLParser{}
}

// Parse 解析镜像URL
func (p *ImageURLParser) Parse(imageURL string) (*ImageURLInfo, error) {
	if imageURL == "" {
		return nil, fmt.Errorf("image URL cannot be empty")
	}

	info := &ImageURLInfo{
		FullURL: imageURL,
		Tag:     "latest", // 默认标签
	}

	// 处理digest格式 (image@sha256:...)
	if strings.Contains(imageURL, "@") {
		parts := strings.SplitN(imageURL, "@", 2)
		imageURL = parts[0]
		info.Digest = parts[1]
	}

	// 处理标签格式 (image:tag)
	if strings.Contains(imageURL, ":") && !strings.Contains(imageURL, "://") {
		// 需要区分registry端口和tag
		parts := strings.Split(imageURL, ":")
		if len(parts) >= 2 {
			// 检查最后一部分是否为标签(不包含'/'和不是端口号格式)
			lastPart := parts[len(parts)-1]
			if !strings.Contains(lastPart, "/") && !isPortNumber(lastPart) {
				info.Tag = lastPart
				imageURL = strings.Join(parts[:len(parts)-1], ":")
			}
		}
	}

	// 解析registry、namespace和repository
	err := p.parseRegistryAndPath(imageURL, info)
	if err != nil {
		return nil, fmt.Errorf("failed to parse registry and path: %w", err)
	}

	return info, nil
}

// parseRegistryAndPath 解析registry和路径
func (p *ImageURLParser) parseRegistryAndPath(imageURL string, info *ImageURLInfo) error {
	// 默认情况下假设没有registry (使用Docker Hub)
	path := imageURL

	// 检查是否包含registry
	if strings.Contains(imageURL, "/") {
		parts := strings.SplitN(imageURL, "/", 2)
		
		// 如果第一部分包含'.'或':'，则认为是registry
		if strings.Contains(parts[0], ".") || strings.Contains(parts[0], ":") {
			info.Registry = parts[0]
			path = parts[1]
		}
	}

	// 解析namespace和repository
	err := p.parseNamespaceAndRepository(path, info)
	if err != nil {
		return fmt.Errorf("failed to parse namespace and repository: %w", err)
	}

	return nil
}

// parseNamespaceAndRepository 解析namespace和repository
func (p *ImageURLParser) parseNamespaceAndRepository(path string, info *ImageURLInfo) error {
	if path == "" {
		return fmt.Errorf("path cannot be empty")
	}

	// 如果没有registry，应用Docker Hub的默认规则
	if info.Registry == "" {
		info.Registry = "docker.io" // Docker Hub默认registry
		
		// Docker Hub的命名空间规则
		if !strings.Contains(path, "/") {
			// 单一名称，使用library命名空间
			info.Namespace = "library"
			info.Repository = path
		} else {
			// 包含斜杠，分割为namespace和repository
			parts := strings.SplitN(path, "/", 2)
			info.Namespace = parts[0]
			info.Repository = parts[1]
		}
	} else {
		// 有明确registry的情况
		if strings.Contains(path, "/") {
			parts := strings.SplitN(path, "/", 2)
			info.Namespace = parts[0]
			info.Repository = parts[1]
		} else {
			// 没有namespace，直接使用repository名称
			info.Repository = path
		}
	}

	return nil
}

// GetRegistryURL 获取registry URL
func (info *ImageURLInfo) GetRegistryURL() string {
	if info.Registry == "" || info.Registry == "docker.io" {
		return "docker.io"
	}
	return info.Registry
}

// GetImageName 获取不带registry的镜像名称
func (info *ImageURLInfo) GetImageName() string {
	var parts []string
	
	if info.Namespace != "" {
		parts = append(parts, info.Namespace)
	}
	
	if info.Repository != "" {
		parts = append(parts, info.Repository)
	}
	
	imageName := strings.Join(parts, "/")
	
	if info.Tag != "" && info.Tag != "latest" {
		imageName += ":" + info.Tag
	}
	
	if info.Digest != "" {
		imageName += "@" + info.Digest
	}
	
	return imageName
}

// GetFullImageName 获取完整的镜像名称
func (info *ImageURLInfo) GetFullImageName() string {
	registry := info.GetRegistryURL()
	imageName := info.GetImageName()
	
	if registry == "docker.io" {
		return imageName
	}
	
	return fmt.Sprintf("%s/%s", registry, imageName)
}

// IsECRImage 检查是否为ECR镜像
func (info *ImageURLInfo) IsECRImage() bool {
	if info.Registry == "" {
		return false
	}
	
	// ECR registry格式: *.dkr.ecr.*.amazonaws.com
	return strings.Contains(info.Registry, ".dkr.ecr.") && strings.HasSuffix(info.Registry, ".amazonaws.com")
}

// IsDockerHubImage 检查是否为Docker Hub镜像
func (info *ImageURLInfo) IsDockerHubImage() bool {
	registry := info.GetRegistryURL()
	return registry == "docker.io" || registry == "registry-1.docker.io" || registry == "index.docker.io"
}

// isPortNumber 检查字符串是否为端口号
func isPortNumber(s string) bool {
	if len(s) == 0 {
		return false
	}
	
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	
	// 简单检查端口号范围
	if len(s) > 5 {
		return false
	}
	
	return true
}