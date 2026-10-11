package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Josedzzz/baky/internal/config"
	"github.com/Josedzzz/baky/internal/daemon"
	"github.com/Josedzzz/baky/internal/restore"
	"github.com/Josedzzz/baky/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

const usage = `baky - respaldo de archivos y configuraciones

Uso:
  baky           Abre la interfaz de terminal
  baky daemon    Ejecuta los respaldos automáticos en segundo plano (lo usa systemd)
  baky import-magiti <config.json>
                 Importa las carpetas vigiladas del daemon magiti-config-backup
  baky help      Muestra esta ayuda
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
		fmt.Fprintf(os.Stderr, "Comando desconocido: %s\n\n%s", command, usage)
		os.Exit(2)
	}
}

// runTUI opens the terminal user interface. The automatic backups are
// not run here: they belong to the daemon.
func runTUI() {
	// Clean up old temporary restore directories (older than 7 days)
	if err := restore.CleanupOldTempDirs(7 * 24 * time.Hour); err != nil {
		fmt.Printf("Warning: Could not cleanup old temp directories: %v\n", err)
	}

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
		fmt.Fprint(os.Stderr, "Uso: baky import-magiti <ruta al config.json del daemon>\n")
		os.Exit(2)
	}

	added, err := config.ImportMagitiConfig(args[0])
	for _, w := range added {
		fmt.Printf("Carpeta agregada: %s  →  %s\n", w.Source, w.Dest)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	fmt.Println("\nEl repositorio no se importa: configure el repositorio de configuraciones")
	fmt.Println("desde la interfaz (Respaldo de configuraciones → Repositorio).")
}
