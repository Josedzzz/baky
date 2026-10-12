// Package textdiff compares two versions of a text file line by line
package textdiff

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// Kind tells what a line of a comparison means
type Kind int

const (
	// Context is a line that is the same in both versions
	Context Kind = iota
	// Removed is a line of the current file that the other version does not have
	Removed
	// Added is a line of the other version that the current file does not have
	Added
	// Hunk marks where a group of changes starts
	Hunk
	// Note is a remark from Git, such as a binary file or a missing final newline
	Note
)

// Line is one line of a comparison between two versions of a file
type Line struct {
	Kind Kind
	Text string
	// Number is the line number in the current file where the group of
	// changes starts. Only set for Hunk.
	Number int
}

// Files returns the changes that turn the file at currentPath into the
// file at wantedPath: removed lines exist only in the current file, added
// lines only in the wanted one. A path that does not exist is compared as
// an empty file. It relies on "git diff", which works outside a repository.
func Files(currentPath, wantedPath string) ([]Line, error) {
	for _, path := range []*string{&currentPath, &wantedPath} {
		if _, err := os.Lstat(*path); os.IsNotExist(err) {
			*path = os.DevNull
		}
	}

	// --no-index compares two files outside of any repository
	cmd := exec.Command("git", "diff", "--no-index", "--no-color", "--unified=3", "--", currentPath, wantedPath)
	out, err := cmd.Output()
	if err != nil {
		// Exit code 1 only means "the files differ"
		if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != 1 {
			return nil, fmt.Errorf("no se pudo comparar el archivo (se necesita git): %w", err)
		}
	}
	return parse(string(out)), nil
}

// hunkHeader matches "@@ -12,7 +12,8 @@" and captures the first line number
var hunkHeader = regexp.MustCompile(`^@@ -(\d+)(?:,\d+)? \+\d+(?:,\d+)? @@`)

// parse turns the unified diff printed by Git into lines ready to show
func parse(out string) []Line {
	var lines []Line
	inHunk := false

	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		switch {
		case hunkHeader.MatchString(line):
			inHunk = true
			number, _ := strconv.Atoi(hunkHeader.FindStringSubmatch(line)[1])
			lines = append(lines, Line{Kind: Hunk, Number: number})
		case strings.HasPrefix(line, "Binary files"):
			lines = append(lines, Line{Kind: Note, Text: "El archivo es binario: no se puede mostrar línea por línea."})
		case !inHunk:
			// Header of the diff (file names, modes): not useful here
		case strings.HasPrefix(line, "+"):
			lines = append(lines, Line{Kind: Added, Text: line[1:]})
		case strings.HasPrefix(line, "-"):
			lines = append(lines, Line{Kind: Removed, Text: line[1:]})
		case strings.HasPrefix(line, `\`):
			lines = append(lines, Line{Kind: Note, Text: "(sin salto de línea al final del archivo)"})
		default:
			lines = append(lines, Line{Kind: Context, Text: strings.TrimPrefix(line, " ")})
		}
	}
	return lines
}
