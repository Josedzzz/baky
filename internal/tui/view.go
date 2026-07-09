// Package tui provides the terminal user interface (TUI) implementation
package tui

import (
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"github.com/Josedzzz/baky/internal/backup"
	"github.com/Josedzzz/baky/internal/config"
	"github.com/Josedzzz/baky/internal/restore"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const logo = `
 ██████╗  █████╗ ██╗  ██╗██╗   ██╗
 ██╔══██╗██╔══██╗██║ ██╔╝╚██╗ ██╔╝
 ██████╔╝███████║█████╔╝  ╚████╔╝ 
 ██╔══██╗██╔══██║██╔═██╗   ╚██╔╝  
 ██████╔╝██║  ██║██║  ██╗   ██║   
 ╚═════╝ ╚═╝  ╚═╝╚═╝  ╚═╝   ╚═╝   
`

const miniLogo = " BAKY"

type backupFinishedMsg struct {
	err  error
	path string
}

type restoreFinishedMsg struct {
	result *restore.RestoreResult
	err    error
}

// Init initializes the TUI model and returns the initial command
func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles messages and updates the TUI model accordingly
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case backupFinishedMsg:
		m.isProcessing = false
		if msg.err != nil {
			m.message = "Backup failed: " + msg.err.Error()
			m.isSuccess = false
		} else {
			m.message = "Backup finished successfully!"
			m.isSuccess = true
			// Refresh history and paths
			hist, _ := config.GetHistory()
			m.history = hist
			paths, _ := config.GetPaths()
			m.paths = paths
		}
		return m, nil

	case restoreFinishedMsg:
		m.isRestoring = false
		if msg.err != nil {
			m.message = "Restore failed: " + msg.err.Error()
			m.isSuccess = false
		} else if msg.result != nil {
			if msg.result.Success {
				m.message = fmt.Sprintf("Restore successful! Files restored to %s", msg.result.DestinationPath)
				m.isSuccess = true
				// Log restore event
				actionStr := "overwrite"
				switch msg.result.Action {
				case restore.RestoreActionRename:
					actionStr = "rename"
				case restore.RestoreActionSkip:
					actionStr = "skip"
				}
				config.LogRestore(
					m.selectedBackup.Filename,
					m.selectedBackup.SourcePath,
					msg.result.DestinationPath,
					actionStr,
					true,
					msg.result.Message,
				)
			} else {
				m.message = "Restore incomplete: " + msg.result.Message
				m.isSuccess = false
			}
		}
		// Refresh backup list and preserve the result message
		savedMsg := m.message
		savedSuccess := m.isSuccess
		m.loadBackups()
		m.message = savedMsg
		m.isSuccess = savedSuccess
		m.state = viewBackupsView
		m.selectedBackup = nil
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		switch m.state {
		case inputView:
			return m.handleInputUpdate(msg)
		case managePathsView:
			return m.handleManagePathsUpdate(msg)
		case configureBackupDestView:
			return m.handleConfigureBackupDestUpdate(msg)
		case backupDestInputView:
			return m.handleBackupDestInputUpdate(msg)
		case backupFilesView:
			return m.handleBackupFilesUpdate(msg)
		case viewBackupsView:
			return m.handleViewBackupsUpdate(msg)
		case selectRestoreDestView:
			return m.handleSelectRestoreDestUpdate(msg)
		case selectRestoreActionView:
			return m.handleSelectRestoreActionUpdate(msg)
		case restoreInputView:
			return m.handleRestoreInputUpdate(msg)
		default:
			return m.handleMenuUpdate(msg)
		}
	}
	return m, nil
}

// handleMenuUpdate processes key messages when the TUI is in the menu view
func (m Model) handleMenuUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}
		case "enter":
			choice := m.choices[m.cursor]
			switch choice {
			case "Manage Paths":
				paths, _ := config.GetPaths()
				m.paths = paths
				m.state = managePathsView
				m.pathsCursor = 0
				m.message = ""
			case "Backup Files":
				paths, _ := config.GetPaths()
				m.paths = paths
				hist, _ := config.GetHistory()
				m.history = hist
				m.state = backupFilesView
				m.pathsCursor = 0
				m.historyOffset = 0
				m.message = ""
			case "View Backups":
				m.loadBackups()
				m.state = viewBackupsView
				m.backupsCursor = 0
				m.backupsOffset = 0
			case "Backup Destination":
				backupDest, _ := config.GetNasPath()
				m.backupDest = backupDest
				m.state = configureBackupDestView
				m.message = ""
			case "Exit":
				m.quitting = true
				return m, tea.Quit
			}
		case "q":
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

