package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Josedzzz/baky/internal/config"
	"github.com/Josedzzz/baky/internal/daemon"
	"github.com/Josedzzz/baky/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

const usage = `baky - versioned backups of configuration files

Usage:
  baky           Opens the terminal interface
  baky daemon    Runs the automatic backups in the background (used by systemd)
  baky import-magiti <config.json>
                 Imports the watched folders of the magiti-config-backup daemon
  baky help      Shows this help
`

func main() {
	command := ""
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	switch command {
	case "":
		runTUI()
	case "daemon":
		runDaemon()
	case "import-magiti":
		importMagiti(os.Args[2:])
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n%s", command, usage)
		os.Exit(2)
	}
}

// runTUI opens the terminal user interface. The automatic backups are
// not run here: they belong to the daemon.
func runTUI() {
	p := tea.NewProgram(tui.NewModel(), tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// runDaemon runs the automatic backups until the process is asked to stop
func runDaemon() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := daemon.Run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

// importMagiti brings the watched folders of the magiti-config-backup
// daemon into the configuration
func importMagiti(args []string) {
	if len(args) != 1 {
		fmt.Fprint(os.Stderr, "Usage: baky import-magiti <path to the daemon's config.json>\n")
		os.Exit(2)
	}

	added, err := config.ImportMagitiConfig(args[0])
	for _, w := range added {
		fmt.Printf("Folder added: %s  →  %s\n", w.Source, w.Dest)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	fmt.Println("\nThe repository is not imported: set the config repository")
	fmt.Println("from the interface (Config backup → Repository).")
}
