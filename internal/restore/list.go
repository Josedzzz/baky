// Package restore provides utilities for managing and restoring backups
package restore

import (
	"sort"
	"time"
)

// FilterBackupsBySource returns backups that match a specific source path
func FilterBackupsBySource(backups []BackupInfo, sourcePath string) []BackupInfo {
	var filtered []BackupInfo
	for _, backup := range backups {
		if backup.SourcePath == sourcePath {
			filtered = append(filtered, backup)
		}
	}
	return filtered
}

// FilterBackupsByDateRange returns backups within a specific date range
func FilterBackupsByDateRange(backups []BackupInfo, from, to time.Time) []BackupInfo {
	var filtered []BackupInfo
	for _, backup := range backups {
		if backup.Timestamp.After(from) && backup.Timestamp.Before(to) {
			filtered = append(filtered, backup)
		}
	}
	return filtered
}

// FilterBackupsByStatus returns backups with a specific result status
func FilterBackupsByStatus(backups []BackupInfo, status string) []BackupInfo {
	var filtered []BackupInfo
	for _, backup := range backups {
		if backup.Result == status {
			filtered = append(filtered, backup)
		}
	}
	return filtered
}

// GroupBackupsBySource organizes backups by their source path
func GroupBackupsBySource(backups []BackupInfo) map[string][]BackupInfo {
	grouped := make(map[string][]BackupInfo)
	for _, backup := range backups {
		grouped[backup.SourcePath] = append(grouped[backup.SourcePath], backup)
	}
	// Sort each group by timestamp (newest first)
	for source := range grouped {
		SortBackupsByTimestamp(grouped[source])
	}
	return grouped
}

// GetLatestBackupForSource returns the most recent backup for a source
func GetLatestBackupForSource(backups []BackupInfo, sourcePath string) *BackupInfo {
	var latest *BackupInfo
	for i := range backups {
		if backups[i].SourcePath == sourcePath {
			if latest == nil || backups[i].Timestamp.After(latest.Timestamp) {
				latest = &backups[i]
			}
		}
	}
	return latest
}

// GetUniqueSources returns a sorted list of unique source paths from backups
func GetUniqueSources(backups []BackupInfo) []string {
	sourceMap := make(map[string]bool)
	for _, backup := range backups {
		sourceMap[backup.SourcePath] = true
	}

	var sources []string
	for source := range sourceMap {
		sources = append(sources, source)
	}

	sort.Strings(sources)
	return sources
}

// SortBackupsByTimestamp sorts backups in-place by timestamp (newest first)
func SortBackupsByTimestamp(backups []BackupInfo) {
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Timestamp.After(backups[j].Timestamp)
	})
}

// SortBackupsBySize sorts backups in-place by file size (largest first)
func SortBackupsBySize(backups []BackupInfo) {
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].FileSize > backups[j].FileSize
	})
}

// SortBackupsByName sorts backups in-place by filename (alphabetically)
func SortBackupsByName(backups []BackupInfo) {
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Filename < backups[j].Filename
	})
}

// BackupStatistics returns aggregated statistics about backups
type BackupStatistics struct {
	TotalCount         int
	SuccessCount       int
	FailureCount       int
	TotalSize          int64
	OldestBackup       time.Time
	NewestBackup       time.Time
	UniqueSourcesCount int
}

// GetStatistics computes statistics from a list of backups
func GetStatistics(backups []BackupInfo) BackupStatistics {
	stats := BackupStatistics{
		TotalCount: len(backups),
	}

	if len(backups) == 0 {
		return stats
	}

	sources := make(map[string]bool)

	for _, backup := range backups {
		stats.TotalSize += backup.FileSize

		if backup.Result == "success" {
			stats.SuccessCount++
		} else {
			stats.FailureCount++
		}

		sources[backup.SourcePath] = true

		if stats.OldestBackup.IsZero() || backup.Timestamp.Before(stats.OldestBackup) {
			stats.OldestBackup = backup.Timestamp
		}
		if stats.NewestBackup.IsZero() || backup.Timestamp.After(stats.NewestBackup) {
			stats.NewestBackup = backup.Timestamp
		}
	}

	stats.UniqueSourcesCount = len(sources)

	return stats
}
