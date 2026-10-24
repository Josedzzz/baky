package gitbackup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExcluded(t *testing.T) {
	ex := newExcluder([]string{"*.bak", "keys/", "certs/server.crt", "[bad"})
	for rel, want := range map[string]bool{
		"rules.conf":             false,
		"server.key":             true, // built-in
		"easy-rsa/pki/ca.crt":    true, // built-in folder, at any depth
		"pki":                    false,
		"ssh/id_ed25519":         true,
		"ssh/id_ed25519.pub":     false,
		"rules.conf.bak":         true,
		"old/rules.conf.bak":     true,
		"keys/a.txt":             true,
		"keys":                   false, // a file, not the folder
		"certs/server.crt":       true,
		"other/certs/server.crt": false, // anchored to the watched folder
		"certs/client.crt":       false,
	} {
		if got := ex.excluded(rel, false); got != want {
			t.Errorf("%s: expected excluded=%v, got %v", rel, want, got)
		}
	}
	if !ex.excluded("keys", true) {
		t.Errorf("expected the folder keys/ to be excluded")
	}

	var none *excluder
	if none.excluded("server.key", false) || none.holdsSecret([]byte("-----BEGIN PRIVATE KEY-----")) {
		t.Errorf("a nil excluder must exclude nothing")
	}
}

func TestHoldsSecret(t *testing.T) {
	ex := newExcluder(nil)
	for content, want := range map[string]bool{
		"client\nremote vpn.example.com 1194\n":                            false,
		"<key>\n-----BEGIN PRIVATE KEY-----\nMIIE...\n</key>\n":            true,
		"-----BEGIN OPENSSH PRIVATE KEY-----\n":                            true,
		"-----BEGIN RSA PRIVATE KEY-----\n":                                true,
		"<tls-auth>\n-----BEGIN OpenVPN Static key V1-----\n</tls-auth>\n": true,
		"-----BEGIN CERTIFICATE-----\n":                                    false,
	} {
		if got := ex.holdsSecret([]byte(content)); got != want {
			t.Errorf("%q: expected %v, got %v", content, want, got)
		}
	}
}

func TestSecretsAreNotBackedUp(t *testing.T) {
	e := newEnv(t)
	e.b.opts.Watches[0].Exclude = []string{"*.bak"}

	e.write("server.conf", "port 1194\n")
	e.write("server.key", "not checked by content\n")
	e.write("pki/private/ca.key", "secret\n")
	e.write("rules.conf.bak", "old\n")
	e.write("client.conf", "<key>\n-----BEGIN PRIVATE KEY-----\nMIIE\n</key>\n")

	if commits := e.b.SyncAll(); commits != 1 {
		t.Fatalf("expected only server.conf to be committed, got %d: %v", commits, e.logs)
	}
	e.assertRepoFile("firewall/server.conf", "port 1194\n")
	for _, rel := range []string{"server.key", "pki", "rules.conf.bak", "client.conf"} {
		if _, err := os.Lstat(filepath.Join(e.repo, "firewall", rel)); !os.IsNotExist(err) {
			t.Errorf("%s must not reach the repository", rel)
		}
	}
	if !e.logged("Not backed up, it holds a private key: " + filepath.Join(e.source, "client.conf")) {
		t.Errorf("expected a warning about client.conf: %v", e.logs)
	}

	// Nothing excluded is pushed either
	if files := run(t, e.remote, "ls-tree", "-r", "--name-only", "main"); files != "firewall/server.conf" {
		t.Errorf("expected only server.conf on the remote, got %q", files)
	}
}

func TestExcludingRemovesAnEarlierBackup(t *testing.T) {
	e := newEnv(t)
	e.write("rules.conf", "rule 1\n")
	e.write("db.password", "hunter2\n")
	e.b.SyncAll()
	e.assertRepoFile("firewall/db.password", "hunter2\n")

	// The password was backed up before it was excluded
	e.b.opts.Watches[0].Exclude = []string{"*.password"}
	if commits := e.b.SyncAll(); commits != 1 {
		t.Fatalf("expected the removal to be committed, got %d: %v", commits, e.logs)
	}
	if _, err := os.Stat(filepath.Join(e.repo, "firewall", "db.password")); !os.IsNotExist(err) {
		t.Errorf("expected db.password to be removed from the repository")
	}
	if !strings.HasPrefix(e.subjects(e.repo)[0], "auto-backup: deleted db.password") {
		t.Errorf("expected a deletion commit, got %v", e.subjects(e.repo))
	}
	if !e.logged("Earlier commits and the remote still hold it") {
		t.Errorf("expected a warning that the history keeps it: %v", e.logs)
	}

	// The live file is never touched
	if got, _ := os.ReadFile(filepath.Join(e.source, "db.password")); string(got) != "hunter2\n" {
		t.Errorf("the watched file must be kept, got %q", got)
	}
}

func TestRestoreStagesExcludedFiles(t *testing.T) {
	e := newEnv(t)
	e.write("rules.conf", "rule 1\n")
	e.write("server.key", "secret\n")
	e.b.SyncAll()
	e.write("rules.conf", "rule 2\n")
	e.b.SyncAll()

	// The check needs the key, which is never in the repository
	check, _ := writeHook(t, "check", `test -f "$1/server.key" || { echo "missing key"; exit 1; }`)
	watches := []Watch{{Source: e.source, Dest: "firewall", Check: check}}
	preview, err := PreviewRestore(e.repo, watches, mustHistory(t, e)[1])
	if err != nil {
		t.Fatal(err)
	}
	if output, err := preview.Restore(); err != nil {
		t.Fatalf("expected the restore to pass the check, got %v: %s", err, output)
	}
}
