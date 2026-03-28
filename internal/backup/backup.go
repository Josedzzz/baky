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

	// Create backup filename: [base_name]_[timestamp].tar.gz
	base := filepath.Base(sourcePath)
	timestamp := time.Now().Format("20060102_150405")
	fileName := fmt.Sprintf("%s_%s.tar.gz", base, timestamp)
	destPath := filepath.Join(nasPath, fileName)

	err = createTarGz(sourcePath, destPath)

	// Log the result
	logErr := config.LogBackup(sourcePath, err == nil, "")
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

	return filepath.Walk(src, func(file string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(fi, fi.Name())
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(filepath.Dir(src), file)
		if err != nil {
			return err
		}
		header.Name = rel

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if !fi.Mode().IsRegular() {
			return nil
		}

		f, err := os.Open(file)
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = io.Copy(tw, f)
		return err
	})
}

// StartWatcher initiates a goroutine to monitor path changes for "on_change" frequency
func StartWatcher() error {
	watcher, err := fsnotify.NewWatcher()
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
					paths, _ := config.GetPaths()
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

	// Add paths to watcher
	paths, _ := config.GetPaths()
	for _, p := range paths {
		if p.Frequency == config.FreqOnChange {
			addRecursive(watcher, p.Path)
		}
	}

	return nil
}

func addRecursive(w *fsnotify.Watcher, path string) {
	filepath.Walk(path, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if fi.IsDir() {
			w.Add(p)
		}
		return nil
	})
}

// StartScheduler initiates a goroutine for daily and weekly backups at 3 AM
func StartScheduler() {
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		for range ticker.C {
			now := time.Now()
			paths, _ := config.GetPaths()

			for _, p := range paths {
				switch p.Frequency {
				case config.FreqDaily:
					// Daily backup logic
					if now.Hour() == 3 && now.Minute() == 0 && p.LastBackup.Format("2006-01-02") != now.Format("2006-01-02") {
						fmt.Printf("Daily backup triggered for: %s\n", p.Path)
						RunBackup(p.Path)
					}
				case config.FreqWeekly:
					// Weekly backup logic
					if now.Weekday() == time.Sunday && now.Hour() == 3 && now.Minute() == 0 &&
						now.Sub(p.LastBackup).Hours() > 24*6 {
						fmt.Printf("Weekly backup triggered for: %s\n", p.Path)
						RunBackup(p.Path)
					}
				}
			}
		}
	}()
}
