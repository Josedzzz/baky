package tui

import (
	"fmt"

	"github.com/Josedzzz/baky/internal/config"
	"github.com/Josedzzz/baky/internal/restore"
	"github.com/charmbracelet/bubbles/textinput"
)

// sessionState represents the different states of the TUI
type sessionState int

const (
	// menuView is the state where the user interacts with the main menu
	menuView sessionState = iota

	// managePathsView is the unified state where the user manages backup paths
	managePathsView

	// configureBackupDestView is the state where the user configures the backup destination path
	configureBackupDestView

	// backupFilesView is the state where the user runs manual backups
	backupFilesView

	// inputView is a sub-state for when the user is typing a path
	inputView

	// backupDestInputView is a sub-state for when the user is typing a backup destination path
	backupDestInputView

	// viewBackupsView is the state where the user views available backups
	viewBackupsView

	// selectRestoreDestView is the state where the user selects restore destination
	selectRestoreDestView

	// selectRestoreActionView is the state where the user selects action for conflicts
	selectRestoreActionView

	// restoreInputView is a sub-state for when the user is typing a restore path
	restoreInputView
)

// Model represents the state of the TUI
type Model struct {
	choices         []string
	cursor          int
	quitting        bool
	state           sessionState
	pathInput       textinput.Model
	backupDestInput textinput.Model
	paths           []config.BackupPathConfig
	pathsCursor     int // cursor for the paths list
	editingIndex    int // index of the path being edited (-1 for new)
	backupDest      string
	history         []config.BackupEvent
	historyOffset   int // for scrolling history
	isProcessing    bool
	message         string
	isSuccess       bool
	width           int
	height          int
	// Backup viewing fields
	allBackups    []restore.BackupInfo // All scanned backups
	backupsCursor int                  // cursor for backup list
	backupsOffset int                  // for scrolling backup list

	// Restore operation fields
	selectedBackup     *restore.BackupInfo // Currently selected backup for restore
	restoreInput       textinput.Model     // Input for restore path
	restorePath        string              // User-specified restore path
	restoreAction      restore.RestoreAction
	restoreActionIndex int // For selecting action (overwrite/rename/skip)
	isRestoring        bool
}

// NewModel init and returns a new Model with default values
func NewModel() Model {
	ti := textinput.New()
	ti.Placeholder = "/path/to/backup"

	di := textinput.New()
	di.Placeholder = "/path/to/backup/destination"

	ri := textinput.New()
	ri.Placeholder = "/path/to/restore/to"

	paths, pathErr := config.GetPaths()
	if pathErr != nil {
		fmt.Printf("Warning: Could not load backup paths: %v\n", pathErr)
		paths = []config.BackupPathConfig{}
	}

	backupDest, destErr := config.GetNasPath()
	if destErr != nil {
		fmt.Printf("Warning: Could not load NAS path: %v\n", destErr)
		backupDest = ""
	}

	history, histErr := config.GetHistory()
	if histErr != nil {
		fmt.Printf("Warning: Could not load backup history: %v\n", histErr)
		history = []config.BackupEvent{}
	}

	return Model{
		choices:         []string{"Manage Paths", "Backup Files", "View Backups", "Backup Destination", "Exit"},
		pathInput:       ti,
		backupDestInput: di,
		restoreInput:    ri,
		state:           menuView,
		editingIndex:    -1,
		paths:           paths,
		backupDest:      backupDest,
		history:         history,
		allBackups:      []restore.BackupInfo{},
	}
}
