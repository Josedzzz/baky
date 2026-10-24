package gitbackup

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// keepFile is never removed from the repository: it is how Git keeps an
// otherwise empty folder
const keepFile = ".gitkeep"

// ignored reports whether a file is editor or tool noise that must not be
// backed up: swap files, backup copies and the probe file vim creates
func ignored(name string) bool {
	base := filepath.Base(name)
	switch {
	case base == "4913":
		return true
	case strings.HasSuffix(base, "~"):
		return true
	case strings.HasPrefix(base, ".#"):
		return true
	}
	switch filepath.Ext(base) {
	case ".swp", ".swo", ".swx", ".swpx":
		return true
	}
	return false
}

// evalPath resolves symlinks so two spellings of the same directory compare equal
func evalPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}

// mirror makes dst an exact copy of src: new and changed files are copied,
// and files that no longer exist in src are removed from dst. Files that
// cannot be read are reported and left as they are in dst.
//
// What ex excludes, and files that hold a private key, are not copied,
// and are removed from dst if an earlier copy is there. A nil ex copies
// everything.
func mirror(src, dst string, ex *excluder, warn func(msg string)) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", src)
	}

	// 1. Copy what is new or different
	secrets := map[string]bool{} // relative paths left out for their content
	err = filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			warn(fmt.Sprintf("Could not read %s: %v", path, walkErr))
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		switch {
		case rel == ".":
			return nil
		case ex.excluded(rel, d.IsDir()):
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		case d.IsDir():
			return nil
		case ignored(path):
			return nil
		case d.Type()&fs.ModeSymlink != 0:
			if err := copySymlink(path, target); err != nil {
				warn(fmt.Sprintf("Could not back up %s: %v", path, err))
			}
		case d.Type().IsRegular():
			data, err := os.ReadFile(path)
			if err != nil {
				warn(fmt.Sprintf("Could not back up %s: %v", path, err))
				return nil
			}
			if ex.holdsSecret(data) {
				secrets[rel] = true
				warn(fmt.Sprintf("Not backed up, it holds a private key: %s", path))
				return nil
			}
			if err := copyFile(path, target, data); err != nil {
				warn(fmt.Sprintf("Could not back up %s: %v", path, err))
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	// 2. Remove what no longer exists in src
	if _, err := os.Stat(dst); os.IsNotExist(err) {
		return nil
	}
	var dirs []string
	err = filepath.WalkDir(dst, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(dst, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			dirs = append(dirs, path)
			return nil
		}
		if d.Name() == keepFile {
			return nil
		}

		// A secret backed up before it was excluded leaves the folder, but
		// stays in the history and on the remote
		if ex.excluded(rel, false) || secrets[rel] {
			warn(fmt.Sprintf("Removed %s from the repository because it is excluded. Earlier commits and the remote still hold it: if it is a secret, change it", path))
			return os.Remove(path)
		}

		// Only a file that is certainly gone is removed: one that cannot
		// be checked (permissions, I/O error) keeps its last backup.
		_, statErr := os.Lstat(filepath.Join(src, rel))
		if os.IsNotExist(statErr) || (statErr == nil && ignored(path)) {
			return os.Remove(path)
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Directories left empty are removed, deepest first
	for i := len(dirs) - 1; i >= 0; i-- {
		os.Remove(dirs[i]) // fails on purpose when the directory is not empty
	}
	return nil
}

// copyFile writes data, the content of src, to dst with the permissions of
// src, unless dst already has the same content
func copyFile(src, dst string, data []byte) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	perm := info.Mode().Perm()

	if existing, err := os.Lstat(dst); err == nil && existing.Mode().IsRegular() {
		if current, err := os.ReadFile(dst); err == nil && bytes.Equal(current, data) {
			if existing.Mode().Perm() != perm {
				return os.Chmod(dst, perm)
			}
			return nil
		}
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	// Write to a temp file and rename, so dst is never left half written
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".baky-tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	// A directory or symlink in the way is replaced by the file
	if existing, err := os.Lstat(dst); err == nil && !existing.Mode().IsRegular() {
		if err := os.RemoveAll(dst); err != nil {
			return err
		}
	}
	return os.Rename(tmp.Name(), dst)
}

// copySymlink recreates the symlink src at dst, pointing to the same target
func copySymlink(src, dst string) error {
	target, err := os.Readlink(src)
	if err != nil {
		return err
	}
	if current, err := os.Readlink(dst); err == nil && current == target {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	return os.Symlink(target, dst)
}
