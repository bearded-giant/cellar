package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/bearded-giant/cellar/internal/tui/types"
)

const maxCellWidth = 40

// cellCap is the per-column truncation width: the default cap, or the full pane
// width when WideCells is on so hashes/tokens read inline.
func (m Model) cellCap() int {
	if m.Browse.WideCells && m.Width > maxCellWidth {
		return m.Width
	}
	return maxCellWidth
}

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

// visibleColsForCursor returns the [start,end) column range that fits in
// `avail` cells and is guaranteed to include `cursor`. It packs columns right
// of the cursor first, then fills any remaining width to the left.
func visibleColsForCursor(widths []int, cursor, avail int) (start, end int) {
	n := len(widths)
	if n == 0 {
		return 0, 0
	}
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= n {
		cursor = n - 1
	}
	const sep = 3 // " │ "
	start, end = cursor, cursor+1
	used := widths[cursor]
	for end < n {
		next := widths[end] + sep
		if used+next > avail {
			break
		}
		used += next
		end++
	}
	for start > 0 {
		prev := widths[start-1] + sep
		if used+prev > avail {
			break
		}
		used += prev
		start--
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

// gridCursorMax is the last selectable index: JSON line, or data+insert row.
func (m Model) gridCursorMax() int {
	if m.Browse.ViewJSON {
		return len(m.Browse.JSONLines) - 1
	}
	return m.Browse.gridRowCount() - 1
}

func (m Model) handleBrowseGridKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "J":
		// JSON view is for query results only — on a wide table preview, rendering
		// the page as JSON can hang on huge cells. Query with a LIMIT instead.
		if m.Browse.Table != "" {
			m.StatusMsg = "JSON view is for query results — run a query with a LIMIT (e)"
			return m, nil
		}
		m.Browse.ViewJSON = !m.Browse.ViewJSON
		m.Browse.RowCursor = 0
		m.Browse.ColCursor = 0
		m.refreshJSONView()
		return m, nil
	case "up", "k":
		if m.Browse.RowCursor > 0 {
			m.Browse.RowCursor--
		}
	case "down", "j":
		if m.Browse.RowCursor < m.gridCursorMax() {
			m.Browse.RowCursor++
		}
	case "left", "h":
		// at the left edge (or in JSON view) back out to the schema tree
		if m.Browse.ViewJSON || m.Browse.ColCursor == 0 {
			m.Focus = types.FocusTree
			return m, nil
		}
		m.Browse.ColCursor--
	case "right", "l":
		if !m.Browse.ViewJSON && m.Browse.ColCursor < len(m.Browse.Columns)-1 {
			m.Browse.ColCursor++
		}
	case "g", "home":
		m.Browse.RowCursor = 0
	case "G", "end":
		m.Browse.RowCursor = max(m.gridCursorMax(), 0)
	case "n", "ctrl+f", "pgdown":
		return m.pageRecords(+1)
	case "p", "ctrl+b", "pgup":
		return m.pageRecords(-1)
	case "d":
		return m.generateDelete()
	case "o":
		return m.generateInsert()
	case "s":
		return m.cycleSort()
	case "/":
		return m.openFilter()
	case "i":
		return m.cycleMeta()
	case "enter":
		return m.jumpFK()
	case "backspace":
		return m.popCrumb()
	case "v":
		return m.openCellView()
	case "w":
		m.Browse.WideCells = !m.Browse.WideCells
		return m, nil
	}
	return m, nil
}

// pageRecords moves a real table's page window (dir +1/-1). Query results and
// tables with staged edits do not paginate.
func (m Model) pageRecords(dir int) (tea.Model, tea.Cmd) {
	if m.Browse.MetaKind != metaRecords {
		return m, nil
	}
	// query results: page the in-memory rows (no driver round-trip)
	if m.Browse.Table == "" {
		if len(m.Browse.QueryRows) == 0 {
			return m, nil
		}
		return m.pageQueryResult(dir), nil
	}
	if dir > 0 && m.Browse.Offset+m.Browse.Limit < m.Browse.Total {
		m.Browse.Offset += m.Browse.Limit
	} else if dir < 0 && m.Browse.Offset > 0 {
		m.Browse.Offset = max(m.Browse.Offset-m.Browse.Limit, 0)
	} else {
		return m, nil
	}
	return m.reloadRecords()
}

