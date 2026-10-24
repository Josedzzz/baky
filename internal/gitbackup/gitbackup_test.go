package gitbackup

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// env is a watched directory, a config repository and its remote, all
// inside a temp directory
type env struct {
	t      *testing.T
	source string
	repo   string
	remote string
	b      *Backuper

	mu   sync.Mutex
	logs []string
}

func run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func newEnv(t *testing.T) *env {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}

	root := t.TempDir()
	e := &env{
		t:      t,
		source: filepath.Join(root, "etc", "firewall"),
		repo:   filepath.Join(root, "repo"),
		remote: filepath.Join(root, "remote.git"),
	}
	for _, dir := range []string{e.source, e.repo, e.remote} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	run(t, e.remote, "init", "--quiet", "--bare", "--initial-branch=main")
	run(t, e.repo, "init", "--quiet", "--initial-branch=main")
	run(t, e.repo, "config", "user.name", "Test")
	run(t, e.repo, "config", "user.email", "test@example.com")
	run(t, e.repo, "remote", "add", "origin", e.remote)

	e.b = e.backuper(Options{})
	return e
}

// backuper builds a Backuper for the environment with a fixed clock
func (e *env) backuper(opts Options) *Backuper {
	e.t.Helper()
	opts.RepoDir = e.repo
	opts.Branch = "main"
	opts.Watches = []Watch{{Source: e.source, Dest: "firewall"}}
	opts.Log = func(level, message string) {
		e.mu.Lock()
		defer e.mu.Unlock()
		e.logs = append(e.logs, "["+level+"] "+message)
	}

	b, err := New(opts)
	if err != nil {
		e.t.Fatal(err)
	}
	b.now = func() time.Time { return time.Date(2026, 7, 24, 21, 46, 14, 0, time.Local) }
	return b
}

func (e *env) write(rel, content string) {
	e.t.Helper()
	path := filepath.Join(e.source, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		e.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		e.t.Fatal(err)
	}
}

// subjects returns the commit messages of a repository, newest first
func (e *env) subjects(dir string) []string {
	e.t.Helper()
	cmd := exec.Command("git", "-C", dir, "log", "--pretty=format:%s", "main")
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		return nil
	}
	return strings.Split(strings.TrimSpace(string(out)), "\n")
}

func (e *env) logged(text string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, l := range e.logs {
		if strings.Contains(l, text) {
			return true
		}
	}
	return false
}

func (e *env) assertRepoFile(rel, want string) {
	e.t.Helper()
	got, err := os.ReadFile(filepath.Join(e.repo, rel))
	if err != nil {
		e.t.Fatalf("expected %s in the repository: %v", rel, err)
	}
	if string(got) != want {
		e.t.Errorf("%s: expected %q, got %q", rel, want, got)
	}
}

func (e *env) assertNoRepoFile(rel string) {
	e.t.Helper()
	if _, err := os.Lstat(filepath.Join(e.repo, rel)); !os.IsNotExist(err) {
		e.t.Errorf("expected %s to be gone from the repository", rel)
	}
}

func TestSyncCommitsAndPushesNewAndModifiedFiles(t *testing.T) {
	e := newEnv(t)
	e.write("rules.conf", "rule 1")

	if commits := e.b.SyncAll(); commits != 1 {
		t.Fatalf("expected 1 commit, got %d", commits)
	}
	e.assertRepoFile("firewall/rules.conf", "rule 1")

	want := "auto-backup: updated rules.conf (2026-07-24 21:46:14)"
	if got := e.subjects(e.repo); len(got) != 1 || got[0] != want {
		t.Errorf("expected commit %q, got %v", want, got)
	}
	if got := e.subjects(e.remote); len(got) != 1 {
		t.Errorf("expected the commit to be pushed, remote has %v", got)
	}
	if !e.logged("Commit created: updated rules.conf -> firewall/rules.conf.") || !e.logged("Push succeeded.") {
		t.Errorf("unexpected log: %v", e.logs)
	}

	// Nothing changed: no new commit
	if commits := e.b.SyncAll(); commits != 0 {
		t.Errorf("expected no commit without changes, got %d", commits)
	}

	e.write("rules.conf", "rule 2")
	if commits := e.b.SyncAll(); commits != 1 {
		t.Errorf("expected 1 commit for the modification, got %d", commits)
	}
	e.assertRepoFile("firewall/rules.conf", "rule 2")
	if got := e.subjects(e.remote); len(got) != 2 {
		t.Errorf("expected 2 commits in the remote, got %v", got)
	}
}

