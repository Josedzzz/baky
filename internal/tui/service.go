package tui

import (
	"strings"
	"time"

	"github.com/Josedzzz/baky/internal/config"
	"github.com/Josedzzz/baky/internal/logging"
	"github.com/Josedzzz/baky/internal/service"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	// maxLogLines is how many lines of the daemon log the log screen loads
	maxLogLines = 500
	// logRefreshInterval is how often the log screen reads the log again
	logRefreshInterval = 3 * time.Second
	// maxServiceDetailLines limits the "systemctl status" output shown
	maxServiceDetailLines = 12
)

type serviceStateMsg service.State

type serviceActionMsg struct {
	done string
	err  error
}

type logsLoadedMsg struct {
	lines []string
	err   error
}

type tickLogsMsg struct{}

// queryServiceCmd asks systemd for the state of the service
func queryServiceCmd() tea.Cmd {
	return func() tea.Msg {
		return serviceStateMsg(service.Query())
	}
}

// serviceActionCmd starts, stops or restarts the service
func serviceActionCmd(action func() error, done string) tea.Cmd {
	return func() tea.Msg {
		return serviceActionMsg{done: done, err: action()}
	}
}

// loadLogsCmd reads the last lines of the daemon log
func loadLogsCmd() tea.Cmd {
	return func() tea.Msg {
		lines, err := logging.Tail(config.LogPath(), maxLogLines)
		return logsLoadedMsg{lines: lines, err: err}
	}
}

// tickLogsCmd schedules the next refresh of the log screen
func tickLogsCmd() tea.Cmd {
	return tea.Tick(logRefreshInterval, func(time.Time) tea.Msg {
		return tickLogsMsg{}
	})
}

// handleServiceState stores the answer of a service query
func (m Model) handleServiceState(msg serviceStateMsg) (tea.Model, tea.Cmd) {
	m.service = service.State(msg)
	m.serviceKnown = true
	m.serviceLoading = false
	return m, nil
}

// handleServiceAction shows the result of start, stop or restart and
// refreshes the state
func (m Model) handleServiceAction(msg serviceActionMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.message = msg.err.Error()
		m.isSuccess = false
	} else {
		m.message = msg.done
		m.isSuccess = true
	}
	return m, queryServiceCmd()
}

// handleServiceStatusUpdate handles key messages in the service screen
func (m Model) handleServiceStatusUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		return m.back(serviceMenuView)
	}
	// One action at a time
	if m.serviceLoading {
		return m, nil
	}

	switch msg.String() {
	case "s":
		m.serviceLoading = true
		m.message = ""
		return m, serviceActionCmd(service.Start, "Service started.")
	case "x":
		m.serviceLoading = true
		m.message = ""
		return m, serviceActionCmd(service.Stop, "Service stopped.")
	case "r":
		m.serviceLoading = true
		m.message = ""
		return m, serviceActionCmd(service.Restart, "Service restarted.")
	case "u":
		m.serviceLoading = true
		return m, queryServiceCmd()
	}
	return m, nil
}

// handleLogsLoaded fills the log screen, following the end of the log
// unless the user has scrolled up to read something
func (m Model) handleLogsLoaded(msg logsLoadedMsg) (tea.Model, tea.Cmd) {
	m.logsErr = ""
	if msg.err != nil {
		m.logsErr = msg.err.Error()
		return m, nil
	}

	follow := len(m.logLines) == 0 || m.logsView.AtBottom()
	m.logLines = msg.lines

	styled := make([]string, len(msg.lines))
	for i, line := range msg.lines {
		styled[i] = styleLogLine(line)
	}
	m.logsView.SetContent(strings.Join(styled, "\n"))
	if follow {
		m.logsView.GotoBottom()
	}
	return m, nil
}

// handleLogsUpdate handles key messages in the log screen
func (m Model) handleLogsUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.logLines = nil
		return m.back(serviceMenuView)
	case "end", "G":
		m.logsView.GotoBottom()
		return m, nil
	}
	var cmd tea.Cmd
	m.logsView, cmd = m.logsView.Update(msg)
	return m, cmd
}

// renderServiceStatus renders the service screen
func (m Model) renderServiceStatus(body *strings.Builder) {
	body.WriteString(titleStyle.Render(" SERVICE STATUS ") + "\n\n")
	body.WriteString(headerStyle.Render("Service: ") + service.Name() + "\n")

	switch {
	case !m.serviceKnown || m.serviceLoading:
		body.WriteString(headerStyle.Render("Status:  ") + processingStyle.Render("checking...") + "\n")
	case m.service.Err != nil:
		body.WriteString(headerStyle.Render("Status:  ") + inactiveStyle.Render("UNKNOWN") + "\n\n")
		body.WriteString(errorStyle.Render(m.service.Err.Error()) + "\n")
	case m.service.Active:
		body.WriteString(headerStyle.Render("Status:  ") + activeStyle.Render("ACTIVE") + "\n")
	default:
		body.WriteString(headerStyle.Render("Status:  ") + inactiveStyle.Render("INACTIVE") + mutedStyle.Render(" ("+m.service.Status+")") + "\n")
	}

	if m.serviceKnown && m.service.Detail != "" {
		lines := strings.Split(m.service.Detail, "\n")
		if len(lines) > maxServiceDetailLines {
			lines = lines[:maxServiceDetailLines]
		}
		width := m.logsView.Width
		for i, line := range lines {
			if width > 0 && len([]rune(line)) > width {
				lines[i] = string([]rune(line)[:width-1]) + "…"
			}
		}
		body.WriteString("\n" + mutedStyle.Render(strings.Join(lines, "\n")) + "\n")
	}

	if m.message != "" {
		body.WriteString("\n")
		m.renderMessage(body)
	}

	body.WriteString(footerStyle.Render("\ns: start • x: stop • r: restart • u: update • esc: back"))
}

// renderLogs renders the daemon log screen
func (m Model) renderLogs(body *strings.Builder) {
	body.WriteString(titleStyle.Render(" SERVICE LOGS ") + "\n\n")

	switch {
	case m.logsErr != "":
		body.WriteString(errorStyle.Render("Could not read the log: "+m.logsErr) + "\n")
		body.WriteString(mutedStyle.Render("The file is created the first time the service runs.") + "\n")
	case len(m.logLines) == 0:
		body.WriteString("No entries yet.\n")
	default:
		body.WriteString(m.logsView.View() + "\n")
	}

	body.WriteString(footerStyle.Render("↑/↓: scroll • pgup/pgdn: page • end: go to the end • refreshes every 3 s • esc: back"))
}
