package tui

import (
	"fmt"
	"strings"

	"github.com/Josedzzz/baky/internal/backup"
	"github.com/Josedzzz/baky/internal/config"
	tea "github.com/charmbracelet/bubbletea"
)

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
			m.pathInput.SetValue(m.paths[m.pathsCursor].Path)
			m.pathInput.Focus()
		}
	case "f":
		if len(m.paths) > 0 {
			// Cycle frequency
			p := &m.paths[m.pathsCursor]
			switch p.Frequency {
			case config.FreqDaily:
				p.Frequency = config.FreqWeekly
			case config.FreqWeekly:
				p.Frequency = config.FreqOnChange
			case config.FreqOnChange:
				p.Frequency = config.FreqDaily
			default:
				p.Frequency = config.FreqDaily
			}
			config.SavePaths(m.paths)
			backup.UpdateWatcher()
		}
	case "d":
		if len(m.paths) > 0 {
			m.paths = append(m.paths[:m.pathsCursor], m.paths[m.pathsCursor+1:]...)
			if m.pathsCursor >= len(m.paths) && m.pathsCursor > 0 {
				m.pathsCursor--
			}
			config.SavePaths(m.paths)
			backup.UpdateWatcher()
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
				m.paths = append(m.paths, config.BackupPathConfig{Path: val, Frequency: config.FreqDaily})
				m.message = "Path added"
			} else {
				m.paths[m.editingIndex].Path = val
				m.message = "Path updated"
			}
			config.SavePaths(m.paths)
			backup.UpdateWatcher()
			m.isSuccess = true
			m.state = managePathsView
			m.pathInput.Reset()
		}
		return m, nil
	}
	m.pathInput, cmd = m.pathInput.Update(msg)
	return m, cmd
}

// renderManagePaths renders the manage paths screen
func (m Model) renderManagePaths(body *strings.Builder) {
	body.WriteString(titleStyle.Render(" MANAGE BACKUP PATHS ") + "\n\n")
	if len(m.paths) == 0 {
		body.WriteString("No paths added yet.\n\n")
	} else {
		for i, p := range m.paths {
			style := itemStyle
			label := p.Path
			if m.pathsCursor == i {
				style = selectedItemStyle
			}
			last := "Never"
			if !p.LastBackup.IsZero() {
				last = p.LastBackup.Format("2006-01-02 15:04")
			}

			var freqLabel string
			switch p.Frequency {
			case config.FreqWeekly:
				freqLabel = "[W]"
			case config.FreqOnChange:
				freqLabel = "[C]"
			default:
				freqLabel = "[D]"
			}

			line := fmt.Sprintf("%-20s %s [%s] (Last: %s)", label, freqLabel, p.Frequency, last)

			body.WriteString(style.Render(line) + "\n")
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
	body.WriteString(footerStyle.Render("a: add • e: edit • f: cycle freq • d: delete • esc: back"))

}

// renderPathInput renders the path input screen
func (m Model) renderPathInput(body *strings.Builder) {
	title := " ADD BACKUP PATH "
	if m.editingIndex != -1 {
		title = " UPDATE BACKUP PATH "
	}
	body.WriteString(titleStyle.Render(title) + "\n\n")
	fmt.Fprintf(body,
		"Enter path:\n\n%s\n\n%s",
		m.pathInput.View(),
		footerStyle.Render("(esc to cancel • enter to save)"),
	)

}
