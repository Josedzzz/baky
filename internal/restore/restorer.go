// Package restore provides utilities for managing and restoring backups
package restore

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// RestoreAction defines what action to take when files exist
type RestoreAction int

const (
	// RestoreActionOverwrite replaces existing files with backup content
	RestoreActionOverwrite RestoreAction = iota
	// RestoreActionRename renames existing files with a timestamp suffix
	RestoreActionRename
	// RestoreActionSkip preserves existing files and aborts the restore
	RestoreActionSkip
)

// RestoreResult contains information about a restore operation
type RestoreResult struct {
	Success         bool
	BackupPath      string
	DestinationPath string
	Action          RestoreAction
	FilesRestored   int
	FilesSkipped    int
	Timestamp       time.Time
	Message         string
	Error           error
}

// ConflictInfo holds information about conflicting files/directories
type ConflictInfo struct {
	Path            string
	IsDirectory     bool
	ExistingSize    int64
	ExistingModTime time.Time
	BackupSize      int64
	BackupModTime   time.Time
}

// CheckForConflicts checks if restoration would overwrite existing files
func CheckForConflicts(destPath string) ([]ConflictInfo, error) {
	var conflicts []ConflictInfo

	if _, err := os.Stat(destPath); err != nil {
		if os.IsNotExist(err) {
			// No conflicts if destination doesn't exist
			return conflicts, nil
		}
		return nil, err
	}

	// Walk through destination and collect existing files
	err := filepath.Walk(destPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(destPath, path)
		if err != nil {
			return fmt.Errorf("cannot compute relative path: %w", err)
		}
		if rel == "." {
			return nil
		}

		conflicts = append(conflicts, ConflictInfo{
			Path:            rel,
			IsDirectory:     info.IsDir(),
			ExistingSize:    info.Size(),
			ExistingModTime: info.ModTime(),
		})

		return nil
	})

	return conflicts, err
}

// stagingPrefix names the temporary files and directories a restore works on
// before anything at the destination is touched
const stagingPrefix = ".baky-restore-"

// Restore performs a restore operation with optional conflict handling.
// The backup is always extracted to a staging location first, so a corrupt
// or unreadable backup never damages what already exists at destPath.
func Restore(backupPath, destPath string, action RestoreAction) (*RestoreResult, error) {
	result := &RestoreResult{
		BackupPath:      backupPath,
		DestinationPath: destPath,
		Action:          action,
		Timestamp:       time.Now(),
	}

	fail := func(prefix string, err error) (*RestoreResult, error) {
		result.Error = err
		result.Message = prefix + err.Error()
		return result, err
	}

	fileName, isSingleFile, err := singleFileBackup(backupPath)
	if err != nil {
		return fail("No se pudo leer el respaldo: ", err)
	}
	if isSingleFile {
		return restoreSingleFile(result, fileName, fail)
	}

	destInfo, statErr := os.Lstat(destPath)
	destExists := statErr == nil
	if statErr != nil && !os.IsNotExist(statErr) {
		return fail("No se pudo revisar el destino: ", statErr)
	}
	if destExists && !destInfo.IsDir() {
		return fail("No se pudo revisar el destino: ", fmt.Errorf("%s existe y no es una carpeta", destPath))
	}

	// Check for conflicts
	conflicts, err := CheckForConflicts(destPath)
	if err != nil {
		return fail("No se pudo revisar el destino: ", err)
	}

	if len(conflicts) > 0 && action == RestoreActionSkip {
		// Don't restore, report as skipped
		result.Success = true
		result.FilesSkipped = len(conflicts)
		result.Message = fmt.Sprintf("Restauración omitida: hay %d archivos existentes en el destino", len(conflicts))
		return result, nil
	}

	fileCount, err := countFilesInBackup(backupPath)
	if err != nil {
		return fail("No se pudo extraer el respaldo: ", err)
	}

	// With "rename" the existing destination is moved away as a whole and
	// replaced; otherwise the backup is merged into the existing directory.
	replaceWhole := !destExists || (len(conflicts) > 0 && action == RestoreActionRename)

	// The staging directory lives next to (or inside) the destination so the
	// final move is a rename on the same filesystem.
	stagingParent := destPath
	if replaceWhole {
		stagingParent = filepath.Dir(destPath)
		if err := os.MkdirAll(stagingParent, 0o755); err != nil {
			return fail("No se pudo extraer el respaldo: ", fmt.Errorf("cannot create destination directory: %w", err))
		}
	}
	staging, err := os.MkdirTemp(stagingParent, stagingPrefix)
	if err != nil {
		return fail("No se pudo extraer el respaldo: ", fmt.Errorf("cannot create staging directory: %w", err))
	}
	defer os.RemoveAll(staging)

	if err := os.Chmod(staging, 0o755); err != nil {
		return fail("No se pudo extraer el respaldo: ", err)
	}
	if err := ExtractBackup(backupPath, staging); err != nil {
		return fail("No se pudo extraer el respaldo: ", err)
	}

	renamedTo := ""
	if replaceWhole {
		if destExists {
			renamedTo = destPath + "_" + time.Now().Format("20060102_150405")
			if err := os.Rename(destPath, renamedTo); err != nil {
				return fail("No se pudo renombrar lo existente: ", err)
			}
		}
		if err := os.Rename(staging, destPath); err != nil {
			if renamedTo != "" {
				// Put the original content back where it was
				os.Rename(renamedTo, destPath)
			}
			return fail("No se pudo extraer el respaldo: ", err)
		}
	} else if err := mergeDir(staging, destPath); err != nil {
		return fail("No se pudo extraer el respaldo: ", err)
	}

	result.Success = true
	result.FilesRestored = fileCount
	result.Message = fmt.Sprintf("Se restauraron %d archivos en %s", fileCount, destPath)
	if fileCount == 1 {
		result.Message = fmt.Sprintf("Se restauró 1 archivo en %s", destPath)
	}
	if renamedTo != "" {
		result.Message += fmt.Sprintf(" (lo existente se renombró a %s)", renamedTo)
	}

	return result, nil
}

