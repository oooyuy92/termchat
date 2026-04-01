# termchat

A terminal chat interface for AI conversations. Supports OpenAI-compatible APIs with streaming, Markdown rendering, and persistent conversation history.

## Purpose
When I want to use a browser or a third-party AI chat client, my computer becomes very slow. In fact, AI chat does not require very many additional features; I just hope to establish a connection with the AI faster, so I wrote this project.

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
  alternate_screen: "auto"  # auto, always, never
```

## Usage

```bash
termchat
# or with a custom config
termchat --config /path/to/config.yaml
# force inline mode (no alternate screen)
termchat --no-alt-screen
```

## Features

**Chat**
- Streaming responses with real-time output
- Markdown rendering with syntax-highlighted code blocks
- Multi-line input (Enter to send, Shift+Enter for newline)
- Status bar showing model name, token usage, and message count

**Conversation management**
- Auto-saves every conversation to SQLite after each response
- `/history` — browse and restore past conversations with a date-grouped picker
- `/save [name]` — save current conversation under a custom name
- `/load [name]` — load a saved conversation
- `/list` — list all saved conversations
- `/clear` — clear the current conversation

**Customization**
- `/settings` — edit model, API key, temperature, and other settings in-app
- `/shortcuts` — customize keyboard shortcuts
- Dark and light themes

### Message Version Browser

- Press `Esc` twice to open the browser.
- In message mode, `↑↓` switches turns and `←→` previews assistant versions.
- Press `v` to open compare mode for horizontally arranged version cards.
- Press `e` to edit the user message, then choose `Regenerate` or `Save Only`.

**Compatibility**
- Works with any OpenAI-compatible API (OpenAI, Azure OpenAI, local models via Ollama, etc.)

## Commands

| Command | Description |
|---------|-------------|
| `/history` | Open conversation history picker |
| `/save [name]` | Save current conversation |
| `/load [name]` | Load a saved conversation |
| `/list` | List all conversations |
| `/clear` | Clear current conversation |
| `/settings` | Open settings editor |
| `/shortcuts` | Open shortcuts editor |

## Roadmap

- [ ] Roles — define system prompt presets and pick one at startup
- [ ] Search across conversation history
- [ ] Multiple simultaneous sessions

## Build from source

Requires Go 1.24+.

```bash
git clone https://github.com/oooyuy92/termchat.git
cd termchat
go build -o termchat .
```
