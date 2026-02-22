# termchat 设计文档

## 概述

termchat 是一个基于 Go 的终端 AI 聊天工具，对接 OpenAI 兼容接口，支持流式输出、Markdown 渲染、对话历史管理。

## 技术选型

- 语言：Go
- TUI 框架：Bubbletea（Charm 生态）
- AI 接口：OpenAI 兼容（net/http 直接对接，不用第三方 SDK）
- 配置格式：YAML

## 项目结构

```
termchat/
├── main.go              # 入口，解析命令行参数
├── internal/
│   ├── ui/              # Bubbletea TUI 层
│   │   ├── model.go     # 主 Model（状态管理）
│   │   ├── view.go      # 渲染逻辑
│   │   ├── update.go    # 消息处理
│   │   ├── input.go     # 输入区组件
│   │   └── statusbar.go # 底部状态栏
│   ├── chat/            # 聊天逻辑
│   │   ├── client.go    # OpenAI 兼容 API 客户端
│   │   ├── stream.go    # SSE 流式处理
│   │   └── history.go   # 对话历史管理
│   ├── config/          # 配置管理
│   │   └── config.go    # YAML 配置读写
│   └── storage/         # 持久化
│       └── conversation.go  # 对话保存/加载（JSON）
├── config.example.yaml  # 示例配置
├── go.mod
└── go.sum
```

## 核心数据流

```
用户输入 → bubbletea Msg → Update() 更新状态
                              ↓
                        调用 chat.Client.SendStream()
                              ↓
                        SSE 逐 token 返回 → bubbletea Msg
                              ↓
                        Update() 追加到当前回复 → View() 渲染
                              ↓
                        流结束 → 追加到 history → 更新状态栏 token 计数
```

## 配置文件

路径：`~/.config/termchat/config.yaml`

```yaml
api:
  base_url: "https://api.openai.com/v1"
  api_key: "sk-xxx"
  model: "gpt-4o"

parameters:
  temperature: 0.7
  max_tokens: 4096

storage:
  dir: "~/.local/share/termchat/conversations"
```

## 界面布局

```
┌─────────────────────────────────────────┐
│ You:                                    │
│ 请解释什么是 goroutine                    │
│                                         │
│ Assistant:                              │
│ Goroutine 是 Go 语言中的轻量级线程...     │
│ ```go                                   │
│ go func() {                             │
│     fmt.Println("hello")               │
│ }()                                     │
│ ```                                     │
│                                         │
│ > 用户输入区域...                         │
├─────────────────────────────────────────┤
│ model: gpt-4o │ tokens: 1234 │ 3 msgs  │
└─────────────────────────────────────────┘
```

- 上方：对话历史，Markdown 渲染（glamour），代码块语法高亮
- 下方输入区：多行输入，Enter 发送，Shift+Enter 换行
- 底部状态栏：当前模型、token 用量、消息数

## 交互命令

| 命令 | 功能 |
|------|------|
| `/save [name]` | 保存当前对话 |
| `/load [name]` | 加载历史对话 |
| `/list` | 列出已保存的对话 |
| `/clear` | 清除当前对话 |
| `/model <name>` | 切换模型 |
| `/exit` 或 Ctrl+C | 退出 |

## 依赖

| 库 | 用途 |
|----|------|
| `github.com/charmbracelet/bubbletea` | TUI 框架 |
| `github.com/charmbracelet/lipgloss` | 终端样式 |
| `github.com/charmbracelet/glamour` | Markdown 渲染 |
| `gopkg.in/yaml.v3` | 配置文件解析 |

API 客户端使用 `net/http` + `encoding/json` 直接实现，不引入第三方 OpenAI SDK。
