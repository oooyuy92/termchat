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

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config from %s: %v\n", configPath, err)
		fmt.Fprintf(os.Stderr, "Create a config file or use --config <path>\n")
		fmt.Fprintf(os.Stderr, "See config.example.yaml for reference.\n")
		os.Exit(1)
	}

	model, err := ui.NewModel(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
