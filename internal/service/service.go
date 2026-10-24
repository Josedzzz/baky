// Package service controls the systemd service that runs the baky daemon
package service

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// DefaultName is the systemd unit installed from deploy/baky.service
const DefaultName = "baky.service"

// EnvName is the environment variable that overrides the unit name
const EnvName = "BAKY_SERVICE"

// Name returns the systemd unit that runs the daemon
func Name() string {
	if name := os.Getenv(EnvName); name != "" {
		return name
	}
	return DefaultName
}

// State describes the current situation of the service
type State struct {
	Active bool
	Status string // "active", "inactive", "failed"... as reported by systemd
	Detail string // Full output of "systemctl status"
	Err    error  // Set when the state could not be queried at all
}

// Query asks systemd for the state of the service. Reading the state
// needs no privileges, so it never goes through sudo.
func Query() State {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return State{Status: "unknown", Err: fmt.Errorf("systemctl is not available on this system")}
	}

	// is-active exits with an error code when the service is not active;
	// the answer is still in its output.
	out, _ := exec.Command("systemctl", "is-active", Name()).Output()
	status := strings.TrimSpace(string(out))
	if status == "" {
		status = "unknown"
	}

	detail, _ := exec.Command("systemctl", "status", Name(), "--no-pager", "-l").CombinedOutput()

	return State{
		Active: status == "active",
		Status: status,
		Detail: strings.TrimSpace(string(detail)),
	}
}

// Start starts the service
func Start() error {
	return runAction("start")
}

// Stop stops the service
func Stop() error {
	return runAction("stop")
}

// Restart restarts the service
func Restart() error {
	return runAction("restart")
}

// runAction changes the state of the service. It needs privileges, so it
// runs through sudo without a password prompt; deploy/baky.sudoers
// grants exactly these commands.
func runAction(action string) error {
	out, err := exec.Command("sudo", "-n", "systemctl", action, Name()).CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(out))
		if detail == "" {
			detail = err.Error()
		}
		return fmt.Errorf("could not run '%s' on %s: %s", action, Name(), detail)
	}
	return nil
}
