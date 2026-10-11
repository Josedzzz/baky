package tui

import (
	"fmt"
	"strings"

	"github.com/Josedzzz/baky/internal/config"
	"github.com/Josedzzz/baky/internal/gitbackup"
	tea "github.com/charmbracelet/bubbletea"
)

// handleConfigRepoUpdate handles key messages in the config repository screen
func (m Model) handleConfigRepoUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "e":
		m.state = configRepoInputView
		m.repoInput.SetValue(m.configBackup.RepoDir)
		m.repoInput.CursorEnd()
		m.repoInput.Focus()
		m.message = ""
	case "t":
		m.testRepo()
	case "esc":
		return m.back(configMenuView)
	}
	return m, nil
}

// testRepo checks that the configured path is a Git repository
func (m *Model) testRepo() {
	switch {
	case m.configBackup.RepoDir == "":
		m.message = "El repositorio no está configurado"
		m.isSuccess = false
	case !gitbackup.IsRepo(m.configBackup.RepoDir):
		m.message = "La ruta no es un repositorio Git. Clónelo primero con 'git clone'."
		m.isSuccess = false
	default:
		m.message = "El repositorio es válido."
		m.isSuccess = true
	}
}

// handleConfigRepoInputUpdate handles the config repository path input
func (m Model) handleConfigRepoInputUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg.String() {
	case "esc":
		m.state = configRepoView
		m.message = ""
		return m, nil
	case "enter":
		val := strings.TrimSpace(m.repoInput.Value())
		if val == "" {
			return m, nil
		}
		if err := config.SaveConfigRepo(val); err != nil {
			m.message = err.Error()
			m.isSuccess = false
			return m, nil
		}
		m.reloadConfigBackup()
		m.state = configRepoView
		m.testRepo()
		if m.isSuccess {
			m.message = "Repositorio actualizado."
		}
		return m, nil
	}
	m.repoInput, cmd = m.repoInput.Update(msg)
	return m, cmd
}

// renderConfigRepo renders the config repository screen
func (m Model) renderConfigRepo(body *strings.Builder) {
	body.WriteString(titleStyle.Render(" REPOSITORIO DE CONFIGURACIONES ") + "\n\n")

	repoDir := m.configBackup.RepoDir
	if repoDir == "" {
		repoDir = "sin configurar"
	}
	fmt.Fprintf(body, "Repositorio local: %s\n", repoDir)
	fmt.Fprintf(body, "Rama remota:       %s\n\n", m.configBackup.Branch)
	body.WriteString(mutedStyle.Render("Es el clon local del repositorio Git donde se guardan las\nconfiguraciones. Cada cambio se registra con un commit y se envía\nal remoto 'origin'.") + "\n\n")

	if m.message != "" {
		m.renderMessage(body)
		body.WriteString("\n")
	}
	body.WriteString(footerStyle.Render("e: editar • t: probar • esc: volver"))
}

// renderConfigRepoInput renders the config repository path input
func (m Model) renderConfigRepoInput(body *strings.Builder) {
	body.WriteString(titleStyle.Render(" EDITAR REPOSITORIO ") + "\n\n")
	fmt.Fprintf(body,
		"Ruta del clon local del repositorio de configuraciones:\n\n%s\n\n",
		m.repoInput.View(),
	)
	m.renderMessage(body)
	body.WriteString(footerStyle.Render("(esc: cancelar • enter: guardar)"))
}
