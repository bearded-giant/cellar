package ui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jorgerojas26/lazysql/internal/tui/types"
)

func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.Screen != types.ScreenBrowse || msg.Action != tea.MouseActionPress {
		return m, nil
	}
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		return m.mouseScroll(-1)
	case tea.MouseButtonWheelDown:
		return m.mouseScroll(+1)
	case tea.MouseButtonLeft:
		return m.mouseClick(msg.X, msg.Y)
	}
	return m, nil
}

func (m Model) mouseScroll(dir int) (tea.Model, tea.Cmd) {
	if m.Focus == types.FocusGrid {
		if dir < 0 && m.Browse.RowCursor > 0 {
			m.Browse.RowCursor--
		} else if dir > 0 && m.Browse.RowCursor < m.gridCursorMax() {
			m.Browse.RowCursor++
		}
		return m, nil
	}
	if dir < 0 && m.Browse.Cursor > 0 {
		m.Browse.Cursor--
	} else if dir > 0 && m.Browse.Cursor < len(m.Browse.Nodes)-1 {
		m.Browse.Cursor++
	}
	return m, nil
}

func (m Model) mouseClick(x, y int) (tea.Model, tea.Cmd) {
	treeW, gridW, bodyH := m.browseLayout()
	if y >= bodyH { // footer / status rows
		return m, nil
	}

	if x < treeW { // tree pane: line 0 = "Schema", nodes windowed below
		start, _ := visibleWindow(len(m.Browse.Nodes), m.Browse.Cursor, bodyH-1)
		idx := start + (y - 1)
		if y >= 1 && idx >= 0 && idx < len(m.Browse.Nodes) {
			m.Focus = types.FocusTree
			m.Browse.Cursor = idx
		}
		return m, nil
	}
	if x == treeW { // separator column
		return m, nil
	}

	m.Focus = types.FocusGrid
	if m.Browse.ViewJSON || len(m.Browse.Columns) == 0 {
		return m, nil
	}
	gx := x - (treeW + 1)
	widths := colWidths(m.Browse.Columns, m.Browse.Rows, maxCellWidth)
	cs, ce := visibleColsForCursor(widths, m.Browse.ColCursor, gridW)
	dataStart, dataEnd := visibleWindow(m.Browse.gridRowCount(), m.Browse.RowCursor, bodyH-4)
	if row, col, ok := gridHitTest(y, gx, widths, cs, ce, dataStart, dataEnd-dataStart); ok {
		m.Browse.RowCursor = row
		m.Browse.ColCursor = col
	}
	return m, nil
}

// gridHitTest maps a click within the grid pane (pane-line y, pane-x gx) to a
// (dataRow, col). Grid lines: 0 title, 1 header, 2 rule, 3.. data rows.
func gridHitTest(y, gx int, widths []int, cs, ce, dataStart, dataCount int) (row, col int, ok bool) {
	if y < 3 {
		return 0, 0, false
	}
	row = dataStart + (y - 3)
	if row < dataStart || row >= dataStart+dataCount {
		return 0, 0, false
	}
	x := 0
	for c := cs; c < ce && c < len(widths); c++ {
		if gx >= x && gx < x+widths[c] {
			return row, c, true
		}
		x += widths[c] + 3 // " │ " separator
	}
	if ce > cs { // click in trailing space -> last visible column
		return row, ce - 1, true
	}
	return 0, 0, false
}
