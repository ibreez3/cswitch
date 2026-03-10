package provider

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/ibreez3/cswitch/config"
	"github.com/ibreez3/cswitch/internal/fileutil"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

// CodexProvider Codex 实现
type CodexProvider struct{}

func init() {
	Register(&CodexProvider{})
}

func (p *CodexProvider) Name() string {
	return "codex"
}

func (p *CodexProvider) ShortDesc() string {
	return "管理 Codex 配置"
}

func (p *CodexProvider) authPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".codex", "auth.json")
}

func (p *CodexProvider) configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".codex", "config.toml")
}

func (p *CodexProvider) envPath() string {
	return filepath.Join(config.ConfigDir(), "codex.env")
}

func (p *CodexProvider) NativeFiles() []string {
	return []string{p.authPath(), p.configPath(), p.envPath()}
}

func (p *CodexProvider) EnvFilePath() string {
	return p.envPath()
}

func (p *CodexProvider) SetupShellHook() (bool, error) {
	// Codex 已在 WriteLive 中处理 shell rc，这里不需要额外操作
	return false, nil
}

func (p *CodexProvider) detectShellRC() string {
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

func (p *CodexProvider) ensureSourceLine(rcPath, envPath string) (bool, error) {
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
		_, err = fmt.Fprintf(f, "\n# cswitch: Codex 环境变量\n%s\n", sourceLine)
		return err == nil, err
	}
	return false, os.WriteFile(rcPath, []byte(fmt.Sprintf("# cswitch: Codex 环境变量\n%s\n", sourceLine)), 0644)
}

func (p *CodexProvider) writeEnv(apiKey string) error {
	envPath := p.envPath()
	if err := os.MkdirAll(filepath.Dir(envPath), 0755); err != nil {
		return err
	}
	content := fmt.Sprintf("export OPENAI_API_KEY='%s'\n", strings.ReplaceAll(apiKey, "'", `'"'"'`))
	return fileutil.WriteFileAtomic(envPath, []byte(content), 0600)
}

