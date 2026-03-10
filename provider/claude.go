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

// ClaudeProvider Claude Code 实现
type ClaudeProvider struct{}

func init() {
	Register(&ClaudeProvider{})
}

func (p *ClaudeProvider) Name() string {
	return "claude"
}

func (p *ClaudeProvider) ShortDesc() string {
	return "管理 Claude Code 配置"
}

func (p *ClaudeProvider) settingsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "settings.json")
}

func (p *ClaudeProvider) NativeFiles() []string {
	return []string{p.settingsPath()}
}

func (p *ClaudeProvider) EnvFilePath() string {
	return filepath.Join(config.ConfigDir(), "claude.env")
}

func (p *ClaudeProvider) detectShellRC() string {
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

func (p *ClaudeProvider) ensureShellFunction(rcPath string) (bool, error) {
	envPath := p.EnvFilePath()
	// 添加文件存在性检查
	funcDef := fmt.Sprintf("cse() { cswitch claude switch \"$1\" --env && [ -f %q ] && source %q; }", envPath, envPath)

	data, err := os.ReadFile(rcPath)
	if err != nil {
		// 文件不存在，创建新文件
		return true, os.WriteFile(rcPath, []byte(fmt.Sprintf("# cswitch: Claude 快捷切换函数\n%s\n", funcDef)), 0644)
	}

	content := string(data)
	// 检查是否已有 cse 函数（旧格式或新格式）
	if strings.Contains(content, "cse() { cswitch claude switch") {
		// 检查是否是旧格式（没有文件存在性检查）
		if !strings.Contains(content, "[ -f ") {
			// 替换旧格式为新格式
			// 使用正则匹配旧行并替换
			lines := strings.Split(content, "\n")
			var newLines []string
			skipped := false
			for _, line := range lines {
				if strings.Contains(line, "cse() { cswitch claude switch") {
					if !skipped {
						newLines = append(newLines, funcDef)
						skipped = true
					}
					continue
				}
				if strings.Contains(line, "# cswitch: Claude 快捷切换函数") {
					continue
				}
				newLines = append(newLines, line)
			}
			newContent := strings.Join(newLines, "\n")
			return true, os.WriteFile(rcPath, []byte(newContent), 0644)
		}
		return false, nil
	}

	// 没有找到 cse 函数，添加
	f, err := os.OpenFile(rcPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return false, err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "\n# cswitch: Claude 快捷切换函数\n%s\n", funcDef)
	return err == nil, err
}

func (p *ClaudeProvider) SetupShellHook() (bool, error) {
	rcPath := p.detectShellRC()
	return p.ensureShellFunction(rcPath)
}

func (p *ClaudeProvider) WriteLive(mc config.ModelConfig, selectedModel string) error {
	path := p.settingsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	settings := map[string]interface{}{}
	if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("无法解析 Claude settings.json: %w", err)
		}
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}

	env, ok := settings["env"].(map[string]interface{})
	if !ok || env == nil {
		env = make(map[string]interface{})
	}

	// 设置基础配置
	env["ANTHROPIC_BASE_URL"] = mc.BaseURL
	env["ANTHROPIC_API_KEY"] = mc.APIKey

	// 设置模型类型映射（优先使用配置中指定的，否则使用选中模型自动检测）
	if mc.OpusModel != "" {
		env["ANTHROPIC_DEFAULT_OPUS_MODEL"] = mc.OpusModel
	} else {
		env["ANTHROPIC_DEFAULT_OPUS_MODEL"] = selectedModel
	}
	if mc.HaikuModel != "" {
		env["ANTHROPIC_DEFAULT_HAIKU_MODEL"] = mc.HaikuModel
	} else {
		env["ANTHROPIC_DEFAULT_HAIKU_MODEL"] = selectedModel
	}
	if mc.SonnetModel != "" {
		env["ANTHROPIC_DEFAULT_SONNET_MODEL"] = mc.SonnetModel
	} else {
		env["ANTHROPIC_DEFAULT_SONNET_MODEL"] = selectedModel
	}

	settings["env"] = env

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return fileutil.WriteFileAtomic(path, out, 0600)
}

