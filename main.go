// main.go
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/termchat/termchat/internal/chat"
	"github.com/termchat/termchat/internal/config"
	"github.com/termchat/termchat/internal/ui"
	"golang.org/x/term"
)

func parseCLIArgs(args []string) (configPath string, noAltScreen bool, err error) {
	configPath = config.DefaultConfigPath()

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--config":
			if i+1 >= len(args) {
				return "", false, errors.New("--config requires a file path")
			}
			i++
			configPath = args[i]
		case "--no-alt-screen":
			noAltScreen = true
		}
	}

	return configPath, noAltScreen, nil
}

func resolveAltScreenMode(noAltScreen bool, mode string) bool {
	if noAltScreen {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "always":
		return true
	case "never":
		return false
	default: // auto + invalid values
		return os.Getenv("ZELLIJ") == ""
	}
}

func setAlternateScroll(w io.Writer, enabled bool) {
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return
	}
	if enabled {
		_, _ = io.WriteString(w, "\x1b[?1007h")
		return
	}
	_, _ = io.WriteString(w, "\x1b[?1007l")
}

func main() {
	configPath, noAltScreen, err := parseCLIArgs(os.Args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	cfg, onboarding, err := config.LoadOrDefault(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config from %s: %v\n", configPath, err)
		fmt.Fprintf(os.Stderr, "Tip: delete the file to reset to defaults.\n")
		os.Exit(1)
	}

	client := chat.NewProvider(cfg.API.Provider, cfg.API.BaseURL, cfg.API.APIKey, cfg.API.Model)

	model, err := ui.NewModel(cfg, configPath, onboarding, client)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize: %v\n", err)
		os.Exit(1)
	}

	useAltScreen := resolveAltScreenMode(noAltScreen, cfg.Settings.AlternateScreen)

	var opts []tea.ProgramOption
	if useAltScreen {
		opts = append(opts, tea.WithAltScreen())
		setAlternateScroll(os.Stdout, true)
		defer setAlternateScroll(os.Stdout, false)
	}
	opts = append(opts, tea.WithMouseCellMotion())
	p := tea.NewProgram(model, opts...)

	finalModel, runErr := p.Run()
	if uiModel, ok := finalModel.(ui.Model); ok {
		uiModel.Close()
	}
	if runErr != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", runErr)
		os.Exit(1)
	}
}
