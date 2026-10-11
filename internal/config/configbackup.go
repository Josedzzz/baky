package config

// DefaultBranch is the branch config backups are pushed to when none is set
const DefaultBranch = "main"

// WatchConfig maps a watched directory to a folder inside the config repository
type WatchConfig struct {
	Source string `json:"source"` // Absolute path of the directory to watch
	Dest   string `json:"dest"`   // Folder inside the repository that receives its files
}

// ConfigBackup holds the settings of the versioned backup of configuration
// files: every change under a watched directory is committed to a Git repository
type ConfigBackup struct {
	RepoDir string        `json:"repo_dir"`
	Branch  string        `json:"branch"`
	Watches []WatchConfig `json:"watches"`
}
