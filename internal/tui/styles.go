package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	primaryColor   = lipgloss.Color("#73D1FF")
	secondaryColor = lipgloss.Color("#ADE8FF")
	whiteColor     = lipgloss.Color("#FFFFFF")
	grayColor      = lipgloss.Color("#A0A0A0")

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
				PaddingLeft(0).
				Foreground(primaryColor).
				Bold(true)

	headerStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true).
			MarginTop(1).
			MarginBottom(1)

	processingStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Italic(true)

	footerStyle = lipgloss.NewStyle().
			Foreground(grayColor).
			Italic(true).
			MarginTop(1)

	containerStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(1).
			Width(60).
			Height(25)

	successStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Bold(true).
			MarginTop(1)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Bold(true).
			MarginTop(1)

	// Log Card Styles
	logCardStyle = lipgloss.NewStyle().
			Padding(0, 1).
			MarginBottom(1).
			Border(lipgloss.NormalBorder(), false, false, false, true)

	logSuccessCard = logCardStyle.
			BorderForeground(secondaryColor)

	logErrorCard = logCardStyle.
			BorderForeground(lipgloss.Color("#FF0000"))

	logTimeStyle = lipgloss.NewStyle().
			Foreground(grayColor).
			Italic(true)
)
