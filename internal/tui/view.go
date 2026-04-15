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

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		switch m.state {
		case inputView:
			return m.handleInputUpdate(msg)
		case managePathsView:
			return m.handleManagePathsUpdate(msg)
		case configureNasView:
			return m.handleConfigureNasUpdate(msg)
		case nasInputView:
			return m.handleNasInputUpdate(msg)
		case backupFilesView:
			return m.handleBackupFilesUpdate(msg)
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
			case "Configure NAS":
				nasPath, _ := config.GetNasPath()
				m.nasPath = nasPath
				m.state = configureNasView
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
		nasStatus := "Not configured"
		if m.nasPath != "" {
			nasStatus = m.nasPath
		}
		summary := fmt.Sprintf("\nNAS: %s\nPaths: %d monitored", nasStatus, len(m.paths))
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

		case configureNasView:
			body.WriteString(titleStyle.Render(" CONFIGURE NAS ") + "\n\n")
			displayPath := m.nasPath
			if displayPath == "" {
				displayPath = "Not configured"
			}
			fmt.Fprintf(&body, "Current NAS Path: %s\n\n", displayPath)

			if m.message != "" {
				style := errorStyle
				if m.isSuccess {
					style = successStyle
				}
				body.WriteString(style.Render(m.message) + "\n\n")
			}
			body.WriteString(footerStyle.Render("e: edit • t: test • esc: back"))

		case nasInputView:
			body.WriteString(titleStyle.Render(" EDIT NAS PATH ") + "\n\n")
			fmt.Fprintf(&body,
				"Enter NAS share path:\n\n%s\n\n%s",
				m.nasInput.View(),
				footerStyle.Render("(esc to cancel • enter to save)"),
			)
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

// handleConfigureNasUpdate handles key messages in the NAS config view
func (m Model) handleConfigureNasUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "e":
		m.state = nasInputView
		m.nasInput.SetValue(m.nasPath)
		m.nasInput.Focus()
		m.message = ""
	case "t":
		if m.nasPath == "" {
			m.message = "NAS path not configured"
			m.isSuccess = false
			return m, nil
		}
		info, err := os.Stat(m.nasPath)
		if err != nil {
			m.message = "Error: " + err.Error()
			m.isSuccess = false
		} else if !info.IsDir() {
			m.message = "Path is not a directory"
			m.isSuccess = false
		} else {
			m.message = "NAS path is accessible!"
			m.isSuccess = true
		}
	case "esc":
		m.state = menuView
		m.message = ""
	}
	return m, nil
}

// handleNasInputUpdate handles NAS path input
func (m Model) handleNasInputUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg.String() {
	case "esc":
		m.state = configureNasView
		return m, nil
	case "enter":
		val := m.nasInput.Value()
		if val != "" {
			if err := config.SaveNasPath(val); err != nil {
				m.message = "Error saving NAS path"
				m.isSuccess = false
			} else {
				m.nasPath = val
				m.message = "NAS path updated"
				m.isSuccess = true
			}
			m.state = configureNasView
		}
		return m, nil
	}
	m.nasInput, cmd = m.nasInput.Update(msg)
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
