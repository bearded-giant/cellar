package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

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

func (m Model) viewYank() string {
	body := titleStyle.Render("Copy to clipboard") + "\n\n" +
		normalStyle.Render("[c] cell   [r] row (TSV)") + "\n\n" +
		helpStyle.Render("esc:cancel")
	return m.renderModal(body)
}
