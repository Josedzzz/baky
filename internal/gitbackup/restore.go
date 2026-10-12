package gitbackup

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Josedzzz/baky/internal/textdiff"
)

// Preview describes what restoring a file to the version of a commit would do
type Preview struct {
	LivePath string // The watched file the restore would write
	Content  []byte // The version of the file recorded in the commit

	// InCommit is false when the commit removed the file: there is no
	// version to restore from it.
	InCommit bool
	// LiveExists is false when the watched file does not exist right now
	LiveExists bool
	// Identical is true when the current file already equals that version
	Identical bool

	// Lines goes from the current file to the version of the commit:
	// removed lines would disappear, added lines would come back.
	Lines []textdiff.Line
}

// LivePath returns the watched file that a path of the repository backs up
func LivePath(watches []Watch, repoPath string) (string, bool) {
	repoPath = filepath.ToSlash(filepath.Clean(repoPath))
	for _, w := range watches {
		if rest, ok := strings.CutPrefix(repoPath, w.Dest+"/"); ok {
			return filepath.Join(w.Source, filepath.FromSlash(rest)), true
		}
	}
	return "", false
}

// FileAt returns a file as it was recorded in a commit. The second value
// is false when the file does not exist in that commit.
func FileAt(repoDir, hash, path string) ([]byte, bool, error) {
	object := hash + ":" + filepath.ToSlash(path)
	if _, err := git(repoDir, "cat-file", "-e", object); err != nil {
		return nil, false, nil
	}
	out, err := git(repoDir, "show", object)
	if err != nil {
		return nil, false, err
	}
	return []byte(out), true, nil
}

// PreviewRestore compares the watched file with its version in a commit
func PreviewRestore(repoDir string, watches []Watch, commit Commit) (*Preview, error) {
	if commit.Path == "" {
		return nil, fmt.Errorf("el commit no registra ningún archivo")
	}

	livePath, ok := LivePath(watches, commit.Path)
	if !ok {
		return nil, fmt.Errorf("la carpeta de %s ya no está entre las carpetas vigiladas", commit.Path)
	}

	content, inCommit, err := FileAt(repoDir, commit.Hash, commit.Path)
	if err != nil {
		return nil, err
	}

	preview := &Preview{LivePath: livePath, Content: content, InCommit: inCommit}
	if !inCommit {
		return preview, nil
	}

	current, err := os.ReadFile(livePath)
	switch {
	case err == nil:
		preview.LiveExists = true
		preview.Identical = bytes.Equal(current, content)
	case !os.IsNotExist(err):
		return nil, fmt.Errorf("no se pudo leer %s: %w", livePath, err)
	}
	if preview.Identical {
		return preview, nil
	}

	preview.Lines, err = diffContent(livePath, content)
	if err != nil {
		return nil, err
	}
	return preview, nil
}

// diffContent returns the changes that turn the file at currentPath into wanted
func diffContent(currentPath string, wanted []byte) ([]textdiff.Line, error) {
	tmp, err := os.CreateTemp("", "baky-diff-*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(wanted); err != nil {
		tmp.Close()
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}
	return textdiff.Files(currentPath, tmp.Name())
}

// Restore writes the version of the preview to the watched file. The file
// is replaced in one step, so it is never left half written, and it keeps
// its current permissions.
func (p *Preview) Restore() error {
	if !p.InCommit {
		return fmt.Errorf("el commit no contiene una versión del archivo")
	}

	perm := os.FileMode(0o644)
	if info, err := os.Stat(p.LivePath); err == nil {
		perm = info.Mode().Perm()
	}

	dir := filepath.Dir(p.LivePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, ".baky-tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(p.Content); err != nil {
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
	return os.Rename(tmp.Name(), p.LivePath)
}
