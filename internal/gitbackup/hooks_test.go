package gitbackup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeHook creates a script to use as a check or apply command. It
// receives the directory as its first argument and appends it to a file,
// so tests can see when and on what it ran.
func writeHook(t *testing.T, name, body string) (command, callsFile string) {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, name+".sh")
	callsFile = filepath.Join(dir, name+".calls")
	content := "#!/bin/sh\necho \"$1\" >> " + callsFile + "\n" + body + "\n"
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	return "sh " + script + " {dir}", callsFile
}

// rejectBroken is a check that fails when rules.conf contains "BROKEN"
const rejectBroken = `if grep -q BROKEN "$1/rules.conf" 2>/dev/null; then echo "ERROR: invalid rule in rules.conf"; exit 1; fi; echo "Configuration verified"`

func (e *env) withHooks(check, apply string) {
	e.b.opts.Watches[0].Check = check
	e.b.opts.Watches[0].Apply = apply
}

func calls(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

func TestValidateHook(t *testing.T) {
	for command, valid := range map[string]bool{
		"":                              true,
		"  ":                            true,
		"sudo -n shorewall check {dir}": true,
		"shorewall check":               false,
	} {
		if err := ValidateHook(command); (err == nil) != valid {
			t.Errorf("%q: expected valid=%v, got %v", command, valid, err)
		}
	}
}

func TestCheckRunsAfterChangesAndMarksInvalidCommits(t *testing.T) {
	e := newEnv(t)
	check, checkCalls := writeHook(t, "check", rejectBroken)
	e.withHooks(check, "")

	// A valid change: normal commit, the check ran once on the watched directory
	e.write("rules.conf", "rule 1\n")
	e.write("other.conf", "x\n")
	if commits := e.b.SyncAll(); commits != 2 {
		t.Fatalf("expected 2 commits, got %d: %v", commits, e.logs)
	}
	if got := calls(t, checkCalls); len(got) != 1 || got[0] != e.source {
		t.Errorf("expected one check on %s, got %v", e.source, got)
	}
	if !e.logged("Validation passed") {
		t.Errorf("expected the successful validation in the log: %v", e.logs)
	}
	for _, c := range mustHistory(t, e) {
		if c.Invalid {
			t.Errorf("unexpected invalid commit: %+v", c)
		}
	}

	// Nothing changed: the check does not run again
	e.b.SyncAll()
	if got := calls(t, checkCalls); len(got) != 1 {
		t.Errorf("expected no check without changes, got %v", got)
	}

	// A broken change is still backed up, but marked
	e.write("rules.conf", "rule BROKEN\n")
	if commits := e.b.SyncAll(); commits != 1 {
		t.Fatalf("expected the broken change to be committed, got %d", commits)
	}
	e.assertRepoFile("firewall/rules.conf", "rule BROKEN\n")

	newest := mustHistory(t, e)[0]
	if !newest.Invalid || !strings.HasSuffix(newest.Message, "[validation failed]") {
		t.Errorf("expected the commit to be marked invalid: %+v", newest)
	}
	if body := commitBody(e.repo, newest.Hash); !strings.Contains(body, "invalid rule in rules.conf") {
		t.Errorf("expected the check output in the commit body, got %q", body)
	}
	if !e.logged("does NOT pass validation") {
		t.Errorf("expected the failed validation in the log: %v", e.logs)
	}

	// Fixing it produces a normal commit again
	e.write("rules.conf", "rule 2\n")
	e.b.SyncAll()
	if newest := mustHistory(t, e)[0]; newest.Invalid {
		t.Errorf("expected the fix to be a valid commit: %+v", newest)
	}
}

func TestCheckThatCannotRunDoesNotMarkCommits(t *testing.T) {
	e := newEnv(t)
	e.withHooks("/does/not/exist/validator {dir}", "")

	e.write("rules.conf", "rule 1\n")
	if commits := e.b.SyncAll(); commits != 1 {
		t.Fatalf("expected the commit, got %d", commits)
	}
	if mustHistory(t, e)[0].Invalid {
		t.Errorf("a check that could not run says nothing about the configuration")
	}
	if !e.logged("Could not validate") {
		t.Errorf("expected the problem in the log: %v", e.logs)
	}
}

func mustHistory(t *testing.T, e *env) []Commit {
	t.Helper()
	commits, err := History(e.repo, 50)
	if err != nil {
		t.Fatal(err)
	}
	return commits
}

func TestRestoreValidatesAndAppliesBeforeWriting(t *testing.T) {
	e := newEnv(t)
	live := filepath.Join(e.source, "rules.conf")

	// History: a good version, a broken one, and the current good one
	e.write("other.conf", "x\n")
	e.write("rules.conf", "rule 1\n")
	e.b.SyncAll()
	e.write("rules.conf", "rule BROKEN\n")
	e.b.SyncAll()
	e.write("rules.conf", "rule 3\n")
	e.b.SyncAll()

	// The apply command records the content it was asked to put into service
	check, checkCalls := writeHook(t, "check", rejectBroken)
	apply, applyCalls := writeHook(t, "apply", `cat "$1/rules.conf" "$1/other.conf" > `+filepath.Join(t.TempDir(), "applied")+`; echo "Shorewall restarted"`)
	watches := []Watch{{Source: e.source, Dest: "firewall", Check: check, Apply: apply}}

	var good, broken Commit
	for _, c := range mustHistory(t, e) {
		content, _, _ := FileAt(e.repo, c.Hash, "firewall/rules.conf")
		switch string(content) {
		case "rule 1\n":
			good = c
		case "rule BROKEN\n":
			broken = c
		}
	}

	// The broken version is rejected by the check: nothing is applied or written
	preview, err := PreviewRestore(e.repo, watches, broken)
	if err != nil {
		t.Fatal(err)
	}
	output, err := preview.Restore()
	if err == nil || !strings.Contains(err.Error(), "does not pass validation") {
		t.Fatalf("expected the restore to be rejected, got %v", err)
	}
	if !strings.Contains(output, "invalid rule") {
		t.Errorf("expected the check output, got %q", output)
	}
	if got, _ := os.ReadFile(live); string(got) != "rule 3\n" {
		t.Errorf("a rejected restore must not touch the file, got %q", got)
	}
	if len(calls(t, applyCalls)) != 0 {
		t.Errorf("apply must not run after a failed check")
	}

	// The check ran on a temporary copy, never on the live directory
	staged := calls(t, checkCalls)[0]
	if staged == e.source {
		t.Errorf("the restore must be validated on a staged copy")
	}
	if _, err := os.Stat(staged); !os.IsNotExist(err) {
		t.Errorf("expected the staged copy to be removed")
	}

	// The good version passes: check, then apply, then the file is written
	preview, err = PreviewRestore(e.repo, watches, good)
	if err != nil {
		t.Fatal(err)
	}
	output, err = preview.Restore()
	if err != nil {
		t.Fatalf("Restore failed: %v\n%s", err, output)
	}
	if !strings.Contains(output, "Configuration verified") || !strings.Contains(output, "Shorewall restarted") {
		t.Errorf("expected the output of both commands, got %q", output)
	}
	if got, _ := os.ReadFile(live); string(got) != "rule 1\n" {
		t.Errorf("expected the good version to be restored, got %q", got)
	}
	if len(calls(t, applyCalls)) != 1 {
		t.Errorf("expected apply to run once, got %v", calls(t, applyCalls))
	}
}

func TestRestoreIsAbortedWhenApplyFails(t *testing.T) {
	e := newEnv(t)
	live := filepath.Join(e.source, "rules.conf")
	e.write("rules.conf", "rule 1\n")
	e.b.SyncAll()
	e.write("rules.conf", "rule 2\n")
	e.b.SyncAll()

	apply, _ := writeHook(t, "apply", `echo "ERROR: could not apply"; exit 1`)
	watches := []Watch{{Source: e.source, Dest: "firewall", Apply: apply}}

	preview, err := PreviewRestore(e.repo, watches, mustHistory(t, e)[1])
	if err != nil {
		t.Fatal(err)
	}
	if _, err := preview.Restore(); err == nil || !strings.Contains(err.Error(), "could not apply") {
		t.Fatalf("expected the restore to fail, got %v", err)
	}
	if got, _ := os.ReadFile(live); string(got) != "rule 2\n" {
		t.Errorf("a failed apply must not touch the file, got %q", got)
	}
}

func TestInstallFileFallsBackToSudo(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can write anywhere")
	}

	dir := filepath.Join(t.TempDir(), "etc")
	os.MkdirAll(dir, 0o755)
	path := filepath.Join(dir, "rules")
	os.WriteFile(path, []byte("old"), 0o640)
	// Like /etc/shorewall for a normal user: readable, not writable
	os.Chmod(dir, 0o555)
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	var got []string
	handed := ""
	original := sudo
	sudo = func(stdin []byte, args ...string) (string, error) {
		got = args
		handed = string(stdin)
		return "", nil
	}
	t.Cleanup(func() { sudo = original })

	if err := installFile(path, []byte("new")); err != nil {
		t.Fatal(err)
	}

	// Only the helper, and only the destination: never a file to read
	if want := []string{"-n", installHelper, path}; strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("expected the sudo command %q, got %q", want, got)
	}
	if handed != "new" {
		t.Errorf("expected the new content on the standard input, got %q", handed)
	}

	sudo = func(stdin []byte, args ...string) (string, error) {
		return "sudo: a password is required", fmt.Errorf("exit status 1")
	}
	if err := installFile(path, []byte("new")); err == nil || !strings.Contains(err.Error(), "a password is required") {
		t.Errorf("expected the sudo error to be reported, got %v", err)
	}
}
