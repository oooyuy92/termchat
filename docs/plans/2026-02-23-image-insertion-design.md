# Image Insertion Design

## 目标

支持通过 Ctrl+V 粘贴图片，作为多模态内容发送给 Gemini API 进行分析。

## 约束

- 仅支持 Gemini 提供商（其他提供商忽略图片）
- 图片不持久化到 SQLite，历史记录只保存 `[image N]` 占位符
- 使用 inline base64 方式发送（非 Files API）
- 剪贴板读取使用系统命令，不引入新 Go 依赖

## 数据模型变更

### Message 扩展

```go
// internal/chat/history.go
type ImageData struct {
    MimeType string
    Data     []byte
}

type Message struct {
    Role    string      `json:"role"`
    Content string      `json:"content"`
    Images  []ImageData `json:"-"`
}
```

### Gemini Part 扩展

```go
// internal/chat/client_gemini.go
type geminiPart struct {
    Text       string        `json:"text,omitempty"`
    InlineData *geminiInline `json:"inline_data,omitempty"`
}

type geminiInline struct {
    MimeType string `json:"mime_type"`
    Data     string `json:"data"` // base64
}
```

## 剪贴板读取

新建 `internal/ui/clipboard_image.go`，复用现有 `writeToClipboard` 的平台检测模式：

- Linux Wayland: `wl-paste --type image/png`
- Linux X11: `xclip -selection clipboard -t image/png -o`
- macOS: `pngpaste` 或 `osascript` 方案
- 同时尝试 image/jpeg 作为 fallback

## UI 交互

### Model 新增字段

```go
pendingImages []chat.ImageData
imageCounter  int
```

### Ctrl+V 处理

1. 拦截 `ctrl+v` 按键
2. 调用剪贴板读取函数
3. 读到图片：存入 `pendingImages`，输入框追加 `[image N]`
4. 没有图片：执行正常文本粘贴

### 发送流程

- Enter 发送时将 `pendingImages` 附加到 `Message.Images`
- 发送后清空 `pendingImages`
- `/clear` 时同时清空 `pendingImages` 和 `imageCounter`

## Gemini 客户端变更

构建 `contents` 时处理 `Message.Images`：

- 图片转为 `geminiPart{InlineData: ...}` (base64)
- 文本转为 `geminiPart{Text: ...}`
- 图片 part 在文本 part 之前

## 不变的部分

- Provider 接口不变
- OpenAI/Anthropic 客户端不变（忽略 Images 字段）
- 存储层不变（json:"-" 跳过图片序列化）
