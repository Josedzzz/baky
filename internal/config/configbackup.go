package config

import (
	"fmt"
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
