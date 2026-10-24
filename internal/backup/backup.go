// Package backup provides the engine for executing and scheduling backups
package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Josedzzz/baky/internal/config"
	"github.com/Josedzzz/baky/internal/logging"
	"github.com/Josedzzz/baky/internal/watch"
	"github.com/fsnotify/fsnotify"
)

// RunBackup compresses the contents of sourcePath into a .tar.gz file on the NAS
func RunBackup(sourcePath string) error {
	nasPath, err := config.GetNasPath()
	if err != nil || nasPath == "" {
		return fmt.Errorf("the backup destination is not configured")
	}

	// Check if source path exists
	if _, err := os.Stat(sourcePath); err != nil {
		config.LogBackup(sourcePath, false, fmt.Sprintf("Cannot access the path: %v", err))
		return fmt.Errorf("cannot access the path: %v", err)
	}

	// Create backup filename: [base_name]_[timestamp].tar.gz
	base := filepath.Base(sourcePath)
	timestamp := time.Now().Format("20060102_150405")
	fileName := fmt.Sprintf("%s_%s.tar.gz", base, timestamp)
	destPath := filepath.Join(nasPath, fileName)

	err = createTarGz(sourcePath, destPath)

	// Log the result
	msg := ""
	if err != nil {
		msg = err.Error()
		// Clean up incomplete backup file on failure
		os.Remove(destPath)
	}
	if logErr := config.LogBackup(sourcePath, err == nil, msg); logErr != nil && err == nil {
		return fmt.Errorf("backup created, but the history could not be updated: %w", logErr)
	}

	return err
}

func createTarGz(src, dest string) (err error) {
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	gw := gzip.NewWriter(out)
	tw := tar.NewWriter(gw)

	// Data is only fully flushed on Close, so a failed Close means a
	// truncated archive and must be reported as a backup failure.
	defer func() {
		for _, c := range []io.Closer{tw, gw, out} {
			if cerr := c.Close(); cerr != nil && err == nil {
				err = cerr
			}
		}
	}()

	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		// Symlinks are stored with their real target, not followed
		link := ""
		if info.Mode()&os.ModeSymlink != 0 {
			if link, err = os.Readlink(path); err != nil {
				return err
			}
		}

		header, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		// When the source is a single file, rel is "." which is not a
		// usable entry name: store the file under its own name instead.
		if rel == "." && !d.IsDir() {
			rel = filepath.Base(src)
		}
		header.Name = filepath.ToSlash(rel)

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if !d.Type().IsRegular() {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}

		_, err = io.Copy(tw, f)
		f.Close()
		return err
	})
}

// Debounce is how long Run waits after the last change before backing up
// an "on_change" path, so a burst of writes becomes one backup
var Debounce = 5 * time.Second

// lastAttempt remembers when the scheduler last tried each path, so a
// backup that keeps failing is not retried every minute
var (
	lastAttempt   = make(map[string]time.Time)
	lastAttemptMu sync.Mutex
)

// Run executes the automatic backups until ctx is cancelled: "on_change"
// paths are backed up when their files change, "daily" and "weekly" paths
// at 3 AM. The paths are read once; call Run again after a config change.
func Run(ctx context.Context, logf logging.Func) error {
	if logf == nil {
		logf = func(level, message string) {}
	}

	paths, err := config.GetPaths()
	if err != nil {
		return err
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	var onChange []string
	for _, p := range paths {
		if p.Frequency == config.FreqOnChange {
			onChange = append(onChange, p.Path)
			watch.AddRecursive(watcher, p.Path)
		}
	}

	due := make(chan string, len(paths)+1)
	timers := make(map[string]*time.Timer)
	defer func() {
		for _, t := range timers {
			t.Stop()
		}
	}()

	backup := func(path, reason string) {
		logf(logging.Info, fmt.Sprintf("%s backup started: %s", reason, path))
		if err := RunBackup(path); err != nil {
			logf(logging.Error, fmt.Sprintf("Backup failed: %s (%v)", path, err))
			return
		}
		logf(logging.Info, fmt.Sprintf("Backup completed: %s", path))
	}

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil

		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) == 0 {
				continue
			}
			for _, path := range onChange {
				if !watch.IsWithin(event.Name, path) {
					continue
				}
				// Directories created after startup must be watched too
				watch.AddIfNewDir(watcher, event)

				// Debounce: restart the countdown on every new change
				if t, ok := timers[path]; ok {
					t.Stop()
				}
				timers[path] = time.AfterFunc(Debounce, func() {
					select {
					case due <- path:
					default: // a backup is already queued
					}
				})
			}

		case path := <-due:
			backup(path, "On-change")

		case now := <-ticker.C:
			// Read the paths again: the last backup times change over time
			current, err := config.GetPaths()
			if err != nil {
				logf(logging.Error, fmt.Sprintf("Could not read the configuration: %v", err))
				continue
			}
			for _, p := range current {
				if !scheduledDue(p, now) {
					continue
				}
				lastAttemptMu.Lock()
				recent := now.Sub(lastAttempt[p.Path]) < time.Hour
				lastAttempt[p.Path] = now
				lastAttemptMu.Unlock()
				if recent {
					continue
				}
				backup(p.Path, "Scheduled")
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			logf(logging.Error, fmt.Sprintf("File watcher error: %v", err))
		}
	}
}

// scheduledDue reports whether a daily or weekly path must be backed up
// at the given time. Backups run during the 3 AM hour, and the recorded
// time of the last successful backup keeps them from running twice.
func scheduledDue(p config.BackupPathConfig, now time.Time) bool {
	if now.Hour() != 3 {
		return false
	}
	sinceLast := now.Sub(p.LastBackup)

	switch p.Frequency {
	case config.FreqDaily:
		return sinceLast > 20*time.Hour
	case config.FreqWeekly:
		return now.Weekday() == time.Sunday && sinceLast > 6*24*time.Hour
	}
	return false
}
