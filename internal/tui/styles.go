package tui

import "github.com/charmbracelet/lipgloss"

var (
	colorAccent   = lipgloss.Color("212") // pink
	colorSubtle   = lipgloss.Color("241") // grey
	colorFresh    = lipgloss.Color("78")  // green
	colorAging    = lipgloss.Color("221") // yellow
	colorStale    = lipgloss.Color("203") // red
	colorNever    = lipgloss.Color("167") // deep red
	colorHeaderBg = lipgloss.Color("236")
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("231")).
			Background(colorAccent).
			Padding(0, 1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(colorSubtle)

	helpStyle = lipgloss.NewStyle().
			Foreground(colorSubtle)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("231"))

	spinnerStyle = lipgloss.NewStyle().Foreground(colorAccent)

	detailBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorAccent).
			Padding(0, 2)

	detailKeyStyle = lipgloss.NewStyle().
			Foreground(colorSubtle).
			Width(12)

	detailValStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("231"))

	filterPromptStyle = lipgloss.NewStyle().Foreground(colorAccent)

	accuracyHintStyle = lipgloss.NewStyle().Foreground(colorAging)
)

// stalenessColor picks a row color based on how long since the app was used.
func stalenessColor(daysSinceUsed int, thresholdDays int) lipgloss.Color {
	switch {
	case daysSinceUsed < 0:
		return colorNever
	case daysSinceUsed >= thresholdDays:
		return colorStale
	case daysSinceUsed >= thresholdDays/2:
		return colorAging
	default:
		return colorFresh
	}
}
