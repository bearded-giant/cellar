package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

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

// focusBGSeq lifts the focused pane's background one shade — #232841, a
// desaturated navy that reads as "elevated surface" on dark themes without
// washing out dim foregrounds the way the ANSI-256 grays do.
const focusBGSeq = "\x1b[48;2;35;40;65m"

// tintBG paints lines with the focus background. Pane content carries its own
// styled runs whose resets would kill a wrapper background mid-line, so the
// sequence is re-applied after every inner reset; lines are padded to width so
// the pane reads as one block.
func tintBG(lines []string, width int) []string {
	out := make([]string, len(lines))
	for i, ln := range lines {
		body := strings.ReplaceAll(ln, "\x1b[0m", "\x1b[0m"+focusBGSeq)
		body = strings.ReplaceAll(body, "\x1b[m", "\x1b[m"+focusBGSeq)
		if pad := width - lipgloss.Width(ln); pad > 0 {
			body += strings.Repeat(" ", pad)
		}
		out[i] = focusBGSeq + body + "\x1b[0m"
	}
	return out
}

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
