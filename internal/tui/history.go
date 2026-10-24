package tui

import (
	"fmt"
	"strings"

	"github.com/Josedzzz/baky/internal/config"
	"github.com/Josedzzz/baky/internal/gitbackup"
	"github.com/Josedzzz/baky/internal/textdiff"
	tea "github.com/charmbracelet/bubbletea"
)

// maxCommits is how many config backups the history screen loads
const maxCommits = 200

type commitsLoadedMsg struct {
	commits []gitbackup.Commit
	err     error
}

type previewLoadedMsg struct {
	preview *gitbackup.Preview
	err     error
}

type configRestoredMsg struct {
	path   string
	output string // what the validation and apply commands printed
	err    error
}

// loadCommitsCmd reads the config backups recorded in the repository
func loadCommitsCmd(repoDir string) tea.Cmd {
	return func() tea.Msg {
		if repoDir == "" {
			return commitsLoadedMsg{err: fmt.Errorf("the repository is not configured")}
		}
		commits, err := gitbackup.History(repoDir, maxCommits)
		return commitsLoadedMsg{commits: commits, err: err}
	}
}

// loadPreviewCmd compares the watched file with its version in a commit
func loadPreviewCmd(cb config.ConfigBackup, commit gitbackup.Commit) tea.Cmd {
	return func() tea.Msg {
		watches := make([]gitbackup.Watch, len(cb.Watches))
		for i, w := range cb.Watches {
			watches[i] = gitbackup.Watch{Source: w.Source, Dest: w.Dest, Check: w.Check, Apply: w.Apply}
		}
		preview, err := gitbackup.PreviewRestore(cb.RepoDir, watches, commit)
		return previewLoadedMsg{preview: preview, err: err}
	}
}

// restoreConfigCmd writes the version being previewed to the watched file
func restoreConfigCmd(preview *gitbackup.Preview) tea.Cmd {
	return func() tea.Msg {
		output, err := preview.Restore()
		return configRestoredMsg{path: preview.LivePath, output: output, err: err}
	}
}

// visibleCommits is how many history entries fit on the screen
func (m Model) visibleCommits() int {
	visible := m.height - 14
	if visible < 3 {
		visible = 3
	}
	if visible > 15 {
		visible = 15
	}
	return visible
}

// handleCommitsLoaded fills the history screen
func (m Model) handleCommitsLoaded(msg commitsLoadedMsg) (tea.Model, tea.Cmd) {
	m.commitsErr = ""
	if msg.err != nil {
		m.commitsErr = msg.err.Error()
		return m, nil
	}

	m.commits = msg.commits
	if m.commitsCursor >= len(m.commits) {
		m.commitsCursor = 0
		m.commitsOffset = 0
	}
	return m, nil
}

// handleConfigHistoryUpdate handles key messages in the history screen
func (m Model) handleConfigHistoryUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	visible := m.visibleCommits()

	switch msg.String() {
	case "up", "k":
		if m.commitsCursor > 0 {
			m.commitsCursor--
		}
	case "down", "j":
		if m.commitsCursor < len(m.commits)-1 {
			m.commitsCursor++
		}
	case "pgup":
		m.commitsCursor -= visible
	case "pgdown":
		m.commitsCursor += visible
	case "home", "g":
		m.commitsCursor = 0
	case "end", "G":
		m.commitsCursor = len(m.commits) - 1
	case "enter":
		if len(m.commits) == 0 {
			return m, nil
		}
		// Compare the current file with the version of the selected commit
		m.selectedCommit = m.commits[m.commitsCursor]
		m.preview = nil
		m.previewErr = ""
		m.previewLoading = true
		m.message = ""
		m.state = configDiffView
		return m, loadPreviewCmd(m.configBackup, m.selectedCommit)
	case "r":
		m.message = ""
		return m, loadCommitsCmd(m.configBackup.RepoDir)
	case "esc", "q":
		return m.back(configMenuView)
	}

	// Keep the cursor inside the list and the list around the cursor
	if m.commitsCursor > len(m.commits)-1 {
		m.commitsCursor = len(m.commits) - 1
	}
	if m.commitsCursor < 0 {
		m.commitsCursor = 0
	}
	if m.commitsCursor < m.commitsOffset {
		m.commitsOffset = m.commitsCursor
	}
	if m.commitsCursor >= m.commitsOffset+visible {
		m.commitsOffset = m.commitsCursor - visible + 1
	}
	return m, nil
}

