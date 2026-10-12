package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Josedzzz/baky/internal/config"
	tea "github.com/charmbracelet/bubbletea"
)

// handleWatchesUpdate handles key messages in the watched folders list
func (m Model) handleWatchesUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	watches := m.configBackup.Watches

	switch msg.String() {
	case "up", "k":
		if m.watchesCursor > 0 {
			m.watchesCursor--
		}
	case "down", "j":
		if m.watchesCursor < len(watches)-1 {
			m.watchesCursor++
		}
	case "a":
		m.state = watchSourceInputView
		m.watchInput.Placeholder = "/etc/openvpn"
		m.watchInput.Reset()
		m.watchInput.Focus()
		m.message = ""
	case "d":
		if len(watches) > 0 {
			m.state = watchConfirmDeleteView
			m.message = ""
		}
	case "c":
		if len(watches) > 0 {
			// Edit the validation command, then the apply command
			m.state = watchCheckInputView
			m.watchInput.Placeholder = "sudo -n shorewall check {dir}"
			m.watchInput.SetValue(watches[m.watchesCursor].Check)
			m.watchInput.CursorEnd()
			m.watchInput.Focus()
			m.message = ""
		}
	case "esc":
		return m.back(configMenuView)
	}
	return m, nil
}

// handleWatchSourceInputUpdate handles the first step of adding a watch:
// the folder to watch
func (m Model) handleWatchSourceInputUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg.String() {
	case "esc":
		m.state = watchesView
		m.message = ""
		return m, nil
	case "enter":
		source := strings.TrimSpace(m.watchInput.Value())
		if source == "" {
			return m, nil
		}
		if !filepath.IsAbs(source) {
			m.message = "La ruta debe ser absoluta (empezar con /)"
			m.isSuccess = false
			return m, nil
		}
		info, err := os.Stat(source)
		if err != nil || !info.IsDir() {
			m.message = "La ruta no existe o no es una carpeta"
			m.isSuccess = false
			return m, nil
		}

		// Second step: the folder name inside the repository, suggested
		// from the name of the watched folder
		m.pendingSource = filepath.Clean(source)
		m.state = watchDestInputView
		m.watchInput.Placeholder = "vpn"
		m.watchInput.SetValue(filepath.Base(m.pendingSource))
		m.watchInput.CursorEnd()
		m.message = ""
		return m, nil
	}
	m.watchInput, cmd = m.watchInput.Update(msg)
	return m, cmd
}

// handleWatchDestInputUpdate handles the second step of adding a watch:
// the folder of the repository that receives the files
func (m Model) handleWatchDestInputUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg.String() {
	case "esc":
		m.state = watchesView
		m.message = ""
		return m, nil
	case "enter":
		dest := strings.TrimSpace(m.watchInput.Value())
		if err := config.AddWatch(m.pendingSource, dest); err != nil {
			m.message = err.Error()
			m.isSuccess = false
			return m, nil
		}
		m.reloadConfigBackup()
		m.state = watchesView
		m.message = fmt.Sprintf("Carpeta '%s' agregada. El servicio aplica el cambio automáticamente.", m.pendingSource)
		m.isSuccess = true
		return m, nil
	}
	m.watchInput, cmd = m.watchInput.Update(msg)
	return m, cmd
}

// handleWatchConfirmDeleteUpdate asks before a folder stops being watched
func (m Model) handleWatchConfirmDeleteUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "s":
		source := m.configBackup.Watches[m.watchesCursor].Source
		if err := config.RemoveWatch(source); err != nil {
			m.message = err.Error()
			m.isSuccess = false
		} else {
			m.message = fmt.Sprintf("La carpeta '%s' ya no se vigila.", source)
			m.isSuccess = true
		}
		m.reloadConfigBackup()
		m.state = watchesView
	case "n", "esc":
		m.state = watchesView
	}
	return m, nil
}

// renderWatches renders the watched folders list
func (m Model) renderWatches(body *strings.Builder) {
	body.WriteString(titleStyle.Render(" CARPETAS VIGILADAS ") + "\n\n")

	if m.configBackup.RepoDir == "" {
		body.WriteString(errorStyle.Render("Aún no hay un repositorio configurado (menú Repositorio).") + "\n\n")
	}

	if len(m.configBackup.Watches) == 0 {
		body.WriteString("No hay carpetas vigiladas.\n\n")
	} else {
		body.WriteString(headerStyle.Render("Carpeta vigilada  →  carpeta en el repositorio") + "\n")
		for i, w := range m.configBackup.Watches {
			style := itemStyle
			if m.watchesCursor == i {
				style = selectedItemStyle
			}
			body.WriteString(style.Render(w.Source+"  →  "+w.Dest) + "\n")
		}

		// Hook commands of the selected folder
		selected := m.configBackup.Watches[m.watchesCursor]
		body.WriteString("\n" + headerStyle.Render("Validación: ") + commandOrNone(selected.Check) + "\n")
		body.WriteString(headerStyle.Render("Aplicación: ") + commandOrNone(selected.Apply) + "\n\n")
	}

	m.renderMessage(body)
	body.WriteString(footerStyle.Render("a: agregar • c: comandos de validación • d: dejar de vigilar • esc: volver"))
}

