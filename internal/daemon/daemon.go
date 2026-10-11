// Package daemon runs the automatic backups without a user interface:
// the versioned backup of configuration files and the scheduled and
// on-change backups of files. It is what "baky daemon" executes.
package daemon

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/Josedzzz/baky/internal/backup"
	"github.com/Josedzzz/baky/internal/config"
	"github.com/Josedzzz/baky/internal/gitbackup"
	"github.com/Josedzzz/baky/internal/logging"
	"github.com/fsnotify/fsnotify"
)

// reloadDelay groups the several writes of one config save into one reload
const reloadDelay = 500 * time.Millisecond

// settings is the part of the configuration the daemon acts on. A change
// to anything else (the history, for example) does not restart the engines.
type settings struct {
	NasPath      string
	BackupPaths  []pathSetting
	ConfigBackup config.ConfigBackup
}

type pathSetting struct {
	Path      string
	Frequency string
}

func loadSettings() (settings, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return settings{}, err
	}

	s := settings{NasPath: cfg.NasPath, ConfigBackup: cfg.ConfigBackup}
	if s.ConfigBackup.Branch == "" {
		s.ConfigBackup.Branch = config.DefaultBranch
	}
	for _, p := range cfg.BackupPaths {
		s.BackupPaths = append(s.BackupPaths, pathSetting{Path: p.Path, Frequency: p.Frequency})
	}
	return s, nil
}

// Run executes the daemon until ctx is cancelled. The engines are
// restarted whenever the configuration file changes, so changes made
// from the TUI apply without restarting the service.
func Run(ctx context.Context) error {
	logger, err := logging.New(config.LogPath(), os.Stdout)
	if err != nil {
		return fmt.Errorf("cannot open the log file: %w", err)
	}
	defer logger.Close()

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	// The directory is watched, not the file: saving replaces the file,
	// which would silently end a watch placed on the file itself.
	configPath := config.Path()
	if err := watcher.Add(filepath.Dir(configPath)); err != nil {
		return fmt.Errorf("cannot watch the config directory: %w", err)
	}

	current, err := loadSettings()
	if err != nil {
		return err
	}
	stop := startEngines(ctx, current, logger)
	defer func() { stop() }()

	var reload <-chan time.Time
	for {
		select {
		case <-ctx.Done():
			logger.Log(logging.Info, "Daemon detenido.")
			return nil

		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if event.Name == configPath {
				reload = time.After(reloadDelay)
			}

		case <-reload:
			reload = nil
			next, err := loadSettings()
			if err != nil {
				logger.Logf(logging.Error, "No se pudo leer la configuración: %v", err)
				continue
			}
			if reflect.DeepEqual(next, current) {
				continue
			}
			logger.Log(logging.Info, "Configuración modificada, aplicando cambios.")
			stop()
			current = next
			stop = startEngines(ctx, current, logger)

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			logger.Logf(logging.Error, "Error del vigilante de configuración: %v", err)
		}
	}
}

// startEngines starts both backup engines with the given settings and
// returns the function that stops them and waits until they finish
func startEngines(ctx context.Context, s settings, logger *logging.Logger) (stop func()) {
	ctx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup

	logger.Log(logging.Info, "Daemon iniciado. "+describe(s))

	// Versioned backup of configuration files
	if s.ConfigBackup.RepoDir != "" && len(s.ConfigBackup.Watches) > 0 {
		opts := gitbackup.Options{
			RepoDir: s.ConfigBackup.RepoDir,
			Branch:  s.ConfigBackup.Branch,
			Log:     logger.Log,
		}
		for _, w := range s.ConfigBackup.Watches {
			opts.Watches = append(opts.Watches, gitbackup.Watch{Source: w.Source, Dest: w.Dest})
		}

		backuper, err := gitbackup.New(opts)
		if err != nil {
			logger.Logf(logging.Error, "Respaldo de configuraciones desactivado: %v", err)
		} else {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := backuper.Run(ctx); err != nil {
					logger.Logf(logging.Error, "Respaldo de configuraciones detenido: %v", err)
				}
			}()
		}
	}

	// Scheduled and on-change backups of files
	if len(s.BackupPaths) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := backup.Run(ctx, logger.Log); err != nil {
				logger.Logf(logging.Error, "Respaldo de archivos detenido: %v", err)
			}
		}()
	}

	return func() {
		cancel()
		wg.Wait()
	}
}

// describe summarizes what the daemon is watching, for the log
func describe(s settings) string {
	var sources []string
	for _, w := range s.ConfigBackup.Watches {
		sources = append(sources, w.Source)
	}
	watching := "ninguna"
	if len(sources) > 0 {
		watching = strings.Join(sources, ", ")
	}
	return fmt.Sprintf("Configuraciones vigiladas: %s. Rutas de respaldo de archivos: %d.", watching, len(s.BackupPaths))
}
