package ui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/bearded-giant/cellar/internal/tui/types"
)

func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.Screen == types.ScreenHelp {
		if wheel, ok := msg.(tea.MouseWheelMsg); ok {
			maxScroll := max(0, len(m.helpLines())-m.helpViewportHeight())
			switch wheel.Button {
			case tea.MouseWheelUp:
				m.HelpScroll = max(m.HelpScroll-1, 0)
			case tea.MouseWheelDown:
				m.HelpScroll = min(m.HelpScroll+1, maxScroll)
			}
		}
		return m, nil
	}
	if m.Screen != types.ScreenBrowse {
		return m, nil
	}
	// a floating popup owns the mouse: wheel scrolls it, clicks are ignored —
	// otherwise events would silently move the pane cursors underneath it
	if m.PeekOpen || m.InspOpen {
		if wheel, ok := msg.(tea.MouseWheelMsg); ok {
			delta := 0
			switch wheel.Button {
			case tea.MouseWheelUp:
				delta = -1
			case tea.MouseWheelDown:
				delta = 1
			}
			if m.PeekOpen {
				m.PeekScroll = max(m.PeekScroll+delta, 0)
			} else {
				m.InspScroll = max(m.InspScroll+delta, 0)
			}
		}
		return m, nil
	}
	switch msg := msg.(type) {
	case tea.MouseWheelMsg:
		switch msg.Button {
		case tea.MouseWheelUp:
			return m.mouseScroll(-1)
		case tea.MouseWheelDown:
			return m.mouseScroll(+1)
		}
	case tea.MouseClickMsg:
		if msg.Button == tea.MouseLeft {
			return m.mouseClick(msg.X, msg.Y)
		}
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
	m.clampTreeTop()
	return m, nil
}

func (m Model) mouseClick(x, y int) (tea.Model, tea.Cmd) {
	top := m.tabBarHeight()
	if y < top { // clicked the tab bar
		return m, nil
	}
	y -= top
	treeW, gridW, bodyH := m.browseLayout()
	if y >= bodyH { // footer / status rows
		return m, nil
	}

	if x < treeW { // tree pane: line 0 = "Schema", nodes windowed below
		start := clampTop(m.Browse.TreeTop, m.Browse.Cursor, len(m.Browse.Nodes), bodyH-1)
		idx := start + (y - 1)
		if y >= 1 && idx >= 0 && idx < len(m.Browse.Nodes) {
			m.Focus = types.FocusTree
			m.Browse.Cursor = idx
			m.clampTreeTop()
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
