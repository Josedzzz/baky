package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Josedzzz/baky/internal/config"
	"github.com/Josedzzz/baky/internal/gitbackup"
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
	case "x":
		if len(watches) > 0 {
			m.state = watchExcludeInputView
			m.watchInput.Placeholder = "*.conf.bak, certs/server.crt, keys/"
			m.watchInput.SetValue(strings.Join(watches[m.watchesCursor].Exclude, ", "))
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
			m.message = "The path must be absolute (start with /)"
			m.isSuccess = false
			return m, nil
		}
		info, err := os.Stat(source)
		if err != nil || !info.IsDir() {
			m.message = "The path does not exist or is not a folder"
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
		m.message = fmt.Sprintf("Folder '%s' added. The service applies the change automatically.", m.pendingSource)
		m.isSuccess = true
		return m, nil
	}
	m.watchInput, cmd = m.watchInput.Update(msg)
	return m, cmd
}

// handleWatchConfirmDeleteUpdate asks before a folder stops being watched
func (m Model) handleWatchConfirmDeleteUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y":
		source := m.configBackup.Watches[m.watchesCursor].Source
		if err := config.RemoveWatch(source); err != nil {
			m.message = err.Error()
			m.isSuccess = false
		} else {
			m.message = fmt.Sprintf("Folder '%s' is no longer watched.", source)
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
	body.WriteString(titleStyle.Render(" WATCHED FOLDERS ") + "\n\n")

	if m.configBackup.RepoDir == "" {
		body.WriteString(errorStyle.Render("No repository is configured yet (Repository menu).") + "\n\n")
	}

	if len(m.configBackup.Watches) == 0 {
		body.WriteString("No folders are being watched.\n\n")
	} else {
		body.WriteString(headerStyle.Render("Watched folder  →  folder in the repository") + "\n")
		for i, w := range m.configBackup.Watches {
			style := itemStyle
			if m.watchesCursor == i {
				style = selectedItemStyle
			}
			body.WriteString(style.Render(w.Source+"  →  "+w.Dest) + "\n")
		}

		// Hook commands of the selected folder
		selected := m.configBackup.Watches[m.watchesCursor]
		body.WriteString("\n" + headerStyle.Render("Validation: ") + commandOrNone(selected.Check) + "\n")
		body.WriteString(headerStyle.Render("Apply:      ") + commandOrNone(selected.Apply) + "\n")
		body.WriteString(headerStyle.Render("Excluded:   ") + excludesOrDefault(selected.Exclude) + "\n\n")
	}

	m.renderMessage(body)
	body.WriteString(footerStyle.Render("a: add • c: validation commands • x: excluded files • d: stop watching • esc: back"))
}

// renderWatchInput renders both steps of adding a watched folder
func (m Model) renderWatchInput(body *strings.Builder) {
	body.WriteString(titleStyle.Render(" ADD WATCHED FOLDER ") + "\n\n")

	if m.state == watchSourceInputView {
		body.WriteString("Step 1 of 2 - Path of the folder to watch:\n\n")
	} else {
		body.WriteString(headerStyle.Render("Folder: ") + itemStyle.Render(m.pendingSource) + "\n\n")
		body.WriteString("Step 2 of 2 - Name of the folder inside the repository:\n\n")
	}
	body.WriteString(m.watchInput.View() + "\n\n")

	m.renderMessage(body)
	body.WriteString(footerStyle.Render("(esc: cancel • enter: continue)"))
}

// renderWatchConfirmDelete renders the confirmation to stop watching a folder
func (m Model) renderWatchConfirmDelete(body *strings.Builder) {
	body.WriteString(titleStyle.Render(" STOP WATCHING ") + "\n\n")

	w := m.configBackup.Watches[m.watchesCursor]
	fmt.Fprintf(body, "Stop watching the folder '%s'?\n\n", w.Source)
	body.WriteString(mutedStyle.Render("No file is deleted: neither the originals nor those already backed\nup in the repository. It just stops being backed up automatically.") + "\n")

	body.WriteString(footerStyle.Render("\ny: confirm • n/esc: cancel"))
}

