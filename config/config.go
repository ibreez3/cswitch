package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const ConfigVersion = 2

// ModelConfig 模型配置
type ModelConfig struct {
	BaseURL      string   `json:"base_url"`
	APIKey       string   `json:"api_key"`
	Models       []string `json:"models"`
	Timeout      int      `json:"timeout,omitempty"`
	MaxTokens    int      `json:"max_tokens,omitempty"`
	ProviderName string   `json:"provider_name,omitempty"`
	WireAPI      string   `json:"wire_api,omitempty"`
	// Claude 专用：模型类型映射
	OpusModel   string `json:"opus_model,omitempty"`
	HaikuModel  string `json:"haiku_model,omitempty"`
	SonnetModel string `json:"sonnet_model,omitempty"`
}

// ToolConfig 工具配置
type ToolConfig struct {
	Current      string                 `json:"current"`
	CurrentModel string                 `json:"current_model,omitempty"`
	Models       map[string]ModelConfig `json:"models"`
}

// Config 主配置
type Config struct {
	Version int                   `json:"version"`
	Tools   map[string]ToolConfig `json:"tools"`
}

// LegacyConfig 旧版配置（用于迁移）
type LegacyConfig struct {
	Current string                 `json:"current"`
	Models  map[string]ModelConfig `json:"models"`
}

// ConfigDir 返回配置目录路径
func ConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cswitch")
}

// ConfigPath 返回配置文件路径
func ConfigPath() string {
	return filepath.Join(ConfigDir(), "config.json")
}

// DefaultConfig 创建默认配置
func DefaultConfig() *Config {
	return &Config{
		Version: ConfigVersion,
		Tools:   make(map[string]ToolConfig),
	}
}

// EnsureTool 确保工具配置存在
func EnsureTool(cfg *Config, tool string) {
	if cfg.Tools == nil {
		cfg.Tools = make(map[string]ToolConfig)
	}
	tc, ok := cfg.Tools[tool]
	if !ok {
		cfg.Tools[tool] = ToolConfig{
			Current: "",
			Models:  make(map[string]ModelConfig),
		}
		return
	}
	if tc.Models == nil {
		tc.Models = make(map[string]ModelConfig)
		cfg.Tools[tool] = tc
	}
}

// LoadConfig 加载配置
func LoadConfig() (*Config, error) {
	path := ConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, err
	}

	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		return nil, err
	}

	if _, hasVersion := probe["version"]; hasVersion {
		var cfg Config
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, err
		}
		if cfg.Version == 0 {
			cfg.Version = ConfigVersion
		}
		return &cfg, nil
	}

	// 检测旧版配置格式
	_, hasCurrent := probe["current"]
	_, hasModels := probe["models"]
	_, hasTools := probe["tools"]
	if !hasCurrent || !hasModels || hasTools {
		return nil, errors.New("无法识别的配置格式，请检查 ~/.cswitch/config.json")
	}

	var legacy LegacyConfig
	if err := json.Unmarshal(data, &legacy); err != nil {
		return nil, err
	}

	migrated := DefaultConfig()
	// 旧版配置迁移到 claude 工具
	tc := ToolConfig{
		Current: legacy.Current,
		Models:  make(map[string]ModelConfig),
	}
	if legacy.Models != nil {
		tc.Models = legacy.Models
	}
	migrated.Tools["claude"] = tc

	if err := os.WriteFile(path+".bak", data, 0600); err != nil {
		return nil, fmt.Errorf("迁移前备份失败: %w", err)
	}
	if err := SaveConfig(migrated); err != nil {
		return nil, err
	}
	return migrated, nil
}

// SaveConfig 保存配置
func SaveConfig(cfg *Config) error {
	path := ConfigPath()
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, data, 0600)
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	return os.Chmod(path, mode)
}
