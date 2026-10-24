package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Josedzzz/baky/internal/backup"
	"github.com/Josedzzz/baky/internal/config"
	"github.com/Josedzzz/baky/internal/gitbackup"
	tea "github.com/charmbracelet/bubbletea"
)

// newTestModel returns a model backed by a config file in a temp directory
func newTestModel(t *testing.T) Model {
	t.Helper()

	old := config.Path()
	config.SetPath(filepath.Join(t.TempDir(), "config.json"))
	t.Cleanup(func() { config.SetPath(old) })

	m := NewModel()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return updated.(Model)
}

// press sends keys to the model; a name like "enter" or "esc" is a special
// key, anything else is typed as text
func press(t *testing.T, m Model, keys ...string) Model {
	t.Helper()
	for _, key := range keys {
		var msg tea.KeyMsg
		switch key {
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		case "up":
			msg = tea.KeyMsg{Type: tea.KeyUp}
		case "down":
			msg = tea.KeyMsg{Type: tea.KeyDown}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
		}
		updated, _ := m.Update(msg)
		m = updated.(Model)
	}
	return m
}

func assertShows(t *testing.T, m Model, texts ...string) {
	t.Helper()
	view := m.View()
	for _, text := range texts {
		if !strings.Contains(view, text) {
			t.Errorf("expected the screen to show %q, got:\n%s", text, view)
		}
	}
}

func TestMenuNavigation(t *testing.T) {
	m := newTestModel(t)
	assertShows(t, m, "MAIN MENU", "File backup", "Config backup", "Service", "Help", "Quit")

	// Every entry of every menu opens a screen that renders something
	for state, current := range menus {
		for i, entry := range current.entries {
			if entry.target == quitEntry {
				continue
			}
			m.state, m.cursor = state, i
			opened := press(t, m, "enter")
			if opened.state != entry.target {
				t.Errorf("%s: expected state %d, got %d", entry.label, entry.target, opened.state)
			}
			if !strings.Contains(opened.View(), "BAKY") && !isMenu(opened.state) {
				t.Errorf("%s: the screen rendered nothing", entry.label)
			}
		}
	}

	// A submenu remembers its place and esc walks back up
	m = newTestModel(t)
	m = press(t, m, "down", "enter")
	assertShows(t, m, "CONFIG BACKUP", "Watched folders", "Change history", "Repository")
	m = press(t, m, "down", "down", "enter")
	assertShows(t, m, "CONFIG REPOSITORY")
	m = press(t, m, "esc")
	if m.state != configMenuView || m.cursor != 2 {
		t.Errorf("expected to return to the third entry of the config menu, got state %d cursor %d", m.state, m.cursor)
	}
	m = press(t, m, "esc")
	if m.state != menuView || m.cursor != 1 {
		t.Errorf("expected to return to the second entry of the main menu, got state %d cursor %d", m.state, m.cursor)
	}

	m = press(t, m, "?")
	assertShows(t, m, "HELP", "Config backup")
	m = press(t, m, "esc")

	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}); cmd == nil {
		t.Errorf("expected q to quit from the main menu")
	}
}

func TestAddAndRemoveWatchedFolder(t *testing.T) {
	m := newTestModel(t)
	source := filepath.Join(t.TempDir(), "openvpn")
	os.MkdirAll(source, 0o755)

	m.state = watchesView
	assertShows(t, m, "No folders are being watched", "No repository is configured yet")

	// A path that does not exist is rejected in the first step
	m = press(t, m, "a", "/does/not/exist", "enter")
	assertShows(t, m, "Step 1 of 2", "does not exist or is not a folder")

	m = press(t, m, "esc", "a", source, "enter")
	assertShows(t, m, "Step 2 of 2", source)
	if m.watchInput.Value() != "openvpn" {
		t.Errorf("expected the folder name as the suggested destination, got %q", m.watchInput.Value())
	}

	m = press(t, m, "enter")
	assertShows(t, m, "WATCHED FOLDERS", source+"  →  openvpn", "added")

	cb, _ := config.GetConfigBackup()
	if len(cb.Watches) != 1 || cb.Watches[0].Source != source || cb.Watches[0].Dest != "openvpn" {
		t.Fatalf("watch not saved: %+v", cb.Watches)
	}

	// Removing asks first, and "n" cancels
	m = press(t, m, "d")
	assertShows(t, m, "Stop watching the folder", "No file is deleted")
	m = press(t, m, "n")
	if cb, _ := config.GetConfigBackup(); len(cb.Watches) != 1 {
		t.Errorf("cancelling must keep the watch")
	}

	m = press(t, m, "d", "y")
	assertShows(t, m, "is no longer watched", "No folders are being watched")
	if cb, _ := config.GetConfigBackup(); len(cb.Watches) != 0 {
		t.Errorf("expected the watch to be removed, got %+v", cb.Watches)
	}
}

