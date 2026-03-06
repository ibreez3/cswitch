package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var version = "dev"
const configVersion = 2

type ModelConfig struct {
	BaseURL      string   `json:"base_url"`
	APIKey       string   `json:"api_key"`
	Models       []string `json:"models"`
	Timeout      int      `json:"timeout,omitempty"`
	MaxTokens    int      `json:"max_tokens,omitempty"`
	ProviderName string   `json:"provider_name,omitempty"`
	WireAPI      string   `json:"wire_api,omitempty"`
}

type Config struct {
	Version int                   `json:"version"`
	Tools   map[string]ToolConfig `json:"tools"`
}

type ToolConfig struct {
	Current      string                 `json:"current"`
	CurrentModel string                 `json:"current_model,omitempty"`
	Models       map[string]ModelConfig `json:"models"`
}

type LegacyConfig struct {
	Current string                 `json:"current"`
	Models  map[string]ModelConfig `json:"models"`
}

type ToolDef struct {
	Name        string
	Short       string
	WriteLive   func(mc ModelConfig, selectedModel string) error
	NativeFiles func() []string
}

func configDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cswitch")
}

func configPath() string {
	return filepath.Join(configDir(), "config.json")
}

func claudeSettingsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "settings.json")
}

func codexAuthPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".codex", "auth.json")
}

func codexConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".codex", "config.toml")
}

func codexEnvPath() string {
	return filepath.Join(configDir(), "codex.env")
}

func detectShellRC() string {
	home, _ := os.UserHomeDir()
	shell := os.Getenv("SHELL")
	switch {
	case strings.HasSuffix(shell, "/zsh"):
		return filepath.Join(home, ".zshrc")
	case strings.HasSuffix(shell, "/bash"):
		if rc := filepath.Join(home, ".bashrc"); fileExists(rc) {
			return rc
		}
		return filepath.Join(home, ".bash_profile")
	default:
		return filepath.Join(home, ".profile")
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func ensureSourceLine(rcPath, envPath string) (bool, error) {
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

func writeCodexEnv(apiKey string) error {
	envPath := codexEnvPath()
	if err := os.MkdirAll(filepath.Dir(envPath), 0755); err != nil {
		return err
	}
	content := fmt.Sprintf("export OPENAI_API_KEY='%s'\n", strings.ReplaceAll(apiKey, "'", `'"'"'`))
	return writeFileAtomic(envPath, []byte(content), 0600)
}

func defaultConfig() *Config {
	return &Config{
		Version: configVersion,
		Tools: map[string]ToolConfig{
			"claude": {
				Current: "",
				Models:  make(map[string]ModelConfig),
			},
			"codex": {
				Current: "",
				Models:  make(map[string]ModelConfig),
			},
		},
	}
}

func ensureTool(cfg *Config, tool string) {
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

func loadConfig() (*Config, error) {
	path := configPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultConfig(), nil
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
			cfg.Version = configVersion
		}
		ensureTool(&cfg, "claude")
		ensureTool(&cfg, "codex")
		return &cfg, nil
	}

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

	migrated := defaultConfig()
	tc := migrated.Tools["claude"]
	tc.Current = legacy.Current
	if legacy.Models != nil {
		tc.Models = legacy.Models
	}
	migrated.Tools["claude"] = tc
	if err := os.WriteFile(path+".bak", data, 0600); err != nil {
		return nil, fmt.Errorf("迁移前备份失败: %w", err)
	}
	if err := saveConfig(migrated); err != nil {
		return nil, err
	}
	return migrated, nil
}

func backupFile(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}

	bakPath := path + ".bak"
	if _, err := os.Stat(bakPath); os.IsNotExist(err) {
		return copyFile(path, bakPath)
	}

	for i := 1; ; i++ {
		candidate := fmt.Sprintf("%s.bak.%d", path, i)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return copyFile(path, candidate)
		}
	}
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, info.Mode())
}

func latestBackup(path string) (string, error) {
	bakPath := path + ".bak"
	if _, err := os.Stat(bakPath); os.IsNotExist(err) {
		return "", fmt.Errorf("没有找到 %s 的备份文件", filepath.Base(path))
	}

	latest := bakPath
	for i := 1; ; i++ {
		candidate := fmt.Sprintf("%s.bak.%d", path, i)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			break
		}
		latest = candidate
	}
	return latest, nil
}

