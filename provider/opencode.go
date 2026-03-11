package provider

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ibreez3/cswitch/config"
	"github.com/ibreez3/cswitch/internal/fileutil"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

type OpenCodeProvider struct{}

func init() {
	Register(&OpenCodeProvider{})
}

func (p *OpenCodeProvider) Name() string {
	return "opencode"
}

func (p *OpenCodeProvider) ShortDesc() string {
	return "管理 OpenCode 配置"
}

func (p *OpenCodeProvider) configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "opencode", "opencode.json")
}

func (p *OpenCodeProvider) envPath() string {
	return filepath.Join(config.ConfigDir(), "opencode.env")
}

func (p *OpenCodeProvider) NativeFiles() []string {
	return []string{p.configPath(), p.envPath()}
}

func (p *OpenCodeProvider) EnvFilePath() string {
	return p.envPath()
}

func (p *OpenCodeProvider) SetupShellHook() (bool, error) {
	return false, nil
}

func (p *OpenCodeProvider) detectShellRC() string {
	home, _ := os.UserHomeDir()
	shell := os.Getenv("SHELL")
	switch {
	case strings.HasSuffix(shell, "/zsh"):
		return filepath.Join(home, ".zshrc")
	case strings.HasSuffix(shell, "/bash"):
		if rc := filepath.Join(home, ".bashrc"); fileutil.FileExists(rc) {
			return rc
		}
		return filepath.Join(home, ".bash_profile")
	default:
		return filepath.Join(home, ".profile")
	}
}

func (p *OpenCodeProvider) ensureSourceLine(rcPath, envPath string) (bool, error) {
	sourceLine := fmt.Sprintf("[ -f %q ] && source %q", envPath, envPath)

	if data, err := os.ReadFile(rcPath); err == nil {
		if strings.Contains(string(data), envPath) {
			return false, nil
		}
		f, err := os.OpenFile(rcPath, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return false, err
		}
		defer f.Close()
		_, err = fmt.Fprintf(f, "\n# cswitch: OpenCode 环境变量\n%s\n", sourceLine)
		return err == nil, err
	}
	return false, os.WriteFile(rcPath, []byte(fmt.Sprintf("# cswitch: OpenCode 环境变量\n%s\n", sourceLine)), 0644)
}

func (p *OpenCodeProvider) writeEnv(apiKey string) error {
	envPath := p.envPath()
	if err := os.MkdirAll(filepath.Dir(envPath), 0755); err != nil {
		return err
	}
	content := fmt.Sprintf("export OPENCODE_API_KEY='%s'\n", strings.ReplaceAll(apiKey, "'", `'"'"'`))
	return fileutil.WriteFileAtomic(envPath, []byte(content), 0600)
}

func (p *OpenCodeProvider) WriteLive(mc config.ModelConfig, selectedModel string) error {
	cfgPath := p.configPath()
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0755); err != nil {
		return err
	}

	cfg := map[string]interface{}{}
	if data, err := os.ReadFile(cfgPath); err == nil && len(data) > 0 {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("无法解析 OpenCode 配置: %w", err)
		}
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}

	providerName := mc.ProviderName
	if providerName == "" {
		providerName = "cswitch"
	}

	modelName := selectedModel
	if !strings.Contains(modelName, "/") {
		modelName = providerName + "/" + modelName
	}

	cfg["$schema"] = "https://opencode.ai/config.json"
	cfg["model"] = modelName

	if mc.SmallModel != "" {
		smallModel := mc.SmallModel
		if !strings.Contains(smallModel, "/") {
			smallModel = providerName + "/" + smallModel
		}
		cfg["small_model"] = smallModel
	}

	providers, ok := cfg["provider"].(map[string]interface{})
	if !ok || providers == nil {
		providers = make(map[string]interface{})
	}

	providerCfg := map[string]interface{}{
		"options": map[string]interface{}{
			"apiKey": "{env:OPENCODE_API_KEY}",
		},
	}

	if mc.BaseURL != "" {
		providerCfg["baseURL"] = mc.BaseURL
	}

	providers[providerName] = providerCfg
	cfg["provider"] = providers

	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	if err := fileutil.WriteFileAtomic(cfgPath, out, 0600); err != nil {
		return err
	}

	if err := p.writeEnv(mc.APIKey); err != nil {
		return fmt.Errorf("写入 opencode.env 失败: %w", err)
	}

	rcPath := p.detectShellRC()
	added, err := p.ensureSourceLine(rcPath, p.envPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "警告：无法写入 %s: %v\n", rcPath, err)
		fmt.Fprintf(os.Stderr, "请手动添加: source %s\n", p.envPath())
	} else if added {
		fmt.Fprintf(os.Stderr, "已在 %s 中添加 source %s\n", rcPath, p.envPath())
	}

	return nil
}

func (p *OpenCodeProvider) ExportEnv(mc config.ModelConfig, selectedModel string) []string {
	quote := func(s string) string {
		return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
	}

	exports := []string{
		fmt.Sprintf("export OPENCODE_API_KEY=%s", quote(mc.APIKey)),
	}

	providerName := mc.ProviderName
	if providerName == "" {
		providerName = "cswitch"
	}

	modelName := selectedModel
	if !strings.Contains(modelName, "/") {
		modelName = providerName + "/" + modelName
	}

	exports = append(exports, fmt.Sprintf("# OpenCode 模型: %s", modelName))

	return exports
}

func (p *OpenCodeProvider) CustomAddFlags(cmd *cobra.Command) {
	cmd.Flags().String("provider-name", "", "OpenCode Provider 名称（用于模型前缀）")
	cmd.Flags().String("small-model", "", "OpenCode small_model 名称（轻量级任务用）")
}

func (p *OpenCodeProvider) ProcessAddConfig(cmd *cobra.Command, mc *config.ModelConfig, alias string, interactive bool) {
	providerName, _ := cmd.Flags().GetString("provider-name")
	smallModel, _ := cmd.Flags().GetString("small-model")

	if interactive {
		if providerName == "" {
			p := promptui.Prompt{
				Label:   "请输入 Provider 名称（模型前缀，如 openai、anthropic，留空使用 cswitch）",
				Default: "cswitch",
			}
			if v, err := p.Run(); err == nil && strings.TrimSpace(v) != "" {
				providerName = strings.TrimSpace(v)
			}
		}
		if smallModel == "" {
			p := promptui.Prompt{
				Label:   "请输入 small_model（轻量级任务用，留空不配置）",
				Default: "",
			}
			if v, err := p.Run(); err == nil && strings.TrimSpace(v) != "" {
				smallModel = strings.TrimSpace(v)
			}
		}
	}

	if providerName == "" {
		providerName = "cswitch"
	}

	mc.ProviderName = providerName
	mc.SmallModel = smallModel
}

func (p *OpenCodeProvider) SwitchSuccessMsg(alias string) string {
	return fmt.Sprintf("已切换 OpenCode 配置到 %s，写入 %s\n环境变量 OPENCODE_API_KEY 已写入 %s，新终端自动生效\n当前终端请执行: source %s",
		alias, p.configPath(), p.envPath(), p.envPath())
}
