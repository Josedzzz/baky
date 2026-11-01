// Package config provides utilities for managing application configuration
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const (
	// ConfigFile is the name of the configuration file
	ConfigFile = "config.json"
	// LogFile is the name of the daemon log file
	LogFile = "baky.log"
	// EnvConfigPath is the environment variable that overrides the config file location
	EnvConfigPath = "BAKY_CONFIG"
)

// Config represents the application configuration. Fields of older
// versions, such as the file backup paths, are ignored when read and
// dropped on the next save.
type Config struct {
	ConfigBackup ConfigBackup `json:"config_backup"`
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

// SetPath changes the location of the config file. The log file lives in
// the same directory.
func SetPath(path string) {
	configLock.Lock()
	defer configLock.Unlock()
	configPath = path
	loadWarning = ""
}

// LogPath returns the location of the daemon log file
func LogPath() string {
	return filepath.Join(filepath.Dir(Path()), LogFile)
}

// Warning returns a message when the config file could not be read and
// was set aside, or an empty string when everything loaded normally.
func Warning() string {
	configLock.Lock()
	defer configLock.Unlock()
	return loadWarning
}

// LoadConfig reads the config file and returns a private copy of the
// configuration. It always reads from disk, because the TUI and the
// daemon are separate processes sharing the same file.
func LoadConfig() (*Config, error) {
	configLock.Lock()
	defer configLock.Unlock()
	return load()
}

// load reads the file. The caller must hold configLock.
func load() (*Config, error) {
	cfg := &Config{}
	if err := readJSON(configPath, cfg); err != nil {
		return nil, err
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

// save writes the file. The caller must hold configLock.
func save(cfg *Config) error {
	out := *cfg
	if out.ConfigBackup.Watches == nil {
		out.ConfigBackup.Watches = []WatchConfig{}
	}
	return writeJSON(configPath, out)
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
