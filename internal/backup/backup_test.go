package backup

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
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
		"file1.txt": "content 1",
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
