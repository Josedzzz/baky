package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestConfig(t *testing.T) {
	// Create a temporary directory for the test config
	tmpDir, err := os.MkdirTemp("", "baky-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Override configPath
	oldConfigPath := configPath
	configPath = filepath.Join(tmpDir, "config.json")
	defer func() { configPath = oldConfigPath }()

	// Test SaveNasPath and GetNasPath
	testNas := "/tmp/nas"
	if err := SaveNasPath(testNas); err != nil {
		t.Errorf("SaveNasPath failed: %v", err)
	}

	nas, err := GetNasPath()
	if err != nil {
		t.Errorf("GetNasPath failed: %v", err)
	}
	if nas != testNas {
		t.Errorf("Expected NAS path %s, got %s", testNas, nas)
	}

	// Test AddPath and GetPaths
	testPath := "/home/user/docs"
	if err := AddPath(testPath); err != nil {
		t.Errorf("AddPath failed: %v", err)
	}

	paths, err := GetPaths()
	if err != nil {
		t.Errorf("GetPaths failed: %v", err)
	}
	if len(paths) != 1 {
		t.Errorf("Expected 1 path, got %d", len(paths))
	}
	if paths[0].Path != testPath {
		t.Errorf("Expected path %s, got %s", testPath, paths[0].Path)
	}

	// Test LogBackup
	if err := LogBackup(testPath, true, "Success"); err != nil {
		t.Errorf("LogBackup failed: %v", err)
	}

	hist, err := GetHistory()
	if err != nil {
		t.Errorf("GetHistory failed: %v", err)
	}
	if len(hist) != 1 {
		t.Errorf("Expected 1 history event, got %d", len(hist))
	}
	if hist[0].Path != testPath || hist[0].Result != "success" {
		t.Errorf("History entry mismatch")
	}
}

// useTempConfig points the package at a config file inside a temp directory
func useTempConfig(t *testing.T) string {
	t.Helper()

	oldConfigPath, oldWarning := configPath, loadWarning
	configPath = filepath.Join(t.TempDir(), "config.json")
	loadWarning = ""
	t.Cleanup(func() { configPath, loadWarning = oldConfigPath, oldWarning })

	return configPath
}

func TestHistoryIsStoredInStateFile(t *testing.T) {
	path := useTempConfig(t)

	if err := AddPath("/home/user/docs"); err != nil {
		t.Fatal(err)
	}
	if err := LogBackup("/home/user/docs", true, ""); err != nil {
		t.Fatal(err)
	}

	settings, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(settings), "history") || strings.Contains(string(settings), "last_backup") {
		t.Errorf("config file must not hold history or last backup times:\n%s", settings)
	}

	state, err := os.ReadFile(StatePath())
	if err != nil {
		t.Fatalf("state file not written: %v", err)
	}
	if !strings.Contains(string(state), "/home/user/docs") {
		t.Errorf("state file must hold the backup event:\n%s", state)
	}

	paths, _ := GetPaths()
	if len(paths) != 1 || paths[0].LastBackup.IsZero() {
		t.Errorf("expected the last backup time to be loaded from the state file, got %+v", paths)
	}

	// Saving the paths from a stale copy must not lose the last backup time
	paths[0].LastBackup = time.Time{}
	paths[0].Frequency = FreqWeekly
	if err := SavePaths(paths); err != nil {
		t.Fatal(err)
	}
	paths, _ = GetPaths()
	if paths[0].Frequency != FreqWeekly || paths[0].LastBackup.IsZero() {
		t.Errorf("expected frequency updated and last backup kept, got %+v", paths[0])
	}
}

func TestLegacyConfigIsMigrated(t *testing.T) {
	path := useTempConfig(t)

	legacy := `{
  "nas_path": "/mnt/nas",
  "backup_paths": [{"path": "/etc/nginx", "frequency": "daily", "last_backup": "2026-04-14T11:07:48Z"}],
  "history": [{"path": "/etc/nginx", "timestamp": "2026-04-14T11:07:48Z", "result": "success"}],
  "restore_history": [{"backup_filename": "nginx_20260414_110748.tar.gz", "result": "success"}]
}`
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.NasPath != "/mnt/nas" || len(cfg.History) != 1 || len(cfg.RestoreHistory) != 1 {
		t.Fatalf("legacy config not read: %+v", cfg)
	}
	if cfg.BackupPaths[0].LastBackup.IsZero() {
		t.Errorf("legacy last backup time not read")
	}

	// Any save moves the history to the state file and keeps everything
	if err := SaveNasPath("/mnt/nas2"); err != nil {
		t.Fatal(err)
	}
	cfg, _ = LoadConfig()
	if cfg.NasPath != "/mnt/nas2" || len(cfg.History) != 1 || len(cfg.RestoreHistory) != 1 || cfg.BackupPaths[0].LastBackup.IsZero() {
		t.Errorf("data lost in migration: %+v", cfg)
	}
	settings, _ := os.ReadFile(path)
	if strings.Contains(string(settings), "history") {
		t.Errorf("history must leave the config file after migration:\n%s", settings)
	}
}

