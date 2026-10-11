// Package tui provides the terminal user interface (TUI) implementation
package tui

import (
	"fmt"
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

// View renders the current state of the TUI as a string
func (m Model) View() string {
	if m.quitting {
		return ""
	}

	var body strings.Builder

	if m.state == menuView {
		m.renderMenu(&body)
	} else {
		// Sub-menu View: Mini logo in top-left
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
		}
	}

	// Centering the container in the terminal
	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		containerStyle.Render(body.String()),
	)
}
