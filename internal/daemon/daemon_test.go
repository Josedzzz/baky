package daemon

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Josedzzz/baky/internal/backup"
	"github.com/Josedzzz/baky/internal/config"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			log, _ := os.ReadFile(config.LogPath())
			t.Fatalf("timed out waiting for %s; daemon log:\n%s", what, log)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// TestDaemonEndToEnd runs the daemon against a temp config and checks
// both engines, including a config change made while it is running
func TestDaemonEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}

	root := t.TempDir()
	oldPath, oldDebounce := config.Path(), backup.Debounce
	config.SetPath(filepath.Join(root, "cfg", "config.json"))
	backup.Debounce = 100 * time.Millisecond
	t.Cleanup(func() {
		config.SetPath(oldPath)
		backup.Debounce = oldDebounce
	})

	repo := filepath.Join(root, "repo")
	firewall := filepath.Join(root, "etc", "firewall")
	vpn := filepath.Join(root, "etc", "vpn")
	docs := filepath.Join(root, "docs")
	nas := filepath.Join(root, "nas")
	for _, dir := range []string{filepath.Dir(config.Path()), repo, firewall, vpn, docs, nas} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	git(t, repo, "init", "--quiet", "--initial-branch=main")
	git(t, repo, "config", "user.name", "Test")
	git(t, repo, "config", "user.email", "test@example.com")

	// A change made before the daemon starts
	os.WriteFile(filepath.Join(firewall, "reglas.conf"), []byte("regla"), 0o644)

	if err := config.SaveConfigRepo(repo); err != nil {
		t.Fatal(err)
	}
	if err := config.AddWatch(firewall, "firewall"); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx) }()

	committed := func(text string) func() bool {
		return func() bool {
			out, err := exec.Command("git", "-C", repo, "log", "--pretty=format:%s").Output()
			return err == nil && strings.Contains(string(out), text)
		}
	}
	waitFor(t, "the startup sync", committed("actualizado reglas.conf"))

	// A watch added while running is picked up without a restart
	if err := config.AddWatch(vpn, "vpn"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the config reload", func() bool {
		log, _ := os.ReadFile(config.LogPath())
		return strings.Contains(string(log), "Configuración modificada")
	})
	os.WriteFile(filepath.Join(vpn, "server.conf"), []byte("vpn"), 0o644)
	waitFor(t, "a file in the new watch", committed("actualizado server.conf"))
	if _, err := os.Stat(filepath.Join(repo, "vpn", "server.conf")); err != nil {
		t.Errorf("expected vpn/server.conf in the repository: %v", err)
	}

	// An on_change file backup is made by the daemon, not by the TUI
	if err := config.SaveNasPath(nas); err != nil {
		t.Fatal(err)
	}
	if err := config.SavePaths([]config.BackupPathConfig{{Path: docs, Frequency: config.FreqOnChange}}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the file backup engine", func() bool {
		log, _ := os.ReadFile(config.LogPath())
		return strings.Contains(string(log), "Rutas de respaldo de archivos: 1")
	})
	// Give the new engine a moment to place its watches
	time.Sleep(300 * time.Millisecond)
	os.WriteFile(filepath.Join(docs, "informe.txt"), []byte("informe"), 0o644)
	waitFor(t, "the on_change backup", func() bool {
		archives, _ := filepath.Glob(filepath.Join(nas, "docs_*.tar.gz"))
		return len(archives) > 0
	})

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run returned an error: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the daemon did not stop")
	}
}
