package restore

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"time"
)

// DiffEntryType describes the kind of change for a file
type DiffEntryType int

const (
	// DiffEntryAdded means the file exists in the new backup but not the old one
	DiffEntryAdded DiffEntryType = iota
	// DiffEntryModified means the file exists in both but with different size or mod time
	DiffEntryModified
	// DiffEntryDeleted means the file exists in the old backup but not the new one
	DiffEntryDeleted
	// DiffEntryUnchanged means the file is identical in both backups
	DiffEntryUnchanged
)

// DiffEntry represents a single file change between two backups
type DiffEntry struct {
	Path       string
	Type       DiffEntryType
	OldSize    int64
	NewSize    int64
	OldModTime time.Time
	NewModTime time.Time
}

// BackupDiff holds the result of comparing two backups
type BackupDiff struct {
	OldBackup     string
	NewBackup     string
	Added         []DiffEntry
	Modified      []DiffEntry
	Deleted       []DiffEntry
	Unchanged     int
	TotalFilesOld int
	TotalFilesNew int
}

// DiffBackups compares two backup archives and returns the differences.
// oldPath is the earlier backup, newPath is the later one.
func DiffBackups(oldPath, newPath string) (*BackupDiff, error) {
	oldFiles, err := readTarIndex(oldPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read old backup: %w", err)
	}

	newFiles, err := readTarIndex(newPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read new backup: %w", err)
	}

	diff := &BackupDiff{
		OldBackup:     oldPath,
		NewBackup:     newPath,
		TotalFilesOld: len(oldFiles),
		TotalFilesNew: len(newFiles),
	}

	// Check for modified and deleted files
	for path, oldInfo := range oldFiles {
		if newInfo, exists := newFiles[path]; exists {
			if oldInfo.Size != newInfo.Size || !oldInfo.ModTime.Equal(newInfo.ModTime) {
				diff.Modified = append(diff.Modified, DiffEntry{
					Path:       path,
					Type:       DiffEntryModified,
					OldSize:    oldInfo.Size,
					NewSize:    newInfo.Size,
					OldModTime: oldInfo.ModTime,
					NewModTime: newInfo.ModTime,
				})
			} else {
				diff.Unchanged++
			}
		} else {
			diff.Deleted = append(diff.Deleted, DiffEntry{
				Path:       path,
				Type:       DiffEntryDeleted,
				OldSize:    oldInfo.Size,
				OldModTime: oldInfo.ModTime,
			})
		}
	}

	// Check for added files
	for path, newInfo := range newFiles {
		if _, exists := oldFiles[path]; !exists {
			diff.Added = append(diff.Added, DiffEntry{
				Path:       path,
				Type:       DiffEntryAdded,
				NewSize:    newInfo.Size,
				NewModTime: newInfo.ModTime,
			})
		}
	}

	return diff, nil
}

// fileInfo stores the metadata we track per file in an archive
type fileInfo struct {
	Size    int64
	ModTime time.Time
}

// readTarIndex reads a tar.gz file and returns a map of file path to metadata
func readTarIndex(path string) (map[string]fileInfo, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return nil, err
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	files := make(map[string]fileInfo)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if header.Typeflag == tar.TypeReg {
			files[header.Name] = fileInfo{
				Size:    header.Size,
				ModTime: header.ModTime,
			}
		}
	}

	return files, nil
}
