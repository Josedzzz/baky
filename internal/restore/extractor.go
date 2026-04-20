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
	// Verify backup file exists
	if _, err := os.Stat(backupPath); err != nil {
		return fmt.Errorf("backup file not found: %w", err)
	}

	// Open the tar.gz file
	file, err := os.Open(backupPath)
	if err != nil {
		return fmt.Errorf("cannot open backup file: %w", err)
	}
	defer file.Close()

	// Create gzip reader
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("cannot read gzip: %w", err)
	}
	defer gzipReader.Close()

	// Create tar reader
	tarReader := tar.NewReader(gzipReader)

	// Create destination directory if it doesn't exist
	if err := os.MkdirAll(destPath, 0o755); err != nil {
		return fmt.Errorf("cannot create destination directory: %w", err)
	}

	// First pass: check if all files are under a single root directory
	var rootDir string
	files := []*tar.Header{}

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("error reading tar: %w", err)
		}
		files = append(files, header)

		// Extract root directory from first path
		if rootDir == "" && header.Name != "" {
			parts := strings.Split(strings.TrimSuffix(header.Name, "/"), string(filepath.Separator))
			if len(parts) > 0 {
				rootDir = parts[0]
			}
		}
	}

	// Check if all files start with the same root directory
	singleRoot := rootDir != "" && len(files) > 0
	for _, h := range files {
		if h.Name != "" && !strings.HasPrefix(h.Name, rootDir+string(filepath.Separator)) && h.Name != rootDir {
			singleRoot = false
			break
		}
	}

	// Second pass: extract files
	file.Seek(0, 0)
	gzipReader, _ = gzip.NewReader(file)
	tarReader = tar.NewReader(gzipReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("error reading tar: %w", err)
		}

		// Remove root directory prefix if all files are under a single root
		targetName := header.Name
		if singleRoot && (strings.HasPrefix(targetName, rootDir+string(filepath.Separator)) || targetName == rootDir) {
			if targetName == rootDir {
				continue // Skip the root directory itself
			}
			targetName = strings.TrimPrefix(targetName, rootDir+string(filepath.Separator))
		}

		// Construct the full file path
		targetPath := filepath.Join(destPath, targetName)

		// Prevent path traversal attacks
		if !isPathSafe(destPath, targetPath) {
			return fmt.Errorf("unsafe path detected: %s", header.Name)
		}

		// Handle different file types
		switch header.Typeflag {
		case tar.TypeDir:
			// Create directory
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("cannot create directory %s: %w", targetPath, err)
			}

		case tar.TypeReg:
			// Create parent directories if needed
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return fmt.Errorf("cannot create parent directory: %w", err)
			}

			// Create file
			outFile, err := os.Create(targetPath)
			if err != nil {
				return fmt.Errorf("cannot create file %s: %w", targetPath, err)
			}

			// Copy file contents
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return fmt.Errorf("cannot write file %s: %w", targetPath, err)
			}

			// Set file permissions
			if err := os.Chmod(targetPath, os.FileMode(header.Mode)); err != nil {
				outFile.Close()
				return fmt.Errorf("cannot set file permissions: %w", err)
			}

			outFile.Close()

		case tar.TypeSymlink:
			// Handle symlinks
			if err := os.Symlink(header.Linkname, targetPath); err != nil {
				// Ignore if symlink already exists
				if !os.IsExist(err) {
					return fmt.Errorf("cannot create symlink %s: %w", targetPath, err)
				}
			}

		default:
			// Skip unsupported file types
			continue
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
	return !filepath.IsAbs(rel) && rel != ".."
}

// ExtractBackupWithProgress extracts a backup and calls a progress callback
func ExtractBackupWithProgress(backupPath, destPath string, progressCallback func(current, total int64)) error {
	// Verify backup file exists
	fileInfo, err := os.Stat(backupPath)
	if err != nil {
		return fmt.Errorf("backup file not found: %w", err)
	}

	totalSize := fileInfo.Size()

	// Open the tar.gz file
	file, err := os.Open(backupPath)
	if err != nil {
		return fmt.Errorf("cannot open backup file: %w", err)
	}
	defer file.Close()

	// Create gzip reader
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("cannot read gzip: %w", err)
	}
	defer gzipReader.Close()

	// Create tar reader
	tarReader := tar.NewReader(gzipReader)

	// Create destination directory if it doesn't exist
	if err := os.MkdirAll(destPath, 0o755); err != nil {
		return fmt.Errorf("cannot create destination directory: %w", err)
	}

	var processedSize int64

	// Extract all files
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("error reading tar: %w", err)
		}

		// Update progress
		if progressCallback != nil && totalSize > 0 {
			processedSize += header.Size
			progressCallback(processedSize, totalSize)
		}

		// Construct the full file path
		targetPath := filepath.Join(destPath, header.Name)

		// Prevent path traversal attacks
		if !isPathSafe(destPath, targetPath) {
			return fmt.Errorf("unsafe path detected: %s", header.Name)
		}

		// Handle different file types
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("cannot create directory %s: %w", targetPath, err)
			}

		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return fmt.Errorf("cannot create parent directory: %w", err)
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

		default:
			continue
		}
	}

	return nil
}
