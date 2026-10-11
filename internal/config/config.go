// Package config provides utilities for managing application configuration
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	// ConfigFile is the name of the configuration file
	ConfigFile = "config.json"
	// StateFile is the name of the file holding backup and restore history
	StateFile = "state.json"
	// LogFile is the name of the daemon log file
	LogFile = "baky.log"
	// EnvConfigPath is the environment variable that overrides the config file location
	EnvConfigPath = "BAKY_CONFIG"
	// FreqOnChange triggers a backup whenever a file change is detected
	FreqOnChange = "on_change"
	// FreqDaily triggers a backup once per day at 3 AM
	FreqDaily = "daily"
	// FreqWeekly triggers a backup once per week on Sunday at 3 AM
	FreqWeekly = "weekly"

	maxHistoryEvents = 100
)

// BackupPathConfig represents a configured backup path
type BackupPathConfig struct {
	Path       string    `json:"path"`
	Frequency  string    `json:"frequency"` // "on_change", "daily", "weekly"
	LastBackup time.Time `json:"last_backup"`
}

// BackupEvent represents a single backup completion event
type BackupEvent struct {
	Path      string    `json:"path"`
	Timestamp time.Time `json:"timestamp"`
	Result    string    `json:"result"` // "success", "error"
	Message   string    `json:"message,omitempty"`
}

// RestoreEvent represents a single restore operation event
type RestoreEvent struct {
	BackupFilename string    `json:"backup_filename"`   // The backup file restored
	SourcePath     string    `json:"source_path"`       // Original source path
	RestorePath    string    `json:"restore_path"`      // Where it was restored to
	Timestamp      time.Time `json:"timestamp"`         // When restore was performed
	Result         string    `json:"result"`            // "success", "error"
	Message        string    `json:"message,omitempty"` // Optional info/error message
	Action         string    `json:"action,omitempty"`  // "overwrite", "rename", "skip"
}

// Config represents the unified application configuration. Settings are
// stored in the config file; history and last backup times are stored in
// the state file, so the TUI and the daemon do not overwrite each other.
type Config struct {
	NasPath        string
	BackupPaths    []BackupPathConfig
	ConfigBackup   ConfigBackup
	History        []BackupEvent
	RestoreHistory []RestoreEvent
}

// settingsFile is the on-disk layout of the config file
type settingsFile struct {
	NasPath      string         `json:"nas_path"`
	BackupPaths  []settingsPath `json:"backup_paths"`
	ConfigBackup ConfigBackup   `json:"config_backup"`

	// Legacy fields: older versions kept the history in the config file.
	// They are read once and moved to the state file on the next save.
	History        []BackupEvent  `json:"history,omitempty"`
	RestoreHistory []RestoreEvent `json:"restore_history,omitempty"`
}

type settingsPath struct {
	Path       string     `json:"path"`
	Frequency  string     `json:"frequency"`
	LastBackup *time.Time `json:"last_backup,omitempty"` // legacy, now in the state file
}

// stateFile is the on-disk layout of the state file
type stateFile struct {
	LastBackups    map[string]time.Time `json:"last_backups"`
	History        []BackupEvent        `json:"history"`
	RestoreHistory []RestoreEvent       `json:"restore_history"`
}

var (
	configLock  sync.Mutex
	configPath  string
	loadWarning string
)

func init() {
	if envPath := os.Getenv(EnvConfigPath); envPath != "" {
		configPath = envPath
		os.MkdirAll(filepath.Dir(configPath), 0o755)
		return
	}

	// Use ~/.config/baky for all platforms (Linux standard)
	homeDir, err := os.UserHomeDir()
	if err != nil {
		configPath = "config.json"
		return
	}

	appDir := filepath.Join(homeDir, ".config", "baky")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		configPath = "config.json"
		return
	}

	configPath = filepath.Join(appDir, "config.json")

	// Migration: Check for config in old locations and migrate if needed
	// 1. Check for config in current directory
	if _, err := os.Stat("config.json"); err == nil {
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			if data, err := os.ReadFile("config.json"); err == nil {
				if err := os.WriteFile(configPath, data, 0o600); err == nil {
					if renameErr := os.Rename("config.json", "config.json.bak"); renameErr != nil {
						fmt.Printf("Warning: Could not backup old config: %v\n", renameErr)
					}
				}
			}
		}
	}

	// 2. Check for config in macOS Library/Application Support location
	macosLegacyPath := filepath.Join(homeDir, "Library", "Application Support", "baky", "config.json")
	if _, err := os.Stat(macosLegacyPath); err == nil {
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			if data, err := os.ReadFile(macosLegacyPath); err == nil {
				if err := os.WriteFile(configPath, data, 0o600); err == nil {
					if renameErr := os.Rename(macosLegacyPath, macosLegacyPath+".bak"); renameErr != nil {
						fmt.Printf("Warning: Could not backup old macOS config: %v\n", renameErr)
					}
				}
			}
		}
	}
}

