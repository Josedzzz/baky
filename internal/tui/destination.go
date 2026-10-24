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
			m.message = "The backup destination is not configured"
			m.isSuccess = false
			return m, nil
		}
		info, err := os.Stat(m.backupDest)
		if err != nil {
			m.message = "Error: " + err.Error()
			m.isSuccess = false
		} else if !info.IsDir() {
			m.message = "The path is not a directory"
			m.isSuccess = false
		} else {
			m.message = "The backup destination is accessible."
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
				m.message = "Could not save the destination: " + err.Error()
				m.isSuccess = false
			} else {
				m.backupDest = val
				m.message = "Backup destination updated"
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
	body.WriteString(titleStyle.Render(" BACKUP DESTINATION ") + "\n\n")
	displayPath := m.backupDest
	if displayPath == "" {
		displayPath = "not configured"
	}
	fmt.Fprintf(body, "Backup destination: %s\n\n", displayPath)

	if m.message != "" {
		m.renderMessage(body)
		body.WriteString("\n")
	}
	body.WriteString(footerStyle.Render("e: edit • t: test access • esc: back"))
}

// renderBackupDestInput renders the backup destination input screen
func (m Model) renderBackupDestInput(body *strings.Builder) {
	body.WriteString(titleStyle.Render(" EDIT BACKUP DESTINATION ") + "\n\n")
	fmt.Fprintf(body,
		"Path where backups are stored:\n\n%s\n\n%s",
		m.backupDestInput.View(),
		footerStyle.Render("(esc: cancel • enter: save)"),
	)
}
