package gitbackup

import (
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

// rejectBroken is a check that fails when reglas.conf contains "ROTA"
const rejectBroken = `if grep -q ROTA "$1/reglas.conf" 2>/dev/null; then echo "ERROR: regla invalida en reglas.conf"; exit 1; fi; echo "Configuration verified"`

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
	e.write("reglas.conf", "regla 1\n")
	e.write("otra.conf", "x\n")
	if commits := e.b.SyncAll(); commits != 2 {
		t.Fatalf("expected 2 commits, got %d: %v", commits, e.logs)
	}
	if got := calls(t, checkCalls); len(got) != 1 || got[0] != e.source {
		t.Errorf("expected one check on %s, got %v", e.source, got)
	}
	if !e.logged("Validación correcta") {
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
	e.write("reglas.conf", "regla ROTA\n")
	if commits := e.b.SyncAll(); commits != 1 {
		t.Fatalf("expected the broken change to be committed, got %d", commits)
	}
	e.assertRepoFile("firewall/reglas.conf", "regla ROTA\n")

	newest := mustHistory(t, e)[0]
	if !newest.Invalid || !strings.HasSuffix(newest.Message, "[validación fallida]") {
		t.Errorf("expected the commit to be marked invalid: %+v", newest)
	}
	if body := commitBody(e.repo, newest.Hash); !strings.Contains(body, "regla invalida en reglas.conf") {
		t.Errorf("expected the check output in the commit body, got %q", body)
	}
	if !e.logged("NO pasa la validación") {
		t.Errorf("expected the failed validation in the log: %v", e.logs)
	}

	// Fixing it produces a normal commit again
	e.write("reglas.conf", "regla 2\n")
	e.b.SyncAll()
	if newest := mustHistory(t, e)[0]; newest.Invalid {
		t.Errorf("expected the fix to be a valid commit: %+v", newest)
	}
}

func TestCheckThatCannotRunDoesNotMarkCommits(t *testing.T) {
	e := newEnv(t)
	e.withHooks("/no/existe/validador {dir}", "")

	e.write("reglas.conf", "regla 1\n")
	if commits := e.b.SyncAll(); commits != 1 {
		t.Fatalf("expected the commit, got %d", commits)
	}
	if mustHistory(t, e)[0].Invalid {
		t.Errorf("a check that could not run says nothing about the configuration")
	}
	if !e.logged("No se pudo validar") {
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
	live := filepath.Join(e.source, "reglas.conf")

	// History: a good version, a broken one, and the current good one
	e.write("otra.conf", "x\n")
	e.write("reglas.conf", "regla 1\n")
	e.b.SyncAll()
	e.write("reglas.conf", "regla ROTA\n")
	e.b.SyncAll()
	e.write("reglas.conf", "regla 3\n")
	e.b.SyncAll()

	// The apply command records the content it was asked to put into service
	check, checkCalls := writeHook(t, "check", rejectBroken)
	apply, applyCalls := writeHook(t, "apply", `cat "$1/reglas.conf" "$1/otra.conf" > `+filepath.Join(t.TempDir(), "applied")+`; echo "Shorewall restarted"`)
	watches := []Watch{{Source: e.source, Dest: "firewall", Check: check, Apply: apply}}

	var good, broken Commit
	for _, c := range mustHistory(t, e) {
		content, _, _ := FileAt(e.repo, c.Hash, "firewall/reglas.conf")
		switch string(content) {
		case "regla 1\n":
			good = c
		case "regla ROTA\n":
			broken = c
		}
	}

	// The broken version is rejected by the check: nothing is applied or written
	preview, err := PreviewRestore(e.repo, watches, broken)
	if err != nil {
		t.Fatal(err)
	}
	output, err := preview.Restore()
	if err == nil || !strings.Contains(err.Error(), "no pasa la validación") {
		t.Fatalf("expected the restore to be rejected, got %v", err)
	}
	if !strings.Contains(output, "regla invalida") {
		t.Errorf("expected the check output, got %q", output)
	}
	if got, _ := os.ReadFile(live); string(got) != "regla 3\n" {
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
	if got, _ := os.ReadFile(live); string(got) != "regla 1\n" {
		t.Errorf("expected the good version to be restored, got %q", got)
	}
	if len(calls(t, applyCalls)) != 1 {
		t.Errorf("expected apply to run once, got %v", calls(t, applyCalls))
	}
}

func TestRestoreIsAbortedWhenApplyFails(t *testing.T) {
	e := newEnv(t)
	live := filepath.Join(e.source, "reglas.conf")
	e.write("reglas.conf", "regla 1\n")
	e.b.SyncAll()
	e.write("reglas.conf", "regla 2\n")
	e.b.SyncAll()

	apply, _ := writeHook(t, "apply", `echo "ERROR: no se pudo aplicar"; exit 1`)
	watches := []Watch{{Source: e.source, Dest: "firewall", Apply: apply}}

	preview, err := PreviewRestore(e.repo, watches, mustHistory(t, e)[1])
	if err != nil {
		t.Fatal(err)
	}
	if _, err := preview.Restore(); err == nil || !strings.Contains(err.Error(), "no se pudo aplicar") {
		t.Fatalf("expected the restore to fail, got %v", err)
	}
	if got, _ := os.ReadFile(live); string(got) != "regla 2\n" {
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
	sudo = func(args ...string) (string, error) {
		got = args
		// The file to install is the argument before the destination
		content, _ := os.ReadFile(args[len(args)-2])
		handed = string(content)
		return "", nil
	}
	t.Cleanup(func() { sudo = original })

	if err := installFile(path, []byte("new")); err != nil {
		t.Fatal(err)
	}

	joined := strings.Join(got, " ")
	for _, want := range []string{"-n install -D -m 0640", " -o ", " -g ", " " + path} {
		if !strings.Contains(joined, want) {
			t.Errorf("expected %q in the sudo command, got %q", want, joined)
		}
	}
	if handed != "new" {
		t.Errorf("expected the new content to be handed to sudo, got %q", handed)
	}
	if _, err := os.Stat(got[len(got)-2]); !os.IsNotExist(err) {
		t.Errorf("expected the temporary copy to be removed")
	}
}
