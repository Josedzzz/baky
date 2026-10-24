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
		m.message = "The repository is not configured"
		m.isSuccess = false
	case !gitbackup.IsRepo(m.configBackup.RepoDir):
		m.message = "The path is not a Git repository. Clone it first with 'git clone'."
		m.isSuccess = false
	default:
		m.message = "The repository is valid."
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
			m.message = "Repository updated."
		}
		return m, nil
	}
	m.repoInput, cmd = m.repoInput.Update(msg)
	return m, cmd
}

// renderConfigRepo renders the config repository screen
func (m Model) renderConfigRepo(body *strings.Builder) {
	body.WriteString(titleStyle.Render(" CONFIG REPOSITORY ") + "\n\n")

	repoDir := m.configBackup.RepoDir
	if repoDir == "" {
		repoDir = "not configured"
	}
	fmt.Fprintf(body, "Local repository: %s\n", repoDir)
	fmt.Fprintf(body, "Remote branch:    %s\n\n", m.configBackup.Branch)
	body.WriteString(mutedStyle.Render("The local clone of the Git repository where configurations are\nstored. Every change is recorded as a commit and pushed to the\n'origin' remote.") + "\n\n")

	if m.message != "" {
		m.renderMessage(body)
		body.WriteString("\n")
	}
	body.WriteString(footerStyle.Render("e: edit • t: test • esc: back"))
}

// renderConfigRepoInput renders the config repository path input
func (m Model) renderConfigRepoInput(body *strings.Builder) {
	body.WriteString(titleStyle.Render(" EDIT REPOSITORY ") + "\n\n")
	fmt.Fprintf(body,
		"Path of the local clone of the config repository:\n\n%s\n\n",
		m.repoInput.View(),
	)
	m.renderMessage(body)
	body.WriteString(footerStyle.Render("(esc: cancel • enter: save)"))
}
