package monitor

import (
	"encoding/json"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config 管控 IP 配置
type Config struct {
	Enabled bool     `yaml:"enabled" json:"enabled"`
	Targets []Target `yaml:"targets" json:"targets"`
}

// Target 监控目标
type Target struct {
	Type  string `yaml:"type" json:"type"`   // "ip" 或 "cidr"
	Value string `yaml:"value" json:"value"` // IP 或 CIDR 字符串
}

// LoadConfig 从文件加载配置（自动识别 YAML 或 JSON）
func LoadConfig(filePath string) (*Config, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	config := &Config{}

	// 尝试 YAML 解析
	if err := yaml.Unmarshal(data, config); err == nil {
		return config, nil
	}

	// 尝试 JSON 解析
	if err := json.Unmarshal(data, config); err == nil {
		return config, nil
	}

	return nil, fmt.Errorf("无法解析配置文件，既不是有效的 YAML 也不是 JSON")
}

