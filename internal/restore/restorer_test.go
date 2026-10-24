package restore

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tarEntry describes one entry of a test backup archive
type tarEntry struct {
	name     string
	typeflag byte
	content  string
	linkname string
}

// writeBackup creates a tar.gz backup at path with the given entries
func writeBackup(t *testing.T, path string, entries []tarEntry) {
	t.Helper()

	out, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gw := gzip.NewWriter(out)
	tw := tar.NewWriter(gw)

	for _, e := range entries {
		header := &tar.Header{
			Name:     e.name,
			Typeflag: e.typeflag,
			Linkname: e.linkname,
			Mode:     0o644,
			Size:     int64(len(e.content)),
		}
		if e.typeflag == tar.TypeDir {
			header.Mode = 0o755
		}
		if err := tw.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(e.content)); err != nil {
			t.Fatal(err)
		}
	}

	for _, c := range []interface{ Close() error }{tw, gw, out} {
		if err := c.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

// dirBackup returns the entries of a directory backup as baky writes them
func dirBackup() []tarEntry {
	return []tarEntry{
		{name: ".", typeflag: tar.TypeDir},
		{name: "a.txt", typeflag: tar.TypeReg, content: "from backup"},
		{name: "sub", typeflag: tar.TypeDir},
		{name: "sub/b.txt", typeflag: tar.TypeReg, content: "nested"},
		{name: "link", typeflag: tar.TypeSymlink, linkname: "a.txt"},
	}
}

func assertContent(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read %s: %v", path, err)
	}
	if string(got) != want {
		t.Errorf("%s: expected %q, got %q", path, want, got)
	}
}

// assertNoStaging fails if a restore left temporary files behind in dir
func assertNoStaging(t *testing.T, dir string) {
	t.Helper()
	leftovers, _ := filepath.Glob(filepath.Join(dir, stagingPrefix+"*"))
	if len(leftovers) > 0 {
		t.Errorf("staging left behind: %v", leftovers)
	}
}

func TestRestoreDirToNewLocation(t *testing.T) {
	tmp := t.TempDir()
	backup := filepath.Join(tmp, "docs_20260101_000000.tar.gz")
	writeBackup(t, backup, dirBackup())
	dest := filepath.Join(tmp, "docs")

	result, err := Restore(backup, dest, RestoreActionOverwrite)
	if err != nil {
		t.Fatalf("Restore failed: %v", err)
	}
	if !result.Success || result.FilesRestored != 2 {
		t.Errorf("unexpected result: %+v", result)
	}

	assertContent(t, filepath.Join(dest, "a.txt"), "from backup")
	assertContent(t, filepath.Join(dest, "sub", "b.txt"), "nested")
	if target, err := os.Readlink(filepath.Join(dest, "link")); err != nil || target != "a.txt" {
		t.Errorf("expected symlink to a.txt, got %q (%v)", target, err)
	}
	assertNoStaging(t, tmp)
}

func TestRestoreOverwriteKeepsUnrelatedFiles(t *testing.T) {
	tmp := t.TempDir()
	backup := filepath.Join(tmp, "docs_20260101_000000.tar.gz")
	writeBackup(t, backup, dirBackup())

	dest := filepath.Join(tmp, "docs")
	os.MkdirAll(filepath.Join(dest, "sub"), 0o755)
	os.WriteFile(filepath.Join(dest, "a.txt"), []byte("old"), 0o644)
	os.WriteFile(filepath.Join(dest, "unrelated.txt"), []byte("keep me"), 0o644)
	os.WriteFile(filepath.Join(dest, "sub", "other.txt"), []byte("keep me too"), 0o644)

	if _, err := Restore(backup, dest, RestoreActionOverwrite); err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	assertContent(t, filepath.Join(dest, "a.txt"), "from backup")
	assertContent(t, filepath.Join(dest, "sub", "b.txt"), "nested")
	assertContent(t, filepath.Join(dest, "unrelated.txt"), "keep me")
	assertContent(t, filepath.Join(dest, "sub", "other.txt"), "keep me too")
	assertNoStaging(t, dest)
}

func TestRestoreCorruptBackupLeavesDestinationIntact(t *testing.T) {
	tmp := t.TempDir()
	backup := filepath.Join(tmp, "docs_20260101_000000.tar.gz")
	os.WriteFile(backup, []byte("this is not a gzip file"), 0o644)

	dest := filepath.Join(tmp, "docs")
	os.MkdirAll(dest, 0o755)
	os.WriteFile(filepath.Join(dest, "a.txt"), []byte("old"), 0o644)

	for _, action := range []RestoreAction{RestoreActionOverwrite, RestoreActionRename} {
		if _, err := Restore(backup, dest, action); err == nil {
			t.Errorf("action %d: expected an error for a corrupt backup", action)
		}
		assertContent(t, filepath.Join(dest, "a.txt"), "old")
	}

	entries, _ := os.ReadDir(tmp)
	if len(entries) != 2 {
		t.Errorf("expected only the backup and the destination, found %d entries", len(entries))
	}
	assertNoStaging(t, dest)
}

