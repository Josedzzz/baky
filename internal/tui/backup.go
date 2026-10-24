package tui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/Josedzzz/baky/internal/backup"
	tea "github.com/charmbracelet/bubbletea"
)

type backupFinishedMsg struct {
	err  error
	path string
}

// handleBackupFinished shows the result of a manual backup
func (m Model) handleBackupFinished(msg backupFinishedMsg) (tea.Model, tea.Cmd) {
	m.isProcessing = false
	// Refresh history and paths: a failed backup is recorded too
	m.reloadPaths()
	if msg.err != nil {
		m.message = "Backup failed: " + msg.err.Error()
		m.isSuccess = false
	} else {
		m.message = "Backup completed successfully."
		m.isSuccess = true
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
		return m.back(filesMenuView)
	}
	return m, nil
}

func (m Model) formatRelativeTime(t time.Time) string {
	if t.IsZero() {
		return "never"
	}

	diff := time.Since(t)
	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		return fmt.Sprintf("%d min ago", int(diff.Minutes()))
	case diff < 24*time.Hour:
		return fmt.Sprintf("%d h ago", int(diff.Hours()))
	default:
		return t.Format("2006-01-02 15:04")
	}
}

// renderBackupFiles renders the manual backup screen with the recent history
func (m Model) renderBackupFiles(body *strings.Builder) {
	body.WriteString(titleStyle.Render(" BACK UP NOW ") + "\n\n")
	if len(m.paths) == 0 {
		body.WriteString("No paths are configured for backup.\n\n")
	} else {
		body.WriteString(headerStyle.Render("Select the path to back up:") + "\n")
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
		body.WriteString(processingStyle.Render("Backing up... Please wait.") + "\n\n")
	} else if m.message != "" {
		m.renderMessage(body)
		body.WriteString("\n")
	}

	body.WriteString(headerStyle.Render("Recent backups:") + "\n")
	if len(m.history) == 0 {
		body.WriteString("No history.\n")
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
			scrollIndicator := fmt.Sprintf("\n[History: %d/%d - %d%%]", m.historyOffset+1, len(m.history), scrollPercentage)
			body.WriteString(statusStyle.Render(scrollIndicator) + "\n")
		}
	}
	body.WriteString(footerStyle.Render("\nenter: back up • ↑/↓: paths • pgup/pgdn: history • esc: back"))
}
