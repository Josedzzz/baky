package main

import (
	"fmt"
	"os"

	"github.com/Josedzzz/baky/internal/backup"
	"github.com/Josedzzz/baky/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// Start background backup engine
	if err := backup.StartWatcher(); err != nil {
		fmt.Printf("Warning: Could not start file watcher: %v\n", err)
	}
	backup.StartScheduler()

	p := tea.NewProgram(tui.NewModel(), tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