// handlePreviewLoaded shows the comparison once it has been computed
func (m Model) handlePreviewLoaded(msg previewLoadedMsg) (tea.Model, tea.Cmd) {
	m.previewLoading = false
	if msg.err != nil {
		m.previewErr = msg.err.Error()
		return m, nil
	}

	m.preview = msg.preview
	m.diffView.SetContent(renderDiffLines(msg.preview.Lines))
	m.diffView.GotoTop()
	return m, nil
}

// canRestore reports whether the previewed version can be written to the file
func (m Model) canRestore() bool {
	return m.preview != nil && m.preview.InCommit && !m.preview.Identical
}

// handleConfigDiffUpdate handles key messages in the comparison screen
func (m Model) handleConfigDiffUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.state = configHistoryView
		m.message = ""
		return m, nil
	case "r":
		if m.canRestore() {
			m.state = configRestoreConfirmView
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.diffView, cmd = m.diffView.Update(msg)
	return m, cmd
}

// handleConfigRestoreConfirmUpdate asks before the watched file is overwritten
func (m Model) handleConfigRestoreConfirmUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// A restore is running: ignore every key until it finishes
	if m.configRestoring {
		return m, nil
	}

	switch msg.String() {
	case "y":
		m.configRestoring = true
		return m, restoreConfigCmd(m.preview)
	case "n", "esc":
		m.state = configDiffView
	}
	return m, nil
}

// handleConfigRestored reports the result of a restore and returns to the history
func (m Model) handleConfigRestored(msg configRestoredMsg) (tea.Model, tea.Cmd) {
	m.configRestoring = false
	if msg.err != nil {
		m.state = configDiffView
		m.message = "Nothing was restored: " + msg.err.Error()
		if msg.output != "" {
			m.message += "\n" + lastOutputLines(msg.output, 6)
		}
		m.isSuccess = false
		return m, nil
	}

	m.state = configHistoryView
	m.commitsCursor = 0
	m.commitsOffset = 0
	m.message = fmt.Sprintf("%s restored. The service will record it as a new change; press r to refresh.", msg.path)
	m.isSuccess = true
	return m, loadCommitsCmd(m.configBackup.RepoDir)
}

// renderDiffLines formats a comparison: where each group of changes
// starts, what would be removed and what would come back
func renderDiffLines(lines []textdiff.Line) string {
	rendered := make([]string, 0, len(lines))
	for i, line := range lines {
		// Tabs would break the alignment inside the scrolling area
		text := strings.ReplaceAll(line.Text, "\t", "    ")

		switch line.Kind {
		case textdiff.Hunk:
			if i > 0 {
				rendered = append(rendered, "")
			}
			rendered = append(rendered, diffHunkStyle.Render(fmt.Sprintf("── Changes from line %d of the current file ──", line.Number)))
		case textdiff.Removed:
			rendered = append(rendered, diffRemovedStyle.Render("- "+text))
		case textdiff.Added:
			rendered = append(rendered, diffAddedStyle.Render("+ "+text))
		case textdiff.Note:
			rendered = append(rendered, mutedStyle.Render(text))
		default:
			rendered = append(rendered, mutedStyle.Render("  "+text))
		}
	}
	return strings.Join(rendered, "\n")
}

// renderConfigHistory renders the list of config backups already made
func (m Model) renderConfigHistory(body *strings.Builder) {
	body.WriteString(titleStyle.Render(" CHANGE HISTORY ") + "\n\n")

	switch {
	case m.commitsErr != "":
		body.WriteString(errorStyle.Render("Could not read the history: "+m.commitsErr) + "\n")
	case len(m.commits) == 0:
		body.WriteString("No config backups have been recorded yet.\n")
	default:
		body.WriteString(headerStyle.Render("Select a change to compare it with the current file:") + "\n")
		visible := m.visibleCommits()
		for i := m.commitsOffset; i < len(m.commits) && i < m.commitsOffset+visible; i++ {
			c := m.commits[i]
			line := fmt.Sprintf("%s  %s  %s", c.Date, c.Hash, c.Message)
			switch {
			case m.commitsCursor == i:
				body.WriteString(selectedItemStyle.Render(line) + "\n")
			case c.Invalid:
				// The configuration did not pass its validation at this point
				body.WriteString(itemStyle.Render(diffRemovedStyle.Render(line)) + "\n")
			default:
				body.WriteString(itemStyle.Render(line) + "\n")
			}
		}
		body.WriteString(statusStyle.Render(fmt.Sprintf("[Change %d of %d]", m.commitsCursor+1, len(m.commits))) + "\n")
	}

	m.renderMessage(body)
	body.WriteString(footerStyle.Render("↑/↓: select • enter: view changes / restore • r: refresh • esc: back"))
}

