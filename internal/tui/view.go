// Package tui provides the terminal user interface (TUI) implementation
package tui

import (
	"fmt"
	"strings"

	"github.com/Josedzzz/baky/internal/config"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const logo = `
 ██████╗  █████╗ ██╗  ██╗██╗   ██╗
 ██╔══██╗██╔══██╗██║ ██╔╝╚██╗ ██╔╝
 ██████╔╝███████║█████╔╝  ╚████╔╝ 
 ██╔══██╗██╔══██║██╔═██╗   ╚██╔╝  
 ██████╔╝██║  ██║██║  ██╗   ██║   
 ╚══════╝╚═╝  ╚═╝╚═╝  ╚═╝   ╚═╝   
`

// Init initializes the TUI model and returns the initial command
func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles messages and updates the TUI model accordingly
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		switch m.state {
		case inputView:
			return m.handleInputUpdate(msg)
		case managePathsView:
			return m.handleManagePathsUpdate(msg)
		default:
			return m.HandleMenuUpdate(msg)
		}
	}
	return m, nil
}

// HandleMenuUpdate processes key messages when the TUI is in the menu view
func (m Model) HandleMenuUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}
		case "enter":
			choice := m.choices[m.cursor]
			switch choice {
			case "Manage Paths":
				paths, _ := config.GetPaths()
				m.paths = paths
				m.state = managePathsView
				m.pathsCursor = 0
				m.message = ""
			case "Exit":
				m.quitting = true
				return m, tea.Quit
			}
		case "q":
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

// View renders the current state of the TUI as a string
func (m Model) View() string {
	if m.quitting {
		return ""
	}

	var body strings.Builder
	body.WriteString(asciiStyle.Render(logo) + "\n")

	switch m.state {
	case inputView:
		title := " ADD BACKUP PATH "
		if m.editingIndex != -1 {
			title = " UPDATE BACKUP PATH "
		}
		body.WriteString(titleStyle.Render(title) + "\n\n")
		fmt.Fprintf(&body,
			"Enter path:\n\n%s\n\n%s",
			m.pathInput.View(),
			footerStyle.Render("(esc to cancel • enter to save)"),
		)

	case managePathsView:
		body.WriteString(titleStyle.Render(" MANAGE BACKUP PATHS ") + "\n\n")
		if len(m.paths) == 0 {
			body.WriteString("No paths added yet.\n\n")
		} else {
			for i, p := range m.paths {
				cursor := " "
				style := itemStyle
				if m.pathsCursor == i {
					cursor = ">"
					style = selectedItemStyle
				}
				body.WriteString(style.Render(fmt.Sprintf("%s %s", cursor, p)) + "\n")
			}
			body.WriteString("\n")
		}

		if m.message != "" {
			style := errorStyle
			if m.isSuccess {
				style = successStyle
			}
			body.WriteString(style.Render(m.message) + "\n")
		}

		body.WriteString(footerStyle.Render("a: add • e: edit • d: delete • esc: back"))

	case menuView:
		body.WriteString(titleStyle.Render(" MAIN MENU ") + "\n\n")
		for i, choice := range m.choices {
			if m.cursor == i {
				fmt.Fprintf(&body, "%s\n", selectedItemStyle.Render("> "+choice))
			} else {
				fmt.Fprintf(&body, "%s\n", itemStyle.Render(choice))
			}
		}
		body.WriteString(footerStyle.Render("\n↑/↓: navigate • enter: select • q: quit"))
	}

	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		containerStyle.Render(body.String()),
	)
}

// handleManagePathsUpdate handles key messages in the manage paths view
func (m Model) handleManagePathsUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.pathsCursor > 0 {
			m.pathsCursor--
		}
	case "down", "j":
		if m.pathsCursor < len(m.paths)-1 {
			m.pathsCursor++
		}
	case "a":
		m.state = inputView
		m.editingIndex = -1
		m.pathInput.Reset()
		m.pathInput.Focus()
	case "e":
		if len(m.paths) > 0 {
			m.state = inputView
			m.editingIndex = m.pathsCursor
			m.pathInput.SetValue(m.paths[m.pathsCursor])
			m.pathInput.Focus()
		}
	case "d":
		if len(m.paths) > 0 {
			// Remove from slice
			m.paths = append(m.paths[:m.pathsCursor], m.paths[m.pathsCursor+1:]...)
			if m.pathsCursor >= len(m.paths) && m.pathsCursor > 0 {
				m.pathsCursor--
			}
			config.SavePaths(m.paths)
			m.message = "Path deleted"
			m.isSuccess = true
		}
	case "esc":
		m.state = menuView
		m.message = ""
	}
	return m, nil
}

// handleInputUpdate processes key messages when typing a path
func (m Model) handleInputUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg.String() {
	case "esc":
		m.state = managePathsView
		m.pathInput.Reset()
		m.message = ""
		return m, nil
	case "enter":
		val := m.pathInput.Value()
		if val != "" {
			if m.editingIndex == -1 {
				m.paths = append(m.paths, val)
				m.message = "Path added"
			} else {
				m.paths[m.editingIndex] = val
				m.message = "Path updated"
			}
			config.SavePaths(m.paths)
			m.isSuccess = true
			m.state = managePathsView
			m.pathInput.Reset()
		}
		return m, nil
	}
	m.pathInput, cmd = m.pathInput.Update(msg)
	return m, cmd
}
