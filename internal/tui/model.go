package tui

import (
	"github.com/Josedzzz/baky/internal/config"
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
}

// NewModel init and returns a new Model with default values
func NewModel() Model {
	ti := textinput.New()
	ti.Placeholder = "/path/to/backup"

	di := textinput.New()
	di.Placeholder = "/path/to/backup/destination"

	paths, _ := config.GetPaths()
	backupDest, _ := config.GetNasPath()
	history, _ := config.GetHistory()

	return Model{
		choices:         []string{"Manage Paths", "Backup Files", "Backup Destination", "Exit"},
		pathInput:       ti,
		backupDestInput: di,
		state:           menuView,
		editingIndex:    -1,
		paths:           paths,
		backupDest:      backupDest,
		history:         history,
	}
}
