# Onboarding Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Show a friendly TUI onboarding wizard on first launch (when `config.yaml` is missing), and update `install.sh` to remove manual setup instructions.

**Architecture:** Detect missing config in `main.go` using `errors.Is(err, fs.ErrNotExist)`; pass an `onboarding bool` to `NewModel`; add a new `modeOnboard` UI mode backed by a trimmed 3-field `configEditor` (Base URL, API Key, Model); reuse existing `updateConfigMode` for field editing logic; add `updateOnboardMode` that intercepts Esc to save defaults + enter chat. `install.sh` gets simplified messaging.

**Tech Stack:** Go, BubbleTea, existing `configEditor` / `saveConfigCmd` infrastructure, `errors.Is` / `io/fs`.

---

## Background: existing code you must understand

### `internal/config/config.go`

- `DefaultConfig()` returns a Config with sensible defaults (BaseURL = `https://api.openai.com/v1`, Model = `gpt-4o`, Temperature = 0.7, MaxTokens = 4096, Theme = `dark`).
- `Load(path)` returns `(Config, error)`. When the file doesn't exist, `err` wraps `fs.ErrNotExist`.
- `Save(path, cfg)` creates parent dirs and writes YAML.

### `internal/ui/configeditor.go`

- `buildConfigFields(cfg)` returns all 7 config fields. For onboarding we only want the 3 API fields.
- `updateConfigMode(msg)` handles all field editing; its non-editing `case "esc"` sets `m.mode = modeChat`.
- `saveConfigCmd()` is an async Cmd that calls `config.Save` and returns `configSavedMsg`.
- `viewConfigEditor()` renders the config editor with title "Model & Parameters Configuration".

### `internal/ui/model.go`

- `uiMode` iota: `modeChat`, `modeConfig`, `modeResume`, `modeShortcuts`, `modeRolePicker`, `modeRoles`.
- `NewModel(cfg config.Config, cfgPath string)` initializes the model.
- `configEd configEditor` field holds the config editor state.

### `internal/ui/update.go`

- `Update()` routes modes via `if m.mode == modeX` checks in the `case tea.KeyMsg:` branch.
- `handleCommand()` routes slash commands.

### `internal/ui/view.go`

- `View()` routes modes via `if m.mode == modeX { return m.viewX() }` checks.

### `main.go`

Currently: if `config.Load` returns any error, print and exit. We need to distinguish "file not found" (→ onboarding) from real errors (→ still exit).

---

## Task 1: `config` package — add `IsNotFound` helper + test

This is the only testable logic change. Adding a small helper makes `main.go` cleaner and the detection logic testable.

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

### Step 1: Write the failing test

Add to `internal/config/config_test.go`:

```go
func TestLoadNotFound(t *testing.T) {
	cfg, ok := LoadOrDefault("/nonexistent/path/config.yaml")
	if !ok {
		t.Error("LoadOrDefault() ok = false, want true")
	}
	// Should return defaults, not zero values
	if cfg.API.BaseURL != "https://api.openai.com/v1" {
		t.Errorf("BaseURL = %q, want default", cfg.API.BaseURL)
	}
	if cfg.Parameters.Temperature != 0.7 {
		t.Errorf("Temperature = %f, want 0.7", cfg.Parameters.Temperature)
	}
}

func TestLoadOrDefaultExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := []byte("api:\n  api_key: \"sk-test\"\n")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}
	cfg, ok := LoadOrDefault(path)
	if ok {
		t.Error("LoadOrDefault() ok = true for existing file, want false")
	}
	if cfg.API.APIKey != "sk-test" {
		t.Errorf("APIKey = %q, want %q", cfg.API.APIKey, "sk-test")
	}
}
```

### Step 2: Run the test to verify it fails

```bash
GOPATH=/home/cc/gopath /home/cc/go/bin/go test ./internal/config/...
```

Expected: FAIL with "undefined: LoadOrDefault"

### Step 3: Implement `LoadOrDefault` in `config.go`

Add to `internal/config/config.go`, after the existing `Load` function:

