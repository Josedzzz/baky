package config

import (
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// DefaultBranch is the branch config backups are pushed to when none is set
const DefaultBranch = "main"

// WatchConfig maps a watched directory to a folder inside the config repository
type WatchConfig struct {
	Source string `json:"source"` // Absolute path of the directory to watch
	Dest   string `json:"dest"`   // Folder inside the repository that receives its files

	// Check is an optional command that validates the configuration in
	// {dir}. It runs after every change and before a restore.
	Check string `json:"check,omitempty"`
	// Apply is an optional command that puts the configuration in {dir}
	// into service safely. It runs when a file is restored.
	Apply string `json:"apply,omitempty"`
	// Exclude lists extra patterns of files and folders that are not
	// backed up, on top of the built-in list of secrets
	Exclude []string `json:"exclude,omitempty"`
}

// DirPlaceholder is replaced in the check and apply commands by the
// directory that holds the configuration
const DirPlaceholder = "{dir}"

// ValidateWatchCommand checks a check or apply command. An empty command
// is valid and means the step is skipped.
func ValidateWatchCommand(command string) error {
	if strings.TrimSpace(command) == "" {
		return nil
	}
	if !strings.Contains(command, DirPlaceholder) {
		return fmt.Errorf("the command must include %s, which is replaced by the folder of the configuration", DirPlaceholder)
	}
	return nil
}

// SetWatchCommands sets the validation and apply commands of a watch
func SetWatchCommands(source, check, apply string) error {
	check, apply = strings.TrimSpace(check), strings.TrimSpace(apply)
	for _, command := range []string{check, apply} {
		if err := ValidateWatchCommand(command); err != nil {
			return err
		}
	}

	return update(func(cfg *Config) error {
		for i, w := range cfg.ConfigBackup.Watches {
			if w.Source == source {
				cfg.ConfigBackup.Watches[i].Check = check
				cfg.ConfigBackup.Watches[i].Apply = apply
				return nil
			}
		}
		return fmt.Errorf("the folder '%s' is not being watched", source)
	})
}

// ValidateExclude checks an exclude pattern. It uses the syntax of
// path.Match and is relative to the watched folder: "*.key" matches a name
// at any depth, "pki/" only folders, "certs/server.crt" that exact path.
func ValidateExclude(pattern string) error {
	glob := strings.TrimSuffix(pattern, "/")
	switch {
	case strings.TrimSpace(glob) == "":
		return fmt.Errorf("the pattern cannot be empty")
	case strings.HasPrefix(pattern, "/"):
		return fmt.Errorf("the pattern must be relative to the watched folder: %s", pattern)
	case strings.Contains(pattern, `\`):
		return fmt.Errorf("the pattern must use / as separator: %s", pattern)
	}
	for _, part := range strings.Split(glob, "/") {
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("the pattern must not contain empty, . or .. parts: %s", pattern)
		}
	}
	if _, err := path.Match(glob, ""); err != nil {
		return fmt.Errorf("invalid pattern %s: %w", pattern, err)
	}
	return nil
}

// SetWatchExcludes sets the extra exclude patterns of a watch. Blank
// patterns and repeats are dropped.
func SetWatchExcludes(source string, patterns []string) error {
	var clean []string
	seen := map[string]bool{}
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			continue
		}
		if err := ValidateExclude(p); err != nil {
			return err
		}
		seen[p] = true
		clean = append(clean, p)
	}

	return update(func(cfg *Config) error {
		for i, w := range cfg.ConfigBackup.Watches {
			if w.Source == source {
				cfg.ConfigBackup.Watches[i].Exclude = clean
				return nil
			}
		}
		return fmt.Errorf("the folder '%s' is not being watched", source)
	})
}

// ConfigBackup holds the settings of the versioned backup of configuration
// files: every change under a watched directory is committed to a Git repository
type ConfigBackup struct {
	RepoDir string        `json:"repo_dir"`
	Branch  string        `json:"branch"`
	Watches []WatchConfig `json:"watches"`
}

// destNamePattern keeps destination folders to one safe path element
var destNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,49}$`)

// ValidateDestName checks that name can be used as a folder inside the
// repository: one path element, no separators, nothing that escapes the repo
func ValidateDestName(name string) error {
	if name == "" {
		return fmt.Errorf("the name cannot be empty")
	}
	if name == ".git" || !destNamePattern.MatchString(name) {
		return fmt.Errorf("only letters, numbers, '.', '-' and '_' are allowed (max. 50 characters, must start with a letter or number)")
	}
	return nil
}

// GetConfigBackup returns the config backup settings
func GetConfigBackup() (ConfigBackup, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return ConfigBackup{}, err
	}
	cb := cfg.ConfigBackup
	if cb.Branch == "" {
		cb.Branch = DefaultBranch
	}
	return cb, nil
}

// SaveConfigRepo sets the repository that receives the config backups
func SaveConfigRepo(repoDir string) error {
	if !filepath.IsAbs(repoDir) {
		return fmt.Errorf("the repository path must be absolute: %s", repoDir)
	}
	return update(func(cfg *Config) error {
		cfg.ConfigBackup.RepoDir = filepath.Clean(repoDir)
		if cfg.ConfigBackup.Branch == "" {
			cfg.ConfigBackup.Branch = DefaultBranch
		}
		return nil
	})
}

// AddWatch starts watching source and maps it to the dest folder of the repository
func AddWatch(source, dest string) error {
	if !filepath.IsAbs(source) {
		return fmt.Errorf("the path to watch must be absolute: %s", source)
	}
	source = filepath.Clean(source)
	if err := ValidateDestName(dest); err != nil {
		return err
	}

	return update(func(cfg *Config) error {
		for _, w := range cfg.ConfigBackup.Watches {
			if w.Source == source {
				return fmt.Errorf("the folder '%s' is already being watched", source)
			}
			if w.Dest == dest {
				return fmt.Errorf("the destination '%s' is already used by '%s'", dest, w.Source)
			}
		}
		cfg.ConfigBackup.Watches = append(cfg.ConfigBackup.Watches, WatchConfig{Source: source, Dest: dest})
		return nil
	})
}

// RemoveWatch stops watching source. Files already in the repository are kept.
func RemoveWatch(source string) error {
	return update(func(cfg *Config) error {
		for i, w := range cfg.ConfigBackup.Watches {
			if w.Source == source {
				cfg.ConfigBackup.Watches = append(cfg.ConfigBackup.Watches[:i], cfg.ConfigBackup.Watches[i+1:]...)
				return nil
			}
		}
		return fmt.Errorf("the folder '%s' is not being watched", source)
	})
}
