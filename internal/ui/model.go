// internal/ui/model.go
package ui

import (
	"context"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/styles"
	"github.com/termchat/termchat/internal/chat"
	"github.com/termchat/termchat/internal/config"
	"github.com/termchat/termchat/internal/storage"
)

type uiMode int

const (
	modeChat uiMode = iota
	modeConfig
)

// streamChunkMsg carries a token and optional thinking text from the streaming response.
type streamChunkMsg struct {
	Content  string
	Thinking string
}

// streamDoneMsg signals the stream has finished.
type streamDoneMsg struct{}

// streamErrMsg signals a streaming error.
type streamErrMsg struct {
	Err error
}

// commandResultMsg carries output from a slash command.
type commandResultMsg struct {
	Text string
}

// streamStartMsg carries the channels for consuming a streaming response.
type streamStartMsg struct {
	chunks <-chan chat.StreamChunk
	errs   <-chan error
}

// configSavedMsg signals that config was saved to disk.
type configSavedMsg struct {
	Err error
}

type configField struct {
	Label   string
	Key     string
	Value   string
	Masked  bool
	Options []string // If non-empty, cycle through these with Enter instead of free text editing
}

type configEditor struct {
	fields  []configField
	cursor  int
	editing bool
	editBuf string
	editErr string
}

type streamControl struct {
	cancel context.CancelFunc
}

type Model struct {
	cfg      config.Config
	client   *chat.Client
	history  *chat.History
	store    *storage.Store
	renderer *glamour.TermRenderer

	// Theme
	theme Theme

	// Streaming channels
	streamCh  <-chan chat.StreamChunk
	streamErr <-chan error

	// Stream cancellation
	streamCtrl *streamControl

	// UI state
	mode            uiMode
	configEd        configEditor
	cfgPath         string
	input           string
	streaming       bool
	confirmQuit     bool
	currentResp     string
	currentThinking string
	statusMsg       string
	totalTokens     int
	width           int
	height          int
	err             error
}

func buildRenderer(theme string, width int) (*glamour.TermRenderer, error) {
	s := styles.DarkStyleConfig
	if theme == "light" {
		s = styles.LightStyleConfig
	}
	zero := uint(0)
	s.Document.Margin = &zero
	return glamour.NewTermRenderer(
		glamour.WithStyles(s),
		glamour.WithWordWrap(width),
	)
}

func NewModel(cfg config.Config, cfgPath string) (Model, error) {
	renderer, err := buildRenderer(cfg.Settings.Theme, 80)
	if err != nil {
		return Model{}, err
	}

	storageDir := cfg.Storage.Dir
	if len(storageDir) > 0 && storageDir[0] == '~' {
		home, _ := os.UserHomeDir()
		storageDir = home + storageDir[1:]
	}

	return Model{
		cfg:      cfg,
		client:   chat.NewClient(cfg.API.BaseURL, cfg.API.APIKey, cfg.API.Model),
		history:  chat.NewHistory(),
		store:    storage.New(storageDir),
		renderer: renderer,
		cfgPath:  cfgPath,
		theme:    ThemeByName(cfg.Settings.Theme),
	}, nil
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m *Model) recreateRenderer(width int) {
	r, err := buildRenderer(m.cfg.Settings.Theme, width)
	if err == nil {
		m.renderer = r
	}
}
