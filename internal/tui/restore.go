package tui

import (
	"fmt"
	"math"
	"strings"

	"github.com/Josedzzz/baky/internal/config"
	"github.com/Josedzzz/baky/internal/restore"
	tea "github.com/charmbracelet/bubbletea"
)

type restoreFinishedMsg struct {
	result *restore.RestoreResult
	err    error
}

// loadBackups loads backups from NAS with full paths and real status from history
func (m *Model) loadBackups() {
	nasPath, _ := config.GetNasPath()
	if nasPath == "" {
		m.message = "Backup destination not configured"
		m.isSuccess = false
		m.allBackups = []restore.BackupInfo{}
		return
	}

	paths, _ := config.GetPaths()
	configPaths := make([]restore.ConfigPath, len(paths))
	for i, p := range paths {
		configPaths[i] = restore.ConfigPath{Path: p.Path}
	}

	hist, _ := config.GetHistory()
	historyEvents := make([]restore.HistoryEvent, len(hist))
	for i, h := range hist {
		historyEvents[i] = restore.HistoryEvent{
			SourcePath: h.Path,
			Timestamp:  h.Timestamp,
			Result:     h.Result,
			Message:    h.Message,
		}
	}

	backups, err := restore.GetAllBackupsEnhanced(nasPath, configPaths, historyEvents)
	if err != nil {
		m.message = "Error loading backups: " + err.Error()
		m.isSuccess = false
		m.allBackups = []restore.BackupInfo{}
		return
	}

	m.allBackups = backups
	m.message = fmt.Sprintf("Found %d backups", len(backups))
	m.isSuccess = true
}

// handleViewBackupsUpdate handles key messages in the view backups
func (m Model) handleViewBackupsUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.backupsCursor > 0 {
			m.backupsCursor--
			// Adjust offset if needed
			if m.backupsCursor < m.backupsOffset {
				m.backupsOffset = m.backupsCursor
			}
		}
	case "down", "j":
		if m.backupsCursor < len(m.allBackups)-1 {
			m.backupsCursor++
			// Adjust offset if needed
			visibleBackups := 6
			if m.backupsCursor >= m.backupsOffset+visibleBackups {
				m.backupsOffset = m.backupsCursor - visibleBackups + 1
			}
		}
	case "pgup":
		// Scroll up
		if m.backupsOffset > 0 {
			m.backupsOffset--
			if m.backupsCursor > m.backupsOffset+6 {
				m.backupsCursor = m.backupsOffset + 6
			}
		}
	case "pgdown":
		visibleBackups := 6
		maxOffset := int(math.Max(float64(len(m.allBackups)-visibleBackups), 0))
		if m.backupsOffset < maxOffset {
			m.backupsOffset++
			if m.backupsCursor < m.backupsOffset {
				m.backupsCursor = m.backupsOffset
			}
		}
	case "enter":
		// Select backup for restore
		if len(m.allBackups) > 0 && m.backupsCursor < len(m.allBackups) {
			selectedBackup := m.allBackups[m.backupsCursor]
			m.selectedBackup = &selectedBackup
			m.state = selectRestoreDestView
			m.restoreActionIndex = 0
			m.message = ""
		}
	case "esc":
		m.state = menuView
		m.message = ""
		m.allBackups = []restore.BackupInfo{}
	}
	return m, nil
}

// handleSelectRestoreDestUpdate handles destination selection (original or custom)
func (m Model) handleSelectRestoreDestUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "o":
		// Use original source path
		m.restorePath = m.selectedBackup.SourcePath
		m.state = selectRestoreActionView
		m.restoreActionIndex = 0
		m.message = ""
	case "c":
		// Choose custom path
		m.state = restoreInputView
		m.restoreInput.Reset()
		m.restoreInput.Focus()
		m.message = ""
	case "esc":
		m.state = viewBackupsView
		m.selectedBackup = nil
		m.message = ""
	}
	return m, nil
}

// handleSelectRestoreActionUpdate handles conflict resolution action selection
func (m Model) handleSelectRestoreActionUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.restoreActionIndex > 0 {
			m.restoreActionIndex--
		}
	case "down", "j":
		if m.restoreActionIndex < 2 {
			m.restoreActionIndex++
		}
	case "enter":
		// A restore is already running: ignore repeated presses
		if m.isRestoring {
			return m, nil
		}
		// Perform the restore
		m.isRestoring = true
		m.message = "Restoring... Please wait."

		actions := []restore.RestoreAction{
			restore.RestoreActionOverwrite,
			restore.RestoreActionRename,
			restore.RestoreActionSkip,
		}
		m.restoreAction = actions[m.restoreActionIndex]

		// Execute restore in background
		return m, func() tea.Msg {
			result, err := restore.Restore(m.selectedBackup.FullPath, m.restorePath, m.restoreAction)
			return restoreFinishedMsg{result: result, err: err}
		}
	case "esc":
		m.state = selectRestoreDestView
		m.message = ""
	}
	return m, nil
}

