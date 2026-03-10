package provider

import (
	"github.com/ibreez3/cswitch/config"
	"github.com/spf13/cobra"
)

// Provider 定义 AI 工具提供者接口
// 参考 cobra 的 Command 设计模式，每个 Provider 负责自己的配置写入和环境变量导出
type Provider interface {
	// Name 返回 provider 名称（如 claude, codex）
	Name() string

	// ShortDesc 返回简短描述
	ShortDesc() string

	// WriteLive 写入配置到工具原生文件
	WriteLive(mc config.ModelConfig, selectedModel string) error

	// NativeFiles 返回需要备份的原生文件列表
	NativeFiles() []string

	// ExportEnv 返回环境变量导出命令
	ExportEnv(mc config.ModelConfig, selectedModel string) []string

	// EnvFilePath 返回 env 文件路径（用于 --env 模式写入）
	// 返回空字符串表示不需要写入文件
	EnvFilePath() string

	// SetupShellHook 在 --env 模式下设置 shell 钩子（如添加快捷函数到 rc 文件）
	// 返回是否添加了新的钩子，以及可能的错误
	SetupShellHook() (added bool, err error)

	// CustomAddFlags 添加 provider 特有的 add 命令 flags（可选）
	// 返回 nil 表示没有特殊 flags
	CustomAddFlags(cmd *cobra.Command)

	// ProcessAddConfig 处理 add 命令的 provider 特有配置
	// interactive 表示是否为交互模式
	ProcessAddConfig(cmd *cobra.Command, mc *config.ModelConfig, alias string, interactive bool)

	// SwitchSuccessMsg 返回切换成功后的提示消息
	SwitchSuccessMsg(alias string) string
}

// Registry 全局 provider 注册表
var Registry = make(map[string]Provider)

// Register 注册 provider
func Register(p Provider) {
	Registry[p.Name()] = p
}

// Get 获取 provider
func Get(name string) (Provider, bool) {
	p, ok := Registry[name]
	return p, ok
}

// All 返回所有已注册的 provider
func All() []Provider {
	providers := make([]Provider, 0, len(Registry))
	for _, p := range Registry {
		providers = append(providers, p)
	}
	return providers
}

// Names 返回所有已注册的 provider 名称
func Names() []string {
	names := make([]string, 0, len(Registry))
	for name := range Registry {
		names = append(names, name)
	}
	return names
}
