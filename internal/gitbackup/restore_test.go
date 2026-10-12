package gitbackup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Josedzzz/baky/internal/textdiff"
)

func (e *env) watches() []Watch {
	return []Watch{{Source: e.source, Dest: "firewall"}}
}

func TestHistoryRecordsTheChangedFile(t *testing.T) {
	e := newEnv(t)
	e.write("extra/nat con espacios.conf", "nat")
	e.b.SyncAll()

	commits, err := History(e.repo, 10)
	if err != nil || len(commits) != 1 {
		t.Fatalf("unexpected history: %+v (%v)", commits, err)
	}
	if commits[0].Path != "firewall/extra/nat con espacios.conf" {
		t.Errorf("unexpected path: %q", commits[0].Path)
	}
}

func TestLivePath(t *testing.T) {
	watches := []Watch{{Source: "/etc/openvpn", Dest: "vpn"}, {Source: "/etc/iptables", Dest: "firewall"}}

	if got, ok := LivePath(watches, "firewall/extra/nat.conf"); !ok || got != "/etc/iptables/extra/nat.conf" {
		t.Errorf("unexpected live path: %q %v", got, ok)
	}
	if _, ok := LivePath(watches, "dns/zona.conf"); ok {
		t.Errorf("expected no live path for a folder that is not watched")
	}
	if _, ok := LivePath(watches, "vpn2/server.conf"); ok {
		t.Errorf("a folder must not match another one that only shares its prefix")
	}
}

func TestPreviewAndRestoreAnOlderVersion(t *testing.T) {
	e := newEnv(t)
	live := filepath.Join(e.source, "reglas.conf")

	e.write("reglas.conf", "regla 1\nregla 2\nregla 3\n")
	e.b.SyncAll()
	e.write("reglas.conf", "regla 1\nregla CAMBIADA\nregla 3\nregla 4\n")
	e.b.SyncAll()

	commits, _ := History(e.repo, 10)
	if len(commits) != 2 {
		t.Fatalf("expected 2 commits, got %+v", commits)
	}
	newest, oldest := commits[0], commits[1]

	// The newest commit is what the file already contains
	preview, err := PreviewRestore(e.repo, e.watches(), newest)
	if err != nil {
		t.Fatal(err)
	}
	if !preview.Identical || len(preview.Lines) != 0 {
		t.Errorf("expected the current file to equal the newest commit: %+v", preview)
	}

	// The oldest commit differs: the diff goes from the current file to it
	preview, err = PreviewRestore(e.repo, e.watches(), oldest)
	if err != nil {
		t.Fatal(err)
	}
	if preview.LivePath != live || !preview.InCommit || !preview.LiveExists || preview.Identical {
		t.Fatalf("unexpected preview: %+v", preview)
	}

	var removed, added []string
	hunkLine := 0
	for _, line := range preview.Lines {
		switch line.Kind {
		case textdiff.Removed:
			removed = append(removed, line.Text)
		case textdiff.Added:
			added = append(added, line.Text)
		case textdiff.Hunk:
			hunkLine = line.Number
		}
	}
	if strings.Join(removed, "|") != "regla CAMBIADA|regla 4" {
		t.Errorf("expected the current lines to be removed, got %v", removed)
	}
	if strings.Join(added, "|") != "regla 2" {
		t.Errorf("expected the old line to come back, got %v", added)
	}
	if hunkLine != 1 {
		t.Errorf("expected the changes to start at line 1, got %d", hunkLine)
	}

	// Restoring writes the old version and keeps the file permissions
	os.Chmod(live, 0o600)
	if _, err := preview.Restore(); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(live)
	if string(got) != "regla 1\nregla 2\nregla 3\n" {
		t.Errorf("unexpected restored content: %q", got)
	}
	if info, _ := os.Stat(live); info.Mode().Perm() != 0o600 {
		t.Errorf("expected the permissions to be kept, got %v", info.Mode().Perm())
	}

	// The restore is itself backed up as a new change
	if commits := e.b.SyncAll(); commits != 1 {
		t.Errorf("expected the restore to be committed, got %d commits", commits)
	}
}

func TestPreviewWhenTheFileWasDeleted(t *testing.T) {
	e := newEnv(t)
	live := filepath.Join(e.source, "vpn.conf")

	e.write("vpn.conf", "vpn\n")
	e.b.SyncAll()
	os.Remove(live)
	e.b.SyncAll()

	commits, _ := History(e.repo, 10)
	deletion, creation := commits[0], commits[1]

	// The deletion commit has no version of the file to go back to
	preview, err := PreviewRestore(e.repo, e.watches(), deletion)
	if err != nil {
		t.Fatal(err)
	}
	if preview.InCommit {
		t.Errorf("expected no version in the deletion commit")
	}
	if _, err := preview.Restore(); err == nil {
		t.Errorf("expected an error restoring from a deletion commit")
	}

	// The earlier commit brings the deleted file back
	preview, err = PreviewRestore(e.repo, e.watches(), creation)
	if err != nil {
		t.Fatal(err)
	}
	if preview.LiveExists || preview.Identical {
		t.Fatalf("unexpected preview: %+v", preview)
	}
	if len(preview.Lines) != 2 || preview.Lines[1].Kind != textdiff.Added || preview.Lines[1].Text != "vpn" {
		t.Errorf("expected the whole file as added lines, got %+v", preview.Lines)
	}
	if _, err := preview.Restore(); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(live); string(got) != "vpn\n" {
		t.Errorf("expected the file to be back, got %q", got)
	}
}

func TestPreviewForAFolderNoLongerWatched(t *testing.T) {
	e := newEnv(t)
	e.write("reglas.conf", "regla\n")
	e.b.SyncAll()
	commits, _ := History(e.repo, 10)

	if _, err := PreviewRestore(e.repo, []Watch{{Source: "/etc/otro", Dest: "otro"}}, commits[0]); err == nil {
		t.Errorf("expected an error when the folder is not watched anymore")
	}
}
