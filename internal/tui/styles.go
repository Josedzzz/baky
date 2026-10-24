package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors
	primaryColor   = lipgloss.Color("#778DA9")
	secondaryColor = lipgloss.Color("#415A77")
	whiteColor     = lipgloss.Color("#FFFFFF")
	grayColor      = lipgloss.Color("#A0A0A0")
	okColor        = lipgloss.Color("#00FF87")
	warnColor      = lipgloss.Color("#FFD866")
	errorColor     = lipgloss.Color("#FF5F5F")

	// Styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(whiteColor).
			Background(primaryColor).
			Padding(0, 2).
			MarginBottom(1)

	miniLogoStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(secondaryColor).
			PaddingLeft(1).
			MarginBottom(1)

	asciiStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true).
			MarginBottom(1)

	itemStyle = lipgloss.NewStyle().
			PaddingLeft(2)

	selectedItemStyle = lipgloss.NewStyle().
				PaddingLeft(1).
				PaddingRight(1).
				Foreground(whiteColor).
				Background(secondaryColor).
				Bold(true)

	statusStyle = lipgloss.NewStyle().
			Foreground(grayColor).
			MarginTop(1).
			Italic(true)

	headerStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true)

	processingStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Italic(true)

	footerStyle = lipgloss.NewStyle().
			Foreground(grayColor).
			Italic(true).
			MarginTop(1)

	containerStyle = lipgloss.NewStyle().
			Padding(1, 2)

	successStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Bold(true).
			MarginTop(1)

	errorStyle = lipgloss.NewStyle().
			Foreground(errorColor).
			Bold(true)

	// Service state styles
	activeStyle = lipgloss.NewStyle().
			Foreground(okColor).
			Bold(true)

	inactiveStyle = lipgloss.NewStyle().
			Foreground(errorColor).
			Bold(true)

	mutedStyle = lipgloss.NewStyle().
			Foreground(grayColor)

	// Log Card Styles
	logCardStyle = lipgloss.NewStyle().
			Padding(0, 1).
			Border(lipgloss.NormalBorder(), false, false, false, true)

	logSuccessCard = logCardStyle.
			BorderForeground(secondaryColor)

	logErrorCard = logCardStyle.
			BorderForeground(errorColor)

	logTimeStyle = lipgloss.NewStyle().
			Foreground(grayColor).
			Italic(true)

	// Diff styles: what a restore would remove from and bring back to a file
	diffRemovedStyle = lipgloss.NewStyle().Foreground(errorColor)
	diffAddedStyle   = lipgloss.NewStyle().Foreground(okColor)
	diffHunkStyle    = lipgloss.NewStyle().Foreground(primaryColor).Bold(true)

	// Daemon log line styles, by level
	logWarnStyle  = lipgloss.NewStyle().Foreground(warnColor)
	logErrorStyle = lipgloss.NewStyle().Foreground(errorColor)
)

// styleLogLine colors a line of the daemon log according to its level
func styleLogLine(line string) string {
	switch {
	case strings.Contains(line, "[ERROR]"):
		return logErrorStyle.Render(line)
	case strings.Contains(line, "[WARN]"):
		return logWarnStyle.Render(line)
	default:
		return line
	}
}