func (m Model) pageQueryResult(dir int) tea.Model {
	limit := m.Browse.Limit
	switch {
	case dir > 0 && m.Browse.Offset+limit < m.Browse.Total:
		m.Browse.Offset += limit
	case dir < 0 && m.Browse.Offset > 0:
		m.Browse.Offset = max(m.Browse.Offset-limit, 0)
	default:
		return m
	}
	m.Browse.Rows = pageOf(m.Browse.QueryRows, m.Browse.Offset, limit)
	m.Browse.RowCursor = 0
	m.refreshJSONView()
	return m
}

// pageOf returns the [offset, offset+limit) slice of rows, clamped.
func pageOf(rows [][]string, offset, limit int) [][]string {
	if offset < 0 || offset >= len(rows) {
		return nil
	}
	end := offset + limit
	if end > len(rows) {
		end = len(rows)
	}
	return rows[offset:end]
}

func (m Model) renderGridLines(width, height int, showTitle bool) []string {
	var lines []string
	chrome := 4 // top rule, header, bottom rule, pagination
	add := func(plain string, render func(string) string) {
		lines = append(lines, render(padRunes(plain, width)))
	}
	dim := func(s string) string { return dimStyle.Render(s) }

	if m.Browse.Label == "" {
		add("Select a table from the tree", dim)
		return lines
	}

	if showTitle {
		title := m.Browse.Label
		if m.Browse.MetaKind != metaRecords {
			title += "  · " + metaNames[m.Browse.MetaKind]
		}
		if m.Browse.Sort != "" {
			title += "  ↕ " + m.Browse.Sort
		}
		if m.Browse.Where != "" {
			title += "  ⧩ filtered"
		}
		if m.Browse.ViewJSON {
			title += "  [json]"
		}
		if m.Browse.WideCells {
			title += "  [wide]"
		}
		if m.Browse.GridLoading {
			title += "  loading…"
		}
		add(title, func(s string) string { return accentStyle.Render(s) })
		chrome++ // title
	}

	if m.Browse.GridErr != "" {
		add(m.Browse.GridErr, func(s string) string { return errorStyle.Render(s) })
		return lines
	}
	if len(m.Browse.Columns) == 0 {
		add("(no rows)", dim)
		return lines
	}

	if m.Browse.ViewJSON {
		bodyH := height // only the title (if shown) is chrome in JSON view
		if showTitle {
			bodyH--
		}
		if bodyH < 1 {
			bodyH = 1
		}
		start, end := visibleWindow(len(m.Browse.JSONLines), m.Browse.RowCursor, bodyH)
		for i := start; i < end; i++ {
			add(m.Browse.JSONLines[i], func(s string) string { return normalStyle.Render(s) })
		}
		return lines
	}

	widths := colWidths(m.Browse.Columns, m.Browse.Rows, m.cellCap())
	cs, ce := visibleColsForCursor(widths, m.Browse.ColCursor, width)

	add(strings.Repeat("─", width), dim) // top border of the column header
	add(gridRowText(m.Browse.Columns, widths, cs, ce), func(s string) string { return headerRowStyle.Render(s) })
	add(strings.Repeat("─", width), dim)

	bodyH := height - chrome
	if bodyH < 1 {
		bodyH = 1
	}
	total := m.Browse.gridRowCount()
	start, end := visibleWindow(total, m.Browse.RowCursor, bodyH)
	for i := start; i < end; i++ {
		lines = append(lines, m.renderStyledRow(i, widths, cs, ce, width))
	}

	from := m.Browse.Offset + 1
	to := m.Browse.Offset + len(m.Browse.Rows)
	if len(m.Browse.Rows) == 0 {
		from = 0
	}
	status := fmt.Sprintf("rows %d-%d of %d   cols %d-%d/%d",
		from, to, m.Browse.Total, cs+1, ce, len(m.Browse.Columns))
	add(status, dim)
	return lines
}

// renderStyledRow renders one grid row (existing or insert) with per-cell DML
// styling, padded to the full pane width.
func (m Model) renderStyledRow(row int, widths []int, cs, ce, width int) string {
	parts := make([]string, 0, ce-cs)
	visible := 0
	for c := cs; c < ce && c < len(widths); c++ {
		plain := padRunes(displayCell(m.cellValue(row, c)), widths[c])
		parts = append(parts, m.cellStyle(row, c).Render(plain))
		visible += widths[c]
	}
	if n := ce - cs; n > 1 {
		visible += 3 * (n - 1)
	}
	line := strings.Join(parts, " │ ")
	if visible < width {
		line += strings.Repeat(" ", width-visible)
	}
	return line
}