// restoreSingleFile restores the backup of one single file. result.DestinationPath
// is the file to write, or an existing directory to place the file into.
func restoreSingleFile(result *RestoreResult, fileName string, fail func(string, error) (*RestoreResult, error)) (*RestoreResult, error) {
	target := result.DestinationPath
	if info, err := os.Stat(target); err == nil && info.IsDir() {
		target = filepath.Join(target, fileName)
	}
	result.DestinationPath = target

	_, statErr := os.Lstat(target)
	targetExists := statErr == nil
	if statErr != nil && !os.IsNotExist(statErr) {
		return fail("No se pudo revisar el destino: ", statErr)
	}

	if targetExists && result.Action == RestoreActionSkip {
		result.Success = true
		result.FilesSkipped = 1
		result.Message = "Restauración omitida: el archivo ya existe en el destino"
		return result, nil
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fail("No se pudo extraer el respaldo: ", fmt.Errorf("cannot create destination directory: %w", err))
	}

	staging, err := os.MkdirTemp(filepath.Dir(target), stagingPrefix)
	if err != nil {
		return fail("No se pudo extraer el respaldo: ", fmt.Errorf("cannot create staging directory: %w", err))
	}
	defer os.RemoveAll(staging)

	staged := filepath.Join(staging, fileName)
	if err := extractSingleFile(result.BackupPath, staged); err != nil {
		return fail("No se pudo extraer el respaldo: ", err)
	}

	renamedTo := ""
	if targetExists && result.Action == RestoreActionRename {
		renamedTo = target + "_" + time.Now().Format("20060102_150405")
		if err := os.Rename(target, renamedTo); err != nil {
			return fail("No se pudo renombrar lo existente: ", err)
		}
	}

	if err := os.Rename(staged, target); err != nil {
		if renamedTo != "" {
			// Put the original file back where it was
			os.Rename(renamedTo, target)
		}
		return fail("No se pudo extraer el respaldo: ", err)
	}

	result.Success = true
	result.FilesRestored = 1
	result.Message = fmt.Sprintf("Se restauró 1 archivo en %s", target)
	if renamedTo != "" {
		result.Message += fmt.Sprintf(" (lo existente se renombró a %s)", renamedTo)
	}

	return result, nil
}

// mergeDir moves everything under src into dst, replacing the files that
// already exist there and leaving every other file in dst untouched.
func mergeDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return fmt.Errorf("cannot compute relative path: %w", err)
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(dst, rel)

		existing, statErr := os.Lstat(target)

		if d.IsDir() {
			if statErr == nil && existing.IsDir() {
				return nil
			}
			if statErr == nil {
				if err := os.Remove(target); err != nil {
					return fmt.Errorf("cannot replace %s: %w", target, err)
				}
			}
			// The directory does not exist in dst: move it in one step
			if err := os.Rename(path, target); err != nil {
				return fmt.Errorf("cannot restore %s: %w", target, err)
			}
			return filepath.SkipDir
		}

		if statErr == nil && existing.IsDir() {
			if err := os.RemoveAll(target); err != nil {
				return fmt.Errorf("cannot replace %s: %w", target, err)
			}
		}
		if err := os.Rename(path, target); err != nil {
			return fmt.Errorf("cannot restore %s: %w", target, err)
		}
		return nil
	})
}

// RestoreToTemporary extracts backup to a temporary location and returns the path
func RestoreToTemporary(backupPath string) (string, error) {
	tempDir := filepath.Join(os.TempDir(), fmt.Sprintf("baky-restore-%d", time.Now().UnixNano()))

	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		return "", fmt.Errorf("cannot create temporary directory: %w", err)
	}

	if err := ExtractBackup(backupPath, tempDir); err != nil {
		os.RemoveAll(tempDir)
		return "", err
	}

	return tempDir, nil
}

// GetOriginalSourcePath attempts to determine the original source path from a backup
// This is done by looking at the backup filename
func GetOriginalSourcePath(backup BackupInfo) string {
	return backup.SourcePath
}

// ValidateRestorePath checks if a path is valid for restoration
func ValidateRestorePath(path string) error {
	// Check if path is empty
	if path == "" {
		return fmt.Errorf("la ruta no puede estar vacía")
	}

	// Check if path is absolute
	if !filepath.IsAbs(path) {
		return fmt.Errorf("la ruta debe ser absoluta: %s", path)
	}

	// Check if parent directory exists
	parentDir := filepath.Dir(path)
	if _, err := os.Stat(parentDir); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("la carpeta padre no existe: %s", parentDir)
		}
		return fmt.Errorf("no se puede acceder a la carpeta padre: %w", err)
	}

	return nil
}

// CleanupTemporary removes a temporary restore directory
func CleanupTemporary(path string) error {
	return os.RemoveAll(path)
}

// countFilesInBackup counts the number of files in a tar.gz backup
func countFilesInBackup(backupPath string) (int, error) {
	file, err := os.Open(backupPath)
	if err != nil {
		return 0, fmt.Errorf("cannot open backup file: %w", err)
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return 0, fmt.Errorf("cannot read gzip: %w", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	fileCount := 0

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, fmt.Errorf("error reading tar: %w", err)
		}

		// Count only regular files, not directories
		if header.Typeflag == tar.TypeReg {
			fileCount++
		}
	}

	return fileCount, nil
}
