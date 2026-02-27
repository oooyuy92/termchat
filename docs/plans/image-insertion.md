# 插入图片功能实现计划

## 现状分析

- Gemini client：已实现图片处理（buildContents 已转换 m.Images → genai.Part）
- Anthropic client：Content 是纯 string，需改成 content blocks 格式
- OpenAI client：Messages 直接序列化 Message struct，需改成 content array 格式
- DeepSeek 复用 OpenAI client，不支持图片，需发送前过滤并提示用户
- 剪贴板读取、pendingImages 存储、UI [image N] 标签已有

## 实现步骤

### Step 1：provider.go 加 SupportsVision() 接口

```go
type Provider interface {
    // ...existing methods...
    SupportsVision() bool
}
```

各 client 实现：
- AnthropicClient.SupportsVision() → true
- GeminiClient.SupportsVision() → true
- OpenAIClient.SupportsVision() → 按 model 名静态判断

OpenAI 白名单（model 名包含以下字符串则返回 true）：
- "gpt-4o", "gpt-4-turbo", "gpt-4-vision", "o1", "o3", "o4"

DeepSeek 通过 baseURL 或 model 名判断为 false（deepseek-chat / deepseek-reasoner）。

### Step 2：Anthropic client 支持图片

anthropicMessage.Content 从 string 改为 []anthropicContentBlock：

```go
type anthropicContentBlock struct {
    Type    string                  `json:"type"`
    Text    string                  `json:"text,omitempty"`
    Source  *anthropicImageSource   `json:"source,omitempty"`
}

type anthropicImageSource struct {
    Type      string `json:"type"`       // "base64"
    MediaType string `json:"media_type"` // "image/png" etc
    Data      string `json:"data"`       // base64 encoded
}
```

buildMessage 时：先加图片 block，再加 text block。

### Step 3：OpenAI client 支持图片

chatRequest.Messages 改为自定义类型，Content 支持 string 或 array：

```go
type openAIMessage struct {
    Role    string      `json:"role"`
    Content interface{} `json:"content"` // string 或 []openAIContentPart
}

type openAIContentPart struct {
    Type     string            `json:"type"`               // "text" or "image_url"
    Text     string            `json:"text,omitempty"`
    ImageURL *openAIImageURL   `json:"image_url,omitempty"`
}

type openAIImageURL struct {
    URL string `json:"url"` // "data:image/png;base64,..."
}
```

有图片时用 array，无图片时用 string（兼容性更好）。

### Step 4：UI 层发送前检查

在 update.go 发送消息前（sendStreamCmd 或发送入口处）：

```go
if len(tab.pendingImages) > 0 && !tab.client.SupportsVision() {
    tab.pendingImages = nil
    tab.imageCounter = 0
    m.statusMsg = "⚠ 当前模型不支持图片，已忽略"
}
```

## 文件改动清单

| 文件 | 改动 |
|------|------|
| internal/chat/provider.go | 加 SupportsVision() 到 Provider 接口 |
| internal/chat/client_anthropic.go | Content 改 blocks，实现 SupportsVision()=true |
| internal/chat/client_openai.go | Content 改 array，实现 SupportsVision() 白名单 |
| internal/chat/client_gemini.go | 仅加 SupportsVision()=true |
| internal/ui/update.go | 发送前检查 SupportsVision，不支持则清空并提示 |
