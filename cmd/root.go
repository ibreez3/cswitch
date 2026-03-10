package cmd

import (
	"fmt"
	"os"

	"github.com/ibreez3/cswitch/config"
	"github.com/ibreez3/cswitch/provider"
	"github.com/spf13/cobra"
)

var version = "dev"

// NewRootCmd 创建根命令
func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:     "cswitch",
		Short:   "轻量化模型切换工具",
		Version: version,
	}

	rootCmd.AddCommand(NewInitCmd())
	rootCmd.AddCommand(NewVersionCmd())

	// 为每个已注册的 provider 创建子命令
	for _, p := range provider.All() {
		rootCmd.AddCommand(NewToolCmd(p))
	}

	return rootCmd
}

// NewVersionCmd 创建版本命令
func NewVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "显示版本信息",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("cswitch %s\n", version)
		},
	}
}

// Execute 执行命令
func Execute() {
	rootCmd := NewRootCmd()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "错误：%v\n", err)
		os.Exit(1)
	}
}

// loadConfig 加载配置并确保工具存在
func loadConfigWithTool(p provider.Provider) (*config.Config, config.ToolConfig, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, config.ToolConfig{}, err
	}
	config.EnsureTool(cfg, p.Name())
	tc := cfg.Tools[p.Name()]
	return cfg, tc, nil
}
