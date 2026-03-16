// Package config provides utilities for managin application configuration
package config

import (
	"os"
	"strings"
)

const ConfigFile = "baki_config"

// AddPath appends a new path to the cofig file
func AddPath(path string) error {
	f, err := os.OpenFile(ConfigFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.WriteString(path + "\n"); err != nil {
		return err
	}
	return nil
}

// GetPaths reads all paths from the config file and returns them as a slice of strings
func GetPaths() ([]string, error) {
	data, err := os.ReadFile(ConfigFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	var paths []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			paths = append(paths, trimmed)
		}
	}
	return paths, nil
}

// SavePaths overwrites the config file with the provided slice of paths
func SavePaths(paths []string) error {
	var sb strings.Builder
	for _, p := range paths {
		if p != "" {
			sb.WriteString(p + "\n")
		}
	}
	return os.WriteFile(ConfigFile, []byte(sb.String()), 0o644)
}
