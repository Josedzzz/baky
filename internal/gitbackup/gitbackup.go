// Package gitbackup keeps a versioned backup of configuration directories:
// every change under a watched directory is copied to a Git repository,
// committed and pushed. It is the Go port of the backup-daemon.sh script
// of the magiti-config-backup project.
package gitbackup

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Josedzzz/baky/internal/logging"
	"github.com/Josedzzz/baky/internal/watch"
	"github.com/fsnotify/fsnotify"
)

const (
	// commitPrefix starts the message of every automatic commit
	commitPrefix = "auto-backup"

	defaultDebounce       = 2 * time.Second
	defaultResyncInterval = 10 * time.Minute
)

// Watch maps a directory to watch to a folder inside the repository
type Watch struct {
	Source string // Absolute path of the directory to watch
	Dest   string // Folder inside the repository that receives its files

	// Check is an optional command that validates the configuration in a
	// directory, such as "sudo -n shorewall check {dir}". It runs after
	// every change, and before a restore is applied.
	Check string
	// Apply is an optional command that puts the configuration of a
	// directory into service safely, such as "sudo -n shorewall try {dir}".
	// It only runs when a file is restored from the history.
	Apply string
}

// Options configures a Backuper
type Options struct {
	RepoDir string  // Local clone that receives the backups
	Branch  string  // Remote branch the commits are pushed to
	Watches []Watch // Directories to back up

	// Log receives every event worth recording. Optional.
	Log logging.Func

	// Debounce is how long to wait after the last change before backing
	// up, so a burst of writes becomes one backup. Defaults to 2 seconds.
	Debounce time.Duration

	// ResyncInterval is how often everything is checked again, to pick up
	// anything missed and to retry a failed push. Defaults to 10 minutes.
	ResyncInterval time.Duration
}

// Backuper backs up a set of watched directories to a Git repository
type Backuper struct {
	opts Options

	mu             sync.Mutex // serializes every operation on the repository
	warnedNoRemote bool
	warned         map[string]bool // warnings already logged
	now            func() time.Time
}

// New validates the options and returns a Backuper ready to run
func New(opts Options) (*Backuper, error) {
	if opts.RepoDir == "" {
		return nil, fmt.Errorf("no config repository is set")
	}
	if !IsRepo(opts.RepoDir) {
		return nil, fmt.Errorf("%s is not a valid Git repository", opts.RepoDir)
	}
	if opts.Branch == "" {
		opts.Branch = "main"
	}
	if opts.Log == nil {
		opts.Log = func(level, message string) {}
	}
	if opts.Debounce <= 0 {
		opts.Debounce = defaultDebounce
	}
	if opts.ResyncInterval <= 0 {
		opts.ResyncInterval = defaultResyncInterval
	}

	for _, w := range opts.Watches {
		if !filepath.IsAbs(w.Source) {
			return nil, fmt.Errorf("the path to watch must be absolute: %s", w.Source)
		}
		// The destination must stay one folder inside the repository
		if w.Dest == "" || w.Dest != filepath.Base(w.Dest) || w.Dest == "." || w.Dest == ".." || w.Dest == ".git" {
			return nil, fmt.Errorf("invalid destination for %s: %q", w.Source, w.Dest)
		}
		for _, hook := range []string{w.Check, w.Apply} {
			if err := ValidateHook(hook); err != nil {
				return nil, fmt.Errorf("%s: %w", w.Source, err)
			}
		}
	}

	return &Backuper{opts: opts, now: time.Now, warned: map[string]bool{}}, nil
}

func (b *Backuper) logf(level, format string, args ...any) {
	b.opts.Log(level, fmt.Sprintf(format, args...))
}

// SyncAll backs up every watched directory and pushes the result. It
// returns the number of commits created.
func (b *Backuper) SyncAll() int {
	commits := 0
	for _, w := range b.opts.Watches {
		commits += b.syncWatch(w)
	}
	b.Push()
	return commits
}

// Sync backs up one watched directory and pushes the result. It returns
// the number of commits created.
func (b *Backuper) Sync(w Watch) int {
	commits := b.syncWatch(w)
	b.Push()
	return commits
}

