// Package backup provides the engine for executing and scheduling backups
package backup

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Josedzzz/baky/internal/config"
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

func createTarGz(src, dest string) error {
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	gw := gzip.NewWriter(out)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		header.Name = rel

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		switch {
		case !d.Type().IsRegular():
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = io.Copy(tw, f)
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

	go func() {
		defer watcher.Close()
		// debounce timers map
		timers := make(map[string]*time.Timer)

		for {
			select {
			case event, ok := <-watcher.Events:
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
						if p.Frequency == config.FreqOnChange && strings.HasPrefix(event.Name, p.Path) {
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
			case err, ok := <-watcher.Errors:
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
			removeRecursive(watcher, path)
			watchedPaths.Delete(path)
		}
		return true
	})

	// Add new paths to watch
	for path := range currentPathsToWatch {
		if _, ok := watchedPaths.Load(path); !ok {
			addRecursive(watcher, path)
			watchedPaths.Store(path, struct{}{})
		}
	}

	return nil
}

func addRecursive(w *fsnotify.Watcher, path string) {
	filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			w.Add(p)
		}
		return nil
	})
}

func removeRecursive(w *fsnotify.Watcher, path string) {
	filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			w.Remove(p)
		}
		return nil
	})
}

// StartScheduler initiates a goroutine for daily and weekly backups at 3 AM
func StartScheduler() {
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		lastDailyRun := time.Time{}
		lastWeeklyRun := time.Time{}
		
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
					// Daily backup logic: run once per day at 3 AM
					if now.Hour() == 3 && now.Sub(lastDailyRun) > 24*time.Hour {
						fmt.Printf("Daily backup triggered for: %s\n", p.Path)
						RunBackup(p.Path)
						lastDailyRun = now
					}
				case config.FreqWeekly:
					// Weekly backup logic: run once per week on Sunday at 3 AM
					if now.Weekday() == time.Sunday && now.Hour() == 3 && now.Sub(lastWeeklyRun) > 24*7*time.Hour {
						fmt.Printf("Weekly backup triggered for: %s\n", p.Path)
						RunBackup(p.Path)
						lastWeeklyRun = now
					}
				}
			}
		}
	}()
}
