package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// useTempConfig points the package at a config file inside a temp directory
func useTempConfig(t *testing.T) string {
	t.Helper()

	oldConfigPath, oldWarning := configPath, loadWarning
	configPath = filepath.Join(t.TempDir(), "config.json")
	loadWarning = ""
	t.Cleanup(func() { configPath, loadWarning = oldConfigPath, oldWarning })

	return configPath
}

func TestOldFileBackupSettingsAreDropped(t *testing.T) {
	path := useTempConfig(t)

	old := `{
  "nas_path": "/mnt/nas",
  "backup_paths": [{"path": "/etc/nginx", "frequency": "daily"}],
  "config_backup": {"repo_dir": "/srv/config", "branch": "main", "watches": [{"source": "/etc/shorewall", "dest": "firewall"}]},
  "history": [{"path": "/etc/nginx", "result": "success"}]
}`
	if err := os.WriteFile(path, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}

	cb, err := GetConfigBackup()
	if err != nil {
		t.Fatal(err)
	}
	if cb.RepoDir != "/srv/config" || len(cb.Watches) != 1 {
		t.Fatalf("config backup settings not read: %+v", cb)
	}

	// Any save keeps the config backup and drops the rest
	if err := AddWatch("/etc/openvpn", "vpn"); err != nil {
		t.Fatal(err)
	}
	if cb, _ := GetConfigBackup(); cb.RepoDir != "/srv/config" || len(cb.Watches) != 2 {
		t.Errorf("config backup settings lost: %+v", cb)
	}
	settings, _ := os.ReadFile(path)
	for _, field := range []string{"nas_path", "backup_paths", "history"} {
		if strings.Contains(string(settings), field) {
			t.Errorf("%s must leave the config file:\n%s", field, settings)
		}
	}
}

func TestCorruptConfigIsSetAside(t *testing.T) {
	path := useTempConfig(t)

	if err := os.WriteFile(path, []byte(`{"config_backup": {"repo_dir": "/srv/config", `), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("a corrupt config must not be a fatal error: %v", err)
	}
	if cfg.ConfigBackup.RepoDir != "" {
		t.Errorf("expected an empty config, got %+v", cfg)
	}
	if Warning() == "" {
		t.Errorf("expected a warning about the corrupt config")
	}

	kept, err := os.ReadFile(path + ".corrupt")
	if err != nil || !strings.Contains(string(kept), "/srv/config") {
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
	if len(cb.Watches) != 1 || !reflect.DeepEqual(cb.Watches[0], WatchConfig{Source: "/etc/openvpn", Dest: "vpn"}) {
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
	if !reflect.DeepEqual(cb.Watches, want) {
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
	if err := SetWatchCommands("/etc/other", "x {dir}", ""); err == nil {
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
	if !reflect.DeepEqual(cb.Watches[0], want) {
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

func TestValidateExclude(t *testing.T) {
	for pattern, valid := range map[string]bool{
		"*.key":            true,
		"pki/":             true,
		"certs/server.crt": true,
		"":                 false,
		" ":                false,
		"/":                false,
		"/etc/x.key":       false,
		"../x":             false,
		"a//b":             false,
		`keys\a`:           false,
		"[bad":             false,
	} {
		if err := ValidateExclude(pattern); (err == nil) != valid {
			t.Errorf("%q: expected valid=%v, got %v", pattern, valid, err)
		}
	}
}

func TestWatchExcludes(t *testing.T) {
	useTempConfig(t)
	AddWatch("/etc/openvpn", "vpn")

	if err := SetWatchExcludes("/etc/openvpn", []string{"*.bak", "/abs"}); err == nil {
		t.Errorf("expected an error for an absolute pattern")
	}
	if err := SetWatchExcludes("/etc/other", []string{"*.bak"}); err == nil {
		t.Errorf("expected an error for a folder that is not watched")
	}

	// Blanks and repeats are dropped
	if err := SetWatchExcludes("/etc/openvpn", []string{" *.bak ", "", "keys/", "*.bak"}); err != nil {
		t.Fatal(err)
	}
	if cb, _ := GetConfigBackup(); !reflect.DeepEqual(cb.Watches[0].Exclude, []string{"*.bak", "keys/"}) {
		t.Errorf("unexpected excludes: %+v", cb.Watches[0].Exclude)
	}

	if err := SetWatchExcludes("/etc/openvpn", nil); err != nil {
		t.Fatal(err)
	}
	if cb, _ := GetConfigBackup(); cb.Watches[0].Exclude != nil {
		t.Errorf("expected the excludes to be removed: %+v", cb.Watches[0].Exclude)
	}
}

func TestAddWatchExcluding(t *testing.T) {
	useTempConfig(t)

	if err := AddWatchExcluding("/etc/openvpn", "vpn", []string{"/abs"}); err == nil {
		t.Errorf("expected an error for a bad pattern")
	}
	if cb, _ := GetConfigBackup(); len(cb.Watches) != 0 {
		t.Errorf("a bad pattern must not add the watch: %+v", cb.Watches)
	}

	if err := AddWatchExcluding("/etc/openvpn/", "vpn", []string{" *.bak ", ""}); err != nil {
		t.Fatal(err)
	}
	cb, _ := GetConfigBackup()
	want := WatchConfig{Source: "/etc/openvpn", Dest: "vpn", Exclude: []string{"*.bak"}}
	if len(cb.Watches) != 1 || !reflect.DeepEqual(cb.Watches[0], want) {
		t.Errorf("unexpected watches: %+v", cb.Watches)
	}

	if err := CheckNewWatch("/etc/openvpn", "other"); err == nil {
		t.Errorf("expected an error for a folder already watched")
	}
	if err := CheckNewWatch("/etc/other", "vpn"); err == nil {
		t.Errorf("expected an error for a destination already used")
	}
	if err := CheckNewWatch("/etc/other", "other"); err != nil {
		t.Errorf("expected a new watch to be valid: %v", err)
	}
}