func (p *ClaudeProvider) ExportEnv(mc config.ModelConfig, selectedModel string) []string {
	quote := func(s string) string {
		return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
	}

	exports := []string{
		fmt.Sprintf("export ANTHROPIC_BASE_URL=%s", quote(mc.BaseURL)),
		fmt.Sprintf("export ANTHROPIC_API_KEY=%s", quote(mc.APIKey)),
	}

	// 设置模型类型映射（优先使用配置中指定的，否则使用选中模型自动检测）
	if mc.OpusModel != "" {
		exports = append(exports, fmt.Sprintf("export ANTHROPIC_DEFAULT_OPUS_MODEL=%s", quote(mc.OpusModel)))
	} else {
		exports = append(exports, fmt.Sprintf("export ANTHROPIC_DEFAULT_OPUS_MODEL=%s", quote(selectedModel)))
	}
	if mc.HaikuModel != "" {
		exports = append(exports, fmt.Sprintf("export ANTHROPIC_DEFAULT_HAIKU_MODEL=%s", quote(mc.HaikuModel)))
	} else {
		exports = append(exports, fmt.Sprintf("export ANTHROPIC_DEFAULT_HAIKU_MODEL=%s", quote(selectedModel)))
	}
	if mc.SonnetModel != "" {
		exports = append(exports, fmt.Sprintf("export ANTHROPIC_DEFAULT_SONNET_MODEL=%s", quote(mc.SonnetModel)))
	} else {
		exports = append(exports, fmt.Sprintf("export ANTHROPIC_DEFAULT_SONNET_MODEL=%s", quote(selectedModel)))
	}

	return exports
}

func (p *ClaudeProvider) CustomAddFlags(cmd *cobra.Command) {
	cmd.Flags().String("opus-model", "", "Opus 模型名称（默认使用 models 第一个）")
	cmd.Flags().String("haiku-model", "", "Haiku 模型名称（默认使用 models 第一个）")
	cmd.Flags().String("sonnet-model", "", "Sonnet 模型名称（默认使用 models 第一个）")
}

func (p *ClaudeProvider) ProcessAddConfig(cmd *cobra.Command, mc *config.ModelConfig, _ string, interactive bool) {
	// 获取第一个模型作为默认值
	var defaultModel string
	if len(mc.Models) > 0 {
		defaultModel = mc.Models[0]
	}

	if interactive {
		// 交互模式：询问是否配置模型类型映射
		promptModelTypes := promptui.Prompt{
			Label:   "是否配置模型类型映射（opus/haiku/sonnet）（y/n，默认 n）",
			Default: "n",
		}
		result, err := promptModelTypes.Run()
		if err == nil && strings.ToLower(strings.TrimSpace(result)) == "y" {
			mc.OpusModel = optionalPromptWithDefault("Opus 模型名称", defaultModel)
			mc.HaikuModel = optionalPromptWithDefault("Haiku 模型名称", defaultModel)
			mc.SonnetModel = optionalPromptWithDefault("Sonnet 模型名称", defaultModel)
			return
		}
		// 不配置则使用默认值
		mc.OpusModel = defaultModel
		mc.HaikuModel = defaultModel
		mc.SonnetModel = defaultModel
		return
	}

	// 非交互模式：从 flags 获取，未设置则使用默认值
	if v, err := cmd.Flags().GetString("opus-model"); err == nil {
		if v != "" {
			mc.OpusModel = v
		} else {
			mc.OpusModel = defaultModel
		}
	}
	if v, err := cmd.Flags().GetString("haiku-model"); err == nil {
		if v != "" {
			mc.HaikuModel = v
		} else {
			mc.HaikuModel = defaultModel
		}
	}
	if v, err := cmd.Flags().GetString("sonnet-model"); err == nil {
		if v != "" {
			mc.SonnetModel = v
		} else {
			mc.SonnetModel = defaultModel
		}
	}
}

// optionalPromptWithDefault 带默认值的可选输入提示
func optionalPromptWithDefault(label, defaultVal string) string {
	prompt := promptui.Prompt{
		Label:   label + fmt.Sprintf("（默认 %s）", defaultVal),
		Default: defaultVal,
	}
	result, err := prompt.Run()
	if err != nil {
		return defaultVal
	}
	v := strings.TrimSpace(result)
	if v == "" {
		return defaultVal
	}
	return v
}

func (p *ClaudeProvider) SwitchSuccessMsg(alias string) string {
	return fmt.Sprintf("已切换 Claude 配置到 %s，写入 %s", alias, p.settingsPath())
}
