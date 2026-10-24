package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// helpSections lists the keyboard shortcuts of every screen
var helpSections = []struct {
	title string
	keys  [][2]string
}{
	{"General", [][2]string{
		{"↑/↓ or k/j", "navigate"},
		{"enter", "select or confirm"},
		{"esc", "go back to the previous screen"},
		{"q", "quit (in the menus)"},
		{"ctrl+c", "quit from any screen"},
	}},
	{"File backup", [][2]string{
		{"a / e / d", "add, edit or delete a path"},
		{"f", "change the frequency: daily, weekly, on change"},
		{"enter", "back up the selected path / restore a backup"},
		{"o / c", "restore to the original location or to another path"},
		{"enter / r", "before restoring: view the differences of a file / continue"},
	}},
	{"Config backup", [][2]string{
		{"a / d", "add a watched folder or stop watching it"},
		{"c", "set the commands that validate and apply a folder"},
		{"e / t", "edit or test the repository"},
		{"enter", "in the history: compare the current file with that version"},
		{"r", "refresh the history / restore the compared version"},
	}},
	{"Service", [][2]string{
		{"s / x / r", "start, stop or restart the service"},
		{"u", "update the status"},
		{"pgup/pgdn, end", "scroll through the logs"},
	}},
}

// handleHelpUpdate handles key messages in the help screen
func (m Model) handleHelpUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q", "?", "enter":
		return m.back(menuView)
	}
	return m, nil
}

// renderHelp renders the keyboard shortcuts
func (m Model) renderHelp(body *strings.Builder) {
	body.WriteString(titleStyle.Render(" HELP ") + "\n\n")

	for _, section := range helpSections {
		body.WriteString(headerStyle.Render(section.title) + "\n")
		for _, key := range section.keys {
			fmt.Fprintf(body, "  %s  %s\n", logTimeStyle.Render(fmt.Sprintf("%-15s", key[0])), key[1])
		}
		body.WriteString("\n")
	}

	body.WriteString(mutedStyle.Render("Automatic backups are run by the service (baky daemon), not by\nthis interface: you can close it without interrupting them.") + "\n")
	body.WriteString(footerStyle.Render("esc: back"))
}
