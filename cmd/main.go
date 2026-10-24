package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Josedzzz/baky/internal/backup"
	"github.com/Josedzzz/baky/internal/config"
	"github.com/Josedzzz/baky/internal/daemon"
	"github.com/Josedzzz/baky/internal/restore"
	"github.com/Josedzzz/baky/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

const usage = `baky - file and configuration backups

Usage:
  baky           Opens the terminal interface
  baky daemon    Runs the automatic backups in the background (used by systemd)
  baky backup <path>
                 Backs up a path to the configured destination (NAS) and shows the file created
  baky restore <backup.tar.gz> <destination> [overwrite|rename|skip]
                 Restores a backup to the destination (by default, renames what exists)
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
	case "backup":
		runBackupCmd(os.Args[2:])
	case "restore":
		runRestoreCmd(os.Args[2:])
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

// runBackupCmd backs up a single path to the configured NAS, without the TUI.
// It uses the same engine as the interface and the daemon (backup.RunBackup).
func runBackupCmd(args []string) {
	if len(args) != 1 {
		fmt.Fprint(os.Stderr, "Usage: baky backup <path>\n")
		os.Exit(2)
	}
	source := args[0]

	if err := backup.RunBackup(source); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	// Report the archive just created (the newest one in the NAS folder).
	if created := newestBackup(source); created != "" {
		fmt.Printf("Backup created: %s\n", created)
	} else {
		fmt.Println("Backup created.")
	}
}

// newestBackup returns the most recent .tar.gz in the NAS folder whose name
// matches the source's base name, or "" if it cannot be determined.
func newestBackup(source string) string {
	nasPath, err := config.GetNasPath()
	if err != nil || nasPath == "" {
		return ""
	}
	prefix := filepath.Base(source) + "_"

	entries, err := os.ReadDir(nasPath)
	if err != nil {
		return ""
	}
	var newest string
	var newestMod time.Time
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ".tar.gz") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(newestMod) {
			newestMod = info.ModTime()
			newest = filepath.Join(nasPath, name)
		}
	}
	return newest
}

// runRestoreCmd restores a backup to a destination, without the TUI.
func runRestoreCmd(args []string) {
	if len(args) < 2 || len(args) > 3 {
		fmt.Fprint(os.Stderr, "Usage: baky restore <backup.tar.gz> <destination> [overwrite|rename|skip]\n")
		os.Exit(2)
	}
	backupPath, destPath := args[0], args[1]

	// Default: rename existing files, so a restore never deletes what is there.
	action := restore.RestoreActionRename
	actionName := "rename"
	if len(args) == 3 {
		switch args[2] {
		case "overwrite":
			action, actionName = restore.RestoreActionOverwrite, "overwrite"
		case "rename":
			action, actionName = restore.RestoreActionRename, "rename"
		case "skip":
			action, actionName = restore.RestoreActionSkip, "skip"
		default:
			fmt.Fprintf(os.Stderr, "Unknown action: %s (use overwrite, rename or skip)\n", args[2])
			os.Exit(2)
		}
	}

	result, err := restore.Restore(backupPath, destPath, action)

	// Record the restore in the history, as the interface does.
	msg := ""
	if result != nil {
		msg = result.Message
	}
	config.LogRestore(filepath.Base(backupPath), "", destPath, actionName, err == nil, msg)

	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	fmt.Printf("Restore completed: %d file(s) in %s\n", result.FilesRestored, destPath)
}
