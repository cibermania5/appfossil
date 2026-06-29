package tui

import tea "github.com/charmbracelet/bubbletea"

// Run launches the interactive TUI and blocks until the user quits.
func Run(cfg Config) error {
	p := tea.NewProgram(NewModel(cfg), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
