package ui

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39")).MarginBottom(1)
	normalStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
	keyStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	accentStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	dimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	helpStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	selectedRowStyle = lipgloss.NewStyle().Background(lipgloss.Color("39")).Foreground(lipgloss.Color("0")).Bold(true)
	headerRowStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))

	// DML pending-change cell colors (mirror the tview palette).
	dmlChangeStyle = lipgloss.NewStyle().Background(lipgloss.Color("214")).Foreground(lipgloss.Color("0")) // edited: orange
	dmlDeleteStyle = lipgloss.NewStyle().Background(lipgloss.Color("160")).Foreground(lipgloss.Color("15")) // deleted: red
	dmlInsertStyle = lipgloss.NewStyle().Background(lipgloss.Color("22")).Foreground(lipgloss.Color("15"))  // inserted: green

	logoStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)

	statsBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1)

	connCardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1).
			MarginBottom(0)

	connCardSelectedStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("39")).
				Padding(0, 1).
				MarginBottom(0)
)

// focusedLabelStyle highlights a form label when its field has focus.
func focusedLabelStyle(focused bool) lipgloss.Style {
	if focused {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	}
	return keyStyle
}

// renderModal wraps content in a centered, accent-bordered box.
func (m Model) renderModal(content string) string {
	modalWidth := min(80, m.Width-10)
	if modalWidth < 20 {
		modalWidth = 20
	}
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("39")).
		Padding(1, 2).
		Width(modalWidth)
	return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, modalStyle.Render(content))
}