// Path returns the location of the config file
func Path() string {
	configLock.Lock()
	defer configLock.Unlock()
	return configPath
}

// SetPath changes the location of the config file. The state and log
// files live in the same directory.
func SetPath(path string) {
	configLock.Lock()
	defer configLock.Unlock()
	configPath = path
	loadWarning = ""
}

// StatePath returns the location of the state file
func StatePath() string {
	return filepath.Join(filepath.Dir(Path()), StateFile)
}

// LogPath returns the location of the daemon log file
func LogPath() string {
	return filepath.Join(filepath.Dir(Path()), LogFile)
}

// statePath is StatePath for callers that already hold configLock
func statePath() string {
	return filepath.Join(filepath.Dir(configPath), StateFile)
}

// Warning returns a message when the config file could not be read and
// was set aside, or an empty string when everything loaded normally.
func Warning() string {
	configLock.Lock()
	defer configLock.Unlock()
	return loadWarning
}

// LoadConfig reads the config and state files and returns a private copy
// of the configuration. It always reads from disk, because the TUI and
// the daemon are separate processes sharing the same files.
func LoadConfig() (*Config, error) {
	configLock.Lock()
	defer configLock.Unlock()
	return load()
}

// load reads both files. The caller must hold configLock.
func load() (*Config, error) {
	cfg := &Config{
		BackupPaths:    []BackupPathConfig{},
		History:        []BackupEvent{},
		RestoreHistory: []RestoreEvent{},
	}
	lastBackups := map[string]time.Time{}

	var settings settingsFile
	if err := readJSON(configPath, &settings); err != nil {
		return nil, err
	}
	cfg.NasPath = settings.NasPath
	cfg.ConfigBackup = settings.ConfigBackup
	for _, p := range settings.BackupPaths {
		cfg.BackupPaths = append(cfg.BackupPaths, BackupPathConfig{Path: p.Path, Frequency: p.Frequency})
		if p.LastBackup != nil {
			lastBackups[p.Path] = *p.LastBackup
		}
	}
	cfg.History = append(cfg.History, settings.History...)
	cfg.RestoreHistory = append(cfg.RestoreHistory, settings.RestoreHistory...)

	// The state file, when present, replaces the legacy values read above
	if _, err := os.Stat(statePath()); err == nil {
		var state stateFile
		if err := readJSON(statePath(), &state); err != nil {
			return nil, err
		}
		cfg.History = append([]BackupEvent{}, state.History...)
		cfg.RestoreHistory = append([]RestoreEvent{}, state.RestoreHistory...)
		for path, last := range state.LastBackups {
			lastBackups[path] = last
		}
	}

	for i := range cfg.BackupPaths {
		cfg.BackupPaths[i].LastBackup = lastBackups[cfg.BackupPaths[i].Path]
	}

	return cfg, nil
}

// readJSON decodes a JSON file into v. A missing file leaves v untouched.
// A file that is not valid JSON is set aside as "<name>.corrupt" instead
// of being silently overwritten by the next save.
func readJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(data) == 0 {
		return nil
	}

	if err := json.Unmarshal(data, v); err != nil {
		corruptPath := path + ".corrupt"
		if renameErr := os.Rename(path, corruptPath); renameErr != nil {
			return fmt.Errorf("%s is not valid JSON and could not be set aside: %w", path, renameErr)
		}
		loadWarning = fmt.Sprintf("%s was not valid JSON and was saved as %s", filepath.Base(path), filepath.Base(corruptPath))
	}
	return nil
}

// SaveConfig persists the given configuration to the config and state files
func SaveConfig(cfg *Config) error {
	configLock.Lock()
	defer configLock.Unlock()
	return save(cfg)
}