// View renders the current state of the TUI as a string
func (m Model) View() string {
	if m.quitting {
		return ""
	}

	var body strings.Builder

	if m.state == menuView {
		body.WriteString(asciiStyle.Render(logo) + "\n")
		body.WriteString(titleStyle.Render(" MAIN MENU ") + "\n\n")

		for i, choice := range m.choices {
			if m.cursor == i {
				fmt.Fprintf(&body, "%s\n", selectedItemStyle.Render(choice))
			} else {
				fmt.Fprintf(&body, "%s\n", itemStyle.Render(choice))
			}
		}

		// Status Summary
		destStatus := "Not configured"
		if m.backupDest != "" {
			destStatus = m.backupDest
		}
		summary := fmt.Sprintf("\nBackup Destination: %s\nPaths: %d monitored", destStatus, len(m.paths))
		body.WriteString(statusStyle.Render(summary) + "\n")

		if m.message != "" {
			style := errorStyle
			if m.isSuccess {
				style = successStyle
			}
			body.WriteString(style.Render(m.message) + "\n")
		}

		body.WriteString(footerStyle.Render("\n↑/↓: navigate • enter: select • q: quit"))
	} else {
		// Sub-menu View: Mini logo in top-left
		body.WriteString(miniLogoStyle.Render(miniLogo) + "\n")

		switch m.state {
		case inputView:
			title := " ADD BACKUP PATH "
			if m.editingIndex != -1 {
				title = " UPDATE BACKUP PATH "
			}
			body.WriteString(titleStyle.Render(title) + "\n\n")
			fmt.Fprintf(&body,
				"Enter path:\n\n%s\n\n%s",
				m.pathInput.View(),
				footerStyle.Render("(esc to cancel • enter to save)"),
			)

		case managePathsView:
			body.WriteString(titleStyle.Render(" MANAGE BACKUP PATHS ") + "\n\n")
			if len(m.paths) == 0 {
				body.WriteString("No paths added yet.\n\n")
			} else {
				for i, p := range m.paths {
					style := itemStyle
					label := p.Path
					if m.pathsCursor == i {
						style = selectedItemStyle
					}
					last := "Never"
					if !p.LastBackup.IsZero() {
						last = p.LastBackup.Format("2006-01-02 15:04")
					}

					var freqLabel string
					switch p.Frequency {
					case config.FreqWeekly:
						freqLabel = "[W]"
					case config.FreqOnChange:
						freqLabel = "[C]"
					default:
						freqLabel = "[D]"
					}

					line := fmt.Sprintf("%-20s %s [%s] (Last: %s)", label, freqLabel, p.Frequency, last)

					body.WriteString(style.Render(line) + "\n")
				}
				body.WriteString("\n")
			}
			if m.message != "" {
				style := errorStyle
				if m.isSuccess {
					style = successStyle
				}
				body.WriteString(style.Render(m.message) + "\n")
			}
			body.WriteString(footerStyle.Render("a: add • e: edit • f: cycle freq • d: delete • esc: back"))

		case backupFilesView:
			body.WriteString(titleStyle.Render(" BACKUP FILES ") + "\n\n")
			if len(m.paths) == 0 {
				body.WriteString("No paths configured to backup.\n\n")
			} else {
				body.WriteString(headerStyle.Render("Select path for manual backup:") + "\n")
				// Only show up to 3 paths to save space
				start := 0
				if m.pathsCursor > 2 {
					start = m.pathsCursor - 2
				}
				for i := start; i < len(m.paths) && i < start+3; i++ {
					p := m.paths[i]
					style := itemStyle
					if m.pathsCursor == i {
						style = selectedItemStyle
					}
					body.WriteString(style.Render(p.Path) + "\n")
				}
				body.WriteString("\n")
			}

			if m.isProcessing {
				body.WriteString(processingStyle.Render("Processing backup... Please wait.") + "\n\n")
			} else if m.message != "" {
				style := errorStyle
				if m.isSuccess {
					style = successStyle
				}
				body.WriteString(style.Render(m.message) + "\n\n")
			}

			body.WriteString(headerStyle.Render("Recent Backups:") + "\n")
			if len(m.history) == 0 {
				body.WriteString("No history available.\n")
			} else {
				// Show up to 4 history items with scrolling
				visibleHistory := 4
				for i := m.historyOffset; i < len(m.history) && i < m.historyOffset+visibleHistory; i++ {
					h := m.history[i]
					status := "OK"
					card := logSuccessCard
					if h.Result != "success" {
						status = "ERROR"
						card = logErrorCard
					}

					timestamp := m.formatRelativeTime(h.Timestamp)
					msg := fmt.Sprintf("%s - %s %s",
						card.Render(status),
						logTimeStyle.Render(timestamp),
						itemStyle.Render(h.Path))

					body.WriteString(msg + "\n")
				}

				// Add scroll indicator
				if len(m.history) > visibleHistory {
					scrollPercentage := int((float64(m.historyOffset+visibleHistory) / float64(len(m.history))) * 100)
					scrollIndicator := fmt.Sprintf("\n[Logs: %d/%d - %d%%]", m.historyOffset+1, len(m.history), scrollPercentage)
					body.WriteString(statusStyle.Render(scrollIndicator) + "\n")
				}
			}
			body.WriteString(footerStyle.Render("\nenter: start • esc: back • ↑/↓: paths • pgup/pgdn: logs"))

		case configureBackupDestView:
			body.WriteString(titleStyle.Render(" BACKUP DESTINATION ") + "\n\n")
			displayPath := m.backupDest
			if displayPath == "" {
				displayPath = "Not configured"
			}
			fmt.Fprintf(&body, "Backup Destination: %s\n\n", displayPath)

			if m.message != "" {
				style := errorStyle
				if m.isSuccess {
					style = successStyle
				}
				body.WriteString(style.Render(m.message) + "\n\n")
			}
			body.WriteString(footerStyle.Render("e: edit • t: test • esc: back"))

		case backupDestInputView:
			body.WriteString(titleStyle.Render(" EDIT BACKUP DESTINATION ") + "\n\n")
			fmt.Fprintf(&body,
				"Enter backup destination path:\n\n%s\n\n%s",
				m.backupDestInput.View(),
				footerStyle.Render("(esc to cancel • enter to save)"),
			)

		case viewBackupsView:
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

		case selectRestoreDestView:
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

		case restoreInputView:
			body.WriteString(titleStyle.Render(" ENTER RESTORE PATH ") + "\n\n")
			fmt.Fprintf(&body,
				"Enter restore destination path:\n\n%s\n\n%s",
				m.restoreInput.View(),
				footerStyle.Render("(esc to cancel • enter to confirm)"),
			)

		case selectRestoreActionView:
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
	}

	// Centering the container in the terminal
	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		containerStyle.Render(body.String()),
	)
}

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
			config.SavePaths(m.paths)
			backup.UpdateWatcher()
		}
	case "d":
		if len(m.paths) > 0 {
			m.paths = append(m.paths[:m.pathsCursor], m.paths[m.pathsCursor+1:]...)
			if m.pathsCursor >= len(m.paths) && m.pathsCursor > 0 {
				m.pathsCursor--
			}
			config.SavePaths(m.paths)
			backup.UpdateWatcher()
			m.message = "Path deleted"
			m.isSuccess = true
		}
	case "esc":
		m.state = menuView
		m.message = ""
	}
	return m, nil
}