func restoreFromBackup(path string) (string, error) {
	bakPath, err := latestBackup(path)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(bakPath)
	if err != nil {
		return "", fmt.Errorf("无法读取备份 %s: %w", bakPath, err)
	}
	info, err := os.Stat(bakPath)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, info.Mode()); err != nil {
		return "", fmt.Errorf("无法恢复到 %s: %w", path, err)
	}
	if err := os.Remove(bakPath); err != nil {
		fmt.Fprintf(os.Stderr, "警告：无法删除已恢复的备份 %s: %v\n", bakPath, err)
	}
	return bakPath, nil
}

func backupNativeFiles(files []string) error {
	for _, f := range files {
		if err := backupFile(f); err != nil {
			return fmt.Errorf("备份 %s 失败: %w", f, err)
		}
	}
	return nil
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

func saveConfig(cfg *Config) error {
	path := configPath()
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, data, 0600)
}

func maskAPIKey(key string) string {
	if len(key) <= 6 {
		return strings.Repeat("*", len(key))
	}
	return key[:3] + "****" + key[len(key)-3:]
}

func parseModels(modelsStr string) ([]string, error) {
	var models []string
	if err := json.Unmarshal([]byte(modelsStr), &models); err != nil {
		return nil, err
	}
	return models, nil
}

func chooseModel(mc ModelConfig, requested string) (string, error) {
	if len(mc.Models) == 0 {
		return "", errors.New("模型列表为空")
	}
	if requested == "" {
		return mc.Models[0], nil
	}
	for _, m := range mc.Models {
		if m == requested {
			return m, nil
		}
	}
	return "", fmt.Errorf("模型 %s 不存在，可用模型：%s", requested, strings.Join(mc.Models, ", "))
}

func writeClaudeLive(mc ModelConfig, selectedModel string) error {
	path := claudeSettingsPath()
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
	env["ANTHROPIC_BASE_URL"] = mc.BaseURL
	env["ANTHROPIC_API_KEY"] = mc.APIKey
	env["ANTHROPIC_MODEL"] = selectedModel
	settings["env"] = env

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, out, 0600)
}

func writeCodexLive(mc ModelConfig, selectedModel string) error {
	authPath := codexAuthPath()
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
	if err := writeFileAtomic(authPath, authData, 0600); err != nil {
		return err
	}

	cfgPath := codexConfigPath()
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

	if err := writeTomlAtomic(cfgPath, cfg, 0644); err != nil {
		return err
	}

	if err := writeCodexEnv(mc.APIKey); err != nil {
		return fmt.Errorf("写入 codex.env 失败: %w", err)
	}
	rcPath := detectShellRC()
	added, err := ensureSourceLine(rcPath, codexEnvPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "警告：无法写入 %s: %v\n", rcPath, err)
		fmt.Fprintf(os.Stderr, "请手动添加: source %s\n", codexEnvPath())
	} else if added {
		fmt.Fprintf(os.Stderr, "已在 %s 中添加 source %s\n", rcPath, codexEnvPath())
	}

	return nil
}