// TestRestoreDetectsCorruptedData checks that a backup whose gzip checksum no
// longer matches fails to restore, instead of restoring silently wrong data.
// The tar reader stops before the gzip trailer, so the restore must read the
// stream to the end to validate the CRC.
func TestRestoreDetectsCorruptedData(t *testing.T) {
	tmp := t.TempDir()
	backup := filepath.Join(tmp, "docs_20260101_000000.tar.gz")
	writeBackup(t, backup, []tarEntry{
		{name: ".", typeflag: tar.TypeDir},
		{name: "a.txt", typeflag: tar.TypeReg, content: strings.Repeat("grid ", 4000)},
	})

	// Corrupt the gzip CRC32 trailer (last 8 bytes are CRC + size). The header
	// and tar structure stay valid, so the archive opens and its entries read,
	// but the checksum no longer matches the data.
	data, err := os.ReadFile(backup)
	if err != nil {
		t.Fatal(err)
	}
	data[len(data)-5] ^= 0xFF
	if err := os.WriteFile(backup, data, 0o644); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(tmp, "docs")
	if _, err := Restore(backup, dest, RestoreActionOverwrite); err == nil {
		t.Error("expected an error restoring a backup with a bad checksum")
	}
	assertNoStaging(t, dest)
}

func TestRestoreRenameMovesExistingContent(t *testing.T) {
	tmp := t.TempDir()
	backup := filepath.Join(tmp, "docs_20260101_000000.tar.gz")
	writeBackup(t, backup, dirBackup())

	dest := filepath.Join(tmp, "docs")
	os.MkdirAll(dest, 0o755)
	os.WriteFile(filepath.Join(dest, "a.txt"), []byte("old"), 0o644)

	if _, err := Restore(backup, dest, RestoreActionRename); err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	assertContent(t, filepath.Join(dest, "a.txt"), "from backup")
	renamed, _ := filepath.Glob(filepath.Join(tmp, "docs_*", "a.txt"))
	if len(renamed) != 1 {
		t.Fatalf("expected the old content in one renamed directory, found %v", renamed)
	}
	assertContent(t, renamed[0], "old")
}

func TestRestoreSkipDoesNothing(t *testing.T) {
	tmp := t.TempDir()
	backup := filepath.Join(tmp, "docs_20260101_000000.tar.gz")
	writeBackup(t, backup, dirBackup())

	dest := filepath.Join(tmp, "docs")
	os.MkdirAll(dest, 0o755)
	os.WriteFile(filepath.Join(dest, "a.txt"), []byte("old"), 0o644)

	result, err := Restore(backup, dest, RestoreActionSkip)
	if err != nil {
		t.Fatalf("Restore failed: %v", err)
	}
	if result.FilesSkipped != 1 || result.FilesRestored != 0 {
		t.Errorf("unexpected result: %+v", result)
	}
	assertContent(t, filepath.Join(dest, "a.txt"), "old")
	if _, err := os.Stat(filepath.Join(dest, "sub")); !os.IsNotExist(err) {
		t.Errorf("skip must not extract anything")
	}
}

func TestRestoreSingleFile(t *testing.T) {
	// "." is how older versions of baky named the entry of a single-file backup
	for _, entryName := range []string{"nginx.conf", "."} {
		t.Run(entryName, func(t *testing.T) {
			tmp := t.TempDir()
			backup := filepath.Join(tmp, "nginx.conf_20260101_000000.tar.gz")
			writeBackup(t, backup, []tarEntry{{name: entryName, typeflag: tar.TypeReg, content: "from backup"}})

			// Destination does not exist yet
			target := filepath.Join(tmp, "etc", "nginx.conf")
			if _, err := Restore(backup, target, RestoreActionOverwrite); err != nil {
				t.Fatalf("Restore to new file failed: %v", err)
			}
			assertContent(t, target, "from backup")

			// Overwrite an existing file
			os.WriteFile(target, []byte("old"), 0o644)
			if _, err := Restore(backup, target, RestoreActionOverwrite); err != nil {
				t.Fatalf("Restore over existing file failed: %v", err)
			}
			assertContent(t, target, "from backup")

			// Skip keeps the existing file
			os.WriteFile(target, []byte("old"), 0o644)
			if _, err := Restore(backup, target, RestoreActionSkip); err != nil {
				t.Fatalf("Restore with skip failed: %v", err)
			}
			assertContent(t, target, "old")

			// Rename keeps the existing file under a new name
			if _, err := Restore(backup, target, RestoreActionRename); err != nil {
				t.Fatalf("Restore with rename failed: %v", err)
			}
			assertContent(t, target, "from backup")
			renamed, _ := filepath.Glob(target + "_*")
			if len(renamed) != 1 {
				t.Fatalf("expected one renamed file, found %v", renamed)
			}
			assertContent(t, renamed[0], "old")
			assertNoStaging(t, filepath.Dir(target))

			// An existing directory as destination receives the file
			dir := filepath.Join(tmp, "custom")
			os.MkdirAll(dir, 0o755)
			result, err := Restore(backup, dir, RestoreActionOverwrite)
			if err != nil {
				t.Fatalf("Restore into directory failed: %v", err)
			}
			assertContent(t, filepath.Join(dir, "nginx.conf"), "from backup")
			if result.DestinationPath != filepath.Join(dir, "nginx.conf") {
				t.Errorf("unexpected destination: %s", result.DestinationPath)
			}
		})
	}
}

func TestParseBackupFilenameUsesLocalTime(t *testing.T) {
	tmp := t.TempDir()
	name := "docs_20260101_030000.tar.gz"
	writeBackup(t, filepath.Join(tmp, name), dirBackup())

	backup, err := parseBackupFilename(name, tmp)
	if err != nil {
		t.Fatal(err)
	}
	if got := backup.Timestamp.Local().Format("20060102_150405"); got != "20260101_030000" {
		t.Errorf("expected local time 20260101_030000, got %s", got)
	}
}
