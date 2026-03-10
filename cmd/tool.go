package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/ibreez3/cswitch/config"
	"github.com/ibreez3/cswitch/internal/fileutil"
	"github.com/ibreez3/cswitch/provider"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

// NewToolCmd 创建工具子命令
func NewToolCmd(p provider.Provider) *cobra.Command {
	toolCmd := &cobra.Command{
		Use:   p.Name(),
		Short: p.ShortDesc(),
	}

	toolCmd.AddCommand(newAddCmd(p))
	toolCmd.AddCommand(newSwitchCmd(p))
	toolCmd.AddCommand(newListCmd(p))
	toolCmd.AddCommand(newCurrentCmd(p))
	toolCmd.AddCommand(newDeleteCmd(p))
	toolCmd.AddCommand(newRollbackCmd(p))

	return toolCmd
}

// newAddCmd 创建添加命令
func newAddCmd(p provider.Provider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add [别名]",
		Short: "添加模型配置",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cfg, tc, err := loadConfigWithTool(p)
			if err != nil {
				fmt.Fprintf(os.Stderr, "错误：无法加载配置: %v\n", err)
				os.Exit(1)
			}

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
					timeout = optionalIntPrompt("请输入 timeout（默认 0）", "0")
					maxTokens = optionalIntPrompt("请输入 max_tokens（默认 0）", "0")
				}
			}

			models, err := parseModels(modelsStr)
			if err != nil {
				fmt.Fprintf(os.Stderr, "错误：模型列表格式无效: %v\n", err)
				os.Exit(1)
			}

			mc := config.ModelConfig{
				BaseURL:   baseURL,
				APIKey:    apiKey,
				Models:    models,
				Timeout:   timeout,
				MaxTokens: maxTokens,
			}

			// 让 provider 处理特殊配置
			p.ProcessAddConfig(cmd, &mc, alias, interactive)

			tc.Models[alias] = mc
			cfg.Tools[p.Name()] = tc

			if err := config.SaveConfig(cfg); err != nil {
				fmt.Fprintf(os.Stderr, "错误：无法保存配置: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("模型 %s 添加成功！\n", alias)
		},
	}

	// 通用 flags
	cmd.Flags().String("base-url", "", "API 基础地址")
	cmd.Flags().String("api-key", "", "API Key")
	cmd.Flags().String("models", "", "模型列表（JSON 数组格式）")

	// provider 特殊 flags
	p.CustomAddFlags(cmd)

	return cmd
}

// newSwitchCmd 创建切换命令
func newSwitchCmd(p provider.Provider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "switch <模型别名>",
		Short: "切换模型配置",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cfg, tc, err := loadConfigWithTool(p)
			if err != nil {
				fmt.Fprintf(os.Stderr, "错误：无法加载配置: %v\n", err)
				os.Exit(1)
			}

			alias := args[0]
			mc, ok := tc.Models[alias]
			if !ok {
				fmt.Fprintf(os.Stderr, "错误：模型 %s 不存在\n", alias)
				os.Exit(1)
			}

			// selectedModel 作为兼容性保留，优先使用 opus_model，否则用 models[0]
			selectedModel := mc.OpusModel
			if selectedModel == "" && len(mc.Models) > 0 {
				selectedModel = mc.Models[0]
			}

			envOnly, _ := cmd.Flags().GetBool("env")
			if envOnly {
				exports := p.ExportEnv(mc, selectedModel)
				// 输出到 stdout
				for _, line := range exports {
					fmt.Println(line)
				}
				// 同时写入 env 文件
				envPath := p.EnvFilePath()
				if envPath != "" {
					envContent := strings.Join(exports, "\n") + "\n"
					if err := fileutil.WriteFileAtomic(envPath, []byte(envContent), 0600); err != nil {
						fmt.Fprintf(os.Stderr, "警告：无法写入 env 文件: %v\n", err)
					}
				}
			} else {
				if err := fileutil.BackupFiles(p.NativeFiles()); err != nil {
					fmt.Fprintf(os.Stderr, "错误：%v\n", err)
					os.Exit(1)
				}
				if err := p.WriteLive(mc, selectedModel); err != nil {
					fmt.Fprintf(os.Stderr, "错误：写入配置失败: %v\n", err)
					os.Exit(1)
				}
				fmt.Println(p.SwitchSuccessMsg(alias))
			}

			tc.Current = alias
			tc.CurrentModel = selectedModel
			cfg.Tools[p.Name()] = tc
			if err := config.SaveConfig(cfg); err != nil {
				fmt.Fprintf(os.Stderr, "错误：无法保存配置: %v\n", err)
				os.Exit(1)
			}
		},
	}

	cmd.Flags().Bool("env", false, "仅输出环境变量，不写入原生配置文件")
	return cmd
}