func TestManagePaths(t *testing.T) {
	m := newTestModel(t)
	m.state = managePathsView

	m = press(t, m, "a", "/home/user/docs", "enter")
	assertShows(t, m, "/home/user/docs", "[daily]", "Path added")

	m = press(t, m, "f")
	assertShows(t, m, "[weekly]")
	m = press(t, m, "f")
	assertShows(t, m, "[on change]")

	paths, _ := config.GetPaths()
	if len(paths) != 1 || paths[0].Frequency != config.FreqOnChange {
		t.Fatalf("path not saved: %+v", paths)
	}

	m = press(t, m, "d")
	assertShows(t, m, "No paths added yet", "Path deleted")
}

func TestConfigRepoScreen(t *testing.T) {
	m := newTestModel(t)
	m.state = configRepoView
	assertShows(t, m, "not configured")

	notRepo := t.TempDir()
	m = press(t, m, "e", notRepo, "enter")
	assertShows(t, m, notRepo, "is not a Git repository")

	if cb, _ := config.GetConfigBackup(); cb.RepoDir != notRepo {
		t.Errorf("repository path not saved: %+v", cb)
	}
}

func TestLogsScreen(t *testing.T) {
	m := newTestModel(t)

	m.state = logsView
	updated, _ := m.Update(logsLoadedMsg{err: os.ErrNotExist})
	assertShows(t, updated.(Model), "Could not read the log")

	updated, _ = m.Update(logsLoadedMsg{lines: []string{
		"2026-07-24 21:46:14 [INFO] Commit created: updated nat.conf -> firewall/nat.conf.",
		"2026-07-24 21:46:17 [ERROR] Push failed.",
	}})
	assertShows(t, updated.(Model), "Commit created", "Push failed")
}

