// internal/ui/model.go
package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/styles"
	"github.com/termchat/termchat/internal/chat"
	"github.com/termchat/termchat/internal/config"
	"github.com/termchat/termchat/internal/roles"
	"github.com/termchat/termchat/internal/shortcuts"
	"github.com/termchat/termchat/internal/storage"
	"golang.org/x/term"
)

type uiMode int

const (
	modeChat uiMode = iota
	modeConfig
	modeResume
	modeShortcuts
	modeRolePicker
	modeRoles
	modeOnboard
	modeSlashComplete
	modeMessageBrowse
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

// autoSavedMsg carries the result of a background auto-save (Err may be nil).
type autoSavedMsg struct{ Err error }

// shortcutsSavedMsg carries the result of saving shortcuts to disk.
type shortcutsSavedMsg struct{ Err error }

// rolesSavedMsg carries the result of saving roles to disk.
type rolesSavedMsg struct{ Err error }

// clipboardImageMsg carries the result of a clipboard paste attempt.
type clipboardImageMsg struct {
	Image *chat.ImageData
	Text  string
	Err   error
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

// shortcutSubMode describes what the shortcut editor is currently doing.
type shortcutSubMode int

const (
	shortcutModeList        shortcutSubMode = iota
	shortcutModeEditName                    // editing the name field
	shortcutModeEditContent                 // editing the content field
)

type shortcutEditor struct {
	items        []shortcuts.Shortcut
	cursor       int // index of selected item
	subMode      shortcutSubMode
	editBuf      string // current text being typed
	savedName    string // name before edit started (for cancel)
	savedContent string // content before edit started (for cancel)
	isNew        bool   // true when 'n' added a new item
	scrollTop    int    // first visible line in content edit mode
}

// roleSubMode describes what the role editor is currently doing.
type roleSubMode int

const (
	roleModeList       roleSubMode = iota
	roleModeEditName               // editing the name field
	roleModeEditPrompt             // editing the prompt field
)

type roleEditor struct {
	items       []roles.Role
	cursor      int
	subMode     roleSubMode
	editBuf     string
	savedName   string
	savedPrompt string
	isNew       bool
	scrollTop   int // first visible line in prompt edit mode
}

type rolePicker struct {
	items  []roles.Role
	cursor int
}

type streamControl struct {
	cancel context.CancelFunc
}

type Model struct {
	cfg      config.Config
	client   chat.Provider
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
	mode             uiMode
	configEd         configEditor
	resumePick       resumePicker
	shortcutsPath    string
	shortcutEd       shortcutEditor
	rolesPath        string
	roleEd           roleEditor
	rolePick         rolePicker
	activeRole       string // name of the selected role; shown in status bar
	cfgPath          string
	autoSaveName     string
	input            string
	pendingImages    []chat.ImageData
	imageCounter     int
	slashAC          slashComplete
	escCount         int // consecutive Esc presses in chat mode for double-Esc detection
	browseCursor     int // index of selected message in modeMessageBrowse
	streaming        bool
	confirmQuit      bool
	currentResp      string
	currentThinking  string
	statusMsg        string
	totalTokens      int
	width            int
	height           int
	viewport         viewport.Model
	chatFollowBottom bool  // still needed: tracks whether to auto-scroll on new content
	err              error
}

func buildRenderer(theme string, width int) (*glamour.TermRenderer, error) {
	if width <= 0 {
		width = 80
	}
	s := styles.DarkStyleConfig
	if theme == "light" {
		s = styles.LightStyleConfig
	}
	zero := uint(0)
	s.Document.Margin = &zero
	// Remove the extra blank line glamour adds after each paragraph block
	s.Paragraph.BlockSuffix = ""
	return glamour.NewTermRenderer(
		glamour.WithStyles(s),
		glamour.WithWordWrap(markdownWrapWidth(width)),
	)
}

func NewModel(cfg config.Config, cfgPath string, onboarding bool, client chat.Provider) (Model, error) {
	// Use actual terminal width so text wraps correctly from the first render.
	// Fall back to 80 if the terminal size cannot be determined.
	initialWidth := 80
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		initialWidth = w
	}
	renderer, err := buildRenderer(cfg.Settings.Theme, initialWidth)
	if err != nil {
		return Model{}, err
	}

	storageDir := cfg.Storage.Dir
	if len(storageDir) > 0 && storageDir[0] == '~' {
		home, _ := os.UserHomeDir()
		storageDir = home + storageDir[1:]
	}

	store, err := storage.New(storageDir)
	if err != nil {
		return Model{}, fmt.Errorf("open storage: %w", err)
	}

	shortcutsPath := filepath.Join(filepath.Dir(cfgPath), "shortcuts.yaml")
	rolesPath := filepath.Join(filepath.Dir(cfgPath), "roles.yaml")

	rolesList, err := roles.Load(rolesPath)
	if err != nil {
		return Model{}, fmt.Errorf("load roles: %w", err)
	}
	initialMode := modeChat
	var rolePick rolePicker
	if len(rolesList) > 0 {
		rolePick = rolePicker{items: rolesList}
		initialMode = modeRolePicker
	}

	// onboarding takes priority over role picker
	if onboarding {
		initialMode = modeOnboard
	}

	var initConfigEd configEditor
	if onboarding {
		initConfigEd = configEditor{fields: buildOnboardFields(cfg)}
	}

	return Model{
		cfg:              cfg,
		client:           client,
		history:          chat.NewHistory(),
		store:            store,
		renderer:         renderer,
		cfgPath:          cfgPath,
		shortcutsPath:    shortcutsPath,
		theme:            ThemeByName(cfg.Settings.Theme),
		autoSaveName:     time.Now().Format("2006-01-02_150405"),
		mode:             initialMode,
		rolesPath:        rolesPath,
		rolePick:         rolePick,
		configEd:         initConfigEd,
		viewport:         viewport.New(0, 0),
		chatFollowBottom: true,
	}, nil
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Close() {
	if m.store != nil {
		m.store.Close()
	}
}

func (m *Model) recreateRenderer(width int) {
	r, err := buildRenderer(m.cfg.Settings.Theme, width)
	if err == nil {
		m.renderer = r
	}
}

func markdownWrapWidth(termWidth int) int {
	// Keep extra headroom to avoid terminal soft-wrap drift on mixed CJK/ASCII
	// content (notably in macOS Terminal.app).
	const safetyMargin = 6
	if termWidth <= 0 {
		return 80
	}
	if termWidth > safetyMargin+1 {
		return termWidth - safetyMargin
	}
	return termWidth
}