// newListCmd 创建列表命令
func newListCmd(p provider.Provider) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "列出所有模型配置",
		Run: func(cmd *cobra.Command, args []string) {
			_, tc, err := loadConfigWithTool(p)
			if err != nil {
				fmt.Fprintf(os.Stderr, "错误：无法加载配置: %v\n", err)
				os.Exit(1)
			}

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
}

// newCurrentCmd 创建当前配置命令
func newCurrentCmd(p provider.Provider) *cobra.Command {
	return &cobra.Command{
		Use:   "current",
		Short: "查看当前生效的模型",
		Run: func(cmd *cobra.Command, args []string) {
			_, tc, err := loadConfigWithTool(p)
			if err != nil {
				fmt.Fprintf(os.Stderr, "错误：无法加载配置: %v\n", err)
				os.Exit(1)
			}

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
}

// newDeleteCmd 创建删除命令
func newDeleteCmd(p provider.Provider) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <模型别名>",
		Short: "删除模型配置",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cfg, tc, err := loadConfigWithTool(p)
			if err != nil {
				fmt.Fprintf(os.Stderr, "错误：无法加载配置: %v\n", err)
				os.Exit(1)
			}

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
			cfg.Tools[p.Name()] = tc

			if err := config.SaveConfig(cfg); err != nil {
				fmt.Fprintf(os.Stderr, "错误：无法保存配置: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("模型 %s 已删除\n", alias)
		},
	}
}

// newRollbackCmd 创建回滚命令
func newRollbackCmd(p provider.Provider) *cobra.Command {
	return &cobra.Command{
		Use:   "rollback",
		Short: "从备份恢复原生配置文件",
		Run: func(cmd *cobra.Command, args []string) {
			files := p.NativeFiles()
			hasBackup := false
			for _, f := range files {
				if _, err := fileutil.LatestBackup(f); err == nil {
					hasBackup = true
					break
				}
			}
			if !hasBackup {
				fmt.Fprintf(os.Stderr, "错误：没有找到 %s 的任何备份文件\n", p.Name())
				os.Exit(1)
			}

			restored := 0
			for _, f := range files {
				bakPath, err := fileutil.RestoreFromBackup(f)
				if err != nil {
					fmt.Fprintf(os.Stderr, "跳过 %s：%v\n", f, err)
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
}

// 辅助函数
func requiredInputPrompt(label string, mask rune) (string, error) {
	prompt := promptui.Prompt{
		Label: label,
		Mask:  mask,
		Validate: func(input string) error {
			if strings.TrimSpace(input) == "" {
				return errors.New("该参数为必填，请重新输入")
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

func optionalIntPrompt(label, defaultVal string) int {
	prompt := promptui.Prompt{Label: label, Default: defaultVal}
	if v, err := prompt.Run(); err == nil {
		if i, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return i
		}
	}
	return 0
}

func parseModels(modelsStr string) ([]string, error) {
	var models []string
	if err := json.Unmarshal([]byte(modelsStr), &models); err != nil {
		return nil, err
	}
	return models, nil
}

func maskAPIKey(key string) string {
	if len(key) <= 6 {
		return strings.Repeat("*", len(key))
	}
	return key[:3] + "****" + key[len(key)-3:]
}
