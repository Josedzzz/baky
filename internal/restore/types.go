// Package restore provides utilities for managing and restoring backups
package restore

import "time"

// BackupInfo represents metadata about a single backup
type BackupInfo struct {
	Filename   string    `json:"filename"`          // "documents_20260417_143022.tar.gz"
	SourcePath string    `json:"source_path"`       // Original source path
	Timestamp  time.Time `json:"timestamp"`         // Extracted from filename
	FileSize   int64     `json:"file_size"`         // Size on disk in bytes
	Result     string    `json:"result"`            // "success" or "error"
	FullPath   string    `json:"full_path"`         // Complete path including NAS location
	Message    string    `json:"message,omitempty"` // Optional error message
}

// BackupList represents a collection of backups, typically all backups for a source
type BackupList struct {
	SourcePath string       `json:"source_path"`
	Backups    []BackupInfo `json:"backups"`
	Count      int          `json:"count"`
}

// ConfigPath represents a backup path from the config
type ConfigPath struct {
	Path string
}
