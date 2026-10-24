package restore

import (
	"archive/tar"
	"os"
	"path/filepath"
	"testing"
)

func kinds(p *Preview) map[string]ChangeKind {
	result := map[string]ChangeKind{}
	for _, f := range p.Files {
		result[f.Path] = f.Kind
	}
	return result
}

func TestPreviewDirectoryBackup(t *testing.T) {
	tmp := t.TempDir()
	backup := filepath.Join(tmp, "docs_20260101_000000.tar.gz")
	writeBackup(t, backup, dirBackup()) // a.txt, sub/b.txt, link -> a.txt

	dest := filepath.Join(tmp, "docs")
	os.MkdirAll(filepath.Join(dest, "sub"), 0o755)
	os.WriteFile(filepath.Join(dest, "a.txt"), []byte("changed since the backup"), 0o644)
	os.WriteFile(filepath.Join(dest, "sub", "b.txt"), []byte("nested"), 0o644)
	os.WriteFile(filepath.Join(dest, "extra.txt"), []byte("not in the backup"), 0o644)

	p, err := NewPreview(backup, dest)
	if err != nil {
		t.Fatal(err)
	}

	got := kinds(p)
	want := map[string]ChangeKind{
		"a.txt":                       ChangeModified,
		filepath.Join("sub", "b.txt"): ChangeSame,
		"link":                        ChangeNew,
	}
	for path, kind := range want {
		if got[path] != kind {
			t.Errorf("%s: expected kind %d, got %d", path, kind, got[path])
		}
	}
	if p.Modified != 1 || p.New != 1 || p.Same != 1 || p.OnlyInDest != 1 || len(p.Files) != 3 {
		t.Errorf("unexpected counts: %+v", p)
	}
	if !p.HasChanges() {
		t.Errorf("expected changes")
	}

	// What changes is listed first
	if p.Files[0].Kind != ChangeModified || p.Files[1].Kind != ChangeNew || p.Files[2].Kind != ChangeSame {
		t.Errorf("unexpected order: %+v", p.Files)
	}
	// Each change points to the extracted file and to where it would go
	assertContent(t, p.Files[0].BackupPath, "from backup")
	if p.Files[0].DestPath != filepath.Join(dest, "a.txt") {
		t.Errorf("unexpected destination: %s", p.Files[0].DestPath)
	}

	// The preview changes nothing at the destination
	assertContent(t, filepath.Join(dest, "a.txt"), "changed since the backup")

	tempDir := p.tempDir
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(tempDir); !os.IsNotExist(err) {
		t.Errorf("expected the temporary copy to be removed")
	}
}

func TestPreviewToEmptyDestination(t *testing.T) {
	tmp := t.TempDir()
	backup := filepath.Join(tmp, "docs_20260101_000000.tar.gz")
	writeBackup(t, backup, dirBackup())

	p, err := NewPreview(backup, filepath.Join(tmp, "new"))
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	if p.New != 3 || p.Modified != 0 || p.Same != 0 || p.OnlyInDest != 0 {
		t.Errorf("expected every file to be new: %+v", p)
	}
}

func TestPreviewSingleFileBackup(t *testing.T) {
	// "." is how older versions of baky named the entry of a single-file backup
	for _, entryName := range []string{"nginx.conf", "."} {
		t.Run(entryName, func(t *testing.T) {
			tmp := t.TempDir()
			backup := filepath.Join(tmp, "nginx.conf_20260101_000000.tar.gz")
			writeBackup(t, backup, []tarEntry{{name: entryName, typeflag: tar.TypeReg, content: "from backup"}})

			target := filepath.Join(tmp, "nginx.conf")
			os.WriteFile(target, []byte("current"), 0o644)

			p, err := NewPreview(backup, target)
			if err != nil {
				t.Fatal(err)
			}
			if len(p.Files) != 1 || p.Files[0].Kind != ChangeModified || p.Files[0].DestPath != target {
				t.Errorf("unexpected preview: %+v", p.Files)
			}
			assertContent(t, p.Files[0].BackupPath, "from backup")
			p.Close()

			// Same content: nothing to restore
			os.WriteFile(target, []byte("from backup"), 0o644)
			p, _ = NewPreview(backup, target)
			if p.HasChanges() || p.Same != 1 {
				t.Errorf("expected no changes: %+v", p)
			}
			p.Close()

			// An existing directory as destination receives the file
			dir := filepath.Join(tmp, "custom")
			os.MkdirAll(dir, 0o755)
			p, _ = NewPreview(backup, dir)
			if p.Files[0].Kind != ChangeNew || p.Files[0].DestPath != filepath.Join(dir, "nginx.conf") {
				t.Errorf("unexpected preview into a directory: %+v", p.Files)
			}
			p.Close()
		})
	}
}

func TestPreviewCorruptBackup(t *testing.T) {
	tmp := t.TempDir()
	backup := filepath.Join(tmp, "docs_20260101_000000.tar.gz")
	os.WriteFile(backup, []byte("not a gzip"), 0o644)

	if _, err := NewPreview(backup, filepath.Join(tmp, "docs")); err == nil {
		t.Errorf("expected an error for a corrupt backup")
	}
}
