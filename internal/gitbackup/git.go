package gitbackup

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// gitTimeout bounds every git command, so a hung network operation
// cannot block the daemon forever
const gitTimeout = 2 * time.Minute

// git runs a git command inside repoDir and returns its standard output
func git(repoDir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", repoDir}, args...)...)
	// Never wait for a password or passphrase: there is nobody to type it
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", args[0], detail)
	}
	return stdout.String(), nil
}

// IsRepo reports whether dir is the root of a Git working tree
func IsRepo(dir string) bool {
	out, err := git(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return false
	}
	top, err1 := evalPath(strings.TrimSpace(out))
	want, err2 := evalPath(dir)
	return err1 == nil && err2 == nil && top == want
}

// change is one pending change inside the repository working tree
type change struct {
	path    string // relative to the repository root
	deleted bool
}

// pendingChanges lists the files under dir that differ from the last commit
func pendingChanges(repoDir, dir string) ([]change, error) {
	out, err := git(repoDir, "status", "--porcelain", "-z", "--untracked-files=all", "--", dir)
	if err != nil {
		return nil, err
	}

	var changes []change
	for _, entry := range strings.Split(out, "\x00") {
		// Each entry is "XY path", where XY is the two-letter status
		if len(entry) < 4 {
			continue
		}
		status := entry[:2]
		changes = append(changes, change{
			path:    entry[3:],
			deleted: strings.Contains(status, "D"),
		})
	}
	return changes, nil
}

// commitPath commits the current state of one path, and nothing else.
// body is optional extra text for the commit message.
func commitPath(repoDir, path, message, body string) error {
	if _, err := git(repoDir, "add", "-A", "--", path); err != nil {
		return err
	}
	args := append(identityArgs(repoDir), "commit", "-m", message)
	if body != "" {
		args = append(args, "-m", body)
	}
	args = append(args, "--", path)
	_, err := git(repoDir, args...)
	return err
}

// commitBody returns the text below the first line of a commit message
func commitBody(repoDir, hash string) string {
	out, err := git(repoDir, "show", "-s", "--format=%b", hash)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// identityArgs returns the options that give the commit an author when
// the user running the daemon has none configured in Git, which is the
// usual case for a system service
func identityArgs(repoDir string) []string {
	if email, err := git(repoDir, "config", "user.email"); err == nil && strings.TrimSpace(email) != "" {
		return nil
	}
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "localhost"
	}
	return []string{"-c", "user.name=baky", "-c", "user.email=baky@" + host}
}

// hasRemote reports whether the repository has an "origin" remote to push to
func hasRemote(repoDir string) bool {
	_, err := git(repoDir, "remote", "get-url", "origin")
	return err == nil
}

// hasUnpushed reports whether the local branch has commits the remote
// branch does not have yet
func hasUnpushed(repoDir, branch string) bool {
	if _, err := git(repoDir, "rev-parse", "--verify", "--quiet", "HEAD"); err != nil {
		return false // no commits at all yet
	}
	out, err := git(repoDir, "rev-list", "--count", "refs/remotes/origin/"+branch+"..HEAD")
	if err != nil {
		return true // the remote branch is not known yet: it was never pushed
	}
	return strings.TrimSpace(out) != "0"
}

// push sends the local commits to the given branch of the remote
func push(repoDir, branch string) error {
	_, err := git(repoDir, "push", "origin", "HEAD:"+branch)
	return err
}

// Commit is one entry of the backup history
type Commit struct {
	Hash    string
	Date    string
	Message string
	Path    string // The file the commit changed, relative to the repository
	// Invalid is true when the configuration did not pass the validation
	// command of its watch at the time of the commit
	Invalid bool
}

const (
	commitSeparator = "|||"
	// commitMarker starts each commit in the output of "git log", where
	// the file names follow the commit line
	commitMarker = "\x01"
)

// History returns the last n automatic backups recorded in the repository,
// newest first
func History(repoDir string, n int) ([]Commit, error) {
	format := commitMarker + "%h" + commitSeparator + "%ad" + commitSeparator + "%s"
	out, err := git(repoDir,
		"-c", "core.quotePath=false",
		"log",
		"-n", strconv.Itoa(n),
		"--grep=^"+commitPrefix,
		"--date=format:%Y-%m-%d %H:%M:%S",
		"--name-only",
		"--pretty=format:"+format,
	)
	if err != nil {
		// A repository without commits yet has no history, which is not an error
		if strings.Contains(err.Error(), "does not have any commits yet") {
			return nil, nil
		}
		return nil, err
	}

	var commits []Commit
	for _, entry := range strings.Split(out, commitMarker) {
		lines := strings.Split(strings.TrimSpace(entry), "\n")
		parts := strings.SplitN(lines[0], commitSeparator, 3)
		if len(parts) != 3 {
			continue
		}
		commit := Commit{Hash: parts[0], Date: parts[1], Message: parts[2]}
		commit.Invalid = strings.HasSuffix(commit.Message, invalidMarker)
		// Each automatic commit changes exactly one file
		for _, line := range lines[1:] {
			if line = strings.TrimSpace(line); line != "" {
				commit.Path = line
				break
			}
		}
		commits = append(commits, commit)
	}
	return commits, nil
}
