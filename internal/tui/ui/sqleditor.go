package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/bearded-giant/cellar/internal/tui/sqlmeta"
)

// sqlEditor is a minimal multi-line text editor with SQL syntax highlighting.
// It replaces bubbles/textarea, which exposes no per-rune style hook, so the
// SQL tokens can be colored inline via sqlmeta.Tokenize + ColorFor.
type sqlEditor struct {
	lines   []string // logical lines, no trailing newline
	row     int      // cursor line index
	col     int      // cursor rune index within lines[row]
	offset  int      // first visible line (vertical scroll)
	width   int      // total render width incl. gutter
	height  int      // visible line count
	focused bool

	undo    []editorSnapshot
	lastIns bool // last edit was a rune-insert, so coalesce a typing run into one undo
}

type editorSnapshot struct {
	lines    []string
	row, col int
}

func newEditor(content string, w, h int) sqlEditor {
	e := sqlEditor{width: w, height: h}
	e.SetValue(content)
	e.CursorEnd()
	return e
}

func (e sqlEditor) Value() string { return strings.Join(e.lines, "\n") }

func (e sqlEditor) Width() int { return e.width }

func (e *sqlEditor) SetWidth(w int)  { e.width = w }
func (e *sqlEditor) SetHeight(h int) { e.height = h; e.clampScroll() }

func (e *sqlEditor) Focus() tea.Cmd { e.focused = true; return nil }

func (e *sqlEditor) SetValue(s string) {
	e.lines = strings.Split(s, "\n")
	if len(e.lines) == 0 {
		e.lines = []string{""}
	}
	if e.row >= len(e.lines) {
		e.row = len(e.lines) - 1
	}
	if e.col > e.lineLen(e.row) {
		e.col = e.lineLen(e.row)
	}
	e.clampScroll()
}

func (e *sqlEditor) CursorEnd() {
	e.row = len(e.lines) - 1
	e.col = e.lineLen(e.row)
	e.clampScroll()
}

func (e sqlEditor) lineLen(row int) int { return len([]rune(e.lines[row])) }

// cursorOffset returns the cursor's rune offset into Value() (used by sqlmeta).
func (e sqlEditor) cursorOffset() int {
	off := 0
	for i := 0; i < e.row; i++ {
		off += e.lineLen(i) + 1
	}
	return off + e.col
}

func isEditMutation(t tea.KeyType) bool {
	switch t {
	case tea.KeyRunes, tea.KeySpace, tea.KeyEnter, tea.KeyBackspace, tea.KeyDelete, tea.KeyTab:
		return true
	}
	return false
}

func (e *sqlEditor) pushUndo() {
	e.undo = append(e.undo, editorSnapshot{
		lines: append([]string(nil), e.lines...),
		row:   e.row,
		col:   e.col,
	})
	if len(e.undo) > 200 {
		e.undo = e.undo[len(e.undo)-200:]
	}
}

func (e *sqlEditor) undoPop() {
	if len(e.undo) == 0 {
		return
	}
	last := e.undo[len(e.undo)-1]
	e.undo = e.undo[:len(e.undo)-1]
	e.lines = last.lines
	e.row, e.col = last.row, last.col
	if e.row >= len(e.lines) {
		e.row = len(e.lines) - 1
	}
	if e.col > e.lineLen(e.row) {
		e.col = e.lineLen(e.row)
	}
	e.lastIns = false
	e.clampScroll()
}