// renderConfigDiff renders the comparison between the current file and
// its version in the selected commit
func (m Model) renderConfigDiff(body *strings.Builder) {
	body.WriteString(titleStyle.Render(" CHANGES TO RESTORE ") + "\n\n")

	c := m.selectedCommit
	body.WriteString(headerStyle.Render("Change:  ") + c.Hash + "  " + c.Date + "\n")
	body.WriteString(headerStyle.Render("Details: ") + c.Message + "\n")

	if c.Invalid {
		body.WriteString(errorStyle.Render("This change did not pass validation when it was backed up.") + "\n")
		if m.preview != nil && m.preview.CheckOutput != "" {
			body.WriteString(mutedStyle.Render(lastOutputLines(m.preview.CheckOutput, 4)) + "\n")
		}
	}

	footer := "esc: back"
	switch {
	case m.previewLoading:
		body.WriteString("\n" + processingStyle.Render("Comparing...") + "\n")

	case m.previewErr != "":
		body.WriteString("\n" + errorStyle.Render(m.previewErr) + "\n")

	case m.preview == nil:
		// Nothing to show yet

	case !m.preview.InCommit:
		body.WriteString(headerStyle.Render("File:    ") + m.preview.LivePath + "\n\n")
		body.WriteString("This change deleted the file, so it holds no version to restore.\n")
		body.WriteString(mutedStyle.Render("To recover the file, choose an earlier change in the history.") + "\n")

	case m.preview.Identical:
		body.WriteString(headerStyle.Render("File:    ") + m.preview.LivePath + "\n\n")
		body.WriteString(successStyle.Render("The current file already matches this version: there is nothing to restore.") + "\n")

	default:
		body.WriteString(headerStyle.Render("File:    ") + m.preview.LivePath + "\n\n")
		if !m.preview.LiveExists {
			body.WriteString(mutedStyle.Render("The file does not exist right now: restoring would create it again.") + "\n")
		}
		body.WriteString("If you restore this version:  " +
			diffRemovedStyle.Render("- current line that is removed") + "   " +
			diffAddedStyle.Render("+ line that comes back") + "\n\n")
		body.WriteString(m.diffView.View() + "\n")
		body.WriteString(statusStyle.Render(fmt.Sprintf("[%d%%]", int(m.diffView.ScrollPercent()*100))) + "\n")
		footer = "↑/↓: scroll • pgup/pgdn: page • r: restore this version • esc: back"
	}

	m.renderMessage(body)
	body.WriteString(footerStyle.Render(footer))
}

// renderConfigRestoreConfirm renders the confirmation before a restore
func (m Model) renderConfigRestoreConfirm(body *strings.Builder) {
	body.WriteString(titleStyle.Render(" CONFIRM RESTORE ") + "\n\n")

	removed, added := 0, 0
	for _, line := range m.preview.Lines {
		switch line.Kind {
		case textdiff.Removed:
			removed++
		case textdiff.Added:
			added++
		}
	}

	fmt.Fprintf(body, "Restore this file to the version from %s?\n\n", m.selectedCommit.Date)
	body.WriteString(headerStyle.Render("File:    ") + m.preview.LivePath + "\n")
	body.WriteString(headerStyle.Render("Changes: ") +
		diffRemovedStyle.Render(countLines(removed, "removed", "removed")) + ", " +
		diffAddedStyle.Render(countLines(added, "comes back", "come back")) + "\n\n")
	// What will run before the file is written
	watch := m.preview.Watch
	if watch.Check != "" || watch.Apply != "" {
		body.WriteString(headerStyle.Render("Before the file is written, on a temporary copy:") + "\n")
		if watch.Check != "" {
			body.WriteString("  1. Validate:  " + watch.Check + "\n")
		}
		if watch.Apply != "" {
			body.WriteString("  2. Apply:     " + watch.Apply + "\n")
		}
		body.WriteString(mutedStyle.Render("If either one fails, nothing is restored.") + "\n\n")
	}

	body.WriteString(mutedStyle.Render("With the service running, the current content is already backed up in\nthe history, and the restore is recorded as a new change.") + "\n")

	if m.configRestoring {
		body.WriteString("\n" + processingStyle.Render("Validating and restoring... Please wait.") + "\n")
		return
	}
	body.WriteString(footerStyle.Render("\ny: restore • n/esc: cancel"))
}

// countLines phrases a number of lines in singular or plural
func countLines(n int, singular, plural string) string {
	if n == 1 {
		return "1 line " + singular
	}
	return fmt.Sprintf("%d lines %s", n, plural)
}

// lastOutputLines keeps the end of a command output, where the error usually is
func lastOutputLines(output string, n int) string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
