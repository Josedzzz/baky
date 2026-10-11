package tui

import (
	"fmt"
	"strings"

	"github.com/Josedzzz/baky/internal/gitbackup"
	tea "github.com/charmbracelet/bubbletea"
)

// maxCommits is how many config backups the history screen loads
const maxCommits = 200

type commitsLoadedMsg struct {
	commits []gitbackup.Commit
	err     error
}

// loadCommitsCmd reads the config backups recorded in the repository
func loadCommitsCmd(repoDir string) tea.Cmd {
	return func() tea.Msg {
		if repoDir == "" {
			return commitsLoadedMsg{err: fmt.Errorf("el repositorio no está configurado")}
		}
		commits, err := gitbackup.History(repoDir, maxCommits)
		return commitsLoadedMsg{commits: commits, err: err}
	}
}

// handleCommitsLoaded fills the history screen
func (m Model) handleCommitsLoaded(msg commitsLoadedMsg) (tea.Model, tea.Cmd) {
	m.commitsErr = ""
	if msg.err != nil {
		m.commitsErr = msg.err.Error()
		return m, nil
	}

	m.commits = msg.commits
	var content strings.Builder
	for _, c := range m.commits {
		content.WriteString(logTimeStyle.Render(c.Date) + "  " + mutedStyle.Render(c.Hash) + "  " + c.Message + "\n")
	}
	m.commitsView.SetContent(strings.TrimRight(content.String(), "\n"))
	m.commitsView.GotoTop()
	return m, nil
}

// handleConfigHistoryUpdate handles key messages in the history screen
func (m Model) handleConfigHistoryUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		return m.back(configMenuView)
	case "r":
		return m, loadCommitsCmd(m.configBackup.RepoDir)
	}
	var cmd tea.Cmd
	m.commitsView, cmd = m.commitsView.Update(msg)
	return m, cmd
}

// renderConfigHistory renders the list of config backups already made
func (m Model) renderConfigHistory(body *strings.Builder) {
	body.WriteString(titleStyle.Render(" HISTORIAL DE CAMBIOS ") + "\n\n")

	switch {
	case m.commitsErr != "":
		body.WriteString(errorStyle.Render("No se pudo leer el historial: "+m.commitsErr) + "\n")
	case len(m.commits) == 0:
		body.WriteString("Aún no hay respaldos de configuraciones registrados.\n")
	default:
		body.WriteString(m.commitsView.View() + "\n")
		body.WriteString(statusStyle.Render(fmt.Sprintf("[%d respaldos - %d%%]", len(m.commits), int(m.commitsView.ScrollPercent()*100))) + "\n")
	}

	body.WriteString(footerStyle.Render("↑/↓: desplazar • pgup/pgdn: página • r: refrescar • esc: volver"))
}
