// Package tui provides the terminal user interface (TUI) implementation
package tui

import (
	"fmt"
	"os"
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
		case configureNasView:
			return m.handleConfigureNasUpdate(msg)
		case nasInputView:
			return m.handleNasInputUpdate(msg)
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
			case "Configure NAS":
				nasPath, _ := config.GetNasPath()
				m.nasPath = nasPath
				m.state = configureNasView
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

	case configureNasView:
		body.WriteString(titleStyle.Render(" CONFIGURE NAS ") + "\n\n")
		displayPath := m.nasPath
		if displayPath == "" {
			displayPath = "Not configured"
		}
		fmt.Fprintf(&body, "Current NAS Path: %s\n\n", displayPath)
		if m.message != "" {
			style := errorStyle
			if m.isSuccess {
				style = successStyle
			}
			body.WriteString(style.Render(m.message) + "\n\n")
		}
		body.WriteString(footerStyle.Render("e: edit • t: test • esc: back"))

	case nasInputView:
		body.WriteString(titleStyle.Render(" EDIT NAS PATH ") + "\n\n")
		fmt.Fprintf(&body,
			"Enter NAS share path:\n\n%s\n\n%s",
			m.nasInput.View(),
			footerStyle.Render("(esc to cancel • enter to save)"),
		)

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

// handleConfigureNasUpdate handles key messages in the NAS config view
func (m Model) handleConfigureNasUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "e":
		m.state = nasInputView
		m.nasInput.SetValue(m.nasPath)
		m.nasInput.Focus()
		m.message = ""
	case "t":
		if m.nasPath == "" {
			m.message = "NAS path not configured"
			m.isSuccess = false
			return m, nil
		}
		info, err := os.Stat(m.nasPath)
		if err != nil {
			m.message = "Error: " + err.Error()
			m.isSuccess = false
		} else if !info.IsDir() {
			m.message = "Path is not a directory"
			m.isSuccess = false
		} else {
			m.message = "NAS path is accessible!"
			m.isSuccess = true
		}
	case "esc":
		m.state = menuView
		m.message = ""
	}
	return m, nil
}

// handleNasInputUpdate handles NAS path input
func (m Model) handleNasInputUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg.String() {
	case "esc":
		m.state = configureNasView
		return m, nil
	case "enter":
		val := m.nasInput.Value()
		if val != "" {
			if err := config.SaveNasPath(val); err != nil {
				m.message = "Error saving NAS path"
				m.isSuccess = false
			} else {
				m.nasPath = val
				m.message = "NAS path updated"
				m.isSuccess = true
			}
			m.state = configureNasView
		}
		return m, nil
	}
	m.nasInput, cmd = m.nasInput.Update(msg)
	return m, cmd
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
