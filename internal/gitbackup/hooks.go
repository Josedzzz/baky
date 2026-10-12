package gitbackup

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const (
	// dirPlaceholder is replaced in a hook command by the directory that
	// holds the configuration to validate or apply
	dirPlaceholder = "{dir}"

	// invalidMarker ends the message of a commit whose configuration did
	// not pass the validation command of its watch
	invalidMarker = "[validación fallida]"

	// hookTimeout bounds a validation or apply command
	hookTimeout = 5 * time.Minute
)

// ValidateHook checks that a hook command can be used: it must name the
// directory it works on. An empty command means "no hook" and is valid.
func ValidateHook(command string) error {
	if strings.TrimSpace(command) == "" {
		return nil
	}
	if !strings.Contains(command, dirPlaceholder) {
		return fmt.Errorf("el comando debe incluir %s, que se reemplaza por la carpeta de la configuración", dirPlaceholder)
	}
	return nil
}

// errHookNotRun wraps the errors of a hook that could not be started at
// all, as opposed to one that ran and reported a failure
var errHookNotRun = errors.New("no se pudo ejecutar el comando")

// runHook runs a hook command on a directory and returns what it printed.
// The command is split on spaces and executed directly, without a shell,
// exactly as written: a command that needs privileges must say so itself
// ("sudo -n shorewall check {dir}").
var runHook = func(command, dir string) (string, error) {
	args := strings.Fields(command)
	if len(args) == 0 {
		return "", nil
	}
	for i := range args {
		args[i] = strings.ReplaceAll(args[i], dirPlaceholder, dir)
	}

	ctx, cancel := context.WithTimeout(context.Background(), hookTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, args[0], args[1:]...).CombinedOutput()
	output := strings.TrimSpace(string(out))

	var exitErr *exec.ExitError
	switch {
	case err == nil:
		return output, nil
	case errors.As(err, &exitErr):
		// The command ran and rejected the configuration
		return output, fmt.Errorf("%s terminó con error (código %d)", args[0], exitErr.ExitCode())
	default:
		return output, fmt.Errorf("%w %q: %v", errHookNotRun, args[0], err)
	}
}

// lastLines keeps the end of a command output, where the error usually is
func lastLines(output string, n int) string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
