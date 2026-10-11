package tui

import (
	"fmt"
	"strings"

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
			m.savePaths("Frecuencia actualizada")
		}
	case "d":
		if len(m.paths) > 0 {
			m.paths = append(m.paths[:m.pathsCursor], m.paths[m.pathsCursor+1:]...)
			if m.pathsCursor >= len(m.paths) && m.pathsCursor > 0 {
				m.pathsCursor--
			}
			m.savePaths("Ruta eliminada")
		}
	case "esc":
		return m.back(filesMenuView)
	}
	return m, nil
}

// savePaths stores the paths and reports the result. The daemon applies
// the change by itself when the config file changes.
func (m *Model) savePaths(done string) {
	if err := config.SavePaths(m.paths); err != nil {
		m.message = "No se pudo guardar: " + err.Error()
		m.isSuccess = false
		return
	}
	m.message = done
	m.isSuccess = true
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
		val := strings.TrimSpace(m.pathInput.Value())
		if val != "" {
			done := "Ruta agregada"
			if m.editingIndex == -1 {
				m.paths = append(m.paths, config.BackupPathConfig{Path: val, Frequency: config.FreqDaily})
			} else {
				m.paths[m.editingIndex].Path = val
				done = "Ruta actualizada"
			}
			m.savePaths(done)
			m.state = managePathsView
			m.pathInput.Reset()
		}
		return m, nil
	}
	m.pathInput, cmd = m.pathInput.Update(msg)
	return m, cmd
}

// frequencyLabel returns the name of a backup frequency as shown to the user
func frequencyLabel(frequency string) string {
	switch frequency {
	case config.FreqWeekly:
		return "semanal"
	case config.FreqOnChange:
		return "al cambiar"
	default:
		return "diario"
	}
}

// renderManagePaths renders the manage paths screen
func (m Model) renderManagePaths(body *strings.Builder) {
	body.WriteString(titleStyle.Render(" GESTIONAR RUTAS DE RESPALDO ") + "\n\n")
	if len(m.paths) == 0 {
		body.WriteString("Aún no hay rutas agregadas.\n\n")
	} else {
		for i, p := range m.paths {
			style := itemStyle
			if m.pathsCursor == i {
				style = selectedItemStyle
			}
			last := "nunca"
			if !p.LastBackup.IsZero() {
				last = p.LastBackup.Format("2006-01-02 15:04")
			}

			line := fmt.Sprintf("%-20s [%s] (último: %s)", p.Path, frequencyLabel(p.Frequency), last)

			body.WriteString(style.Render(line) + "\n")
		}
		body.WriteString("\n")
	}
	m.renderMessage(body)
	body.WriteString(footerStyle.Render("a: agregar • e: editar • f: cambiar frecuencia • d: eliminar • esc: volver"))
}

// renderPathInput renders the path input screen
func (m Model) renderPathInput(body *strings.Builder) {
	title := " AGREGAR RUTA DE RESPALDO "
	if m.editingIndex != -1 {
		title = " EDITAR RUTA DE RESPALDO "
	}
	body.WriteString(titleStyle.Render(title) + "\n\n")
	fmt.Fprintf(body,
		"Ruta del archivo o carpeta:\n\n%s\n\n%s",
		m.pathInput.View(),
		footerStyle.Render("(esc: cancelar • enter: guardar)"),
	)
}