func (e sqlEditor) Update(msg tea.KeyMsg) (sqlEditor, tea.Cmd) {
	if msg.Type == tea.KeyCtrlZ {
		e.undoPop()
		return e, nil
	}
	if isEditMutation(msg.Type) {
		typing := msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace
		if !(typing && e.lastIns) {
			e.pushUndo() // coalesce a run of typing into a single undo step
		}
		e.lastIns = typing
	} else {
		e.lastIns = false
	}
	switch msg.Type {
	case tea.KeyRunes, tea.KeySpace:
		e.insert(string(msg.Runes))
	case tea.KeyEnter:
		e.insertNewline()
	case tea.KeyBackspace:
		e.backspace()
	case tea.KeyDelete:
		e.deleteForward()
	case tea.KeyTab:
		e.insert("    ") // ponytail: literal tab breaks the width=1-per-rune renderer
	case tea.KeyLeft:
		e.moveLeft()
	case tea.KeyRight:
		e.moveRight()
	case tea.KeyUp:
		e.moveUp()
	case tea.KeyDown:
		e.moveDown()
	case tea.KeyHome, tea.KeyCtrlA:
		e.col = 0
	case tea.KeyEnd, tea.KeyCtrlE:
		e.col = e.lineLen(e.row)
	}
	e.clampScroll()
	return e, nil
}

func (e *sqlEditor) insert(s string) {
	ins := []rune(s)
	line := []rune(e.lines[e.row])
	out := make([]rune, 0, len(line)+len(ins))
	out = append(out, line[:e.col]...)
	out = append(out, ins...)
	out = append(out, line[e.col:]...)
	e.lines[e.row] = string(out)
	e.col += len(ins)
}

func (e *sqlEditor) insertNewline() {
	line := []rune(e.lines[e.row])
	before, after := string(line[:e.col]), string(line[e.col:])
	e.lines[e.row] = before
	tail := append([]string{after}, e.lines[e.row+1:]...)
	e.lines = append(e.lines[:e.row+1], tail...)
	e.row++
	e.col = 0
}

func (e *sqlEditor) backspace() {
	if e.col > 0 {
		line := []rune(e.lines[e.row])
		out := append(line[:e.col-1:e.col-1], line[e.col:]...)
		e.lines[e.row] = string(out)
		e.col--
		return
	}
	if e.row > 0 {
		e.col = e.lineLen(e.row - 1)
		e.lines[e.row-1] += e.lines[e.row]
		e.lines = append(e.lines[:e.row], e.lines[e.row+1:]...)
		e.row--
	}
}

func (e *sqlEditor) deleteForward() {
	line := []rune(e.lines[e.row])
	if e.col < len(line) {
		out := append(line[:e.col:e.col], line[e.col+1:]...)
		e.lines[e.row] = string(out)
		return
	}
	if e.row < len(e.lines)-1 {
		e.lines[e.row] += e.lines[e.row+1]
		e.lines = append(e.lines[:e.row+1], e.lines[e.row+2:]...)
	}
}

func (e *sqlEditor) moveLeft() {
	if e.col > 0 {
		e.col--
	} else if e.row > 0 {
		e.row--
		e.col = e.lineLen(e.row)
	}
}

func (e *sqlEditor) moveRight() {
	if e.col < e.lineLen(e.row) {
		e.col++
	} else if e.row < len(e.lines)-1 {
		e.row++
		e.col = 0
	}
}

func (e *sqlEditor) moveUp() {
	if e.row > 0 {
		e.row--
		e.clampCol()
	}
}

func (e *sqlEditor) moveDown() {
	if e.row < len(e.lines)-1 {
		e.row++
		e.clampCol()
	}
}

func (e sqlEditor) currentLine() string { return e.lines[e.row] }

// rowForOffset maps a rune offset into Value() to its logical line index.
func (e sqlEditor) rowForOffset(off int) int {
	if off < 0 {
		off = 0
	}
	acc := 0
	for i := range e.lines {
		acc += e.lineLen(i) + 1
		if off < acc {
			return i
		}
	}
	return len(e.lines) - 1
}

