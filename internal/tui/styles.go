package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	primaryColor   = lipgloss.Color("#7D56F4")
	secondaryColor = lipgloss.Color("#04B575")
	whiteColor     = lipgloss.Color("#EEEEEE")
	grayColor      = lipgloss.Color("#777777")

	// Styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(whiteColor).
			Background(primaryColor).
			Padding(0, 2).
			MarginBottom(1).
			Align(lipgloss.Center)

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

	logSuccessCard = logCardStyle.Copy().
			BorderForeground(secondaryColor)

	logErrorCard = logCardStyle.Copy().
			BorderForeground(lipgloss.Color("#FF0000"))

	logTimeStyle = lipgloss.NewStyle().
			Foreground(grayColor).
			Italic(true)
)
