// Package tui provides the terminal user interface (TUI) implementation
package tui

import (
	"strings"

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
		m.resizeViewports()
		return m, nil

	case backupFinishedMsg:
		return m.handleBackupFinished(msg)

	case restoreFinishedMsg:
		return m.handleRestoreFinished(msg)

	case commitsLoadedMsg:
		return m.handleCommitsLoaded(msg)

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
		case watchesView:
			return m.handleWatchesUpdate(msg)
		case watchSourceInputView:
			return m.handleWatchSourceInputUpdate(msg)
		case watchDestInputView:
			return m.handleWatchDestInputUpdate(msg)
		case watchConfirmDeleteView:
			return m.handleWatchConfirmDeleteUpdate(msg)
		case configRepoView:
			return m.handleConfigRepoUpdate(msg)
		case configRepoInputView:
			return m.handleConfigRepoInputUpdate(msg)
		case configHistoryView:
			return m.handleConfigHistoryUpdate(msg)
		default:
			return m.handleMenuUpdate(msg)
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

	if isMenu(m.state) {
		m.renderMenu(&body)
	} else {
		// Screen: Mini logo in top-left
		body.WriteString(miniLogoStyle.Render(miniLogo) + "\n")

		switch m.state {
		case inputView:
			m.renderPathInput(&body)
		case managePathsView:
			m.renderManagePaths(&body)
		case backupFilesView:
			m.renderBackupFiles(&body)
		case configureBackupDestView:
			m.renderBackupDest(&body)
		case backupDestInputView:
			m.renderBackupDestInput(&body)
		case viewBackupsView:
			m.renderViewBackups(&body)
		case selectRestoreDestView:
			m.renderSelectRestoreDest(&body)
		case restoreInputView:
			m.renderRestoreInput(&body)
		case selectRestoreActionView:
			m.renderSelectRestoreAction(&body)
		case watchesView:
			m.renderWatches(&body)
		case watchSourceInputView, watchDestInputView:
			m.renderWatchInput(&body)
		case watchConfirmDeleteView:
			m.renderWatchConfirmDelete(&body)
		case configRepoView:
			m.renderConfigRepo(&body)
		case configRepoInputView:
			m.renderConfigRepoInput(&body)
		case configHistoryView:
			m.renderConfigHistory(&body)
		}
	}

	// Centering the container in the terminal
	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		containerStyle.Render(body.String()),
	)
}

// renderMessage writes the current status message, if any
func (m Model) renderMessage(body *strings.Builder) {
	if m.message == "" {
		return
	}
	style := errorStyle
	if m.isSuccess {
		style = successStyle
	}
	body.WriteString(style.Render(m.message) + "\n")
}

// resizeViewports fits the scrolling areas to the terminal, leaving room
// for the logo, the title and the footer
func (m *Model) resizeViewports() {
	width := m.width - 8
	if width > 110 {
		width = 110
	}
	if width < 20 {
		width = 20
	}
	height := m.height - 12
	if height < 3 {
		height = 3
	}

	m.logsView.Width, m.logsView.Height = width, height
	m.commitsView.Width, m.commitsView.Height = width, height
}

// reloadPaths refreshes the file backup settings from the config
func (m *Model) reloadPaths() {
	m.paths, _ = config.GetPaths()
	m.history, _ = config.GetHistory()
	m.backupDest, _ = config.GetNasPath()
}

// reloadConfigBackup refreshes the config backup settings from the config
func (m *Model) reloadConfigBackup() {
	m.configBackup, _ = config.GetConfigBackup()
	if m.watchesCursor >= len(m.configBackup.Watches) {
		m.watchesCursor = 0
	}
}

// actionLabel returns the name of a restore action as stored in the history
func actionLabel(action restore.RestoreAction) string {
	switch action {
	case restore.RestoreActionRename:
		return "rename"
	case restore.RestoreActionSkip:
		return "skip"
	}
	return "overwrite"
}
