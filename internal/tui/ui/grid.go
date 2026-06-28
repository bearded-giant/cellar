package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jorgerojas26/lazysql/internal/tui/types"
)

const maxCellWidth = 40

// displayCell maps the driver's row sentinels to display text. GetRecords emits
// "NULL&" for SQL NULL and "EMPTY&" for the empty string.
func displayCell(s string) string {
	switch s {
	case "NULL&":
		return "NULL"
	case "EMPTY&":
		return ""
	}
	return s
}

// truncateRunes / padRunes are rune-counted, not display-width-counted.
// ponytail: wide-rune (CJK) cells under-measure; swap in ansi.StringWidth if
// double-width data shows up.
func truncateRunes(s string, width int) string {
	if width <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= width {
		return s
	}
	if width == 1 {
		return "…"
	}
	return string(r[:width-1]) + "…"
}

func padRunes(s string, width int) string {
	if width <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) > width {
		return truncateRunes(s, width)
	}
	return s + strings.Repeat(" ", width-len(r))
}

// colWidths sizes each column to the widest of its header and visible cells,
// floored at 3 and capped at cap.
func colWidths(columns []string, rows [][]string, cap int) []int {
	w := make([]int, len(columns))
	for i, c := range columns {
		w[i] = len([]rune(displayCell(c)))
	}
	for _, row := range rows {
		for i := 0; i < len(columns) && i < len(row); i++ {
			if n := len([]rune(displayCell(row[i]))); n > w[i] {
				w[i] = n
			}
		}
	}
	for i := range w {
		if w[i] > cap {
			w[i] = cap
		}
		if w[i] < 3 {
			w[i] = 3
		}
	}
	return w
}

// visibleWindow returns [start,end) row indices for a viewport of `height` rows
// that keeps `cursor` in view.
func visibleWindow(total, cursor, height int) (start, end int) {
	if height < 1 {
		height = 1
	}
	if total <= height {
		return 0, total
	}
	if cursor >= height {
		start = cursor - height + 1
	}
	end = start + height
	if end > total {
		end = total
		start = max(end-height, 0)
	}
	return start, end
}

// visibleCols returns the [start,end) column range that fits in `avail` cells
// starting at colOffset. Always yields at least one column.
func visibleCols(widths []int, colOffset, avail int) (start, end int) {
	n := len(widths)
	if n == 0 {
		return 0, 0
	}
	if colOffset >= n {
		colOffset = n - 1
	}
	if colOffset < 0 {
		colOffset = 0
	}
	const sep = 3 // " │ "
	start = colOffset
	end = start
	used := 0
	for end < n {
		next := widths[end] + sep
		if used+next > avail && end > start {
			break
		}
		used += next
		end++
	}
	if end == start {
		end = start + 1
	}
	return start, end
}

func gridRowText(cells []string, widths []int, start, end int) string {
	parts := make([]string, 0, end-start)
	for i := start; i < end && i < len(widths); i++ {
		val := ""
		if i < len(cells) {
			val = displayCell(cells[i])
		}
		parts = append(parts, padRunes(val, widths[i]))
	}
	return strings.Join(parts, " │ ")
}

func (m Model) handleBrowseGridKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.Browse.RowCursor > 0 {
			m.Browse.RowCursor--
		}
	case "down", "j":
		if m.Browse.RowCursor < len(m.Browse.Rows)-1 {
			m.Browse.RowCursor++
		}
	case "left", "h":
		if m.Browse.ColOffset > 0 {
			m.Browse.ColOffset--
		}
	case "right", "l":
		if m.Browse.ColOffset < len(m.Browse.Columns)-1 {
			m.Browse.ColOffset++
		}
	case "g", "home":
		m.Browse.RowCursor = 0
	case "G", "end":
		m.Browse.RowCursor = max(len(m.Browse.Rows)-1, 0)
	case "n", "ctrl+f", "pgdown":
		if m.Browse.Offset+m.Browse.Limit < m.Browse.Total {
			m.Browse.Offset += m.Browse.Limit
			m.Browse.GridLoading = true
			return m, m.Cmds.LoadRecords(m.ActiveDriver, m.Browse.TableDB, m.Browse.Table, "", "", m.Browse.Offset, m.Browse.Limit)
		}
	case "p", "ctrl+b", "pgup":
		if m.Browse.Offset > 0 {
			m.Browse.Offset = max(m.Browse.Offset-m.Browse.Limit, 0)
			m.Browse.GridLoading = true
			return m, m.Cmds.LoadRecords(m.ActiveDriver, m.Browse.TableDB, m.Browse.Table, "", "", m.Browse.Offset, m.Browse.Limit)
		}
	}
	return m, nil
}

func (m Model) renderGridLines(width, height int) []string {
	var lines []string
	add := func(plain string, render func(string) string) {
		lines = append(lines, render(padRunes(plain, width)))
	}
	dim := func(s string) string { return dimStyle.Render(s) }

	if m.Browse.Label == "" {
		add("Select a table from the tree", dim)
		return lines
	}

	title := m.Browse.Label
	if m.Browse.GridLoading {
		title += "  loading…"
	}
	add(title, func(s string) string { return accentStyle.Render(s) })

	if m.Browse.GridErr != "" {
		add(m.Browse.GridErr, func(s string) string { return errorStyle.Render(s) })
		return lines
	}
	if len(m.Browse.Columns) == 0 {
		add("(no rows)", dim)
		return lines
	}

	widths := colWidths(m.Browse.Columns, m.Browse.Rows, maxCellWidth)
	cs, ce := visibleCols(widths, m.Browse.ColOffset, width)

	add(gridRowText(m.Browse.Columns, widths, cs, ce), func(s string) string { return headerRowStyle.Render(s) })
	add(strings.Repeat("─", width), dim)

	bodyH := height - 4 // title, header, rule, pagination
	if bodyH < 1 {
		bodyH = 1
	}
	start, end := visibleWindow(len(m.Browse.Rows), m.Browse.RowCursor, bodyH)
	for i := start; i < end; i++ {
		txt := gridRowText(m.Browse.Rows[i], widths, cs, ce)
		if i == m.Browse.RowCursor && m.Focus == types.FocusGrid {
			lines = append(lines, selectedRowStyle.Render(padRunes(txt, width)))
		} else {
			lines = append(lines, normalStyle.Render(padRunes(txt, width)))
		}
	}

	from := m.Browse.Offset + 1
	to := m.Browse.Offset + len(m.Browse.Rows)
	if len(m.Browse.Rows) == 0 {
		from = 0
	}
	add(fmt.Sprintf("rows %d-%d of %d   cols %d-%d/%d",
		from, to, m.Browse.Total, cs+1, ce, len(m.Browse.Columns)), dim)
	return lines
}