// renderWatchInput renders both steps of adding a watched folder
func (m Model) renderWatchInput(body *strings.Builder) {
	body.WriteString(titleStyle.Render(" AGREGAR CARPETA VIGILADA ") + "\n\n")

	if m.state == watchSourceInputView {
		body.WriteString("Paso 1 de 2 - Ruta de la carpeta a vigilar:\n\n")
	} else {
		body.WriteString(headerStyle.Render("Carpeta: ") + itemStyle.Render(m.pendingSource) + "\n\n")
		body.WriteString("Paso 2 de 2 - Nombre de la carpeta dentro del repositorio:\n\n")
	}
	body.WriteString(m.watchInput.View() + "\n\n")

	m.renderMessage(body)
	body.WriteString(footerStyle.Render("(esc: cancelar • enter: continuar)"))
}

// renderWatchConfirmDelete renders the confirmation to stop watching a folder
func (m Model) renderWatchConfirmDelete(body *strings.Builder) {
	body.WriteString(titleStyle.Render(" DEJAR DE VIGILAR ") + "\n\n")

	w := m.configBackup.Watches[m.watchesCursor]
	fmt.Fprintf(body, "¿Dejar de vigilar la carpeta '%s'?\n\n", w.Source)
	body.WriteString(mutedStyle.Render("No se borra ningún archivo: ni los originales ni los ya respaldados\nen el repositorio. Solo deja de respaldarse automáticamente.") + "\n")

	body.WriteString(footerStyle.Render("\ns/y: confirmar • n/esc: cancelar"))
}

// commandOrNone shows a hook command, or that there is none
func commandOrNone(command string) string {
	if command == "" {
		return mutedStyle.Render("(ninguna)")
	}
	return command
}

// handleWatchCommandInputUpdate handles the two steps of editing the hook
// commands of a watch: the validation command, then the apply command
func (m Model) handleWatchCommandInputUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg.String() {
	case "esc":
		m.state = watchesView
		m.message = ""
		return m, nil
	case "enter":
		value := strings.TrimSpace(m.watchInput.Value())
		if err := config.ValidateWatchCommand(value); err != nil {
			m.message = err.Error()
			m.isSuccess = false
			return m, nil
		}
		watch := m.configBackup.Watches[m.watchesCursor]

		if m.state == watchCheckInputView {
			m.pendingCheck = value
			m.state = watchApplyInputView
			m.watchInput.Placeholder = "sudo -n shorewall try {dir}"
			m.watchInput.SetValue(watch.Apply)
			m.watchInput.CursorEnd()
			m.message = ""
			return m, nil
		}

		if err := config.SetWatchCommands(watch.Source, m.pendingCheck, value); err != nil {
			m.message = err.Error()
			m.isSuccess = false
			return m, nil
		}
		m.reloadConfigBackup()
		m.state = watchesView
		m.message = "Comandos guardados. El servicio aplica el cambio automáticamente."
		m.isSuccess = true
		return m, nil
	}
	m.watchInput, cmd = m.watchInput.Update(msg)
	return m, cmd
}

// renderWatchCommandInput renders both steps of editing the hook commands
func (m Model) renderWatchCommandInput(body *strings.Builder) {
	body.WriteString(titleStyle.Render(" COMANDOS DE VALIDACIÓN ") + "\n\n")
	body.WriteString(headerStyle.Render("Carpeta: ") + itemStyle.Render(m.configBackup.Watches[m.watchesCursor].Source) + "\n\n")

	if m.state == watchCheckInputView {
		body.WriteString("Paso 1 de 2 - Comando que valida la configuración (vacío = ninguno):\n\n")
		body.WriteString(m.watchInput.View() + "\n\n")
		body.WriteString(mutedStyle.Render("Se ejecuta después de cada cambio y antes de restaurar. Si falla, el\ncambio se respalda igual, pero queda marcado como inválido.") + "\n")
	} else {
		body.WriteString("Paso 2 de 2 - Comando que aplica la configuración (vacío = ninguno):\n\n")
		body.WriteString(m.watchInput.View() + "\n\n")
		body.WriteString(mutedStyle.Render("Solo se ejecuta al restaurar un archivo desde el historial, después\nde validar y antes de escribir el archivo.") + "\n")
	}
	body.WriteString(mutedStyle.Render("{dir} se reemplaza por la carpeta con la configuración. El comando se\nejecuta tal cual: si necesita privilegios, debe empezar con \"sudo -n\".") + "\n\n")

	m.renderMessage(body)
	body.WriteString(footerStyle.Render("(esc: cancelar • enter: continuar)"))
}
