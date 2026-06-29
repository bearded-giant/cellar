package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/bearded-giant/cellar/internal/tui/types"
	"github.com/bearded-giant/cellar/lib"
)

func (m Model) openYank() (tea.Model, tea.Cmd) {
	if len(m.Browse.Columns) == 0 {
		m.StatusMsg = "Nothing to copy"
		return m, nil
	}
	m.Screen = types.ScreenYank
	return m, nil
}

func (m Model) handleYankScreen(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var text, what string
	switch msg.String() {
	case "c":
		text, what = m.yankCell(), "cell"
	case "r":
		text, what = m.yankRow(), "row"
	case "a":
		text, what = m.yankAll(), "result"
	case "esc", "q":
		m.Screen = m.GridReturnScreen
		return m, nil
	default:
		return m, nil
	}
	m.Screen = m.GridReturnScreen
	if err := lib.NewClipboard().Write(text); err != nil {
		m.StatusMsg = "Copy failed: " + err.Error()
	} else {
		m.StatusMsg = "Copied " + what + " to clipboard"
	}
	return m, nil
}

func (m Model) yankCell() string {
	if len(m.Browse.Columns) == 0 {
		return ""
	}
	return displayCell(m.cellValue(m.Browse.RowCursor, m.Browse.ColCursor))
}

func (m Model) yankRow() string {
	cells := make([]string, len(m.Browse.Columns))
	for c := range m.Browse.Columns {
		cells[c] = displayCell(m.cellValue(m.Browse.RowCursor, c))
	}
	return strings.Join(cells, "\t")
}

// yankAll returns the loaded result as TSV (header + data rows).
func (m Model) yankAll() string {
	var b strings.Builder
	b.WriteString(strings.Join(m.Browse.Columns, "\t"))
	b.WriteByte('\n')
	for _, row := range m.Browse.Rows {
		cells := make([]string, len(row))
		for i, c := range row {
			cells[i] = displayCell(c)
		}
		b.WriteString(strings.Join(cells, "\t"))
		b.WriteByte('\n')
	}
	return b.String()
}

func (m Model) viewYank() string {
	body := titleStyle.Render("Copy to clipboard") + "\n\n" +
		normalStyle.Render("[c] cell   [r] row (TSV)   [a] full result (TSV)") + "\n\n" +
		helpStyle.Render("esc:cancel")
	return m.renderModal(body)
}