// toggleCommentSpan toggles "-- " across every non-blank line the rune span
// [start,end) touches: uncomments when all are already commented, else comments.
func (e *sqlEditor) toggleCommentSpan(start, end int) {
	first := e.rowForOffset(start)
	last := e.rowForOffset(max(end-1, start))
	if first > last {
		first, last = last, first
	}

	allCommented, any := true, false
	for r := first; r <= last; r++ {
		body := strings.TrimLeft(e.lines[r], " \t")
		if body == "" {
			continue
		}
		any = true
		if !strings.HasPrefix(body, "--") {
			allCommented = false
		}
	}
	if !any {
		return
	}

	e.pushUndo()
	e.lastIns = false
	for r := first; r <= last; r++ {
		body := strings.TrimLeft(e.lines[r], " \t")
		if body == "" {
			continue
		}
		indent := e.lines[r][:len(e.lines[r])-len(body)]
		switch {
		case !allCommented:
			e.lines[r] = indent + "-- " + body
		case strings.HasPrefix(body, "-- "):
			e.lines[r] = indent + body[len("-- "):]
		default:
			e.lines[r] = indent + body[len("--"):]
		}
	}
	e.clampCol()
}

func (e *sqlEditor) clampCol() {
	if e.col > e.lineLen(e.row) {
		e.col = e.lineLen(e.row)
	}
}

func (e *sqlEditor) clampScroll() {
	if e.height <= 0 {
		return
	}
	if e.row < e.offset {
		e.offset = e.row
	}
	if e.row >= e.offset+e.height {
		e.offset = e.row - e.height + 1
	}
	if e.offset < 0 {
		e.offset = 0
	}
}

// View renders exactly e.height rows: a dim line-number gutter, the SQL text
// colored per token, a reverse-video block at the cursor, with vertical and
// (cursor-following) horizontal scrolling.
func (e sqlEditor) View() string {
	full := e.Value()
	colors := make([]string, len([]rune(full)))
	for _, t := range sqlmeta.Tokenize(full) {
		c := sqlmeta.ColorFor(t.Type)
		if c == "" {
			continue
		}
		for i := t.Start; i < t.End && i < len(colors); i++ {
			colors[i] = c
		}
	}

	starts := make([]int, len(e.lines))
	acc := 0
	for i := range e.lines {
		starts[i] = acc
		acc += e.lineLen(i) + 1
	}

	digits := len(fmt.Sprintf("%d", len(e.lines)))
	gutterW := digits + 3 // "<n> │ "
	contentW := e.width - gutterW
	if contentW < 1 {
		contentW = 1
	}

	out := make([]string, 0, e.height)
	for vis := 0; vis < e.height; vis++ {
		row := e.offset + vis
		if row >= len(e.lines) {
			out = append(out, "")
			continue
		}
		lineRunes := []rune(e.lines[row])
		end := min(starts[row]+len(lineRunes), len(colors))
		lineColors := colors[starts[row]:end]
		hoff := 0
		if row == e.row && e.col >= contentW {
			hoff = e.col - contentW + 1
		}
		gutter := dimStyle.Render(fmt.Sprintf("%*d │ ", digits, row+1))
		out = append(out, gutter+e.renderCells(lineRunes, lineColors, row == e.row, hoff, contentW))
	}
	return strings.Join(out, "\n")
}

func (e sqlEditor) renderCells(line []rune, colors []string, cursorRow bool, hoff, contentW int) string {
	end := len(line)
	if cursorRow && e.focused && e.col+1 > end {
		end = e.col + 1
	}

	var b strings.Builder
	var buf []rune
	curColor := ""
	curCursor := false
	flush := func() {
		if len(buf) == 0 {
			return
		}
		st := lipgloss.NewStyle()
		switch {
		case curCursor:
			st = st.Reverse(true)
		case curColor != "":
			st = st.Foreground(lipgloss.Color(curColor))
		}
		b.WriteString(st.Render(string(buf)))
		buf = buf[:0]
	}

	for c := hoff; c < end && c < hoff+contentW; c++ {
		r := ' '
		col := ""
		if c < len(line) {
			r = line[c]
			if c < len(colors) {
				col = colors[c]
			}
		}
		cur := cursorRow && e.focused && c == e.col
		if len(buf) > 0 && (col != curColor || cur != curCursor) {
			flush()
		}
		if len(buf) == 0 {
			curColor, curCursor = col, cur
		}
		buf = append(buf, r)
	}
	flush()
	return b.String()
}
