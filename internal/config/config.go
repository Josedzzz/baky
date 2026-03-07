// Package config provides utilities for managin application configuration
package config

import "os"

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

// TODO: Get paths func