// save writes both files. The caller must hold configLock.
func save(cfg *Config) error {
	settings := settingsFile{
		NasPath:      cfg.NasPath,
		BackupPaths:  []settingsPath{},
		ConfigBackup: cfg.ConfigBackup,
	}
	if settings.ConfigBackup.Watches == nil {
		settings.ConfigBackup.Watches = []WatchConfig{}
	}
	state := stateFile{
		LastBackups:    map[string]time.Time{},
		History:        cfg.History,
		RestoreHistory: cfg.RestoreHistory,
	}
	if state.History == nil {
		state.History = []BackupEvent{}
	}
	if state.RestoreHistory == nil {
		state.RestoreHistory = []RestoreEvent{}
	}

	for _, p := range cfg.BackupPaths {
		settings.BackupPaths = append(settings.BackupPaths, settingsPath{Path: p.Path, Frequency: p.Frequency})
		if !p.LastBackup.IsZero() {
			state.LastBackups[p.Path] = p.LastBackup
		}
	}

	// The state goes first: if the config file still carries legacy history,
	// it must be safely stored before the config is rewritten without it.
	if err := writeJSON(statePath(), state); err != nil {
		return err
	}
	return writeJSON(configPath, settings)
}

// writeJSON writes v to path atomically: a reader never sees a partial file
func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// update applies fn to a fresh copy of the configuration and saves it.
// A file lock keeps the TUI and the daemon from losing each other's changes.
func update(fn func(cfg *Config) error) error {
	configLock.Lock()
	defer configLock.Unlock()

	unlock, err := lockFile(configPath + ".lock")
	if err != nil {
		return err
	}
	defer unlock()

	cfg, err := load()
	if err != nil {
		return err
	}
	if err := fn(cfg); err != nil {
		return err
	}
	return save(cfg)
}

// AddPath adds a new backup path with a default frequency
func AddPath(path string) error {
	return update(func(cfg *Config) error {
		cfg.BackupPaths = append(cfg.BackupPaths, BackupPathConfig{
			Path:      path,
			Frequency: FreqDaily,
		})
		return nil
	})
}

// GetPaths returns all configured backup paths
func GetPaths() ([]BackupPathConfig, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	return cfg.BackupPaths, nil
}

// SavePaths overwrites the backup paths list. The last backup time of each
// path is owned by the state file and is kept as recorded there.
func SavePaths(paths []BackupPathConfig) error {
	return update(func(cfg *Config) error {
		lastBackups := map[string]time.Time{}
		for _, p := range cfg.BackupPaths {
			lastBackups[p.Path] = p.LastBackup
		}

		cfg.BackupPaths = make([]BackupPathConfig, len(paths))
		for i, p := range paths {
			cfg.BackupPaths[i] = BackupPathConfig{
				Path:       p.Path,
				Frequency:  p.Frequency,
				LastBackup: lastBackups[p.Path],
			}
		}
		return nil
	})
}

// GetNasPath returns the configured NAS path
func GetNasPath() (string, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return "", err
	}
	return cfg.NasPath, nil
}

// SaveNasPath updates the NAS path
func SaveNasPath(path string) error {
	return update(func(cfg *Config) error {
		cfg.NasPath = path
		return nil
	})
}

// LogBackup adds a new event to the history and updates the LastBackup time for the path
func LogBackup(path string, success bool, message string) error {
	return update(func(cfg *Config) error {
		result := "success"
		if !success {
			result = "error"
		}

		event := BackupEvent{
			Path:      path,
			Timestamp: time.Now(),
			Result:    result,
			Message:   message,
		}

		// Prepend to history
		cfg.History = append([]BackupEvent{event}, cfg.History...)
		if len(cfg.History) > maxHistoryEvents {
			cfg.History = cfg.History[:maxHistoryEvents]
		}

		// Update the last backup time of the path
		if success {
			for i, p := range cfg.BackupPaths {
				if p.Path == path {
					cfg.BackupPaths[i].LastBackup = event.Timestamp
					break
				}
			}
		}
		return nil
	})
}

// GetHistory returns the backup history
func GetHistory() ([]BackupEvent, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	return cfg.History, nil
}

// LogRestore adds a new restore event to the restore history
func LogRestore(backupFilename, sourcePath, restorePath, action string, success bool, message string) error {
	return update(func(cfg *Config) error {
		result := "success"
		if !success {
			result = "error"
		}

		event := RestoreEvent{
			BackupFilename: backupFilename,
			SourcePath:     sourcePath,
			RestorePath:    restorePath,
			Timestamp:      time.Now(),
			Result:         result,
			Message:        message,
			Action:         action,
		}

		// Prepend to restore history
		cfg.RestoreHistory = append([]RestoreEvent{event}, cfg.RestoreHistory...)
		if len(cfg.RestoreHistory) > maxHistoryEvents {
			cfg.RestoreHistory = cfg.RestoreHistory[:maxHistoryEvents]
		}
		return nil
	})
}

// GetRestoreHistory returns the restore history
func GetRestoreHistory() ([]RestoreEvent, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	return cfg.RestoreHistory, nil
}
