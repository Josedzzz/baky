package main

import (
	"fmt"
	"os"
	"time"

	"github.com/Josedzzz/baky/internal/backup"
	"github.com/Josedzzz/baky/internal/restore"
	"github.com/Josedzzz/baky/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// Clean up old temporary restore directories (older than 7 days)
	if err := restore.CleanupOldTempDirs(7 * 24 * time.Hour); err != nil {
		fmt.Printf("Warning: Could not cleanup old temp directories: %v\n", err)
	}

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