func writeTomlAtomic(path string, data map[string]interface{}, mode os.FileMode) error {
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

func exportForTool(tool string, mc ModelConfig, selectedModel string) []string {
	quote := func(s string) string {
		return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
	}
	switch tool {
	case "claude":
		return []string{
			fmt.Sprintf("export ANTHROPIC_BASE_URL=%s", quote(mc.BaseURL)),
			fmt.Sprintf("export ANTHROPIC_API_KEY=%s", quote(mc.APIKey)),
			fmt.Sprintf("export ANTHROPIC_MODEL=%s", quote(selectedModel)),
		}
	case "codex":
		return []string{
			fmt.Sprintf("export OPENAI_BASE_URL=%s", quote(mc.BaseURL)),
			fmt.Sprintf("export OPENAI_API_KEY=%s", quote(mc.APIKey)),
			fmt.Sprintf("# Codex 模型请使用: codex --model %s", selectedModel),
		}
	default:
		return nil
	}
}

func requiredInputPrompt(label string, mask rune) (string, error) {
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

func main() {
	var rootCmd = &cobra.Command{
		Use:     "cswitch",
		Short:   "轻量化模型切换工具",
		Version: version,
	}

	tools := []ToolDef{
		{
			Name:      "claude",
			Short:     "管理 Claude Code 配置",
			WriteLive: writeClaudeLive,
			NativeFiles: func() []string {
				return []string{claudeSettingsPath()}
			},
		},
		{
			Name:      "codex",
			Short:     "管理 Codex 配置",
			WriteLive: writeCodexLive,
			NativeFiles: func() []string {
				return []string{codexAuthPath(), codexConfigPath(), codexEnvPath()}
			},
		},
	}

	var initCmd = &cobra.Command{
		Use:   "init",
		Short: "初始化配置目录",
		Run: func(cmd *cobra.Command, args []string) {
			dir := configDir()
			if err := os.MkdirAll(dir, 0755); err != nil {
				fmt.Fprintf(os.Stderr, "错误：无法创建目录 %s: %v\n", dir, err)
				os.Exit(1)
			}
			// 创建空的 config.json
			cfg := defaultConfig()
			if err := saveConfig(cfg); err != nil {
				fmt.Fprintf(os.Stderr, "错误：无法创建配置文件: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("初始化完成：%s\n", dir)
		},
	}

	var versionCmd = &cobra.Command{
		Use:   "version",
		Short: "显示版本信息",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("cswitch %s\n", version)
		},
	}

	rootCmd.AddCommand(initCmd, versionCmd)

	for _, toolDef := range tools {
		td := toolDef
		toolCmd := &cobra.Command{
			Use:   td.Name,
			Short: td.Short,
		}

		addCmd := &cobra.Command{
			Use:   "add [别名]",
			Short: "添加模型配置",
			Args:  cobra.MaximumNArgs(1),
			Run: func(cmd *cobra.Command, args []string) {
				cfg, err := loadConfig()
				if err != nil {
					fmt.Fprintf(os.Stderr, "错误：无法加载配置: %v\n", err)
					os.Exit(1)
				}
				ensureTool(cfg, td.Name)
				tc := cfg.Tools[td.Name]

				alias := ""
				if len(args) > 0 {
					alias = strings.TrimSpace(args[0])
				}

				baseURL, _ := cmd.Flags().GetString("base-url")
				apiKey, _ := cmd.Flags().GetString("api-key")
				modelsStr, _ := cmd.Flags().GetString("models")
				interactive := baseURL == "" || apiKey == "" || modelsStr == ""

				if !interactive && alias == "" {
					fmt.Fprintln(os.Stderr, "错误：非交互模式下必须提供模型别名")
					os.Exit(1)
				}

				timeout := 0
				maxTokens := 0
				if interactive {
					var err error
					if alias == "" {
						alias, err = requiredInputPrompt("请输入模型别名（唯一标识，如 sonnet4.6）", 0)
						if err != nil {
							fmt.Fprintf(os.Stderr, "错误：%v\n", err)
							os.Exit(1)
						}
					}
					if baseURL == "" {
						baseURL, err = requiredInputPrompt("请输入 API 基础地址（必填）", 0)
						if err != nil {
							fmt.Fprintf(os.Stderr, "错误：%v\n", err)
							os.Exit(1)
						}
					}
					if apiKey == "" {
						apiKey, err = requiredInputPrompt("请输入 API Key（必填，输入时隐藏显示）", '*')
						if err != nil {
							fmt.Fprintf(os.Stderr, "错误：%v\n", err)
							os.Exit(1)
						}
					}
					if modelsStr == "" {
						modelsStr, err = requiredInputPrompt("请输入模型列表（必填，格式如 [\"claude-sonnet-4\"]）", 0)
						if err != nil {
							fmt.Fprintf(os.Stderr, "错误：%v\n", err)
							os.Exit(1)
						}
					}
					promptOptional := promptui.Prompt{
						Label:   "是否需要配置可选参数（y/n，默认 n）",
						Default: "n",
					}
					result, _ := promptOptional.Run()
					if strings.ToLower(strings.TrimSpace(result)) == "y" {
						promptTimeout := promptui.Prompt{Label: "请输入 timeout（默认 0）", Default: "0"}
						if timeoutStr, err := promptTimeout.Run(); err == nil {
							if v, err := strconv.Atoi(strings.TrimSpace(timeoutStr)); err == nil {
								timeout = v
							}
						}
						promptMaxTokens := promptui.Prompt{Label: "请输入 max_tokens（默认 0）", Default: "0"}
						if maxStr, err := promptMaxTokens.Run(); err == nil {
							if v, err := strconv.Atoi(strings.TrimSpace(maxStr)); err == nil {
								maxTokens = v
							}
						}
					}
				}

				models, err := parseModels(modelsStr)
				if err != nil {
					fmt.Fprintf(os.Stderr, "错误：模型列表格式无效: %v\n", err)
					os.Exit(1)
				}

				providerName, _ := cmd.Flags().GetString("provider-name")
				wireAPI, _ := cmd.Flags().GetString("wire-api")
				if td.Name == "codex" {
					if interactive && providerName == "" {
						p := promptui.Prompt{
							Label:   "请输入 Provider 名称（Codex model_provider，留空自动生成）",
							Default: "cswitch_" + alias,
						}
						if v, err := p.Run(); err == nil && strings.TrimSpace(v) != "" {
							providerName = strings.TrimSpace(v)
						}
					}
					if providerName == "" {
						providerName = "cswitch_" + alias
					}
					if interactive && wireAPI == "" {
						p := promptui.Prompt{
							Label:   "请输入 wire_api（chat=Chat Completions / responses=Responses API，默认 chat）",
							Default: "chat",
						}
						if v, err := p.Run(); err == nil && strings.TrimSpace(v) != "" {
							wireAPI = strings.TrimSpace(v)
						}
					}
				}

				tc.Models[alias] = ModelConfig{
					BaseURL:      baseURL,
					APIKey:       apiKey,
					Models:       models,
					Timeout:      timeout,
					MaxTokens:    maxTokens,
					ProviderName: providerName,
					WireAPI:      wireAPI,
				}
				cfg.Tools[td.Name] = tc

				if err := saveConfig(cfg); err != nil {
					fmt.Fprintf(os.Stderr, "错误：无法保存配置: %v\n", err)
					os.Exit(1)
				}
				fmt.Printf("模型 %s 添加成功！\n", alias)
			},
		}
		addCmd.Flags().String("base-url", "", "API 基础地址")
		addCmd.Flags().String("api-key", "", "API Key")
		addCmd.Flags().String("models", "", "模型列表（JSON 数组格式）")
		addCmd.Flags().String("provider-name", "", "Codex Provider 名称（仅 Codex 有效）")
		addCmd.Flags().String("wire-api", "", "Codex wire_api（chat 或 responses，默认 chat）")

		switchCmd := &cobra.Command{
			Use:   "switch <模型别名> [模型名称]",
			Short: "切换模型配置",
			Args:  cobra.RangeArgs(1, 2),
			Run: func(cmd *cobra.Command, args []string) {
				cfg, err := loadConfig()
				if err != nil {
					fmt.Fprintf(os.Stderr, "错误：无法加载配置: %v\n", err)
					os.Exit(1)
				}
				ensureTool(cfg, td.Name)
				tc := cfg.Tools[td.Name]

				alias := args[0]
				mc, ok := tc.Models[alias]
				if !ok {
					fmt.Fprintf(os.Stderr, "错误：模型 %s 不存在\n", alias)
					os.Exit(1)
				}
				requestedModel := ""
				if len(args) > 1 {
					requestedModel = strings.TrimSpace(args[1])
				}
				selectedModel, err := chooseModel(mc, requestedModel)
				if err != nil {
					fmt.Fprintf(os.Stderr, "错误：%v\n", err)
					os.Exit(1)
				}

				if td.Name == "codex" && mc.ProviderName == "" {
					mc.ProviderName = alias
				}

				envOnly, _ := cmd.Flags().GetBool("env")
				if envOnly {
					for _, line := range exportForTool(td.Name, mc, selectedModel) {
						fmt.Println(line)
					}
				} else {
					if err := backupNativeFiles(td.NativeFiles()); err != nil {
						fmt.Fprintf(os.Stderr, "错误：%v\n", err)
						os.Exit(1)
					}
					if err := td.WriteLive(mc, selectedModel); err != nil {
						fmt.Fprintf(os.Stderr, "错误：写入配置失败: %v\n", err)
						os.Exit(1)
					}
					switch td.Name {
					case "claude":
						fmt.Printf("已切换 Claude 配置（模型: %s），写入 %s\n", selectedModel, claudeSettingsPath())
					case "codex":
						fmt.Printf("已切换 Codex 配置（模型: %s），写入 %s 和 %s\n", selectedModel, codexAuthPath(), codexConfigPath())
						fmt.Fprintf(os.Stderr, "环境变量 OPENAI_API_KEY 已写入 %s，新终端自动生效\n", codexEnvPath())
						fmt.Fprintf(os.Stderr, "当前终端请执行: source %s\n", codexEnvPath())
					}
				}

				tc.Current = alias
				tc.CurrentModel = selectedModel
				cfg.Tools[td.Name] = tc
				if err := saveConfig(cfg); err != nil {
					fmt.Fprintf(os.Stderr, "错误：无法保存配置: %v\n", err)
					os.Exit(1)
				}
			},
		}
		switchCmd.Flags().Bool("env", false, "仅输出环境变量，不写入原生配置文件")

		listCmd := &cobra.Command{
			Use:   "list",
			Short: "列出所有模型配置",
			Run: func(cmd *cobra.Command, args []string) {
				cfg, err := loadConfig()
				if err != nil {
					fmt.Fprintf(os.Stderr, "错误：无法加载配置: %v\n", err)
					os.Exit(1)
				}
				ensureTool(cfg, td.Name)
				tc := cfg.Tools[td.Name]

				if len(tc.Models) == 0 {
					fmt.Println("暂无模型配置，请先执行 add")
					return
				}
				names := make([]string, 0, len(tc.Models))
				for name := range tc.Models {
					names = append(names, name)
				}
				sort.Strings(names)
				for _, name := range names {
					if name == tc.Current {
						fmt.Printf("* %s (current)\n", name)
					} else {
						fmt.Printf("  %s\n", name)
					}
				}
			},
		}

		currentCmd := &cobra.Command{
			Use:   "current",
			Short: "查看当前生效的模型",
			Run: func(cmd *cobra.Command, args []string) {
				cfg, err := loadConfig()
				if err != nil {
					fmt.Fprintf(os.Stderr, "错误：无法加载配置: %v\n", err)
					os.Exit(1)
				}
				ensureTool(cfg, td.Name)
				tc := cfg.Tools[td.Name]
				if tc.Current == "" {
					fmt.Println("当前未切换任何模型")
					return
				}
				mc, ok := tc.Models[tc.Current]
				if !ok {
					fmt.Println("当前未切换任何模型")
					return
				}
				modelsJSON, _ := json.Marshal(mc.Models)
				fmt.Printf("当前模型：%s\n", tc.Current)
				fmt.Printf("Base URL：%s\n", mc.BaseURL)
				fmt.Printf("API Key：%s\n", maskAPIKey(mc.APIKey))
				fmt.Printf("Models：%s\n", string(modelsJSON))
				if tc.CurrentModel != "" {
					fmt.Printf("当前使用：%s\n", tc.CurrentModel)
				}
			},
		}

		deleteCmd := &cobra.Command{
			Use:   "delete <模型别名>",
			Short: "删除模型配置",
			Args:  cobra.ExactArgs(1),
			Run: func(cmd *cobra.Command, args []string) {
				cfg, err := loadConfig()
				if err != nil {
					fmt.Fprintf(os.Stderr, "错误：无法加载配置: %v\n", err)
					os.Exit(1)
				}
				ensureTool(cfg, td.Name)
				tc := cfg.Tools[td.Name]
				alias := args[0]
				if _, ok := tc.Models[alias]; !ok {
					fmt.Fprintf(os.Stderr, "错误：模型 %s 不存在\n", alias)
					os.Exit(1)
				}
				delete(tc.Models, alias)
				if tc.Current == alias {
					tc.Current = ""
					tc.CurrentModel = ""
				}
				cfg.Tools[td.Name] = tc
				if err := saveConfig(cfg); err != nil {
					fmt.Fprintf(os.Stderr, "错误：无法保存配置: %v\n", err)
					os.Exit(1)
				}
				fmt.Printf("模型 %s 已删除\n", alias)
			},
		}

		rollbackCmd := &cobra.Command{
			Use:   "rollback",
			Short: "从备份恢复原生配置文件",
			Run: func(cmd *cobra.Command, args []string) {
				files := td.NativeFiles()
				hasBackup := false
				for _, f := range files {
					if _, err := latestBackup(f); err == nil {
						hasBackup = true
						break
					}
				}
				if !hasBackup {
					fmt.Fprintf(os.Stderr, "错误：没有找到 %s 的任何备份文件\n", td.Name)
					os.Exit(1)
				}

				restored := 0
				for _, f := range files {
					bakPath, err := restoreFromBackup(f)
					if err != nil {
						fmt.Fprintf(os.Stderr, "跳过 %s：%v\n", filepath.Base(f), err)
						continue
					}
					fmt.Printf("已恢复 %s（来自 %s）\n", f, bakPath)
					restored++
				}
				if restored == 0 {
					fmt.Fprintln(os.Stderr, "错误：未能恢复任何文件")
					os.Exit(1)
				}
				fmt.Printf("回滚完成，共恢复 %d 个文件\n", restored)
			},
		}

		toolCmd.AddCommand(addCmd, switchCmd, listCmd, currentCmd, deleteCmd, rollbackCmd)
		rootCmd.AddCommand(toolCmd)
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "错误：%v\n", err)
		os.Exit(1)
	}
}
