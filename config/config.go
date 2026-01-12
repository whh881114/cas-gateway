package config

import (
	"fmt"
	"os"
	"cas-gateway/models"

	"gopkg.in/yaml.v3"
)

var AppConfig *models.Config

// LoadConfig 加载配置文件
func LoadConfig(path string) (*models.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg models.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 验证配置
	if err := validateConfig(&cfg); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	AppConfig = &cfg
	return &cfg, nil
}

// validateConfig 验证配置有效性
func validateConfig(cfg *models.Config) error {
	if cfg.Server.Port <= 0 || cfg.Server.Port > 65535 {
		return fmt.Errorf("服务器端口无效: %d", cfg.Server.Port)
	}

	if len(cfg.Server.SessionKey) < 32 {
		return fmt.Errorf("session_key 必须至少32字节")
	}

	// 验证CAS配置
	if cfg.CAS.BaseURL == "" {
		return fmt.Errorf("CAS base_url 不能为空")
	}

	// 验证路由配置
	if len(cfg.Routes) == 0 {
		return fmt.Errorf("至少需要配置一个路由")
	}

	// 验证路由配置
	pathMap := make(map[string]bool)
	for _, route := range cfg.Routes {
		if route.Name == "" {
			return fmt.Errorf("路由名称不能为空")
		}
		if route.Path == "" {
			return fmt.Errorf("路由路径不能为空: %s", route.Name)
		}
		if route.Target == "" {
			return fmt.Errorf("路由目标不能为空: %s", route.Name)
		}
		if pathMap[route.Path] {
			return fmt.Errorf("路由路径重复: %s", route.Path)
		}
		pathMap[route.Path] = true
	}

	return nil
}
