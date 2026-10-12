package gitbackup

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// DiffKind tells what a line of a comparison means
type DiffKind int

const (
	// DiffContext is a line that is the same in both versions
	DiffContext DiffKind = iota
	// DiffRemoved is a line of the current file that the other version does not have
	DiffRemoved
	// DiffAdded is a line of the other version that the current file does not have
	DiffAdded
	// DiffHunk marks where a group of changes starts
	DiffHunk
	// DiffNote is a remark from Git, such as a binary file or a missing final newline
	DiffNote
)

// DiffLine is one line of a comparison between two versions of a file
type DiffLine struct {
	Kind DiffKind
	Text string
	// Line is the line number in the current file where the group of
	// changes starts. Only set for DiffHunk.
	Line int
}

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
	Lines []DiffLine
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

	preview.Lines, err = diff(livePath, preview.LiveExists, content)
	if err != nil {
		return nil, err
	}
	return preview, nil
}

// diff returns the changes that turn the file at currentPath into wanted
func diff(currentPath string, currentExists bool, wanted []byte) ([]DiffLine, error) {
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

	if !currentExists {
		currentPath = os.DevNull
	}

	// --no-index compares two files outside of any repository
	cmd := exec.Command("git", "diff", "--no-index", "--no-color", "--unified=3", "--", currentPath, tmp.Name())
	out, err := cmd.Output()
	if err != nil {
		// Exit code 1 only means "the files differ"
		if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != 1 {
			return nil, fmt.Errorf("no se pudo comparar el archivo: %w", err)
		}
	}
	return parseDiff(string(out)), nil
}

// hunkHeader matches "@@ -12,7 +12,8 @@" and captures the first line number
var hunkHeader = regexp.MustCompile(`^@@ -(\d+)(?:,\d+)? \+\d+(?:,\d+)? @@`)

// parseDiff turns the unified diff printed by Git into lines ready to show
func parseDiff(out string) []DiffLine {
	var lines []DiffLine
	inHunk := false

	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		switch {
		case hunkHeader.MatchString(line):
			inHunk = true
			number, _ := strconv.Atoi(hunkHeader.FindStringSubmatch(line)[1])
			lines = append(lines, DiffLine{Kind: DiffHunk, Line: number})
		case strings.HasPrefix(line, "Binary files"):
			lines = append(lines, DiffLine{Kind: DiffNote, Text: "El archivo es binario: no se puede mostrar línea por línea."})
		case !inHunk:
			// Header of the diff (file names, modes): not useful here
		case strings.HasPrefix(line, "+"):
			lines = append(lines, DiffLine{Kind: DiffAdded, Text: line[1:]})
		case strings.HasPrefix(line, "-"):
			lines = append(lines, DiffLine{Kind: DiffRemoved, Text: line[1:]})
		case strings.HasPrefix(line, `\`):
			lines = append(lines, DiffLine{Kind: DiffNote, Text: "(sin salto de línea al final del archivo)"})
		default:
			lines = append(lines, DiffLine{Kind: DiffContext, Text: strings.TrimPrefix(line, " ")})
		}
	}
	return lines
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
