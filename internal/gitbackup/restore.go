package gitbackup

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Josedzzz/baky/internal/textdiff"
)

// Preview describes what restoring a file to the version of a commit would do
type Preview struct {
	LivePath string // The watched file the restore would write
	Content  []byte // The version of the file recorded in the commit
	Watch    Watch  // The watch the file belongs to, with its hook commands

	// CheckOutput is what the validation command reported when this
	// commit was made, if the configuration did not pass it
	CheckOutput string

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
	path, _, ok := liveFile(watches, repoPath)
	return path, ok
}

// liveFile returns the watched file that a path of the repository backs
// up, and the watch it belongs to
func liveFile(watches []Watch, repoPath string) (string, Watch, bool) {
	repoPath = filepath.ToSlash(filepath.Clean(repoPath))
	for _, w := range watches {
		if rest, ok := strings.CutPrefix(repoPath, w.Dest+"/"); ok {
			return filepath.Join(w.Source, filepath.FromSlash(rest)), w, true
		}
	}
	return "", Watch{}, false
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

	livePath, watch, ok := liveFile(watches, commit.Path)
	if !ok {
		return nil, fmt.Errorf("la carpeta de %s ya no está entre las carpetas vigiladas", commit.Path)
	}

	content, inCommit, err := FileAt(repoDir, commit.Hash, commit.Path)
	if err != nil {
		return nil, err
	}

	preview := &Preview{LivePath: livePath, Content: content, InCommit: inCommit, Watch: watch}
	if commit.Invalid {
		preview.CheckOutput = commitBody(repoDir, commit.Hash)
	}
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

// Restore writes the version of the preview to the watched file and
// returns what the hook commands printed.
//
// When the watch has hook commands, the restore never touches the live
// configuration before it is known to be good: the configuration is
// staged in a temporary directory with the restored file, validated there
// with the check command and put into service with the apply command.
// Only then is the file installed. A failure at any step leaves the
// watched directory exactly as it was.
func (p *Preview) Restore() (string, error) {
	if !p.InCommit {
		return "", fmt.Errorf("el commit no contiene una versión del archivo")
	}

	var output []string
	if p.Watch.Check != "" || p.Watch.Apply != "" {
		stage, err := p.stage()
		if err != nil {
			return "", err
		}
		defer os.RemoveAll(stage)

		for _, hook := range []struct{ command, failure string }{
			{p.Watch.Check, "la versión a restaurar no pasa la validación"},
			{p.Watch.Apply, "no se pudo aplicar la versión a restaurar"},
		} {
			if hook.command == "" {
				continue
			}
			out, err := runHook(hook.command, stage)
			if out != "" {
				output = append(output, out)
			}
			if err != nil {
				return strings.Join(output, "\n"), fmt.Errorf("%s: %w", hook.failure, err)
			}
		}
	}

	if err := installFile(p.LivePath, p.Content); err != nil {
		return strings.Join(output, "\n"), err
	}
	return strings.Join(output, "\n"), nil
}

// stage copies the watched directory to a temporary one and puts the
// version to restore in place of the current file
func (p *Preview) stage() (string, error) {
	stage, err := os.MkdirTemp("", "baky-stage-*")
	if err != nil {
		return "", err
	}
	// The hook commands usually run as another user (root, through sudo)
	if err := os.Chmod(stage, 0o755); err != nil {
		os.RemoveAll(stage)
		return "", err
	}

	// Files that cannot be read are left out: the tool being validated
	// falls back to its own copy of them
	if err := mirror(p.Watch.Source, stage, func(string) {}); err != nil {
		os.RemoveAll(stage)
		return "", fmt.Errorf("no se pudo preparar la validación: %w", err)
	}

	rel, err := filepath.Rel(p.Watch.Source, p.LivePath)
	if err != nil {
		os.RemoveAll(stage)
		return "", err
	}
	target := filepath.Join(stage, rel)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		os.RemoveAll(stage)
		return "", err
	}
	if err := os.WriteFile(target, p.Content, 0o644); err != nil {
		os.RemoveAll(stage)
		return "", err
	}
	return stage, nil
}

// installFile replaces the file at path with content in one step, so it
// is never left half written, keeping its permissions. When the current
// user may not write there, the file is installed through sudo, keeping
// its owner too.
func installFile(path string, content []byte) error {
	perm := os.FileMode(0o644)
	info, statErr := os.Stat(path)
	if statErr == nil {
		perm = info.Mode().Perm()
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err == nil {
		if tmp, err := os.CreateTemp(dir, ".baky-tmp-*"); err == nil {
			defer os.Remove(tmp.Name())
			if _, err := tmp.Write(content); err != nil {
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
			return os.Rename(tmp.Name(), path)
		} else if !os.IsPermission(err) {
			return err
		}
	} else if !os.IsPermission(err) {
		return err
	}

	// No permission to write there: hand a temporary copy to sudo
	tmp, err := os.CreateTemp("", "baky-install-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	// The file keeps the owner it has; a new file takes the owner of its directory
	ownerOf := path
	if statErr != nil {
		ownerOf = dir
	}
	args := []string{"-n", "install", "-D", "-m", fmt.Sprintf("%04o", perm)}
	if uid, gid, ok := owner(ownerOf); ok {
		args = append(args, "-o", uid, "-g", gid)
	}
	args = append(args, tmp.Name(), path)

	if out, err := sudo(args...); err != nil {
		return fmt.Errorf("no hay permiso para escribir %s y sudo no lo permitió: %s", path, lastLines(out, 2))
	}
	return nil
}

// sudo runs a command through sudo and returns what it printed
var sudo = func(args ...string) (string, error) {
	out, err := exec.Command("sudo", args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}
