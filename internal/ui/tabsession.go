package ui

import (
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/termchat/termchat/internal/chat"
	"github.com/termchat/termchat/internal/config"
)

// TabSession holds all state specific to one chat tab.
type TabSession struct {
	name             string
	client           chat.Provider
	history          *chat.History
	viewport         viewport.Model
	textarea         textarea.Model
	spinner          spinner.Model
	renderer         *glamour.TermRenderer
	streaming        bool
	streamCh         <-chan chat.StreamChunk
	streamErr        <-chan error
	streamCtrl       *streamControl
	currentResp      string
	currentThinking  string
	chatFollowBottom bool
	pendingImages    []chat.ImageData
	imageCounter     int
	autoSaveName     string
	totalTokens      int
	err              error
}

type tabHitAction int

const (
	tabHitSelect   tabHitAction = iota // click tab to switch
	tabHitClose                        // click × to close
	tabHitNew                          // click + to create
	tabHitOverflow                     // click … to open overflow
)

type tabHitZone struct {
	startX int
	endX   int
	action tabHitAction
	tabIdx int // for tabHitSelect and tabHitClose
}

// newTabSession creates a new TabSession with sensible defaults.
func newTabSession(cfg config.Config, client chat.Provider, width int) (TabSession, error) {
	renderer, err := buildRenderer(cfg.Settings.Theme, width)
	if err != nil {
		return TabSession{}, err
	}

	ta := textarea.New()
	ta.Placeholder = "Message... (Shift+Enter for newline)"
	ta.Focus()
	ta.SetHeight(3)
	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline.SetKeys("shift+enter")
	ta.FocusedStyle.Base = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62"))
	ta.BlurredStyle.Base = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240"))

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	vp := viewport.New(0, 0)
	vp.MouseWheelEnabled = true // Enable mouse wheel scrolling in viewport

	return TabSession{
		name:             client.Model(),
		client:           client,
		history:          chat.NewHistory(),
		viewport:         vp,
		textarea:         ta,
		spinner:          sp,
		renderer:         renderer,
		chatFollowBottom: true,
		autoSaveName:     time.Now().Format("2006-01-02_150405"),
	}, nil
}
