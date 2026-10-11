package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// magitiConfig is the config.json of the magiti-config-backup daemon,
// the Bash predecessor of the config backup
type magitiConfig struct {
	WatchDir string            `json:"watch_dir"`
	RepoDir  string            `json:"repo_dir"`
	Carpetas map[string]string `json:"carpetas"`
}

// ImportMagitiConfig adds the folders watched by a magiti-config-backup
// daemon to the configuration. The old model watched subfolders of one
// directory; each one becomes a watch with its absolute path.
//
// The repository is not imported: the old one mixed the daemon's code
// with the backups, and the configurations now go to a dedicated one.
func ImportMagitiConfig(path string) ([]WatchConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var old magitiConfig
	if err := json.Unmarshal(data, &old); err != nil {
		return nil, fmt.Errorf("%s no contiene JSON válido: %w", path, err)
	}
	if !filepath.IsAbs(old.WatchDir) {
		return nil, fmt.Errorf("'watch_dir' debe ser una ruta absoluta: %q", old.WatchDir)
	}

	// Sorted, so the result does not depend on the order of the JSON map
	folders := make([]string, 0, len(old.Carpetas))
	for folder := range old.Carpetas {
		folders = append(folders, folder)
	}
	sort.Strings(folders)

	var added []WatchConfig
	for _, folder := range folders {
		watch := WatchConfig{
			Source: filepath.Join(old.WatchDir, folder),
			Dest:   old.Carpetas[folder],
		}
		if err := AddWatch(watch.Source, watch.Dest); err != nil {
			return added, fmt.Errorf("%s: %w", folder, err)
		}
		added = append(added, watch)
	}
	return added, nil
}