func TestSyncDeleteAndRename(t *testing.T) {
	e := newEnv(t)
	e.write("temp.conf", "content")
	e.write("vpn.conf", "vpn")
	e.b.SyncAll()

	// Delete
	os.Remove(filepath.Join(e.source, "vpn.conf"))
	if commits := e.b.SyncAll(); commits != 1 {
		t.Fatalf("expected 1 commit for the deletion, got %d", commits)
	}
	e.assertNoRepoFile("firewall/vpn.conf")
	if got := e.subjects(e.repo)[0]; !strings.HasPrefix(got, "auto-backup: deleted vpn.conf (") {
		t.Errorf("unexpected commit: %q", got)
	}

	// Rename: recorded as the removal of the old name and the new file
	os.Rename(filepath.Join(e.source, "temp.conf"), filepath.Join(e.source, "renamed.conf"))
	if commits := e.b.SyncAll(); commits != 2 {
		t.Fatalf("expected 2 commits for the rename, got %d", commits)
	}
	e.assertNoRepoFile("firewall/temp.conf")
	e.assertRepoFile("firewall/renamed.conf", "content")

	all := strings.Join(e.subjects(e.repo), "\n")
	if !strings.Contains(all, "deleted temp.conf") || !strings.Contains(all, "updated renamed.conf") {
		t.Errorf("unexpected commits:\n%s", all)
	}
}

func TestSyncNestedFoldersAndFolderMove(t *testing.T) {
	e := newEnv(t)
	e.write("rules_extra/nat.conf", "nat rule")
	e.write("rules_extra/deep/more.conf", "more")
	e.write("rules.conf", "rule")

	if commits := e.b.SyncAll(); commits != 3 {
		t.Fatalf("expected 3 commits, got %d", commits)
	}
	e.assertRepoFile("firewall/rules_extra/nat.conf", "nat rule")
	e.assertRepoFile("firewall/rules_extra/deep/more.conf", "more")

	// Moving a whole folder out of the watched directory removes it from the repo
	os.Rename(filepath.Join(e.source, "rules_extra"), filepath.Join(filepath.Dir(e.source), "moved"))
	if commits := e.b.SyncAll(); commits != 2 {
		t.Fatalf("expected 2 commits for the moved folder, got %d", commits)
	}
	e.assertNoRepoFile("firewall/rules_extra")
	e.assertRepoFile("firewall/rules.conf", "rule")

	// Moving it back in backs it up again
	os.Rename(filepath.Join(filepath.Dir(e.source), "moved"), filepath.Join(e.source, "rules_extra"))
	if commits := e.b.SyncAll(); commits != 2 {
		t.Fatalf("expected 2 commits for the folder moved in, got %d", commits)
	}
	e.assertRepoFile("firewall/rules_extra/deep/more.conf", "more")
}

func TestSyncIgnoresEditorTempFiles(t *testing.T) {
	e := newEnv(t)
	e.write("rules.conf", "rule")
	for _, name := range []string{".rules.conf.swp", "rules.conf~", "4913", ".#rules.conf"} {
		e.write(name, "noise")
	}

	if commits := e.b.SyncAll(); commits != 1 {
		t.Fatalf("expected only the real file to be committed, got %d commits: %v", commits, e.subjects(e.repo))
	}
	e.assertNoRepoFile("firewall/.rules.conf.swp")
	e.assertNoRepoFile("firewall/rules.conf~")
}

func TestSyncMissingSourceKeepsTheBackup(t *testing.T) {
	e := newEnv(t)
	e.write("rules.conf", "rule")
	e.b.SyncAll()

	// The watched directory disappears (unmounted disk, wrong path...)
	os.Rename(e.source, e.source+".gone")

	if commits := e.b.SyncAll(); commits != 0 {
		t.Errorf("expected no commit, got %d", commits)
	}
	e.assertRepoFile("firewall/rules.conf", "rule")
	if !e.logged("Watched folder not accessible") {
		t.Errorf("expected an error in the log, got %v", e.logs)
	}
}

func TestPushFailureIsRetried(t *testing.T) {
	e := newEnv(t)
	run(t, e.repo, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "missing.git"))

	e.write("rules.conf", "rule")
	if commits := e.b.SyncAll(); commits != 1 {
		t.Fatalf("expected the commit to be kept locally, got %d", commits)
	}
	if !e.logged("Push failed") {
		t.Errorf("expected the push failure in the log, got %v", e.logs)
	}

	// The remote comes back: the next sync pushes without new changes
	run(t, e.repo, "remote", "set-url", "origin", e.remote)
	if commits := e.b.SyncAll(); commits != 0 {
		t.Errorf("expected no new commit, got %d", commits)
	}
	if got := e.subjects(e.remote); len(got) != 1 {
		t.Errorf("expected the pending commit to reach the remote, got %v", got)
	}
}

