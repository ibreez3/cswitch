# cswitch

[![Build](https://github.com/ibreez3/cswitch/actions/workflows/release.yml/badge.svg)](https://github.com/ibreez3/cswitch/actions/workflows/release.yml)
[![Go Version](https://img.shields.io/badge/go-%3E%3D%201.21-blue)](https://golang.org/)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)

一款超轻量化的 Claude Code / Codex 模型切换 CLI 工具。

参考 [cc-switch](https://github.com/farion1231/cc-switch) 的设计思路，专注于**配置管理与快速切换**，无多余功能，单文件编译即可运行。

## 特性

- **双协议支持**：同时管理 Claude Code (Anthropic 协议) 和 Codex (OpenAI 协议)
- **原生配置写入**：直接修改各工具的配置文件，即时生效
- **交互式添加**：向导式输入，降低配置复杂度
- **模型智能选择**：支持多模型别名，自动默认或指定切换
- **自动备份回滚**：每次切换前自动备份，支持一键回滚
- **环境变量持久化**：Codex API Key 自动注入 Shell 配置
- **API 端点可配置**：支持 Chat Completions (`chat`) 和 Responses API (`responses`)
- **原子写入**：关键文件采用临时文件 + rename，防止配置损坏
- **权限安全**：敏感配置文件默认 `0600` 权限

## 安装

### 预编译二进制

从 [GitHub Releases](https://github.com/ibreez3/cswitch/releases) 下载对应平台的二进制文件：

```bash
# macOS (Apple Silicon)
curl -L -o cswitch "https://github.com/ibreez3/cswitch/releases/latest/download/cswitch-darwin-arm64"
chmod +x cswitch
sudo mv cswitch /usr/local/bin/

# macOS (Intel)
curl -L -o cswitch "https://github.com/ibreez3/cswitch/releases/latest/download/cswitch-darwin-amd64"
chmod +x cswitch
sudo mv cswitch /usr/local/bin/

# Linux (x86_64)
curl -L -o cswitch "https://github.com/ibreez3/cswitch/releases/latest/download/cswitch-linux-amd64"
chmod +x cswitch
sudo mv cswitch /usr/local/bin/

# Linux (ARM64)
curl -L -o cswitch "https://github.com/ibreez3/cswitch/releases/latest/download/cswitch-linux-arm64"
chmod +x cswitch
sudo mv cswitch /usr/local/bin/
```

### 从源码编译

```bash
git clone https://github.com/ibreez3/cswitch.git
cd cswitch
go build -ldflags="-s -w" -o cswitch .
# 可选：安装到 $GOPATH/bin
go install .
```

### 使用 Homebrew（待发布）

```bash
brew tap ibreez3/cswitch
brew install cswitch
```

## 快速开始

```bash
# 1. 初始化配置目录
cswitch init

# 2. 添加 Claude Code 配置（交互式）
cswitch claude add

# 3. 添加 Codex 配置（非交互式，适合脚本）
cswitch codex add myprovider \
  --base-url "https://api.example.com/v1" \
  --api-key "sk-xxxx" \
  --models '["gpt-4o", "gpt-4o-mini"]'

# 4. 切换到指定配置
cswitch claude switch myprovider
cswitch codex switch myprovider gpt-4o

# 5. 查看当前配置
cswitch claude current
cswitch codex current
```

## 命令详解

### 全局命令

| 命令 | 说明 |
|------|------|
| `cswitch init` | 初始化 `~/.cswitch/` 配置目录 |
| `cswitch version` | 显示版本信息 |
| `cswitch claude` | Claude Code 配置管理子命令 |
| `cswitch codex` | Codex 配置管理子命令 |

### Claude Code 子命令

```bash
cswitch claude add [别名]              # 添加模型配置（交互式/非交互式）
                                      #   --opus-model: Opus 模型名称（默认使用 models 第一个）
                                      #   --haiku-model: Haiku 模型名称（默认使用 models 第一个）
                                      #   --sonnet-model: Sonnet 模型名称（默认使用 models 第一个）
cswitch claude switch <别名>          # 切换到指定配置，写入 ~/.claude/settings.json
cswitch claude list                   # 列出所有配置
cswitch claude current                # 查看当前配置
cswitch claude delete <别名>          # 删除配置
cswitch claude rollback               # 从备份恢复配置
```

### Codex 子命令

```bash
cswitch codex add [别名]               # 添加模型配置
                                      #   --provider-name: 自定义 provider 名称
                                      #   --wire-api: chat 或 responses（默认 chat）
cswitch codex switch <别名> [模型]     # 切换到指定配置
                                      #   写入 ~/.codex/auth.json 和 config.toml
                                      #   写入 ~/.cswitch/codex.env
                                      #   自动注入 ~/.zshrc 或 ~/.bashrc
cswitch codex list                    # 列出所有配置
cswitch codex current                 # 查看当前配置
cswitch codex delete <别名>           # 删除配置
cswitch codex rollback                # 从备份恢复配置
```

## 使用示例

### 场景 1：配置 DashScope 通义千问（Claude 协议）

```bash
# 交互式添加
cswitch claude add
# ? 请输入模型别名（唯一标识，如 sonnet4.6）：dashscope
# ? 请输入 API 基础地址（必填）：https://coding.dashscope.aliyuncs.com/apps/anthropic
# ? 请输入 API Key（必填，输入时隐藏显示）：sk-xxxx
# ? 请输入模型列表（必填，格式如 ["claude-sonnet-4"]）：["qwen-coder-plus", "qwen-max"]
# ? 是否需要配置可选参数（y/n，默认 n）：n
# ? 是否配置模型类型映射（opus/haiku/sonnet）（y/n，默认 n）：y
# ? Opus 模型名称（默认 qwen-coder-plus）：qwen-max
# ? Haiku 模型名称（默认 qwen-coder-plus）：qwen-coder-plus
# ? Sonnet 模型名称（默认 qwen-coder-plus）：qwen-coder-plus
# 模型 dashscope 添加成功！

# 切换到该配置
cswitch claude switch dashscope
# 已切换 Claude 配置到 dashscope，写入 ~/.claude/settings.json
```

### 场景 2：配置智谱 AI GLM（Codex 协议）

```bash
# 非交互式添加
cswitch codex add zhipu \
  --base-url "https://open.bigmodel.cn/api/coding/paas/v4" \
  --api-key "your-api-key" \
  --models '["glm-5", "glm-4.7"]' \
  --provider-name "Z.AI" \
  --wire-api "chat"

# 切换并指定模型
cswitch codex switch zhipu glm-5
# 已切换 Codex 配置（模型: glm-5），写入 ~/.codex/auth.json 和 ~/.codex/config.toml
# 环境变量 OPENAI_API_KEY 已写入 ~/.cswitch/codex.env，新终端自动生效
# 当前终端请执行: source ~/.cswitch/codex.env
```

**关于 `wire_api`**：
- 设置为 `chat`：使用 `/chat/completions` 端点（大多数第三方厂商支持）
- 设置为 `responses`：使用 `/responses` 端点（OpenAI 原生 Responses API）

### 场景 3：备份与回滚

```bash
# 每次 switch 会自动创建备份（*.bak, *.bak.1, ...）
cswitch claude switch openai

# 如果配置出错，一键回滚到上一版本
cswitch claude rollback
# 已恢复 ~/.claude/settings.json（来自 ~/.claude/settings.json.bak）
# 回滚完成，共恢复 1 个文件

# Codex 回滚同时恢复 auth.json、config.toml 和 codex.env
cswitch codex rollback
```

### 场景 4：查看和管理配置

```bash
# 列出所有 Claude 配置
cswitch claude list
#   openai
# * dashscope (current)

# 查看当前 Claude 配置详情
cswitch claude current
# 当前模型：dashscope
# Base URL：https://coding.dashscope.aliyuncs.com/apps/anthropic
# API Key：sk-****est
# Models：["qwen-coder-plus","qwen-max"]
# 当前使用：qwen-coder-plus

# 删除不再使用的配置
cswitch claude delete openai
# 模型 openai 已删除
```

### 场景 5：仅输出环境变量（不写入配置）

适用于 CI/CD 或需要手动控制配置的场景：

```bash
cswitch claude switch dashscope --env
# export ANTHROPIC_BASE_URL='https://coding.dashscope.aliyuncs.com/apps/anthropic'
# export ANTHROPIC_API_KEY='sk-****'
# export ANTHROPIC_DEFAULT_OPUS_MODEL='qwen-coder-plus'
# export ANTHROPIC_DEFAULT_HAIKU_MODEL='qwen-coder-plus'
# export ANTHROPIC_DEFAULT_SONNET_MODEL='qwen-coder-plus'

# 配合 eval 使用
eval "$(cswitch claude switch dashscope --env)"
claude
```

## 配置文件说明

### cswitch 自身配置

位置：`~/.cswitch/config.json`

```json
{
  "version": 2,
  "tools": {
    "claude": {
      "current": "dashscope",
      "current_model": "qwen-coder-plus",
      "models": {
        "dashscope": {
          "base_url": "https://coding.dashscope.aliyuncs.com/apps/anthropic",
          "api_key": "sk-xxxx",
          "models": ["qwen-coder-plus", "qwen-max"],
          "opus_model": "qwen-max",
          "haiku_model": "qwen-coder-plus",
          "sonnet_model": "qwen-coder-plus",
          "timeout": 0,
          "max_tokens": 0
        }
      }
    },
    "codex": {
      "current": "zhipu",
      "current_model": "glm-5",
      "models": {
        "zhipu": {
          "base_url": "https://open.bigmodel.cn/api/coding/paas/v4",
          "api_key": "xxxx",
          "models": ["glm-5", "glm-4.7"],
          "provider_name": "Z.AI",
          "wire_api": "chat"
        }
      }
    }
  }
}
```

### Claude Code 配置

**写入位置**：`~/.claude/settings.json`

**写入内容**（合并到现有配置，保留其他字段）：

```json
{
  "env": {
    "ANTHROPIC_BASE_URL": "https://coding.dashscope.aliyuncs.com/apps/anthropic",
    "ANTHROPIC_API_KEY": "sk-xxxx",
    "ANTHROPIC_DEFAULT_OPUS_MODEL": "qwen-coder-plus",
    "ANTHROPIC_DEFAULT_HAIKU_MODEL": "qwen-coder-plus",
    "ANTHROPIC_DEFAULT_SONNET_MODEL": "qwen-coder-plus"
  },
  "permissions": {
    "allow_file_access": true
  }
}
```

> Claude Code 支持**热切换**，配置写入后立即生效，无需重启终端。

### Codex 配置

**写入位置**：
- `~/.codex/auth.json`：API Key
- `~/.codex/config.toml`：base_url、model、model_provider、model_reasoning_effort（合并更新，保留所有其他字段）
- `~/.cswitch/codex.env`：`OPENAI_API_KEY` 环境变量

**auth.json**：

```json
{
  "OPENAI_API_KEY": "sk-xxxx"
}
```

**config.toml**（合并更新）：

```toml
model = "glm-5"
model_provider = "Z.AI"
model_reasoning_effort = "medium"

[model_providers]
  [model_providers."Z.AI"]
    name = "Z.AI"
    base_url = "https://open.bigmodel.cn/api/coding/paas/v4"
    env_key = "OPENAI_API_KEY"
    wire_api = "chat"

[projects]
  [projects."/Users/sunyang/project"]
    trust_level = "trusted"
```

> Codex 切换后需**重启 Codex CLI** 或**在新终端**中生效。

## 与 cc-switch 的对比

| 特性 | cc-switch | cswitch（本项目） |
|------|-----------|------------------|
| 定位 | 桌面 All-in-One 管理工具 | 轻量 CLI 工具 |
| 界面 | GUI（Tauri） | 命令行 |
| 架构 | 数据库存储（SQLite） | JSON 文件 |
| 多工具 | 5+（Claude、Codex、Gemini、OpenCode、OpenClaw） | 2（Claude、Codex）|
| 配置写入 | 完整的 live 配置快照 | 仅更新关键字段 |
| 自动备份 | ❌ | ✅ |
| 一键回滚 | ❌ | ✅ |
| MCP 管理 | 支持 | 不支持（由各工具原生管理）|
| 云端同步 | 支持（WebDAV、Dropbox 等） | 不支持 |
| 用量查询 | 支持 | 不支持 |
| 代理模式 | 支持 | 不支持 |
| 体积 | ~10MB+ | ~5MB |

cswitch 适合：追求极简、命令行优先、不需要复杂管理功能的用户。

## 版本兼容性

### 配置文件迁移

- **v1 -> v2**：cswitch 会自动检测旧格式并迁移，迁移前自动创建备份 `config.json.bak`
- **降级**：v2 配置文件无法在旧版本 cswitch 中使用

### 工具版本支持

- Claude Code：支持 `ANTHROPIC_BASE_URL`、`ANTHROPIC_API_KEY`、`ANTHROPIC_DEFAULT_OPUS_MODEL`、`ANTHROPIC_DEFAULT_HAIKU_MODEL`、`ANTHROPIC_DEFAULT_SONNET_MODEL` 的所有版本
- Codex：支持使用 `model_provider` + `[model_providers]` 配置方式的版本

## 常见问题

### Q: 切换后 Claude Code / Codex 没有生效？

- **Claude Code**：支持热切换，通常立即生效。如未生效，尝试重启 Claude Code。
- **Codex**：需要**重启 Codex CLI** 或**在新终端**运行（环境变量通过 `~/.cswitch/codex.env` 注入）。

### Q: 回滚后配置还是不对？

每次 `switch` 都会创建递增备份（`.bak` → `.bak.1` → `.bak.2`...）。连续执行 `rollback` 可以逐级恢复：

```bash
cswitch codex rollback  # 恢复到 .bak
ccswitch codex rollback  # 恢复到 .bak.1
```

### Q: Codex 报错 `Missing environment variable: OPENAI_API_KEY`？

这是因为 Codex 使用自定义 `model_provider` 时从环境变量读取 API Key。cswitch 已自动处理：

1. 写入 `~/.cswitch/codex.env`
2. 自动注入 `~/.zshrc` 或 `~/.bashrc`
3. 提示执行 `source ~/.cswitch/codex.env`

### Q: 如何恢复到官方默认配置？

```bash
# 使用 rollback 恢复到切换前状态
cswitch claude rollback

# 或删除 cswitch 管理的配置后手动配置
cswitch claude delete <别名>
```

### Q: 可以同时管理多个提供商吗？

可以。为每个提供商创建不同别名，按需切换：

```bash
cswitch claude add openai --base-url "https://api.openai.com" ...
cswitch claude add dashscope --base-url "https://coding.dashscope..." ...

# 快速切换
cswitch claude switch openai
cswitch claude switch dashscope
```

### Q: 配置文件的权限是什么？

敏感配置文件默认 `0600`（仅所有者可读写）：
- `~/.cswitch/config.json`
- `~/.cswitch/codex.env`
- `~/.claude/settings.json`
- `~/.codex/auth.json`

### Q: 如何查看工具的帮助信息？

```bash
cswitch --help
cswitch claude --help
cswitch claude switch --help
```

## 技术细节

### 原子写入机制

为防止配置损坏，所有关键文件写入采用：

1. 写入临时文件
2. `fsync` 确保落盘
3. `rename` 原子替换目标文件

### 自动备份机制

每次 `switch` 前自动备份原配置文件：

| 备份顺序 | 文件命名 |
|----------|----------|
| 首次备份 | `config.toml.bak` |
| 第二次 | `config.toml.bak.1` |
| 第三次 | `config.toml.bak.2` |
| ... | 依次递增 |

`rollback` 命令自动选择最新备份恢复。

### 安全考虑

- API Key 仅在内存中短暂存在，不打印到日志
- 配置文件权限 `0600`，防止其他用户读取
- `--env` 输出做 shell 安全转义，防止命令注入
- 备份文件与原文件保持相同权限

## 贡献

欢迎 Issue 和 PR！

## 许可证

MIT License

## 致谢

- 设计灵感来自 [cc-switch](https://github.com/farion1231/cc-switch) by @farion1231
- 使用 [Cobra](https://github.com/spf13/cobra) CLI 框架
- 使用 [promptui](https://github.com/manifoldco/promptui) 交互输入
- 使用 [BurntSushi/toml](https://github.com/BurntSushi/toml) TOML 解析
