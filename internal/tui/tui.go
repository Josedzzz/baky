// Package tui provides the terminal user interface (TUI) implementation
package tui

import (
	"fmt"
	"strings"

	"github.com/Josedzzz/baky/internal/config"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// Init initializes the TUI model and returns the initial command
func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles messages and updates the TUI model accordingly
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if m.state == inputView {
			return m.handleInputUpdate(msg)
		}
		return m.HandleMenuUpdate(msg)
	}
	return m, nil
}

// HandleMenuUpdate processes key messages when the TUI is in the menu view
func (m Model) HandleMenuUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}

		case "enter":
			if m.choices[m.cursor] == "Add Backup Path" {
				m.state = inputView
				return m, nil
			}
			if m.choices[m.cursor] == "Exit" {
				m.quitting = true
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

// View renders the current state of the TUI as a string
func (m Model) View() string {
	if m.state == inputView {
		return fmt.Sprintf("Enter path:\n\n%s\n\n(esc to cancel)", m.pathInput.View())
	}

	var sb strings.Builder
	sb.WriteString("baky - Backup Utility \n\n")

	for i, choice := range m.choices {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
		}
		fmt.Fprintf(&sb, "%s %s\n", cursor, choice)
	}
	if m.message != "" {
		fmt.Fprintf(&sb, "\n%s\n", m.message)
	}

	sb.WriteString("\npress q to quit\n")
	return sb.String()
}

// handleInputUpdate processes key messages when the TUI is in the input view
func (m Model) handleInputUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg.String() {
	case "esc":
		m.state = menuView
		m.pathInput.Reset()
		return m, nil

	case "enter":
		path := m.pathInput.Value()
		if path != "" {
			err := config.AddPath(path)
			if err != nil {
				m.message = "Error saving path"
			} else {
				m.message = "Path added successfully"
				m.state = menuView
				m.pathInput.Reset()
			}
		}
		return m, nil
	}

	m.pathInput, cmd = m.pathInput.Update(msg)
	return m, cmd
}
