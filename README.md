# termchat

A terminal chat interface for AI conversations. Supports OpenAI-compatible APIs.

## Install

```bash
curl -sfL https://raw.githubusercontent.com/oooyuy92/termchat/master/install.sh | bash
```

Or install to a custom directory:

```bash
INSTALL_DIR=~/.local/bin curl -sfL https://raw.githubusercontent.com/oooyuy92/termchat/master/install.sh | bash
```

## Setup

Create a config file at `~/.config/termchat/config.yaml`:

```yaml
api:
  base_url: "https://api.openai.com/v1"
  api_key: "your-api-key-here"
  model: "gpt-4o"

parameters:
  temperature: 0.7
  max_tokens: 4096

storage:
  dir: "~/.local/share/termchat/conversations"

settings:
  theme: "dark"  # dark or light
```

## Usage

```bash
termchat
```

Use a custom config:

```bash
termchat --config /path/to/config.yaml
```

## Build from source

Requires Go 1.24+.

```bash
git clone https://github.com/oooyuy92/termchat.git
cd termchat
go build -o termchat .
```
