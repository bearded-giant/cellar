package ui

import "charm.land/lipgloss/v2"

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

	// query-tab bar: active gets its own green fill (distinct from the cyan-39
	// accent shared by the title + selected cell); inactive stays legible.
	queryTabActiveStyle   = lipgloss.NewStyle().Background(lipgloss.Color("42")).Foreground(lipgloss.Color("0")).Bold(true)
	queryTabInactiveStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))

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
	return m.renderModalW(content, 80)
}

// renderModalW is renderModal with a custom max width (for wider modals).
func (m Model) renderModalW(content string, maxW int) string {
	modalWidth := min(maxW, m.Width-10)
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
