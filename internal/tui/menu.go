package tui

import (
	"fmt"
	"strings"

	"github.com/Josedzzz/baky/internal/config"
	tea "github.com/charmbracelet/bubbletea"
)

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

// renderMenu renders the main menu
func (m Model) renderMenu(body *strings.Builder) {
	body.WriteString(asciiStyle.Render(logo) + "\n")
	body.WriteString(titleStyle.Render(" MAIN MENU ") + "\n\n")

	for i, choice := range m.choices {
		if m.cursor == i {
			fmt.Fprintf(body, "%s\n", selectedItemStyle.Render(choice))
		} else {
			fmt.Fprintf(body, "%s\n", itemStyle.Render(choice))
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
}
