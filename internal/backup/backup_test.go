package backup

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Josedzzz/baky/internal/config"
)

func TestCreateTarGz(t *testing.T) {
	// Create a temporary directory for the source
	srcDir, err := os.MkdirTemp("", "baky-src")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(srcDir)

	// Create some files
	files := map[string]string{
		"file1.txt":      "content 1",
		"dir1/file2.txt": "content 2",
	}
	for path, content := range files {
		fullPath := filepath.Join(srcDir, path)
		os.MkdirAll(filepath.Dir(fullPath), 0o755)
		os.WriteFile(fullPath, []byte(content), 0o644)
	}

	// Create a temporary directory for the destination
	destDir, err := os.MkdirTemp("", "baky-dest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(destDir)

	destPath := filepath.Join(destDir, "backup.tar.gz")

	// Test createTarGz
	if err := createTarGz(srcDir, destPath); err != nil {
		t.Fatalf("createTarGz failed: %v", err)
	}

	// Verify the contents of the tar.gz
	f, err := os.Open(destPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	foundFiles := make(map[string]bool)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		foundFiles[header.Name] = true
	}

	// The tar should contain the files with relative paths from the source directory
	// After the fix, filepath.Rel(src, path) is used, so files are at the root of the tar

	if !foundFiles["file1.txt"] {
		t.Errorf("Expected file1.txt in tar, not found. Found: %v", foundFiles)
	}
	if !foundFiles["dir1/file2.txt"] {
		t.Errorf("Expected dir1/file2.txt in tar, not found. Found: %v", foundFiles)
	}
}

// readHeaders returns the headers of a tar.gz file indexed by entry name
func readHeaders(t *testing.T, path string) map[string]*tar.Header {
	t.Helper()

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gr.Close()

	headers := make(map[string]*tar.Header)
	tr := tar.NewReader(gr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		headers[header.Name] = header
	}
	return headers
}

func TestCreateTarGzSingleFile(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "nginx.conf")
	os.WriteFile(src, []byte("content"), 0o644)
	destPath := filepath.Join(tmp, "backup.tar.gz")

	if err := createTarGz(src, destPath); err != nil {
		t.Fatalf("createTarGz failed: %v", err)
	}

	headers := readHeaders(t, destPath)
	if len(headers) != 1 || headers["nginx.conf"] == nil {
		t.Errorf("Expected a single entry named nginx.conf, found: %v", headers)
	}
}

func TestCreateTarGzSymlink(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src")
	os.MkdirAll(src, 0o755)
	os.WriteFile(filepath.Join(src, "real.txt"), []byte("content"), 0o644)
	if err := os.Symlink("real.txt", filepath.Join(src, "link")); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}
	destPath := filepath.Join(tmp, "backup.tar.gz")

	if err := createTarGz(src, destPath); err != nil {
		t.Fatalf("createTarGz failed: %v", err)
	}

	link := readHeaders(t, destPath)["link"]
	if link == nil {
		t.Fatal("Expected link in tar, not found")
	}
	if link.Typeflag != tar.TypeSymlink || link.Linkname != "real.txt" {
		t.Errorf("Expected symlink to real.txt, got type %c to %q", link.Typeflag, link.Linkname)
	}
}

func TestScheduledDue(t *testing.T) {
	// 2026-10-11 is a Sunday
	sunday3am := time.Date(2026, 10, 11, 3, 10, 0, 0, time.Local)
	monday3am := sunday3am.Add(24 * time.Hour)

	cases := []struct {
		name      string
		frequency string
		last      time.Time
		now       time.Time
		want      bool
	}{
		{"daily never backed up", config.FreqDaily, time.Time{}, sunday3am, true},
		{"daily backed up yesterday", config.FreqDaily, sunday3am.Add(-24 * time.Hour), sunday3am, true},
		{"daily already done this hour", config.FreqDaily, sunday3am.Add(-5 * time.Minute), sunday3am, false},
		{"daily outside the 3 AM hour", config.FreqDaily, time.Time{}, sunday3am.Add(2 * time.Hour), false},
		{"weekly on sunday", config.FreqWeekly, sunday3am.Add(-7 * 24 * time.Hour), sunday3am, true},
		{"weekly on monday", config.FreqWeekly, time.Time{}, monday3am, false},
		{"weekly already done", config.FreqWeekly, sunday3am.Add(-5 * time.Minute), sunday3am, false},
		{"on_change is not scheduled", config.FreqOnChange, time.Time{}, sunday3am, false},
	}
	for _, c := range cases {
		p := config.BackupPathConfig{Path: "/x", Frequency: c.frequency, LastBackup: c.last}
		if got := scheduledDue(p, c.now); got != c.want {
			t.Errorf("%s: expected %v, got %v", c.name, c.want, got)
		}
	}
}
