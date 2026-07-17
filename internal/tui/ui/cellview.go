package ui

import (
	"encoding/json"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/bearded-giant/cellar/internal/tui/types"
	"github.com/bearded-giant/cellar/lib"
)

// prettyCellLines renders a cell value for the full-screen viewer: JSON is
// pretty-printed (2-space), everything else is shown as-is, split into lines.
func prettyCellLines(value string) []string {
	v := displayCell(value)
	trimmed := strings.TrimSpace(v)
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		var js any
		if json.Unmarshal([]byte(trimmed), &js) == nil {
			if b, err := json.MarshalIndent(js, "", "  "); err == nil {
				v = string(b)
			}
		}
	}
	return strings.Split(v, "\n")
}

// wrapLine soft-wraps s into rune-counted chunks of at most width; width <= 0
// disables wrapping. Chunks concatenate back to the original line exactly.
func wrapLine(s string, width int) []string {
	r := []rune(s)
	if width <= 0 || len(r) <= width {
		return []string{s}
	}
	out := make([]string, 0, (len(r)+width-1)/width)
	for len(r) > width {
		out = append(out, string(r[:width]))
		r = r[width:]
	}
	return append(out, string(r))
}

func wrapLines(lines []string, width int) []string {
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		out = append(out, wrapLine(l, width)...)
	}
	return out
}

func (m Model) cellViewWrapWidth() int {
	if m.Width < 20 || m.Height < 8 {
		return 0
	}
	return m.Width
}

func (m Model) cellViewDisplayLines() []string {
	return wrapLines(m.CellViewLines, m.cellViewWrapWidth())
}

func (m Model) openCellView() (tea.Model, tea.Cmd) {
	if len(m.Browse.Columns) == 0 {
		return m, nil
	}
	m.CellViewLines = prettyCellLines(m.cellValue(m.Browse.RowCursor, m.Browse.ColCursor))
	m.CellViewScroll = 0
	m.CellViewCol = ""
	if m.Browse.ColCursor < len(m.Browse.Columns) {
		m.CellViewCol = m.Browse.Columns[m.Browse.ColCursor]
	}
	m.Screen = types.ScreenCellView
	return m, nil
}

func (m Model) handleCellViewScreen(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	last := len(m.cellViewDisplayLines()) - 1
	if m.CellViewScroll > last { // width may have shrunk/grown the wrapped line count
		m.CellViewScroll = max(last, 0)
	}
	switch msg.String() {
	case "esc", "q", "v":
		m.Screen = m.GridReturnScreen
	case "y": // copy the raw cell value and drop back to the grid (where status shows)
		m.Screen = m.GridReturnScreen
		if err := lib.NewClipboard().Write(m.yankCell()); err != nil {
			m.StatusMsg = "Copy failed: " + err.Error()
		} else {
			m.StatusMsg = "Copied cell to clipboard"
		}
	case "up", "k":
		if m.CellViewScroll > 0 {
			m.CellViewScroll--
		}
	case "down", "j":
		if m.CellViewScroll < last {
			m.CellViewScroll++
		}
	case "g", "home":
		m.CellViewScroll = 0
	case "G", "end":
		m.CellViewScroll = max(last, 0)
	}
	return m, nil
}

func (m Model) viewCellView() string {
	w, h := m.Width, m.Height
	if w < 20 || h < 8 {
		return strings.Join(m.CellViewLines, "\n")
	}
	title := accentStyle.Render("Cell: "+m.CellViewCol) + dimStyle.Render(strings.Repeat(" ", 1))
	bodyH := h - 2 // title + footer
	if bodyH < 1 {
		bodyH = 1
	}
	lines := m.cellViewDisplayLines()
	scroll := m.CellViewScroll
	if last := len(lines) - 1; scroll > last {
		scroll = max(last, 0)
	}
	start, end := visibleWindow(len(lines), scroll, bodyH)
	rows := make([]string, 0, bodyH)
	for i := start; i < end; i++ {
		rows = append(rows, normalStyle.Render(padRunes(lines[i], w)))
	}
	for len(rows) < bodyH {
		rows = append(rows, strings.Repeat(" ", w))
	}
	footer := badge("↑/↓", "236", "255") + dimStyle.Render(" scroll  ") +
		badge("g/G", "236", "255") + dimStyle.Render(" top/bottom  ") +
		badge("y", "236", "255") + dimStyle.Render(" copy  ") +
		badge("esc", "236", "255") + dimStyle.Render(" back")
	return title + "\n" + strings.Join(rows, "\n") + "\n" + footer
}