func (m Model) handleBackupFilesUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.pathsCursor > 0 {
			m.pathsCursor--
		}
	case "down", "j":
		if m.pathsCursor < len(m.paths)-1 {
			m.pathsCursor++
		}
	case "pgup": // Scroll history up
		if m.historyOffset > 0 {
			m.historyOffset--
		}
	case "pgdown": // Scroll history down
		maxOffset := int(math.Max(0, float64(len(m.history)-4)))
		if m.historyOffset < maxOffset {
			m.historyOffset++
		}
	case "enter":
		if len(m.paths) > 0 && !m.isProcessing {
			m.isProcessing = true
			m.message = ""
			path := m.paths[m.pathsCursor].Path
			return m, func() tea.Msg {
				err := backup.RunBackup(path)
				return backupFinishedMsg{err: err, path: path}
			}
		}
	case "esc":
		m.state = menuView
		m.message = ""
	}
	return m, nil
}

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
			m.message = "Backup destination not configured"
			m.isSuccess = false
			return m, nil
		}
		info, err := os.Stat(m.backupDest)
		if err != nil {
			m.message = "Error: " + err.Error()
			m.isSuccess = false
		} else if !info.IsDir() {
			m.message = "Path is not a directory"
			m.isSuccess = false
		} else {
			m.message = "Backup destination is accessible!"
			m.isSuccess = true
		}
	case "esc":
		m.state = menuView
		m.message = ""
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
		val := m.backupDestInput.Value()
		if val != "" {
			if err := config.SaveNasPath(val); err != nil {
				m.message = "Error saving backup destination"
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
		val := m.pathInput.Value()
		if val != "" {
			if m.editingIndex == -1 {
				m.paths = append(m.paths, config.BackupPathConfig{Path: val, Frequency: config.FreqDaily})
				m.message = "Path added"
			} else {
				m.paths[m.editingIndex].Path = val
				m.message = "Path updated"
			}
			config.SavePaths(m.paths)
			backup.UpdateWatcher()
			m.isSuccess = true
			m.state = managePathsView
			m.pathInput.Reset()
		}
		return m, nil
	}
	m.pathInput, cmd = m.pathInput.Update(msg)
	return m, cmd
}

func (m Model) formatRelativeTime(t time.Time) string {
	if t.IsZero() {
		return "Never"
	}

	diff := time.Since(t)
	switch {
	case diff < time.Minute:
		return "Just now"
	case diff < time.Hour:
		return fmt.Sprintf("%d mins ago", int(diff.Minutes()))
	case diff < 24*time.Hour:
		return fmt.Sprintf("%d hours ago", int(diff.Hours()))
	default:
		return t.Format("Jan 02 15:04")
	}
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
