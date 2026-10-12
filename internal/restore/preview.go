package restore

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// ChangeKind tells what restoring a backup would do to one file
type ChangeKind int

const (
	// ChangeModified means the file exists at the destination with different content
	ChangeModified ChangeKind = iota
	// ChangeNew means the file does not exist at the destination and would be created
	ChangeNew
	// ChangeSame means the file at the destination already equals the backup
	ChangeSame
)

// FileChange describes what a restore would do to one file of the backup
type FileChange struct {
	Path       string // Relative to the restore destination
	Kind       ChangeKind
	BackupPath string // The file as extracted from the backup
	DestPath   string // Where the restore would write it
}

// Preview describes what restoring a backup to a destination would change,
// without changing anything. The backup is extracted to a temporary
// directory; Close removes it.
type Preview struct {
	DestPath string
	Files    []FileChange // Modified files first, then new ones, then unchanged ones

	Modified int
	New      int
	Same     int
	// OnlyInDest counts the files of the destination that are not in the
	// backup. "Overwrite" keeps them; "rename" moves them away with the rest.
	OnlyInDest int

	tempDir string
}

// NewPreview extracts the backup to a temporary directory and compares
// it with what exists at destPath
func NewPreview(backupPath, destPath string) (*Preview, error) {
	fileName, isSingleFile, err := singleFileBackup(backupPath)
	if err != nil {
		return nil, err
	}

	tempDir, err := RestoreToTemporary(backupPath)
	if err != nil {
		return nil, err
	}
	p := &Preview{DestPath: destPath, tempDir: tempDir}

	if isSingleFile {
		// Same rule as the restore: an existing directory receives the file
		target := destPath
		if info, err := os.Stat(target); err == nil && info.IsDir() {
			target = filepath.Join(target, fileName)
		}
		p.DestPath = target
		p.add(filepath.Base(target), filepath.Join(tempDir, fileName), target)
		return p, nil
	}

	inBackup := map[string]bool{}
	err = filepath.WalkDir(tempDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(tempDir, path)
		if err != nil {
			return err
		}
		inBackup[rel] = true
		p.add(rel, path, filepath.Join(destPath, rel))
		return nil
	})
	if err != nil {
		p.Close()
		return nil, fmt.Errorf("no se pudo revisar el respaldo: %w", err)
	}

	// Files of the destination the backup knows nothing about
	if info, err := os.Stat(destPath); err == nil && info.IsDir() {
		filepath.WalkDir(destPath, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil || d.IsDir() {
				return nil
			}
			if rel, err := filepath.Rel(destPath, path); err == nil && !inBackup[rel] {
				p.OnlyInDest++
			}
			return nil
		})
	}

	// What changes comes first: it is what the user needs to review
	sort.SliceStable(p.Files, func(i, j int) bool {
		if p.Files[i].Kind != p.Files[j].Kind {
			return p.Files[i].Kind < p.Files[j].Kind
		}
		return p.Files[i].Path < p.Files[j].Path
	})
	return p, nil
}

// add classifies one file of the backup against the destination
func (p *Preview) add(rel, backupPath, destPath string) {
	change := FileChange{Path: rel, BackupPath: backupPath, DestPath: destPath}

	switch same, exists := sameFile(backupPath, destPath); {
	case !exists:
		change.Kind = ChangeNew
		p.New++
	case same:
		change.Kind = ChangeSame
		p.Same++
	default:
		change.Kind = ChangeModified
		p.Modified++
	}
	p.Files = append(p.Files, change)
}

// HasChanges reports whether the restore would create or modify any file
func (p *Preview) HasChanges() bool {
	return p.Modified > 0 || p.New > 0
}

// Close removes the temporary copy of the backup
func (p *Preview) Close() error {
	if p == nil || p.tempDir == "" {
		return nil
	}
	return CleanupTemporary(p.tempDir)
}

// sameFile reports whether two files have the same content, and whether
// the second one exists at all. Symlinks are compared by their target.
func sameFile(a, b string) (same, exists bool) {
	infoA, errA := os.Lstat(a)
	infoB, errB := os.Lstat(b)
	if errB != nil {
		return false, false
	}
	if errA != nil {
		return false, true
	}

	linkA := infoA.Mode()&os.ModeSymlink != 0
	linkB := infoB.Mode()&os.ModeSymlink != 0
	if linkA || linkB {
		targetA, _ := os.Readlink(a)
		targetB, _ := os.Readlink(b)
		return linkA && linkB && targetA == targetB, true
	}
	if !infoB.Mode().IsRegular() || infoA.Size() != infoB.Size() {
		return false, true
	}

	fileA, err := os.Open(a)
	if err != nil {
		return false, true
	}
	defer fileA.Close()
	fileB, err := os.Open(b)
	if err != nil {
		return false, true
	}
	defer fileB.Close()

	// Compare in chunks, so large files are never loaded whole
	bufA := make([]byte, 64*1024)
	bufB := make([]byte, 64*1024)
	for {
		nA, errA := io.ReadFull(fileA, bufA)
		nB, errB := io.ReadFull(fileB, bufB)
		if nA != nB || !bytes.Equal(bufA[:nA], bufB[:nB]) {
			return false, true
		}
		if errA != nil || errB != nil {
			// Both files ended at the same point when both reads stopped
			return errA != nil && errB != nil, true
		}
	}
}