```go
// LoadOrDefault loads config from path. If the file does not exist, it returns
// DefaultConfig() and ok=true. For any other error it returns the error.
// ok=true means "file was missing" (caller should show onboarding).
// ok=false means "file existed and was loaded" (normal startup).
func LoadOrDefault(path string) (cfg Config, missing bool, err error) {
	cfg, err = Load(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return DefaultConfig(), true, nil
		}
		return cfg, false, err
	}
	return cfg, false, nil
}
```

Add imports `"errors"` and `"io/fs"` to the import block in `config.go`.

Update the test to match the 3-return signature:

```go
func TestLoadNotFound(t *testing.T) {
	cfg, missing, err := LoadOrDefault("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("LoadOrDefault() error = %v", err)
	}
	if !missing {
		t.Error("LoadOrDefault() missing = false, want true")
	}
	if cfg.API.BaseURL != "https://api.openai.com/v1" {
		t.Errorf("BaseURL = %q, want default", cfg.API.BaseURL)
	}
	if cfg.Parameters.Temperature != 0.7 {
		t.Errorf("Temperature = %f, want 0.7", cfg.Parameters.Temperature)
	}
}

func TestLoadOrDefaultExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := []byte("api:\n  api_key: \"sk-test\"\n")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}
	cfg, missing, err := LoadOrDefault(path)
	if err != nil {
		t.Fatalf("LoadOrDefault() error = %v", err)
	}
	if missing {
		t.Error("LoadOrDefault() missing = true for existing file, want false")
	}
	if cfg.API.APIKey != "sk-test" {
		t.Errorf("APIKey = %q, want %q", cfg.API.APIKey, "sk-test")
	}
}
```

### Step 4: Run tests to verify they pass

```bash
GOPATH=/home/cc/gopath /home/cc/go/bin/go test ./internal/config/...
```

Expected: all tests PASS

