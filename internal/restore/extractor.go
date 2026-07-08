// Package restore provides utilities for managing and restoring backups
package restore

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ExtractBackup extracts a tar.gz backup file to a destination directory
// If the backup contains a single root directory, it extracts the contents of that directory
// Otherwise, it extracts all files to the destination
func ExtractBackup(backupPath, destPath string) error {
	rootDir, singleRoot, err := inspectBackup(backupPath)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(destPath, 0o755); err != nil {
		return fmt.Errorf("cannot create destination directory: %w", err)
	}

	return extractFiles(backupPath, destPath, rootDir, singleRoot)
}

// inspectBackup opens a tar.gz file and scans headers to detect
// if all files are under a single root directory.
func inspectBackup(backupPath string) (string, bool, error) {
	file, err := os.Open(backupPath)
	if err != nil {
		return "", false, fmt.Errorf("cannot open backup file: %w", err)
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return "", false, fmt.Errorf("cannot read gzip: %w", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)

	var rootDir string
	headerCount := 0

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", false, fmt.Errorf("error reading tar: %w", err)
		}
		headerCount++

		if rootDir == "" && header.Name != "" {
			parts := strings.Split(strings.TrimSuffix(header.Name, "/"), string(filepath.Separator))
			if len(parts) > 0 {
				rootDir = parts[0]
			}
		}
	}

	// Second pass to verify all files share the same root directory
	singleRoot := rootDir != "" && headerCount > 0
	if _, err := file.Seek(0, 0); err != nil {
		return "", false, fmt.Errorf("cannot seek backup file: %w", err)
	}
	if err := gzipReader.Reset(file); err != nil {
		return "", false, fmt.Errorf("cannot reset gzip reader: %w", err)
	}
	tarReader = tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", false, fmt.Errorf("error reading tar: %w", err)
		}
		if header.Name != "" && !strings.HasPrefix(header.Name, rootDir+string(filepath.Separator)) && header.Name != rootDir {
			singleRoot = false
			break
		}
	}

	return rootDir, singleRoot, nil
}

// extractFiles reads a tar.gz and extracts files, applying root directory stripping
func extractFiles(backupPath, destPath string, rootDir string, singleRoot bool) error {
	file, err := os.Open(backupPath)
	if err != nil {
		return fmt.Errorf("cannot open backup file: %w", err)
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("cannot read gzip: %w", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("error reading tar: %w", err)
		}

		targetName := header.Name
		if singleRoot && (strings.HasPrefix(targetName, rootDir+string(filepath.Separator)) || targetName == rootDir) {
			if targetName == rootDir {
				continue
			}
			targetName = strings.TrimPrefix(targetName, rootDir+string(filepath.Separator))
		}

		targetPath := filepath.Join(destPath, targetName)

		if !isPathSafe(destPath, targetPath) {
			return fmt.Errorf("unsafe path detected: %s", header.Name)
		}

		if err := writeFile(targetPath, targetPath != destPath, header, tarReader); err != nil {
			return err
		}
	}

	return nil
}

// extractFilesWithCallback extracts a tar.gz and calls progressCallback per file
func extractFilesWithCallback(backupPath, destPath string, rootDir string, singleRoot bool, progressCallback func(current, total int64)) error {
	file, err := os.Open(backupPath)
	if err != nil {
		return fmt.Errorf("cannot open backup file: %w", err)
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("cannot read gzip: %w", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)

	var processedSize int64
	fileInfo, err := os.Stat(backupPath)
	totalSize := int64(0)
	if err == nil {
		totalSize = fileInfo.Size()
	}

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("error reading tar: %w", err)
		}

		if progressCallback != nil && totalSize > 0 {
			processedSize += header.Size
			progressCallback(processedSize, totalSize)
		}

		targetName := header.Name
		if singleRoot && (strings.HasPrefix(targetName, rootDir+string(filepath.Separator)) || targetName == rootDir) {
			if targetName == rootDir {
				continue
			}
			targetName = strings.TrimPrefix(targetName, rootDir+string(filepath.Separator))
		}

		targetPath := filepath.Join(destPath, targetName)

		if !isPathSafe(destPath, targetPath) {
			return fmt.Errorf("unsafe path detected: %s", header.Name)
		}

		if err := writeFile(targetPath, targetPath != destPath, header, tarReader); err != nil {
			return err
		}
	}

	return nil
}

// writeFile creates a file or directory from a tar header
func writeFile(targetPath string, ensureParent bool, header *tar.Header, tarReader *tar.Reader) error {
	switch header.Typeflag {
	case tar.TypeDir:
		if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
			return fmt.Errorf("cannot create directory %s: %w", targetPath, err)
		}

	case tar.TypeReg:
		if ensureParent {
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return fmt.Errorf("cannot create parent directory: %w", err)
			}
		}

		outFile, err := os.Create(targetPath)
		if err != nil {
			return fmt.Errorf("cannot create file %s: %w", targetPath, err)
		}

		if _, err := io.Copy(outFile, tarReader); err != nil {
			outFile.Close()
			return fmt.Errorf("cannot write file %s: %w", targetPath, err)
		}

		if err := os.Chmod(targetPath, os.FileMode(header.Mode)); err != nil {
			outFile.Close()
			return fmt.Errorf("cannot set file permissions: %w", err)
		}

		outFile.Close()

	case tar.TypeSymlink:
		if err := os.Symlink(header.Linkname, targetPath); err != nil {
			if !os.IsExist(err) {
				return fmt.Errorf("cannot create symlink %s: %w", targetPath, err)
			}
		}
	}

	return nil
}

// isPathSafe checks if a target path is within the destination directory
// to prevent path traversal attacks
func isPathSafe(destPath, targetPath string) bool {
	absDestPath, err := filepath.Abs(destPath)
	if err != nil {
		return false
	}

	absTargetPath, err := filepath.Abs(targetPath)
	if err != nil {
		return false
	}

	// Check if target is within dest
	rel, err := filepath.Rel(absDestPath, absTargetPath)
	if err != nil {
		return false
	}

	// If rel starts with "..", it's outside destPath
	return !strings.HasPrefix(rel, "..")
}

// ExtractBackupWithProgress extracts a backup and calls a progress callback
func ExtractBackupWithProgress(backupPath, destPath string, progressCallback func(current, total int64)) error {
	rootDir, singleRoot, err := inspectBackup(backupPath)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(destPath, 0o755); err != nil {
		return fmt.Errorf("cannot create destination directory: %w", err)
	}

	return extractFilesWithCallback(backupPath, destPath, rootDir, singleRoot, progressCallback)
}
