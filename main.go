// main.go
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/termchat/termchat/internal/config"
	"github.com/termchat/termchat/internal/ui"
)

func main() {
	configPath := config.DefaultConfigPath()

	if len(os.Args) > 2 && os.Args[1] == "--config" {
		configPath = os.Args[2]
	}

	cfg, onboarding, err := config.LoadOrDefault(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config from %s: %v\n", configPath, err)
		os.Exit(1)
	}

	model, err := ui.NewModel(cfg, configPath, onboarding)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(model, tea.WithAltScreen())

	finalModel, runErr := p.Run()
	if uiModel, ok := finalModel.(ui.Model); ok {
		uiModel.Close()
	}
	if runErr != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", runErr)
		os.Exit(1)
	}
}