### Step 5: Commit

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat: add config.LoadOrDefault for missing-file detection"
```

---

## Task 2: `model.go` — add `modeOnboard` and update `NewModel`

**Files:**
- Modify: `internal/ui/model.go`

### Step 1: Read the full file first

Read `internal/ui/model.go` before making any changes.

### Step 2: Make the changes

**2a. Add `modeOnboard` to the uiMode iota** — append after `modeRoles`:

```go
const (
	modeChat uiMode = iota
	modeConfig
	modeResume
	modeShortcuts
	modeRolePicker
	modeRoles
	modeOnboard
)
```

**2b. Add `buildOnboardFields` function** — add this function near `buildConfigFields` in `configeditor.go` (or in `model.go`, either is fine — put it in `configeditor.go` since that's where `buildConfigFields` lives):

Actually, add it to `internal/ui/configeditor.go` (see Task 3 which touches that file). Skip in this task.

**2c. Change `NewModel` signature** — add `onboarding bool` parameter:

```go
func NewModel(cfg config.Config, cfgPath string, onboarding bool) (Model, error) {
```

**2d. In `NewModel`, set initial mode based on `onboarding`**:

After the existing `initialMode := modeChat` / `modeRolePicker` logic (which currently decides between modeChat and modeRolePicker), modify to:

```go
// onboarding takes priority over role picker
if onboarding {
    initialMode = modeOnboard
}
```

(This must come AFTER the existing rolesList check, so that if there are no roles AND it's onboarding, we still show onboarding.)

**2e. In `NewModel`, initialise `configEd` for onboarding**:

The onboard configEd only has 3 fields. Since `buildOnboardFields` will live in `configeditor.go` (Task 3), for now just note that the returned Model needs:

```go
// In the returned Model literal, add:
configEd: func() configEditor {
    if onboarding {
        return configEditor{fields: buildOnboardFields(cfg)}
    }
    return configEditor{}
}(),
```

Or more cleanly, compute it before the return:

```go
var initConfigEd configEditor
if onboarding {
    initConfigEd = configEditor{fields: buildOnboardFields(cfg)}
}

return Model{
    ...
    configEd: initConfigEd,
    ...
}, nil
```

**2f. Update `main.go` to pass the new parameter** — `main.go` currently calls `ui.NewModel(cfg, configPath)`. Change to `ui.NewModel(cfg, configPath, false)` so it compiles. Task 3 will update `main.go` properly.

### Step 3: Verify it compiles

```bash
GOPATH=/home/cc/gopath /home/cc/go/bin/go build ./...
```

Expected: PASS (you may need to update main.go call site first)

### Step 4: Commit

```bash
git add internal/ui/model.go
git commit -m "feat: add modeOnboard to model, update NewModel signature"
```

---

## Task 3: `configeditor.go` + `onboard.go` — onboard fields and view/update

**Files:**
- Modify: `internal/ui/configeditor.go` (add `buildOnboardFields`)
- Create: `internal/ui/onboard.go`

### Step 1: Add `buildOnboardFields` to `configeditor.go`

Read `internal/ui/configeditor.go` first. Then add after `buildConfigFields`:

```go
// buildOnboardFields returns only the 3 API fields needed for first-run onboarding.
func buildOnboardFields(cfg config.Config) []configField {
	return []configField{
		{Label: "API Base URL", Key: "base_url", Value: cfg.API.BaseURL},
		{Label: "API Key", Key: "api_key", Value: cfg.API.APIKey, Masked: true},
		{Label: "Model", Key: "model", Value: cfg.API.Model},
	}
}
```

### Step 2: Create `internal/ui/onboard.go`

```go
// internal/ui/onboard.go
package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// updateOnboardMode handles key events in onboarding mode.
// It delegates field editing to updateConfigMode, but intercepts non-editing
// Esc to save the current config (even defaults) and enter chat.
func (m Model) updateOnboardMode(msg tea.KeyMsg) (Model, tea.Cmd) {
	// Non-editing Esc: save whatever we have and enter chat.
	if msg.String() == "esc" && !m.configEd.editing {
		m.mode = modeChat
		m.statusMsg = "Ready! Type a message to start chatting."
		return m, m.saveConfigCmd()
	}

	// Ctrl+C: double-quit pattern (same as other modes).
	if msg.String() != "ctrl+c" {
		m.confirmQuit = false
	}
	if msg.String() == "ctrl+c" {
		if m.confirmQuit {
			return m, tea.Quit
		}
		m.confirmQuit = true
		m.statusMsg = "Press Ctrl+C again to quit"
		return m, nil
	}

	// Delegate all other keys to the config editor logic.
	// updateConfigMode's own "esc" handler (non-editing) sets m.mode = modeChat,
	// but we've already handled that case above. In editing mode, esc cancels
	// the edit — that's correct for onboarding too.
	return m.updateConfigMode(msg)
}

// viewOnboard renders the first-run onboarding wizard.
func (m Model) viewOnboard() string {
	var b strings.Builder
	ed := m.configEd

	b.WriteString(m.theme.ConfigTitleStyle().Render("Welcome to termchat!"))
	b.WriteString("\n")
	b.WriteString(m.theme.ConfigHelpStyle().Render("Configure your API access to get started."))
	b.WriteString("\n\n")

	for i, field := range ed.fields {
		cursor := "  "
		if i == ed.cursor {
			cursor = m.theme.ConfigCursorStyle().Render("> ")
		}

		label := m.theme.ConfigLabelStyle().Render(field.Label + ":")

		var value string
		if ed.editing && i == ed.cursor {
			value = m.theme.ConfigEditStyle().Render(ed.editBuf + "\u2588")
		} else {
			displayVal := field.Value
			if displayVal == "" {
				displayVal = "(not set)"
			} else if field.Masked {
				displayVal = maskValue(displayVal)
			}
			value = m.theme.ConfigValueStyle().Render(displayVal)
		}

		b.WriteString(cursor + label + value + "\n")
	}

	b.WriteString("\n")

	if ed.editErr != "" {
		b.WriteString(m.theme.ConfigErrStyle().Render("  Error: "+ed.editErr) + "\n\n")
	}

	if ed.editing {
		b.WriteString(m.theme.ConfigHelpStyle().Render("  Enter: confirm  |  Esc: cancel"))
	} else {
		b.WriteString(m.theme.ConfigHelpStyle().Render("  ↑↓: navigate  |  Enter: edit  |  Esc: skip and start chatting"))
	}
	b.WriteString("\n")

	return b.String() + "\n" + m.renderStatusBar()
}
```

### Step 3: Verify it compiles

```bash
GOPATH=/home/cc/gopath /home/cc/go/bin/go build ./...
```

Expected: PASS

### Step 4: Commit

```bash
git add internal/ui/configeditor.go internal/ui/onboard.go
git commit -m "feat: add onboarding view and update handler"
```

---

## Task 4: Wire up `update.go`, `view.go`, `main.go`, and `install.sh`

**Files:**
- Modify: `internal/ui/update.go`
- Modify: `internal/ui/view.go`
- Modify: `main.go`
- Modify: `install.sh`

Read all four files before making changes.

### Step 1: `update.go` — route `modeOnboard`

In `Update()`, inside `case tea.KeyMsg:`, after the `modeRoles` check:

```go
if m.mode == modeRoles {
    return m.updateRolesMode(msg)
}
if m.mode == modeOnboard {
    return m.updateOnboardMode(msg)
}
```

### Step 2: `view.go` — route `modeOnboard`

In `View()`, after the `modeRoles` check:

```go
if m.mode == modeRoles {
    return m.viewRolesEditor()
}
if m.mode == modeOnboard {
    return m.viewOnboard()
}
```

### Step 3: `main.go` — use `LoadOrDefault`, pass `onboarding` to `NewModel`

Replace the current config loading block:

```go
cfg, err := config.Load(configPath)
if err != nil {
    fmt.Fprintf(os.Stderr, "Failed to load config from %s: %v\n", configPath, err)
    fmt.Fprintf(os.Stderr, "Create a config file or use --config <path>\n")
    fmt.Fprintf(os.Stderr, "See config.example.yaml for reference.\n")
    os.Exit(1)
}
```

With:

```go
cfg, onboarding, err := config.LoadOrDefault(configPath)
if err != nil {
    fmt.Fprintf(os.Stderr, "Failed to load config from %s: %v\n", configPath, err)
    os.Exit(1)
}
```

And update the `ui.NewModel` call:

```go
model, err := ui.NewModel(cfg, configPath, onboarding)
```

### Step 4: `install.sh` — update messaging

Find and replace the "Next steps" block at the bottom of `install.sh`:

Current:
```bash
echo ""
echo "termchat installed to ${INSTALL_DIR}/${BINARY}"
echo ""
echo "Next steps:"
echo "  1. Create config: mkdir -p ~/.config/termchat"
echo "  2. Add your API key to ~/.config/termchat/config.yaml"
echo "  3. Run: termchat"
```

Replace with:
```bash
echo ""
echo "termchat installed to ${INSTALL_DIR}/${BINARY}"
echo ""
echo "Run 'termchat' to get started — it will guide you through setup."
```

### Step 5: Build and test

```bash
GOPATH=/home/cc/gopath /home/cc/go/bin/go build ./...
GOPATH=/home/cc/gopath /home/cc/go/bin/go test ./...
```

Expected: all PASS

### Step 6: Commit

```bash
git add internal/ui/update.go internal/ui/view.go main.go install.sh
git commit -m "feat: wire onboarding into app startup and simplify install.sh"
```

---

## Manual Testing Checklist

1. **Config exists** — app starts normally, no onboarding screen
2. **Config missing** — app shows "Welcome to termchat!" with 3 API fields
3. **Edit API Key** — press Enter on API Key, type key, press Enter → field saved, config file created at `~/.config/termchat/config.yaml`
4. **Esc to skip** — press Esc on onboarding → default config written to file, enters chat immediately
5. **Next launch after Esc** — config file exists → no onboarding shown (normal startup)
6. **Corrupted config** — real parse error still exits with error message (not onboarding)
7. **`/settings` still works** — existing config editor unaffected
8. **install.sh messaging** — shows "Run 'termchat' to get started" instead of manual steps

---

## Run all tests

```bash
GOPATH=/home/cc/gopath /home/cc/go/bin/go test ./...
```

Expected: all packages pass.
