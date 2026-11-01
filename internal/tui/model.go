package tui

import (
	"github.com/Josedzzz/baky/internal/config"
	"github.com/Josedzzz/baky/internal/gitbackup"
	"github.com/Josedzzz/baky/internal/service"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
)

// sessionState represents the different states of the TUI
type sessionState int

const (
	// menuView is the state where the user interacts with the main menu
	menuView sessionState = iota

	// configMenuView is the submenu of the versioned backup of configuration files
	configMenuView

	// serviceMenuView is the submenu of the daemon service
	serviceMenuView

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

	// watchExcludeInputView is a sub-state for typing the exclude patterns of a watch
	watchExcludeInputView

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
	cursor      int
	menuCursors map[sessionState]int // remembers the selected entry of each menu
	quitting    bool
	state       sessionState
	message     string
	isSuccess   bool
	width       int
	height      int

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
	wi := textinput.New()
	wi.Placeholder = "/etc/openvpn"

	pi := textinput.New()
	pi.Placeholder = "/path/to/config/repository"

	// A config that cannot be read shows as empty; config.Warning() explains why
	configBackup, _ := config.GetConfigBackup()

	m := Model{
		menuCursors:  map[sessionState]int{},
		watchInput:   wi,
		repoInput:    pi,
		state:        menuView,
		configBackup: configBackup,
		diffView:     viewport.New(0, 0),
		logsView:     viewport.New(0, 0),
	}
	if warning := config.Warning(); warning != "" {
		m.message = "Warning: " + warning
	}
	return m
}