// handleRestoreInputUpdate handles custom restore path input
func (m Model) handleRestoreInputUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg.String() {
	case "esc":
		m.state = selectRestoreDestView
		m.restoreInput.Reset()
		return m, nil
	case "enter":
		path := m.restoreInput.Value()
		if path != "" {
			// Validate path
			if err := restore.ValidateRestorePath(path); err != nil {
				m.message = "Invalid path: " + err.Error()
				m.isSuccess = false
				return m, nil
			}
			m.restorePath = path
			m.state = selectRestoreActionView
			m.restoreActionIndex = 0
			m.restoreInput.Reset()
			m.message = ""
		}
		return m, nil
	}
	m.restoreInput, cmd = m.restoreInput.Update(msg)
	return m, cmd
}

// renderViewBackups renders the list of available backups
func (m Model) renderViewBackups(body *strings.Builder) {
	body.WriteString(titleStyle.Render(" VIEW BACKUPS ") + "\n\n")
	if len(m.allBackups) == 0 {
		body.WriteString("No backups found.\n\n")
	} else {
		body.WriteString(headerStyle.Render("Available Backups:") + "\n")
		// Show up to 6 backups at a time
		visibleBackups := 6
		for i := m.backupsOffset; i < len(m.allBackups) && i < m.backupsOffset+visibleBackups; i++ {
			b := m.allBackups[i]
			style := itemStyle
			if m.backupsCursor-m.backupsOffset == i-m.backupsOffset {
				style = selectedItemStyle
			}

			// Format the backup info
			status := "✓"
			if b.Result != "success" {
				status = "✗"
			}
			sizeStr := restore.FormatFileSize(b.FileSize)
			timeStr := b.Timestamp.Format("2006-01-02 15:04:05")
			line := fmt.Sprintf("%s %s | %s | %s | %s",
				status,
				timeStr,
				sizeStr,
				b.SourcePath,
				b.Filename)

			body.WriteString(style.Render(line) + "\n")
		}

		// Add scroll indicator
		if len(m.allBackups) > visibleBackups {
			scrollPercentage := int((float64(m.backupsOffset+visibleBackups) / float64(len(m.allBackups))) * 100)
			scrollIndicator := fmt.Sprintf("\n[Backups: %d/%d - %d%%]", m.backupsOffset+1, len(m.allBackups), scrollPercentage)
			body.WriteString(statusStyle.Render(scrollIndicator) + "\n")
		}
	}

	if m.message != "" {
		style := errorStyle
		if m.isSuccess {
			style = successStyle
		}
		body.WriteString(style.Render(m.message) + "\n")
	}

	body.WriteString(footerStyle.Render("\nenter: restore • esc: back • ↑/↓: scroll • pgup/pgdn: scroll more"))

}

// renderSelectRestoreDest renders the restore destination selection screen
func (m Model) renderSelectRestoreDest(body *strings.Builder) {
	body.WriteString(titleStyle.Render(" SELECT RESTORE DESTINATION ") + "\n\n")
	body.WriteString(headerStyle.Render("Backup: ") + itemStyle.Render(m.selectedBackup.Filename) + "\n")
	body.WriteString(headerStyle.Render("Source: ") + itemStyle.Render(m.selectedBackup.SourcePath) + "\n\n")

	body.WriteString(headerStyle.Render("Where do you want to restore?") + "\n\n")
	body.WriteString("o - Original location: " + m.selectedBackup.SourcePath + "\n")
	body.WriteString("c - Choose custom path\n\n")

	if m.message != "" {
		style := errorStyle
		if m.isSuccess {
			style = successStyle
		}
		body.WriteString(style.Render(m.message) + "\n")
	}

	body.WriteString(footerStyle.Render("o: original • c: custom • esc: back"))

}

// renderRestoreInput renders the custom restore path input screen
func (m Model) renderRestoreInput(body *strings.Builder) {
	body.WriteString(titleStyle.Render(" ENTER RESTORE PATH ") + "\n\n")
	fmt.Fprintf(body,
		"Enter restore destination path:\n\n%s\n\n%s",
		m.restoreInput.View(),
		footerStyle.Render("(esc to cancel • enter to confirm)"),
	)

}

// renderSelectRestoreAction renders the conflict resolution screen
func (m Model) renderSelectRestoreAction(body *strings.Builder) {
	body.WriteString(titleStyle.Render(" CONFLICT RESOLUTION ") + "\n\n")
	body.WriteString(headerStyle.Render("Restore to: ") + itemStyle.Render(m.restorePath) + "\n\n")
	body.WriteString(headerStyle.Render("Files exist at destination. What should we do?") + "\n\n")

	actions := []string{
		"✓ Overwrite - Replace existing files",
		"↻ Rename    - Rename existing files with timestamp",
		"✗ Skip      - Don't restore",
	}

	for i, action := range actions {
		style := itemStyle
		if m.restoreActionIndex == i {
			style = selectedItemStyle
		}
		body.WriteString(style.Render(action) + "\n")
	}

	body.WriteString("\n")
	if m.isRestoring {
		body.WriteString(processingStyle.Render("Restoring... Please wait.") + "\n")
	} else if m.message != "" {
		style := errorStyle
		if m.isSuccess {
			style = successStyle
		}
		body.WriteString(style.Render(m.message) + "\n")
	}

	body.WriteString(footerStyle.Render("\n↑/↓: select • enter: confirm • esc: back"))
}
