// Package config provides utilities for managing application configuration
package config

import (
	"encoding/json"
	"os"
	"sync"
)

const ConfigFile = "config.json"

// Config represents the unified application configuration
type Config struct {
	NasPath     string   `json:"nas_path"`
	BackupPaths []string `json:"backup_paths"`
}

var (
	configLock sync.RWMutex
	currentCfg *Config
)

// loadConfig reads the config file into the currentCfg variable
func loadConfig() (*Config, error) {
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
			currentCfg = &Config{BackupPaths: []string{}}
			return currentCfg, nil
		}
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.BackupPaths == nil {
		cfg.BackupPaths = []string{}
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

// AddPath adds a new backup path
func AddPath(path string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	cfg.BackupPaths = append(cfg.BackupPaths, path)
	return SaveConfig(cfg)
}

// GetPaths returns all configured backup paths
func GetPaths() ([]string, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, err
	}
	return cfg.BackupPaths, nil
}

// SavePaths overwrites the backup paths list
func SavePaths(paths []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	cfg.BackupPaths = paths
	return SaveConfig(cfg)
}

// GetNasPath returns the configured NAS path
func GetNasPath() (string, error) {
	cfg, err := loadConfig()
	if err != nil {
		return "", err
	}
	return cfg.NasPath, nil
}

// SaveNasPath updates the NAS path
func SaveNasPath(path string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	cfg.NasPath = path
	return SaveConfig(cfg)
}
