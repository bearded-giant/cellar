package ui

import (
	"encoding/json"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

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

var peekBoxStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("39")).
	Padding(0, 1)

// peekSize is ~80% of the terminal, floored for small windows and clamped to
// the terminal itself.
func (m Model) peekSize() (w, h int) {
	w = m.Width * 4 / 5
	if w < 30 {
		w = 30
	}
	if w > m.Width {
		w = max(m.Width, 1)
	}
	h = m.Height * 4 / 5
	if h < 8 {
		h = 8
	}
	if h > m.Height {
		h = max(m.Height, 1)
	}
	return w, h
}

// peekWrapWidth is the box's inner content width: border (2) + padding (2).
func (m Model) peekWrapWidth() int {
	w, _ := m.peekSize()
	return max(w-4, 1)
}

func (m Model) peekDisplayLines() []string {
	return wrapLines(m.PeekLines, m.peekWrapWidth())
}

func (m Model) openPeek() (tea.Model, tea.Cmd) {
	if len(m.Browse.Columns) == 0 {
		return m, nil
	}
	m.PeekLines = prettyCellLines(m.cellValue(m.Browse.RowCursor, m.Browse.ColCursor))
	m.PeekRaw = m.yankCell()
	m.PeekScroll = 0
	m.PeekCol = ""
	if m.Browse.ColCursor < len(m.Browse.Columns) {
		m.PeekCol = m.Browse.Columns[m.Browse.ColCursor]
	}
	m.PeekOpen = true
	return m, nil
}

func (m *Model) closePeek() {
	m.PeekOpen = false
	m.PeekLines = nil
	m.PeekScroll = 0
	m.PeekCol = ""
	m.PeekRaw = ""
}

func (m Model) handlePeekKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	last := len(m.peekDisplayLines()) - 1
	if m.PeekScroll > last { // resize may have re-wrapped to fewer lines
		m.PeekScroll = max(last, 0)
	}
	switch msg.String() {
	case "esc", "q", "v":
		m.closePeek()
	case "y": // copy the raw cell value and close (status shows on the grid)
		raw := m.PeekRaw
		m.closePeek()
		if err := lib.NewClipboard().Write(raw); err != nil {
			m.StatusMsg = "Copy failed: " + err.Error()
		} else {
			m.StatusMsg = "Copied cell to clipboard"
		}
	case "up", "k":
		if m.PeekScroll > 0 {
			m.PeekScroll--
		}
	case "down", "j":
		if m.PeekScroll < last {
			m.PeekScroll++
		}
	case "g", "home":
		m.PeekScroll = 0
	case "G", "end":
		m.PeekScroll = max(last, 0)
	}
	return m, nil
}

func (m Model) renderPeek() string {
	w, h := m.peekSize()
	innerW := max(w-4, 1)
	bodyH := max(h-4, 1) // border (2) + title + footer
	lines := m.peekDisplayLines()
	scroll := m.PeekScroll
	if last := len(lines) - 1; scroll > last {
		scroll = max(last, 0)
	}
	start, end := visibleWindow(len(lines), scroll, bodyH)
	rows := make([]string, 0, bodyH+2)
	rows = append(rows, accentStyle.Render(truncateRunes("Cell: "+m.PeekCol, innerW)))
	for i := start; i < end; i++ {
		rows = append(rows, normalStyle.Render(padRunes(lines[i], innerW)))
	}
	for len(rows) < bodyH+1 {
		rows = append(rows, strings.Repeat(" ", innerW))
	}
	hint := "j/k scroll  g/G top/bottom  y copy  esc close"
	if len(lines) > bodyH {
		hint = fmt.Sprintf("%d-%d/%d  ", start+1, end, len(lines)) + hint
	}
	rows = append(rows, dimStyle.Render(truncateRunes(hint, innerW)))
	return peekBoxStyle.Render(strings.Join(rows, "\n"))
}

// composePeek floats the open peek over the current screen's render.
func (m Model) composePeek(base string) string {
	if !m.PeekOpen {
		return base
	}
	return overlayCenter(base, m.renderPeek(), m.Width, m.Height)
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
