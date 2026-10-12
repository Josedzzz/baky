package tui

import (
	"github.com/Josedzzz/baky/internal/config"
	"github.com/Josedzzz/baky/internal/gitbackup"
	"github.com/Josedzzz/baky/internal/restore"
	"github.com/Josedzzz/baky/internal/service"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
)

// sessionState represents the different states of the TUI
type sessionState int

const (
	// menuView is the state where the user interacts with the main menu
	menuView sessionState = iota

	// filesMenuView is the submenu of the file backups (tar.gz to the backup destination)
	filesMenuView

	// configMenuView is the submenu of the versioned backup of configuration files
	configMenuView

	// serviceMenuView is the submenu of the daemon service
	serviceMenuView

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

	// restorePreviewView lists what restoring the backup would change
	restorePreviewView

	// restoreFileDiffView compares one file of the backup with the current one
	restoreFileDiffView

	// restoreInputView is a sub-state for when the user is typing a restore path
	restoreInputView

	// watchesView is the state where the user manages the watched config folders
	watchesView

	// watchSourceInputView is a sub-state for typing the folder to watch
	watchSourceInputView

	// watchDestInputView is a sub-state for typing the folder name inside the repository
	watchDestInputView

	// watchConfirmDeleteView asks for confirmation before a folder stops being watched
	watchConfirmDeleteView

	// watchCheckInputView is a sub-state for typing the validation command of a watch
	watchCheckInputView

	// watchApplyInputView is a sub-state for typing the apply command of a watch
	watchApplyInputView

	// configRepoView is the state where the user configures the config repository
	configRepoView

	// configRepoInputView is a sub-state for typing the config repository path
	configRepoInputView

	// configHistoryView shows the config backups already made (commits)
	configHistoryView

	// configDiffView compares the current file with its version in a commit
	configDiffView

	// configRestoreConfirmView asks for confirmation before a file is restored
	configRestoreConfirmView

	// serviceStatusView shows the daemon service and lets the user start or stop it
	serviceStatusView

	// logsView shows the daemon log, refreshed automatically
	logsView

	// helpView lists the keyboard shortcuts
	helpView
)

// Model represents the state of the TUI
type Model struct {
	cursor          int
	menuCursors     map[sessionState]int // remembers the selected entry of each menu
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

	// Restore preview fields
	restorePreview        *restore.Preview // what the restore would change
	restorePreviewErr     string
	restorePreviewLoading bool
	previewCursor         int // cursor for the list of files
	previewOffset         int // for scrolling the list of files
	fileDiffErr           string
	fileDiffLoading       bool
	fileDiffEmpty         bool // the comparison found no differences to show

	// Config backup fields
	configBackup  config.ConfigBackup
	repoValid     bool            // whether the configured repository is a Git repository
	watchesCursor int             // cursor for the watched folders list
	watchInput    textinput.Model // Input for the folder to watch and its destination name
	repoInput     textinput.Model // Input for the config repository path
	pendingSource string          // folder typed in the first step of adding a watch
	pendingCheck  string          // validation command typed before the apply command
	commits       []gitbackup.Commit
	commitsErr    string
	commitsCursor int // cursor for the history list
	commitsOffset int // for scrolling the history list

	// Restore of a config file from the history
	selectedCommit  gitbackup.Commit   // commit chosen in the history
	preview         *gitbackup.Preview // what restoring it would change
	previewErr      string
	previewLoading  bool
	configRestoring bool // a restore from the history is running
	diffView        viewport.Model

	// Service fields
	service        service.State
	serviceKnown   bool // false until the first query answers
	serviceLoading bool
	logLines       []string
	logsErr        string
	logsView       viewport.Model
}

// NewModel init and returns a new Model with default values
func NewModel() Model {
	ti := textinput.New()
	ti.Placeholder = "/ruta/a/respaldar"

	di := textinput.New()
	di.Placeholder = "/ruta/del/destino/de/respaldos"

	ri := textinput.New()
	ri.Placeholder = "/ruta/donde/restaurar"

	wi := textinput.New()
	wi.Placeholder = "/etc/openvpn"

	pi := textinput.New()
	pi.Placeholder = "/ruta/al/repositorio/de/configuraciones"

	// A config that cannot be read shows as empty; config.Warning() explains why
	paths, _ := config.GetPaths()
	backupDest, _ := config.GetNasPath()
	history, _ := config.GetHistory()
	configBackup, _ := config.GetConfigBackup()

	m := Model{
		menuCursors:     map[sessionState]int{},
		pathInput:       ti,
		backupDestInput: di,
		restoreInput:    ri,
		watchInput:      wi,
		repoInput:       pi,
		state:           menuView,
		editingIndex:    -1,
		paths:           paths,
		backupDest:      backupDest,
		history:         history,
		allBackups:      []restore.BackupInfo{},
		configBackup:    configBackup,
		diffView:        viewport.New(0, 0),
		logsView:        viewport.New(0, 0),
	}
	if warning := config.Warning(); warning != "" {
		m.message = "Aviso: " + warning
	}
	return m
}
