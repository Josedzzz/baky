package tui

import "github.com/charmbracelet/bubbles/textinput"

// sessionState represents the different states of the TUI
type sessionState int

const (
	// menuView is the state where the user interacts with the main menu
	menuView sessionState = iota

	// inputView is the state where the user provides input, such as backup path
	inputView

	// viewPathsView is the state where the user views the current backup paths
	viewPathsView
)

// Model represents the state of the TUI
type Model struct {
	choices   []string
	cursor    int
	quitting  bool
	state     sessionState
	pathInput textinput.Model
	paths     []string
	message   string
}

// NewModel init and returns a new Model with default values
func NewModel() Model {
	ti := textinput.New()
	ti.Placeholder = "/path/to/backup"
	ti.Focus()

	return Model{
		choices:   []string{"Add Backup Path", "View Paths", "Backup Files", "Configure NAS", "Exit"},
		pathInput: ti,
		state:     menuView,
	}
}