// syncWatch mirrors one watched directory into the repository and commits
// each changed file separately, so the history shows one entry per file
func (b *Backuper) syncWatch(w Watch) int {
	b.mu.Lock()
	defer b.mu.Unlock()

	dest := filepath.Join(b.opts.RepoDir, w.Dest)
	// A file that cannot be read is reported once, not on every sync
	warn := func(msg string) {
		if !b.warned[msg] {
			b.warned[msg] = true
			b.logf(logging.Warn, "%s", msg)
		}
	}

	if err := mirror(w.Source, dest, warn); err != nil {
		// The backup already in the repository is left untouched: a folder
		// that is missing or unreadable must not erase its own history.
		b.logf(logging.Error, "Watched folder not accessible, skipping: %s (%v)", w.Source, err)
		return 0
	}

	changes, err := pendingChanges(b.opts.RepoDir, w.Dest)
	if err != nil {
		b.logf(logging.Error, "Could not get the status of %s: %v", w.Dest, err)
		return 0
	}

	// The configuration is validated as it is right now, once per group of
	// changes. An invalid one is still backed up: the backup must reflect
	// what is on disk, but the commits say so.
	suffix, checkOutput := "", ""
	if len(changes) > 0 && w.Check != "" {
		output, err := runHook(w.Check, w.Source)
		switch {
		case err == nil:
			b.logf(logging.Info, "Validation passed: %s", w.Source)
		case errors.Is(err, errHookNotRun):
			b.logf(logging.Error, "Could not validate %s: %v", w.Source, err)
		default:
			suffix, checkOutput = " "+invalidMarker, lastLines(output, 20)
			b.logf(logging.Warn, "The configuration of %s does NOT pass validation: %s", w.Source, strings.ReplaceAll(lastLines(output, 3), "\n", " | "))
		}
	}

	commits := 0
	for _, c := range changes {
		description := "updated " + filepath.Base(c.path)
		if c.deleted {
			description = "deleted " + filepath.Base(c.path)
		}
		message := fmt.Sprintf("%s: %s (%s)%s", commitPrefix, description, b.now().Format("2006-01-02 15:04:05"), suffix)

		if err := commitPath(b.opts.RepoDir, c.path, message, checkOutput); err != nil {
			b.logf(logging.Error, "Commit of %s failed: %v", c.path, err)
			continue
		}
		b.logf(logging.Info, "Commit created: %s -> %s.", description, c.path)
		commits++
	}

	return commits
}

// Push sends the commits the remote does not have yet. A failure is
// logged and retried on the next sync; the commits stay safe in the local
// repository. It reports whether the remote is up to date afterwards.
func (b *Backuper) Push() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !hasUnpushed(b.opts.RepoDir, b.opts.Branch) {
		return true
	}
	if !hasRemote(b.opts.RepoDir) {
		if !b.warnedNoRemote {
			b.logf(logging.Warn, "The repository has no 'origin' remote; the commits stay local only.")
			b.warnedNoRemote = true
		}
		return false
	}

	if err := push(b.opts.RepoDir, b.opts.Branch); err != nil {
		b.logf(logging.Error, "Push failed. Local commits are saved and will be retried: %v", err)
		return false
	}
	b.logf(logging.Info, "Push succeeded.")
	return true
}

// Run backs up everything once, then watches the directories and backs up
// every change until ctx is cancelled
func (b *Backuper) Run(ctx context.Context) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	addWatches := func() {
		for _, w := range b.opts.Watches {
			watch.AddRecursive(watcher, w.Source)
		}
	}
	addWatches()

	// Catch up with every change made while the daemon was not running,
	// and push whatever a previous run left unpushed
	b.SyncAll()

	due := make(chan int, len(b.opts.Watches))
	timers := make(map[int]*time.Timer)
	defer func() {
		for _, t := range timers {
			t.Stop()
		}
	}()

	resync := time.NewTicker(b.opts.ResyncInterval)
	defer resync.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil

		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename|fsnotify.Chmod) == 0 {
				continue
			}
			if ignored(event.Name) {
				continue
			}
			watch.AddIfNewDir(watcher, event)

			for i, w := range b.opts.Watches {
				if !watch.IsWithin(event.Name, w.Source) {
					continue
				}
				// Debounce: restart the countdown on every new change
				if t, ok := timers[i]; ok {
					t.Stop()
				}
				timers[i] = time.AfterFunc(b.opts.Debounce, func() {
					select {
					case due <- i:
					default: // a backup of this watch is already queued
					}
				})
			}

		case i := <-due:
			b.Sync(b.opts.Watches[i])

		case <-resync.C:
			// A watched directory that was deleted and created again has
			// lost its watch; adding it back is harmless when it has not.
			addWatches()
			b.SyncAll()

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			b.logf(logging.Error, "File watcher error: %v", err)
		}
	}
}
