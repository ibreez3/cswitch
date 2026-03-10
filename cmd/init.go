package cmd

import (
	"fmt"
	"os"

	"github.com/ibreez3/cswitch/config"
	"github.com/ibreez3/cswitch/provider"
	"github.com/spf13/cobra"
)

// NewInitCmd 创建初始化命令
func NewInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "初始化配置目录",
		Run: func(cmd *cobra.Command, args []string) {
			dir := config.ConfigDir()
			if err := os.MkdirAll(dir, 0755); err != nil {
				fmt.Fprintf(os.Stderr, "错误：无法创建目录 %s: %v\n", dir, err)
				os.Exit(1)
			}
			cfg := config.DefaultConfig()
			if err := config.SaveConfig(cfg); err != nil {
				fmt.Fprintf(os.Stderr, "错误：无法创建配置文件: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("✓ 配置目录已创建：%s\n", dir)

			// 为所有 provider 设置 shell 钩子
			for _, p := range provider.All() {
				added, err := p.SetupShellHook()
				if err != nil {
					fmt.Fprintf(os.Stderr, "警告：无法为 %s 设置 shell 钩子: %v\n", p.Name(), err)
				} else if added {
					fmt.Printf("✓ 已添加 %s 快捷函数到 shell 配置\n", p.Name())
				}
			}

			// 显示使用说明
			fmt.Println("\n使用说明：")
			fmt.Println("  1. 添加配置: cswitch claude add")
			fmt.Println("  2. 切换配置: cswitch claude switch <别名>")
			fmt.Println("  3. 快捷切换: cse <别名>  (切换并立即生效)")
			fmt.Println("\n首次使用快捷函数，请执行: source ~/.zshrc (或 ~/.bashrc)")
		},
	}
}
