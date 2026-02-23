// internal/ui/slashcomplete.go
package ui

import "strings"

type slashCmd struct {
	Name string // e.g. "/resume"
	Desc string // e.g. "Browse conversations"
}

type slashComplete struct {
	matches []slashCmd
	cursor  int
	offset  int // first visible item index (viewport)
}

// slashCmds is the canonical list of all slash commands with descriptions.
var slashCmds = []slashCmd{
	{"/clear", "Clear conversation"},
	{"/exit", "Quit termchat"},
	{"/list", "List saved conversations"},
	{"/load", "Load a conversation"},
	{"/resume", "Browse conversations by date"},
	{"/roles", "Edit role presets"},
	{"/save", "Save conversation"},
	{"/settings", "Model & parameters"},
	{"/shortcuts", "Edit keyboard shortcuts"},
}

// filterSlashCmds returns commands whose Name has input as a prefix.
// input must start with "/".
func filterSlashCmds(input string) []slashCmd {
	var out []slashCmd
	for _, c := range slashCmds {
		if strings.HasPrefix(c.Name, input) {
			out = append(out, c)
		}
	}
	return out
}