func TestCorruptConfigIsSetAside(t *testing.T) {
	path := useTempConfig(t)

	if err := os.WriteFile(path, []byte(`{"nas_path": "/mnt/nas", `), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("a corrupt config must not be a fatal error: %v", err)
	}
	if cfg.NasPath != "" {
		t.Errorf("expected an empty config, got %+v", cfg)
	}
	if Warning() == "" {
		t.Errorf("expected a warning about the corrupt config")
	}

	kept, err := os.ReadFile(path + ".corrupt")
	if err != nil || !strings.Contains(string(kept), "/mnt/nas") {
		t.Errorf("the corrupt file must be kept for recovery: %v", err)
	}
}

func TestConfigBackupWatches(t *testing.T) {
	useTempConfig(t)

	if err := SaveConfigRepo("relative/repo"); err == nil {
		t.Errorf("expected an error for a relative repository path")
	}
	if err := SaveConfigRepo("/var/lib/baky/config-repo"); err != nil {
		t.Fatal(err)
	}

	if err := AddWatch("/etc/openvpn/", "vpn"); err != nil {
		t.Fatal(err)
	}
	for name, add := range map[string][2]string{
		"relative source":  {"etc/iptables", "firewall"},
		"duplicate source": {"/etc/openvpn", "vpn2"},
		"duplicate dest":   {"/etc/wireguard", "vpn"},
		"dest with slash":  {"/etc/iptables", "fire/wall"},
		"dest escaping":    {"/etc/iptables", ".."},
		"dest .git":        {"/etc/iptables", ".git"},
	} {
		if err := AddWatch(add[0], add[1]); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}

	cb, err := GetConfigBackup()
	if err != nil {
		t.Fatal(err)
	}
	if cb.RepoDir != "/var/lib/baky/config-repo" || cb.Branch != DefaultBranch {
		t.Errorf("unexpected repo settings: %+v", cb)
	}
	if len(cb.Watches) != 1 || cb.Watches[0] != (WatchConfig{Source: "/etc/openvpn", Dest: "vpn"}) {
		t.Errorf("unexpected watches: %+v", cb.Watches)
	}

	if err := RemoveWatch("/etc/openvpn"); err != nil {
		t.Fatal(err)
	}
	if err := RemoveWatch("/etc/openvpn"); err == nil {
		t.Errorf("expected an error removing a watch that does not exist")
	}
	if cb, _ := GetConfigBackup(); len(cb.Watches) != 0 {
		t.Errorf("expected no watches, got %+v", cb.Watches)
	}
}

func TestImportMagitiConfig(t *testing.T) {
	useTempConfig(t)

	old := filepath.Join(t.TempDir(), "config.json")
	os.WriteFile(old, []byte(`{
  "watch_dir": "/home/miguel/simulation-configs",
  "repo_dir": "/home/miguel/projects/magiti-config-backup",
  "carpetas": {"vpn": "vpn", "firewall": "firewall", "dns2": "dns2"}
}`), 0o644)

	added, err := ImportMagitiConfig(old)
	if err != nil {
		t.Fatal(err)
	}
	if len(added) != 3 {
		t.Fatalf("expected 3 watches, got %+v", added)
	}

	cb, _ := GetConfigBackup()
	want := []WatchConfig{
		{Source: "/home/miguel/simulation-configs/dns2", Dest: "dns2"},
		{Source: "/home/miguel/simulation-configs/firewall", Dest: "firewall"},
		{Source: "/home/miguel/simulation-configs/vpn", Dest: "vpn"},
	}
	if len(cb.Watches) != 3 || cb.Watches[0] != want[0] || cb.Watches[1] != want[1] || cb.Watches[2] != want[2] {
		t.Errorf("unexpected watches: %+v", cb.Watches)
	}
	if cb.RepoDir != "" {
		t.Errorf("the old repository must not be imported, got %q", cb.RepoDir)
	}

	// Importing twice reports the duplicate instead of adding it again
	if _, err := ImportMagitiConfig(old); err == nil {
		t.Errorf("expected an error importing the same folders twice")
	}

	os.WriteFile(old, []byte(`not json`), 0o644)
	if _, err := ImportMagitiConfig(old); err == nil {
		t.Errorf("expected an error for an invalid file")
	}
}

func TestWatchCommands(t *testing.T) {
	useTempConfig(t)
	AddWatch("/etc/shorewall", "firewall")

	if err := SetWatchCommands("/etc/shorewall", "shorewall check", ""); err == nil {
		t.Errorf("expected an error for a command without {dir}")
	}
	if err := SetWatchCommands("/etc/otro", "x {dir}", ""); err == nil {
		t.Errorf("expected an error for a folder that is not watched")
	}

	if err := SetWatchCommands("/etc/shorewall", " sudo -n shorewall check {dir} ", "sudo -n shorewall try {dir}"); err != nil {
		t.Fatal(err)
	}
	cb, _ := GetConfigBackup()
	want := WatchConfig{
		Source: "/etc/shorewall", Dest: "firewall",
		Check: "sudo -n shorewall check {dir}", Apply: "sudo -n shorewall try {dir}",
	}
	if cb.Watches[0] != want {
		t.Errorf("unexpected watch: %+v", cb.Watches[0])
	}

	// Empty commands remove the steps
	if err := SetWatchCommands("/etc/shorewall", "", ""); err != nil {
		t.Fatal(err)
	}
	if cb, _ := GetConfigBackup(); cb.Watches[0].Check != "" || cb.Watches[0].Apply != "" {
		t.Errorf("expected the commands to be removed: %+v", cb.Watches[0])
	}
}
