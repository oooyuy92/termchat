// internal/ui/model.go
package ui

import (
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/termchat/termchat/internal/chat"
	"github.com/termchat/termchat/internal/config"
	"github.com/termchat/termchat/internal/storage"
)

type uiMode int

const (
	modeChat uiMode = iota
	modeConfig
)

// streamChunkMsg carries a token from the streaming response.
type streamChunkMsg struct {
	Content string
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
	chunks <-chan string
	errs   <-chan error
}

// configSavedMsg signals that config was saved to disk.
type configSavedMsg struct {
	Err error
}

type configField struct {
	Label  string
	Key    string
	Value  string
	Masked bool
}

type configEditor struct {
	fields  []configField
	cursor  int
	editing bool
	editBuf string
	editErr string
}

type Model struct {
	cfg      config.Config
	client   *chat.Client
	history  *chat.History
	store    *storage.Store
	renderer *glamour.TermRenderer

	// Streaming channels
	streamCh  <-chan string
	streamErr <-chan error

	// UI state
	mode        uiMode
	configEd    configEditor
	cfgPath     string
	input       string
	streaming   bool
	currentResp string
	statusMsg   string
	totalTokens int
	width       int
	height      int
	err         error
}

func NewModel(cfg config.Config, cfgPath string) (Model, error) {
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(0),
	)
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
	}, nil
}

func (m Model) Init() tea.Cmd {
	return nil
}
