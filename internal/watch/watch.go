// Package watch provides helpers to watch directory trees with fsnotify
package watch

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
)

// IsWithin reports whether name is root itself or lives somewhere under it
func IsWithin(name, root string) bool {
	root = filepath.Clean(root)
	return name == root || strings.HasPrefix(name, root+string(filepath.Separator))
}

// AddRecursive watches path and every directory below it. fsnotify is not
// recursive, so each directory needs its own watch.
func AddRecursive(w *fsnotify.Watcher, path string) {
	// A single file is watched through its parent directory, so the watch
	// survives editors that replace the file instead of writing in place.
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		w.Add(filepath.Dir(path))
		return
	}

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

// RemoveRecursive stops watching path and every directory below it
func RemoveRecursive(w *fsnotify.Watcher, path string) {
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

// AddIfNewDir starts watching the directory an event reports as created.
// Without it, files written inside directories created after startup
// would go unnoticed.
func AddIfNewDir(w *fsnotify.Watcher, event fsnotify.Event) {
	if event.Op&fsnotify.Create == 0 {
		return
	}
	if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
		AddRecursive(w, event.Name)
	}
}