func TestRestoreConfigFromHistory(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	m := newTestModel(t)

	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	source := filepath.Join(root, "firewall")
	live := filepath.Join(source, "rules.conf")
	os.MkdirAll(repo, 0o755)
	os.MkdirAll(source, 0o755)
	for _, args := range [][]string{
		{"init", "--quiet", "--initial-branch=main"},
		{"config", "user.name", "Test"},
		{"config", "user.email", "test@example.com"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	config.SaveConfigRepo(repo)
	config.AddWatch(source, "firewall")

	// Two versions of the file in the history
	backuper, err := gitbackup.New(gitbackup.Options{RepoDir: repo, Watches: []gitbackup.Watch{{Source: source, Dest: "firewall"}}})
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(live, []byte("rule 1\nrule 2\n"), 0o644)
	backuper.SyncAll()
	os.WriteFile(live, []byte("rule 1\nrule BROKEN\n"), 0o644)
	backuper.SyncAll()

	// run executes the command a key press returned and feeds its result back
	run := func(m Model, key string) Model {
		t.Helper()
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
		if key == "enter" {
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		}
		updated, cmd := m.Update(msg)
		if cmd == nil {
			t.Fatalf("expected %q to start a command", key)
		}
		updated, _ = updated.(Model).Update(cmd())
		return updated.(Model)
	}

	// Open the history from the config menu
	m.state, m.cursor = configMenuView, 1
	m = run(m, "enter")
	assertShows(t, m, "CHANGE HISTORY", "updated rules.conf", "[Change 1 of 2]")

	// The newest change is what the file already has
	m = run(m, "enter")
	assertShows(t, m, "CHANGES TO RESTORE", live, "already matches this version")
	if m = press(t, m, "r"); m.state != configDiffView {
		t.Errorf("nothing to restore: r must do nothing")
	}

	// The older change shows what restoring it would do
	m = press(t, m, "esc", "down")
	m = run(m, "enter")
	assertShows(t, m, "Changes from line 1 of the current file", "- rule BROKEN", "+ rule 2", "r: restore this version")

	// Cancelling keeps the file; confirming restores it
	m = press(t, m, "r")
	assertShows(t, m, "CONFIRM RESTORE", "1 line removed", "1 line comes back")
	m = press(t, m, "n")
	if got, _ := os.ReadFile(live); string(got) != "rule 1\nrule BROKEN\n" {
		t.Fatalf("cancelling must not change the file, got %q", got)
	}

	m = press(t, m, "r")
	m = run(m, "y")
	assertShows(t, m, "CHANGE HISTORY", "restored")
	if got, _ := os.ReadFile(live); string(got) != "rule 1\nrule 2\n" {
		t.Errorf("expected the old version to be restored, got %q", got)
	}
}

func TestRestorePreviewAndFileDiff(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	m := newTestModel(t)

	root := t.TempDir()
	source := filepath.Join(root, "docs")
	nas := filepath.Join(root, "nas")
	os.MkdirAll(source, 0o755)
	os.MkdirAll(nas, 0o755)
	os.WriteFile(filepath.Join(source, "report.txt"), []byte("line 1\nline 2\n"), 0o644)
	os.WriteFile(filepath.Join(source, "same.txt"), []byte("no changes\n"), 0o644)
	os.WriteFile(filepath.Join(source, "deleted.txt"), []byte("comes back\n"), 0o644)

	config.SaveNasPath(nas)
	config.SavePaths([]config.BackupPathConfig{{Path: source, Frequency: config.FreqDaily}})
	if err := backup.RunBackup(source); err != nil {
		t.Fatal(err)
	}

	// The folder changes after the backup
	os.WriteFile(filepath.Join(source, "report.txt"), []byte("line 1\nline CHANGED\n"), 0o644)
	os.Remove(filepath.Join(source, "deleted.txt"))
	os.WriteFile(filepath.Join(source, "new.txt"), []byte("not in the backup\n"), 0o644)

	// run executes the command a key press returned and feeds its result back
	run := func(m Model, key string) Model {
		t.Helper()
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
		if key == "enter" {
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		}
		updated, cmd := m.Update(msg)
		if cmd == nil {
			t.Fatalf("expected %q to start a command", key)
		}
		updated, _ = updated.(Model).Update(cmd())
		return updated.(Model)
	}

	// Files menu → view backups → select the backup → original location
	m.state, m.cursor = filesMenuView, 2
	m = press(t, m, "enter", "enter")
	assertShows(t, m, "RESTORE DESTINATION")
	m = run(m, "o")

	assertShows(t, m, "RESTORE CHANGES",
		"M modified: 1", "+ created: 1", "= unchanged: 1",
		"report.txt  (modified)", "deleted.txt  (created)", "same.txt  (unchanged)",
		"Files at the destination that are not in the backup: 1")
	tempCopy := m.restorePreview.Files[0].BackupPath

	// The modified file is first: enter shows its differences
	m = run(m, "enter")
	assertShows(t, m, "FILE DIFFERENCES", filepath.Join(source, "report.txt"),
		"Changes from line 1 of the current file", "- line CHANGED", "+ line 2")

	// Back to the list, then on to the restore itself
	m = press(t, m, "esc")
	assertShows(t, m, "RESTORE CHANGES")
	m = press(t, m, "r")
	assertShows(t, m, "EXISTING FILES", "Overwrite")
	if m = press(t, m, "esc"); m.state != restorePreviewView {
		t.Errorf("expected esc to return to the preview, got state %d", m.state)
	}

	m = press(t, m, "r")
	m = run(m, "enter")
	assertShows(t, m, "VIEW BACKUPS", "Restored 3 files")

	// The restore brought the backup back and kept the unrelated file
	for name, want := range map[string]string{
		"report.txt":  "line 1\nline 2\n",
		"deleted.txt": "comes back\n",
		"new.txt":     "not in the backup\n",
	} {
		if got, _ := os.ReadFile(filepath.Join(source, name)); string(got) != want {
			t.Errorf("%s: expected %q, got %q", name, want, got)
		}
	}

	// The temporary copy of the backup is gone
	if m.restorePreview != nil {
		t.Errorf("expected the preview to be closed after the restore")
	}
	if _, err := os.Stat(tempCopy); !os.IsNotExist(err) {
		t.Errorf("expected the temporary copy to be removed")
	}
}

func TestEditWatchCommands(t *testing.T) {
	m := newTestModel(t)
	source := filepath.Join(t.TempDir(), "shorewall")
	os.MkdirAll(source, 0o755)
	config.AddWatch(source, "firewall")

	m.state = watchesView
	m.reloadConfigBackup()
	assertShows(t, m, "Validation: ", "Apply: ", "(none)")

	// A command without {dir} is rejected
	m = press(t, m, "c", "shorewall check", "enter")
	assertShows(t, m, "Step 1 of 2", "must include {dir}")

	m = press(t, m, " {dir}", "enter")
	assertShows(t, m, "Step 2 of 2")
	m = press(t, m, "shorewall try {dir}", "enter")
	assertShows(t, m, "WATCHED FOLDERS", "Commands saved",
		"Validation: shorewall check {dir}", "Apply:      shorewall try {dir}")

	cb, _ := config.GetConfigBackup()
	if cb.Watches[0].Check != "shorewall check {dir}" || cb.Watches[0].Apply != "shorewall try {dir}" {
		t.Errorf("commands not saved: %+v", cb.Watches[0])
	}
}
