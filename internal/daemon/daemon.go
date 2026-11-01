// Package daemon runs the versioned backup of configuration files without
// a user interface. It is what "baky daemon" executes.
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

	"github.com/Josedzzz/baky/internal/config"
	"github.com/Josedzzz/baky/internal/gitbackup"
	"github.com/Josedzzz/baky/internal/logging"
	"github.com/fsnotify/fsnotify"
)

// reloadDelay groups the several writes of one config save into one reload
const reloadDelay = 500 * time.Millisecond

func loadSettings() (config.ConfigBackup, error) {
	return config.GetConfigBackup()
}

// Run executes the daemon until ctx is cancelled. The backup is restarted
// whenever the configuration file changes, so changes made from the TUI
// apply without restarting the service.
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
	stop := startBackup(ctx, current, logger)
	defer func() { stop() }()

	var reload <-chan time.Time
	for {
		select {
		case <-ctx.Done():
			logger.Log(logging.Info, "Daemon stopped.")
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
				logger.Logf(logging.Error, "Could not read the configuration: %v", err)
				continue
			}
			if reflect.DeepEqual(next, current) {
				continue
			}
			logger.Log(logging.Info, "Configuration changed, applying the changes.")
			stop()
			current = next
			stop = startBackup(ctx, current, logger)

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			logger.Logf(logging.Error, "Config watcher error: %v", err)
		}
	}
}

// startBackup starts the config backup with the given settings and
// returns the function that stops it and waits until it finishes
func startBackup(ctx context.Context, cb config.ConfigBackup, logger *logging.Logger) (stop func()) {
	ctx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup

	logger.Log(logging.Info, "Daemon started. "+describe(cb))

	if cb.RepoDir != "" && len(cb.Watches) > 0 {
		opts := gitbackup.Options{
			RepoDir: cb.RepoDir,
			Branch:  cb.Branch,
			Log:     logger.Log,
		}
		for _, w := range cb.Watches {
			opts.Watches = append(opts.Watches, gitbackup.Watch{Source: w.Source, Dest: w.Dest, Check: w.Check, Apply: w.Apply, Exclude: w.Exclude})
		}

		backuper, err := gitbackup.New(opts)
		if err != nil {
			logger.Logf(logging.Error, "Config backup disabled: %v", err)
		} else {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := backuper.Run(ctx); err != nil {
					logger.Logf(logging.Error, "Config backup stopped: %v", err)
				}
			}()
		}
	}

	return func() {
		cancel()
		wg.Wait()
	}
}

// describe summarizes what the daemon is watching, for the log
func describe(cb config.ConfigBackup) string {
	var sources []string
	for _, w := range cb.Watches {
		sources = append(sources, w.Source)
	}
	watching := "none"
	if len(sources) > 0 {
		watching = strings.Join(sources, ", ")
	}
	return fmt.Sprintf("Watched configurations: %s.", watching)
}
