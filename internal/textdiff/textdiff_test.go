package textdiff

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestFiles(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	current := filepath.Join(dir, "current.conf")
	wanted := filepath.Join(dir, "wanted.conf")
	os.WriteFile(current, []byte("a\nb\nc\n"), 0o644)
	os.WriteFile(wanted, []byte("a\nB\nc\nd\n"), 0o644)

	lines, err := Files(current, wanted)
	if err != nil {
		t.Fatal(err)
	}
	want := []Line{
		{Kind: Hunk, Number: 1},
		{Kind: Context, Text: "a"},
		{Kind: Removed, Text: "b"},
		{Kind: Added, Text: "B"},
		{Kind: Context, Text: "c"},
		{Kind: Added, Text: "d"},
	}
	if len(lines) != len(want) {
		t.Fatalf("expected %d lines, got %+v", len(want), lines)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line %d: expected %+v, got %+v", i, want[i], lines[i])
		}
	}

	// Identical files have no differences
	if lines, err := Files(current, current); err != nil || len(lines) != 0 {
		t.Errorf("expected no differences, got %+v (%v)", lines, err)
	}

	// A missing file is compared as an empty one
	lines, err = Files(filepath.Join(dir, "missing.conf"), current)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 4 || lines[1] != (Line{Kind: Added, Text: "a"}) {
		t.Errorf("expected the whole file as added lines, got %+v", lines)
	}

	// Binary content is reported, not printed
	os.WriteFile(wanted, []byte{0, 1, 2, 0, 255}, 0o644)
	lines, err = Files(current, wanted)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 1 || lines[0].Kind != Note {
		t.Errorf("expected a note for a binary file, got %+v", lines)
	}
}