// commandOrNone shows a hook command, or that there is none
func commandOrNone(command string) string {
	if command == "" {
		return mutedStyle.Render("(none)")
	}
	return command
}

// excludesOrDefault shows the extra exclude patterns of a watch on top of
// the built-in list of secrets
func excludesOrDefault(patterns []string) string {
	if len(patterns) == 0 {
		return mutedStyle.Render("private keys and secrets (built-in list)")
	}
	return strings.Join(patterns, ", ") + mutedStyle.Render(" + private keys and secrets")
}

// handleWatchExcludeInputUpdate handles editing the exclude patterns of a
// watch, typed on one line separated by commas
func (m Model) handleWatchExcludeInputUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg.String() {
	case "esc":
		m.state = watchesView
		m.message = ""
		return m, nil
	case "enter":
		source := m.configBackup.Watches[m.watchesCursor].Source
		if err := config.SetWatchExcludes(source, strings.Split(m.watchInput.Value(), ",")); err != nil {
			m.message = err.Error()
			m.isSuccess = false
			return m, nil
		}
		m.reloadConfigBackup()
		m.state = watchesView
		m.message = "Excluded files saved. Files already backed up that match are removed from the repository, but stay in its history."
		m.isSuccess = true
		return m, nil
	}
	m.watchInput, cmd = m.watchInput.Update(msg)
	return m, cmd
}

// renderWatchExcludeInput renders the editing of the exclude patterns
func (m Model) renderWatchExcludeInput(body *strings.Builder) {
	body.WriteString(titleStyle.Render(" EXCLUDED FILES ") + "\n\n")
	body.WriteString(headerStyle.Render("Folder: ") + itemStyle.Render(m.configBackup.Watches[m.watchesCursor].Source) + "\n\n")

	body.WriteString("Extra patterns that are not backed up, separated by commas (empty = none):\n\n")
	body.WriteString(m.watchInput.View() + "\n\n")
	body.WriteString(mutedStyle.Render("\"*.bak\" matches a name in any folder, \"keys/\" a whole folder, and\n\"certs/server.crt\" that path inside the watched folder.") + "\n\n")
	body.WriteString(mutedStyle.Render("Always excluded, as well as any file that holds a private key:") + "\n")
	body.WriteString(mutedStyle.Width(72).Render(strings.Join(gitbackup.DefaultExcludes, " ")) + "\n\n")

	m.renderMessage(body)
	body.WriteString(footerStyle.Render("(esc: cancel • enter: save)"))
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
		m.message = "Commands saved. The service applies the change automatically."
		m.isSuccess = true
		return m, nil
	}
	m.watchInput, cmd = m.watchInput.Update(msg)
	return m, cmd
}

// renderWatchCommandInput renders both steps of editing the hook commands
func (m Model) renderWatchCommandInput(body *strings.Builder) {
	body.WriteString(titleStyle.Render(" VALIDATION COMMANDS ") + "\n\n")
	body.WriteString(headerStyle.Render("Folder: ") + itemStyle.Render(m.configBackup.Watches[m.watchesCursor].Source) + "\n\n")

	if m.state == watchCheckInputView {
		body.WriteString("Step 1 of 2 - Command that validates the configuration (empty = none):\n\n")
		body.WriteString(m.watchInput.View() + "\n\n")
		body.WriteString(mutedStyle.Render("Runs after every change and before a restore. If it fails, the change\nis still backed up, but it is marked as invalid.") + "\n")
	} else {
		body.WriteString("Step 2 of 2 - Command that applies the configuration (empty = none):\n\n")
		body.WriteString(m.watchInput.View() + "\n\n")
		body.WriteString(mutedStyle.Render("Only runs when a file is restored from the history, after validating\nand before writing the file.") + "\n")
	}
	body.WriteString(mutedStyle.Render("{dir} is replaced by the folder with the configuration. The command runs\nas written: if it needs privileges, it must start with \"sudo -n\".") + "\n\n")

	m.renderMessage(body)
	body.WriteString(footerStyle.Render("(esc: cancel • enter: continue)"))
}
