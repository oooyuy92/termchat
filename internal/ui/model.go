// internal/ui/model.go
package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
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
	modeTabRename   // F2 rename active tab
	modeTabOverflow // … overflow dropdown
)

// streamChunkMsg carries a token and optional thinking text from the streaming response.
type streamChunkMsg struct {
	TabIdx   int
	Content  string
	Thinking string
}

// streamDoneMsg signals the stream has finished.
type streamDoneMsg struct{ TabIdx int }

// streamErrMsg signals a streaming error.
type streamErrMsg struct {
	TabIdx int
	Err    error
}

// commandResultMsg carries output from a slash command.
type commandResultMsg struct {
	Text string
}

// streamStartMsg carries the channels for consuming a streaming response.
type streamStartMsg struct {
	TabIdx int
	chunks <-chan chat.StreamChunk
	errs   <-chan error
}

// configSavedMsg signals that config was saved to disk.
type configSavedMsg struct {
	Err error
}

// autoSavedMsg carries the result of a background auto-save (Err may be nil).
type autoSavedMsg struct {
	TabIdx int
	Err    error
}

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

type browseMode int

const (
	browseModeMessage browseMode = iota
	browseModeCompare
)

type browseTurn struct {
	User              chat.Message
	AssistantVersions []chat.Message
	ActiveVersion     int
	PreviewVersion    int
}

func (t browseTurn) VersionCount() int {
	return len(t.AssistantVersions)
}

type messageBrowseState struct {
	mode               browseMode
	turns              []browseTurn
	turnIdx            int
	leftScroll         int
	rightScroll        int
	compareCardIdx     int
	compareCardScrolls map[int]int
}

type Model struct {
	cfg      config.Config
	store    *storage.Store

	// Theme
	theme Theme

	// Tab management
	tabs             []TabSession
	activeTab        int
	tabRename        string      // input buffer when in modeTabRename
	tabBarZones      []tabHitZone
	tabOverflowOffset int

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
	slashAC          slashComplete
	escCount         int // consecutive Esc presses in chat mode for double-Esc detection
	messageBrowse    messageBrowseState
	browseCursor     int // index of selected message in modeMessageBrowse
	confirmQuit      bool
	statusMsg        string
	width            int
	height           int
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

	tab, err := newTabSession(cfg, client, initialWidth)
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
		cfg:           cfg,
		store:         store,
		cfgPath:       cfgPath,
		shortcutsPath: shortcutsPath,
		theme:         ThemeByName(cfg.Settings.Theme),
		mode:          initialMode,
		rolesPath:     rolesPath,
		rolePick:      rolePick,
		configEd:      initConfigEd,
		tabs:          []TabSession{tab},
		activeTab:     0,
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
	if err != nil {
		m.statusMsg = "renderer error: " + err.Error()
		return
	}
	m.activeTabSession().renderer = r
}

// activeTabSession returns a pointer to the active TabSession.
// Safe to call from both pointer and value receivers.
func (m Model) activeTabSession() *TabSession {
	return &m.tabs[m.activeTab]
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