func TestCommitWithoutGitIdentity(t *testing.T) {
	e := newEnv(t)
	run(t, e.repo, "config", "--unset", "user.name")
	run(t, e.repo, "config", "--unset", "user.email")
	// Hide any identity configured for the user running the tests
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")

	e.write("rules.conf", "rule")
	if commits := e.b.SyncAll(); commits != 1 {
		t.Fatalf("expected the commit to succeed with a default identity, got %d: %v", commits, e.logs)
	}
	if author := run(t, e.repo, "log", "-1", "--pretty=format:%an"); author != "baky" {
		t.Errorf("expected the default author, got %q", author)
	}
}

func TestHistoryListsOnlyBackups(t *testing.T) {
	e := newEnv(t)

	if commits, err := History(e.repo, 10); err != nil || len(commits) != 0 {
		t.Errorf("expected an empty history for a new repository, got %v (%v)", commits, err)
	}

	os.WriteFile(filepath.Join(e.repo, "README.md"), []byte("docs"), 0o644)
	run(t, e.repo, "add", "README.md")
	run(t, e.repo, "commit", "--quiet", "-m", "docs: manual commit")

	e.write("rules.conf", "rule")
	e.b.SyncAll()

	commits, err := History(e.repo, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 1 || commits[0].Message != "auto-backup: updated rules.conf (2026-07-24 21:46:14)" {
		t.Errorf("unexpected history: %+v", commits)
	}
	if commits[0].Hash == "" || commits[0].Date == "" {
		t.Errorf("expected hash and date, got %+v", commits[0])
	}
}

func TestHistoryRecognizesLegacyInvalidMarker(t *testing.T) {
	e := newEnv(t)

	// A commit made before the messages were translated to English
	os.WriteFile(filepath.Join(e.repo, "old.conf"), []byte("old"), 0o644)
	run(t, e.repo, "add", "old.conf")
	run(t, e.repo, "commit", "--quiet", "-m", "auto-backup: actualizado old.conf (2026-07-24 21:46:14) [validación fallida]")

	commits, err := History(e.repo, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 1 || !commits[0].Invalid {
		t.Errorf("expected the legacy commit to be marked invalid: %+v", commits)
	}
}

func TestNewValidatesOptions(t *testing.T) {
	e := newEnv(t)

	cases := map[string]Options{
		"no repository":   {},
		"not a git repo":  {RepoDir: t.TempDir()},
		"relative source": {RepoDir: e.repo, Watches: []Watch{{Source: "etc/firewall", Dest: "firewall"}}},
		"nested dest":     {RepoDir: e.repo, Watches: []Watch{{Source: e.source, Dest: "a/b"}}},
		"escaping dest":   {RepoDir: e.repo, Watches: []Watch{{Source: e.source, Dest: ".."}}},
		"dest .git":       {RepoDir: e.repo, Watches: []Watch{{Source: e.source, Dest: ".git"}}},
	}
	for name, opts := range cases {
		if _, err := New(opts); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestRunBacksUpChangesAsTheyHappen(t *testing.T) {
	e := newEnv(t)
	// A change made while the daemon was not running
	e.write("before.conf", "offline")

	b := e.backuper(Options{Debounce: 50 * time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- b.Run(ctx) }()

	waitFor := func(what string, cond func() bool) {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for !cond() {
			if time.Now().After(deadline) {
				cancel()
				t.Fatalf("timed out waiting for %s; log: %v", what, e.logs)
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	committed := func(text string) func() bool {
		return func() bool { return strings.Contains(strings.Join(e.subjects(e.remote), "\n"), text) }
	}

	waitFor("the startup sync", committed("updated before.conf"))

	e.write("rules.conf", "rule")
	waitFor("a new file", committed("updated rules.conf"))

	// A directory created after startup is watched too
	e.write("new/nat.conf", "nat")
	waitFor("a file in a new directory", committed("updated nat.conf"))

	os.Remove(filepath.Join(e.source, "rules.conf"))
	waitFor("a deletion", committed("deleted rules.conf"))

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run returned an error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not stop after the context was cancelled")
	}
}
