// Package restore provides utilities for managing and restoring backups
package restore

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// BackupFilenamePattern matches the backup filename format: name_YYYYMMdd_HHmmss.tar.gz
var backupFilenamePattern = regexp.MustCompile(`^(.+?)_(\d{8})_(\d{6})\.tar\.gz$`)

// ScanBackups scans the NAS directory and returns all available backups
// organized by source path
func ScanBackups(nasPath string) (map[string]*BackupList, error) {
	if nasPath == "" {
		return nil, fmt.Errorf("NAS path not configured")
	}

	// Check if NAS path exists
	info, err := os.Stat(nasPath)
	if err != nil {
		return nil, fmt.Errorf("cannot access NAS path: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("NAS path is not a directory: %s", nasPath)
	}

	// Read all files in NAS directory
	entries, err := os.ReadDir(nasPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read NAS directory: %w", err)
	}

	backupsBySource := make(map[string]*BackupList)

	// Process each file
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()

		// Check if it's a backup file
		if !strings.HasSuffix(filename, ".tar.gz") {
			continue
		}

		// Parse the filename
		backup, err := parseBackupFilename(filename, nasPath)
		if err != nil {
			// Skip files that don't match the pattern
			continue
		}

		// Group by source path
		if _, exists := backupsBySource[backup.SourcePath]; !exists {
			backupsBySource[backup.SourcePath] = &BackupList{
				SourcePath: backup.SourcePath,
				Backups:    []BackupInfo{},
			}
		}

		backupsBySource[backup.SourcePath].Backups = append(
			backupsBySource[backup.SourcePath].Backups,
			*backup,
		)
	}

	// Update counts and sort each backup list by timestamp (newest first)
	for _, backupList := range backupsBySource {
		backupList.Count = len(backupList.Backups)
		sortBackupsByTimestamp(backupList.Backups)
	}

	return backupsBySource, nil
}

// GetAllBackups returns a flat list of all backups, sorted by timestamp (newest first)
func GetAllBackups(nasPath string) ([]BackupInfo, error) {
	backupsBySource, err := ScanBackups(nasPath)
	if err != nil {
		return nil, err
	}

	var allBackups []BackupInfo
	for _, backupList := range backupsBySource {
		allBackups = append(allBackups, backupList.Backups...)
	}

	// Sort all backups by timestamp (newest first)
	sortBackupsByTimestamp(allBackups)

	return allBackups, nil
}

// GetAllBackupsEnhanced returns a flat list of all backups with full source paths from config
func GetAllBackupsEnhanced(nasPath string, configPaths []ConfigPath) ([]BackupInfo, error) {
	backups, err := GetAllBackups(nasPath)
	if err != nil {
		return nil, err
	}

	// Create a map of basename to full paths from config
	pathMap := make(map[string]string)
	for _, configPath := range configPaths {
		basename := filepath.Base(configPath.Path)
		pathMap[basename] = configPath.Path
	}

	// Enhance backups with full paths from config
	for i := range backups {
		basename := backups[i].SourcePath
		if fullPath, exists := pathMap[basename]; exists {
			backups[i].SourcePath = fullPath
		}
	}

	return backups, nil
}

// GetBackupsForSource returns all backups for a specific source path
func GetBackupsForSource(nasPath, sourcePath string) (*BackupList, error) {
	backupsBySource, err := ScanBackups(nasPath)
	if err != nil {
		return nil, err
	}

	if backupList, exists := backupsBySource[sourcePath]; exists {
		return backupList, nil
	}

	return &BackupList{
		SourcePath: sourcePath,
		Backups:    []BackupInfo{},
		Count:      0,
	}, nil
}

// parseBackupFilename parses a backup filename and returns BackupInfo
// Expected format: name_YYYYMMdd_HHmmss.tar.gz
func parseBackupFilename(filename, nasPath string) (*BackupInfo, error) {
	matches := backupFilenamePattern.FindStringSubmatch(filename)
	if len(matches) != 4 {
		return nil, fmt.Errorf("filename does not match backup pattern: %s", filename)
	}

	sourceName := matches[1]
	dateStr := matches[2]
	timeStr := matches[3]

	// Parse timestamp
	timestamp, err := time.Parse("20060102 150405", dateStr+" "+timeStr)
	if err != nil {
		return nil, fmt.Errorf("invalid timestamp in filename: %s", filename)
	}

	// Get file size
	fullPath := filepath.Join(nasPath, filename)
	fileInfo, err := os.Stat(fullPath)
	if err != nil {
		return nil, fmt.Errorf("cannot stat backup file: %w", err)
	}

	backup := &BackupInfo{
		Filename:   filename,
		SourcePath: sourceName, // Will be enhanced by config history
		Timestamp:  timestamp,
		FileSize:   fileInfo.Size(),
		FullPath:   fullPath,
		Result:     "success", // Default to success unless proven otherwise
		Message:    "",
	}

	return backup, nil
}

// EnhanceBackupsWithHistory enriches backup info with data from config history
// This matches backups with their original source paths and result status
func EnhanceBackupsWithHistory(backups []BackupInfo, historyEvents []HistoryEvent) []BackupInfo {
	for i := range backups {
		// Try to match with history based on filename and timestamp
		for _, event := range historyEvents {
			if matchesBackup(&backups[i], event) {
				backups[i].SourcePath = event.SourcePath
				backups[i].Result = event.Result
				backups[i].Message = event.Message
				break
			}
		}
	}
	return backups
}

// matchesBackup checks if a backup matches a history event
func matchesBackup(backup *BackupInfo, event HistoryEvent) bool {
	// Match by source name and timestamp (within 1 second tolerance)
	sourceName := filepath.Base(event.SourcePath)
	return strings.HasPrefix(backup.Filename, sourceName) &&
		backup.Timestamp.Sub(event.Timestamp).Abs() < time.Second
}

// sortBackupsByTimestamp sorts backups by timestamp in descending order (newest first)
// sortBackupsByTimestamp sorts backups by timestamp in descending order (newest first)
func sortBackupsByTimestamp(backups []BackupInfo) {
	sort.Slice(backups, func(i, j int) bool {
		return backups[j].Timestamp.Before(backups[i].Timestamp)
	})
}

// FormatFileSize converts bytes to human-readable format (KB, MB, GB, etc.)
func FormatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %c", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// HistoryEvent is a simple type for history data from config
type HistoryEvent struct {
	SourcePath string
	Timestamp  time.Time
	Result     string
	Message    string
}

// CleanupOldTempDirs removes temporary restore directories older than maxAge
func CleanupOldTempDirs(maxAge time.Duration) error {
	tempRoot := os.TempDir()
	entries, err := os.ReadDir(tempRoot)
	if err != nil {
		return fmt.Errorf("cannot read temp directory: %w", err)
	}

	now := time.Now()
	for _, entry := range entries {
		// Look for baky temp directories
		if !strings.HasPrefix(entry.Name(), "baky-restore-") {
			continue
		}

		tempPath := filepath.Join(tempRoot, entry.Name())
		if !entry.IsDir() {
			continue
		}

		// Get modification time
		info, err := os.Stat(tempPath)
		if err != nil {
			continue
		}

		// Remove if older than maxAge
		if now.Sub(info.ModTime()) > maxAge {
			if err := os.RemoveAll(tempPath); err != nil {
				fmt.Printf("Warning: Could not cleanup temp directory %s: %v\n", tempPath, err)
			}
		}
	}

	return nil
}
