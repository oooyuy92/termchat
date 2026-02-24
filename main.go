// main.go
package main

import (
	"errors"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/termchat/termchat/internal/chat"
	"github.com/termchat/termchat/internal/config"
	"github.com/termchat/termchat/internal/ui"
)

func parseCLIArgs(args []string) (configPath string, useAltScreen bool, err error) {
	configPath = config.DefaultConfigPath()
	useAltScreen = true

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--config":
			if i+1 >= len(args) {
				return "", false, errors.New("--config requires a file path")
			}
			i++
			configPath = args[i]
		case "--no-alt-screen":
			useAltScreen = false
		}
	}

	return configPath, useAltScreen, nil
}

func main() {
	configPath, useAltScreen, err := parseCLIArgs(os.Args)
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

	var opts []tea.ProgramOption
	if useAltScreen {
		opts = append(opts, tea.WithAltScreen())
	}
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
