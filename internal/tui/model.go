package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
)

// sessionState represents the different states of the TUI
type sessionState int

const (
	// menuView is the state where the user interacts with the main menu
	menuView sessionState = iota

	// managePathsView is the unified state where the user manages backup paths
	managePathsView

	// inputView is a sub-state for when the user is typing a path
	inputView
)

// Model represents the state of the TUI
type Model struct {
	choices   []string
	cursor    int
	quitting  bool
	state     sessionState
	pathInput textinput.Model
	paths     []string
	pathsCursor int // cursor for the paths list
	editingIndex int // index of the path being edited (-1 for new)
	message   string
	isSuccess bool
	width     int
	height    int
}

// NewModel init and returns a new Model with default values
func NewModel() Model {
	ti := textinput.New()
	ti.Placeholder = "/path/to/backup"

	return Model{
		choices:   []string{"Manage Paths", "Backup Files", "Configure NAS", "Exit"},
		pathInput: ti,
		state:     menuView,
		editingIndex: -1,
	}
}
