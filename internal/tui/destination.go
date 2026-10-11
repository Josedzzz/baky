package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/Josedzzz/baky/internal/config"
	tea "github.com/charmbracelet/bubbletea"
)

// handleConfigureBackupDestUpdate handles key messages in the backup destination config view
func (m Model) handleConfigureBackupDestUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "e":
		m.state = backupDestInputView
		m.backupDestInput.SetValue(m.backupDest)
		m.backupDestInput.Focus()
		m.message = ""
	case "t":
		if m.backupDest == "" {
			m.message = "El destino de respaldo no está configurado"
			m.isSuccess = false
			return m, nil
		}
		info, err := os.Stat(m.backupDest)
		if err != nil {
			m.message = "Error: " + err.Error()
			m.isSuccess = false
		} else if !info.IsDir() {
			m.message = "La ruta no es un directorio"
			m.isSuccess = false
		} else {
			m.message = "El destino de respaldo es accesible."
			m.isSuccess = true
		}
	case "esc":
		return m.back(filesMenuView)
	}
	return m, nil
}

// handleBackupDestInputUpdate handles backup destination path input
func (m Model) handleBackupDestInputUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg.String() {
	case "esc":
		m.state = configureBackupDestView
		return m, nil
	case "enter":
		val := strings.TrimSpace(m.backupDestInput.Value())
		if val != "" {
			if err := config.SaveNasPath(val); err != nil {
				m.message = "No se pudo guardar el destino: " + err.Error()
				m.isSuccess = false
			} else {
				m.backupDest = val
				m.message = "Destino de respaldo actualizado"
				m.isSuccess = true
			}
			m.state = configureBackupDestView
		}
		return m, nil
	}
	m.backupDestInput, cmd = m.backupDestInput.Update(msg)
	return m, cmd
}

// renderBackupDest renders the backup destination screen
func (m Model) renderBackupDest(body *strings.Builder) {
	body.WriteString(titleStyle.Render(" DESTINO DE RESPALDO ") + "\n\n")
	displayPath := m.backupDest
	if displayPath == "" {
		displayPath = "sin configurar"
	}
	fmt.Fprintf(body, "Destino de respaldo: %s\n\n", displayPath)

	if m.message != "" {
		m.renderMessage(body)
		body.WriteString("\n")
	}
	body.WriteString(footerStyle.Render("e: editar • t: probar acceso • esc: volver"))
}

// renderBackupDestInput renders the backup destination input screen
func (m Model) renderBackupDestInput(body *strings.Builder) {
	body.WriteString(titleStyle.Render(" EDITAR DESTINO DE RESPALDO ") + "\n\n")
	fmt.Fprintf(body,
		"Ruta donde se guardan los respaldos:\n\n%s\n\n%s",
		m.backupDestInput.View(),
		footerStyle.Render("(esc: cancelar • enter: guardar)"),
	)
}
