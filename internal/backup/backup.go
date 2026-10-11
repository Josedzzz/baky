// Package backup provides the engine for executing and scheduling backups
package backup

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Josedzzz/baky/internal/config"
	"github.com/Josedzzz/baky/internal/watch"
	"github.com/fsnotify/fsnotify"
)

// RunBackup compresses the contents of sourcePath into a .tar.gz file on the NAS
func RunBackup(sourcePath string) error {
	nasPath, err := config.GetNasPath()
	if err != nil || nasPath == "" {
		return fmt.Errorf("NAS path not configured")
	}

	// Check if source path exists
	if _, err := os.Stat(sourcePath); err != nil {
		config.LogBackup(sourcePath, false, fmt.Sprintf("Source path error: %v", err))
		return fmt.Errorf("source path error: %v", err)
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
		if rmErr := os.Remove(destPath); rmErr != nil {
			fmt.Printf("Warning: Could not clean up failed backup: %v\n", rmErr)
		}
	}
	logErr := config.LogBackup(sourcePath, err == nil, msg)
	if logErr != nil {
		fmt.Printf("Error logging backup: %v\n", logErr)
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

var (
	watcher      *fsnotify.Watcher
	watchedPaths sync.Map // map[string]struct{}
)

// StartWatcher initiates a goroutine to monitor path changes for "on_change" frequency
func StartWatcher() error {
	if watcher != nil {
		watcher.Close()
	}

	var err error
	watcher, err = fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	w := watcher
	go func() {
		defer w.Close()
		// debounce timers map
		timers := make(map[string]*time.Timer)

		for {
			select {
			case event, ok := <-w.Events:
				if !ok {
					return
				}
				if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) != 0 {
					// Find which backup path this event belongs to
					// Take snapshot of paths to avoid race condition with config updates
					paths, pathErr := config.GetPaths()
					if pathErr != nil {
						fmt.Printf("Error getting paths: %v\n", pathErr)
						continue
					}
					for _, p := range paths {
						if p.Frequency == config.FreqOnChange && watch.IsWithin(event.Name, p.Path) {
							// Directories created after startup must be watched too
							watch.AddIfNewDir(w, event)
							// Debounce 5 seconds
							if t, ok := timers[p.Path]; ok {
								t.Stop()
							}
							timers[p.Path] = time.AfterFunc(5*time.Second, func() {
								fmt.Printf("OnChange backup triggered for: %s\n", p.Path)
								RunBackup(p.Path)
							})
						}
					}
				}
			case err, ok := <-w.Errors:
				if !ok {
					return
				}
				fmt.Printf("Watcher error: %v\n", err)
			}
		}
	}()

	return UpdateWatcher()
}

// UpdateWatcher refreshes the list of watched paths based on current config
func UpdateWatcher() error {
	if watcher == nil {
		return nil
	}

	paths, err := config.GetPaths()
	if err != nil {
		return err
	}

	// Paths that should be watched now
	currentPathsToWatch := make(map[string]bool)
	for _, p := range paths {
		if p.Frequency == config.FreqOnChange {
			currentPathsToWatch[p.Path] = true
		}
	}

	// Remove paths that are no longer watched
	watchedPaths.Range(func(key, value any) bool {
		path := key.(string)
		if !currentPathsToWatch[path] {
			watch.RemoveRecursive(watcher, path)
			watchedPaths.Delete(path)
		}
		return true
	})

	// Add paths to watch. Paths already watched are re-added on purpose
	// (Add is idempotent): a removal above may have dropped a directory
	// that another configured path still needs.
	for path := range currentPathsToWatch {
		watch.AddRecursive(watcher, path)
		watchedPaths.Store(path, struct{}{})
	}

	return nil
}

// StartScheduler initiates a goroutine for daily and weekly backups at 3 AM
func StartScheduler() {
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		lastDailyRun := make(map[string]time.Time)
		lastWeeklyRun := make(map[string]time.Time)

		for range ticker.C {
			now := time.Now()
			paths, pathErr := config.GetPaths()
			if pathErr != nil {
				fmt.Printf("Error getting paths for scheduler: %v\n", pathErr)
				continue
			}

			for _, p := range paths {
				switch p.Frequency {
				case config.FreqDaily:
					last, ok := lastDailyRun[p.Path]
					if !ok {
						last = time.Time{}
					}
					if now.Hour() == 3 && (last.IsZero() || now.Sub(last) > 24*time.Hour) {
						fmt.Printf("Daily backup triggered for: %s\n", p.Path)
						RunBackup(p.Path)
						lastDailyRun[p.Path] = now
					}
				case config.FreqWeekly:
					last, ok := lastWeeklyRun[p.Path]
					if !ok {
						last = time.Time{}
					}
					if now.Weekday() == time.Sunday && now.Hour() == 3 && (last.IsZero() || now.Sub(last) > 24*7*time.Hour) {
						fmt.Printf("Weekly backup triggered for: %s\n", p.Path)
						RunBackup(p.Path)
						lastWeeklyRun[p.Path] = now
					}
				}
			}
		}
	}()
}