func (p *CodexProvider) writeTomlAtomic(path string, data map[string]interface{}, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	enc := toml.NewEncoder(tmp)
	if err := enc.Encode(data); err != nil {
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

func (p *CodexProvider) WriteLive(mc config.ModelConfig, selectedModel string) error {
	// 写入 auth.json
	authPath := p.authPath()
	if err := os.MkdirAll(filepath.Dir(authPath), 0755); err != nil {
		return err
	}

	authObj := map[string]interface{}{}
	if data, err := os.ReadFile(authPath); err == nil && len(data) > 0 {
		if err := json.Unmarshal(data, &authObj); err != nil {
			return fmt.Errorf("无法解析 Codex auth.json: %w", err)
		}
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	authObj["OPENAI_API_KEY"] = mc.APIKey

	authData, err := json.MarshalIndent(authObj, "", "  ")
	if err != nil {
		return err
	}
	if err := fileutil.WriteFileAtomic(authPath, authData, 0600); err != nil {
		return err
	}

	// 写入 config.toml
	cfgPath := p.configPath()
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0755); err != nil {
		return err
	}
	cfg := make(map[string]interface{})
	if data, err := os.ReadFile(cfgPath); err == nil && len(data) > 0 {
		if err := toml.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("无法解析 Codex config.toml: %w", err)
		}
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}

	providerName := mc.ProviderName
	if providerName == "" {
		providerName = "cswitch_provider"
	}

	cfg["model_provider"] = providerName
	cfg["model"] = selectedModel
	delete(cfg, "base_url")

	providers, _ := cfg["model_providers"].(map[string]interface{})
	if providers == nil {
		providers = make(map[string]interface{})
	}
	wireAPI := mc.WireAPI
	if wireAPI == "" {
		wireAPI = "responses"
	}
	providerCfg := map[string]interface{}{
		"name":     providerName,
		"base_url": mc.BaseURL,
		"env_key":  "OPENAI_API_KEY",
		"wire_api": wireAPI,
	}
	providers[providerName] = providerCfg
	cfg["model_providers"] = providers

	if err := p.writeTomlAtomic(cfgPath, cfg, 0644); err != nil {
		return err
	}

	// 写入环境变量文件
	if err := p.writeEnv(mc.APIKey); err != nil {
		return fmt.Errorf("写入 codex.env 失败: %w", err)
	}

	// 确保 shell rc 中有 source 行
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

func (p *CodexProvider) ExportEnv(mc config.ModelConfig, selectedModel string) []string {
	quote := func(s string) string {
		return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
	}
	return []string{
		fmt.Sprintf("export OPENAI_BASE_URL=%s", quote(mc.BaseURL)),
		fmt.Sprintf("export OPENAI_API_KEY=%s", quote(mc.APIKey)),
		fmt.Sprintf("# Codex 模型请使用: codex --model %s", selectedModel),
	}
}

func (p *CodexProvider) CustomAddFlags(cmd *cobra.Command) {
	cmd.Flags().String("provider-name", "", "Codex Provider 名称")
	cmd.Flags().String("wire-api", "", "Codex wire_api（chat 或 responses）")
}

func (p *CodexProvider) ProcessAddConfig(cmd *cobra.Command, mc *config.ModelConfig, alias string, interactive bool) {
	providerName, _ := cmd.Flags().GetString("provider-name")
	wireAPI, _ := cmd.Flags().GetString("wire-api")

	if interactive {
		if providerName == "" {
			p := promptui.Prompt{
				Label:   "请输入 Provider 名称（Codex model_provider，留空自动生成）",
				Default: "cswitch_" + alias,
			}
			if v, err := p.Run(); err == nil && strings.TrimSpace(v) != "" {
				providerName = strings.TrimSpace(v)
			}
		}
		if wireAPI == "" {
			p := promptui.Prompt{
				Label:   "请输入 wire_api（chat=Chat Completions / responses=Responses API，默认 chat）",
				Default: "chat",
			}
			if v, err := p.Run(); err == nil && strings.TrimSpace(v) != "" {
				wireAPI = strings.TrimSpace(v)
			}
		}
	}

	if providerName == "" {
		providerName = "cswitch_" + alias
	}
	if wireAPI == "" {
		wireAPI = "chat"
	}

	mc.ProviderName = providerName
	mc.WireAPI = wireAPI
}

func (p *CodexProvider) SwitchSuccessMsg(selectedModel string) string {
	return fmt.Sprintf("已切换 Codex 配置（模型: %s），写入 %s 和 %s\n环境变量 OPENAI_API_KEY 已写入 %s，新终端自动生效\n当前终端请执行: source %s",
		selectedModel, p.authPath(), p.configPath(), p.envPath(), p.envPath())
}

// MaskAPIKey 遮蔽 API Key
func MaskAPIKey(key string) string {
	if len(key) <= 6 {
		return strings.Repeat("*", len(key))
	}
	return key[:3] + "****" + key[len(key)-3:]
}

// ParseModels 解析模型列表
func ParseModels(modelsStr string) ([]string, error) {
	var models []string
	if err := json.Unmarshal([]byte(modelsStr), &models); err != nil {
		return nil, err
	}
	return models, nil
}

// RequiredInputPrompt 必填输入提示
func RequiredInputPrompt(label string, mask rune) (string, error) {
	prompt := promptui.Prompt{
		Label: label,
		Mask:  mask,
		Validate: func(input string) error {
			if strings.TrimSpace(input) == "" {
				return fmt.Errorf("该参数为必填，请重新输入")
			}
			return nil
		},
	}
	result, err := prompt.Run()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(result), nil
}

// OptionalIntPrompt 可选整数输入提示
func OptionalIntPrompt(label, defaultVal string) int {
	prompt := promptui.Prompt{Label: label, Default: defaultVal}
	if v, err := prompt.Run(); err == nil {
		if i, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return i
		}
	}
	return 0
}
