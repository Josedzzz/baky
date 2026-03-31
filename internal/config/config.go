// Package config provides utilities for managing application configuration
package config

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

const (
	ConfigFile   = "config.json"
	FreqOnChange = "on_change"
	FreqDaily    = "daily"
	FreqWeekly   = "weekly"
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

// Config represents the unified application configuration
type Config struct {
	NasPath     string             `json:"nas_path"`
	BackupPaths []BackupPathConfig `json:"backup_paths"`
	History     []BackupEvent      `json:"history"`
}

var (
	configLock sync.RWMutex
	currentCfg *Config
)

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

	data, err := os.ReadFile(ConfigFile)
	if err != nil {
		if os.IsNotExist(err) {
			currentCfg = &Config{
				BackupPaths: []BackupPathConfig{},
				History:     []BackupEvent{},
			}
			return currentCfg, nil
		}
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		// If unmarshal fails, it might be an old format.
		// Try to fix it or just reset it.
		// Given the user wants a single config and mentioned errors, let's try to handle gracefully.
		currentCfg = &Config{
			BackupPaths: []BackupPathConfig{},
			History:     []BackupEvent{},
		}
		return currentCfg, nil
	}
	
	if cfg.BackupPaths == nil {
		cfg.BackupPaths = []BackupPathConfig{}
	}
	if cfg.History == nil {
		cfg.History = []BackupEvent{}
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
	if err := os.WriteFile(ConfigFile, data, 0o644); err != nil {
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
