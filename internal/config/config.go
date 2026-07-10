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
	// FreqOnChange triggers a backup whenever a file change is detected
	FreqOnChange = "on_change"
	// FreqDaily triggers a backup once per day at 3 AM
	FreqDaily = "daily"
	// FreqWeekly triggers a backup once per week on Sunday at 3 AM
	FreqWeekly = "weekly"
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

// Config represents the unified application configuration
type Config struct {
	NasPath        string             `json:"nas_path"`
	BackupPaths    []BackupPathConfig `json:"backup_paths"`
	History        []BackupEvent      `json:"history"`
	RestoreHistory []RestoreEvent     `json:"restore_history"`
}

var (
	configLock sync.RWMutex
	currentCfg *Config
	configPath string
)

func init() {
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

// LoadConfig reads the config file into the currentCfg variable
func LoadConfig() (*Config, error) {
	configLock.RLock()
	if currentCfg != nil {
		defer configLock.RUnlock()
		return currentCfg, nil
	}
	configLock.RUnlock()

	configLock.Lock()
	defer configLock.Unlock()

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			currentCfg = &Config{
				BackupPaths:    []BackupPathConfig{},
				History:        []BackupEvent{},
				RestoreHistory: []RestoreEvent{},
			}
			return currentCfg, nil
		}
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		// If unmarshal fails, just reset it.
		currentCfg = &Config{
			BackupPaths:    []BackupPathConfig{},
			History:        []BackupEvent{},
			RestoreHistory: []RestoreEvent{},
		}
		return currentCfg, nil
	}

	if cfg.BackupPaths == nil {
		cfg.BackupPaths = []BackupPathConfig{}
	}
	if cfg.History == nil {
		cfg.History = []BackupEvent{}
	}
	if cfg.RestoreHistory == nil {
		cfg.RestoreHistory = []RestoreEvent{}
	}

	currentCfg = &cfg
	return currentCfg, nil
}

// SaveConfig persists the current configuration to the JSON file
func SaveConfig(cfg *Config) error {
	configLock.Lock()
	defer configLock.Unlock()

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		return err
	}
	currentCfg = cfg
	return nil
}

// AddPath adds a new backup path with a default frequency
func AddPath(path string) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	cfg.BackupPaths = append(cfg.BackupPaths, BackupPathConfig{
		Path:      path,
		Frequency: FreqDaily,
	})
	return SaveConfig(cfg)
}

// GetPaths returns all configured backup paths
func GetPaths() ([]BackupPathConfig, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	return cfg.BackupPaths, nil
}

// SavePaths overwrites the backup paths list
func SavePaths(paths []BackupPathConfig) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	cfg.BackupPaths = paths
	return SaveConfig(cfg)
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
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	cfg.NasPath = path
	return SaveConfig(cfg)
}

// LogBackup adds a new event to the history and updates the LastBackup time for the path
func LogBackup(path string, success bool, message string) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}

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
	if len(cfg.History) > 100 { // Keep last 100 events
		cfg.History = cfg.History[:100]
	}

	// Update last_backup in config
	if success {
		for i, p := range cfg.BackupPaths {
			if p.Path == path {
				cfg.BackupPaths[i].LastBackup = event.Timestamp
				break
			}
		}
	}

	return SaveConfig(cfg)
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
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}

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
	if len(cfg.RestoreHistory) > 100 { // Keep last 100 events
		cfg.RestoreHistory = cfg.RestoreHistory[:100]
	}

	return SaveConfig(cfg)
}

// GetRestoreHistory returns the restore history
func GetRestoreHistory() ([]RestoreEvent, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	return cfg.RestoreHistory, nil
}
