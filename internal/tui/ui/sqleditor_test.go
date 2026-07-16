package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func key(t tea.KeyType) tea.KeyMsg  { return tea.KeyMsg{Type: t} }
func typeRunes(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }
func drive(e sqlEditor, ms ...tea.KeyMsg) sqlEditor {
	for _, m := range ms {
		e, _ = e.Update(m)
	}
	return e
}

func TestSQLEditor_ToggleCommentLine(t *testing.T) {
	e := newEditor("  select 1", 40, 10)

	e.toggleCommentSpan(0, e.lineLen(0))
	if e.Value() != "  -- select 1" {
		t.Fatalf("comment: %q", e.Value())
	}
	e.toggleCommentSpan(0, e.lineLen(0))
	if e.Value() != "  select 1" {
		t.Fatalf("uncomment: %q", e.Value())
	}
	e.undoPop()
	if e.Value() != "  -- select 1" {
		t.Fatalf("undo should restore the commented form: %q", e.Value())
	}
}

func TestSQLEditor_ToggleCommentMultiLine(t *testing.T) {
	e := newEditor("select a,\n  b\nfrom t", 40, 10)

	e.toggleCommentSpan(0, len([]rune(e.Value())))
	if e.Value() != "-- select a,\n  -- b\n-- from t" {
		t.Fatalf("comment all: %q", e.Value())
	}
	// still-commented range toggles back off in one press
	e.toggleCommentSpan(0, len([]rune(e.Value())))
	if e.Value() != "select a,\n  b\nfrom t" {
		t.Fatalf("uncomment all: %q", e.Value())
	}
}

func TestSQLEditor_TypeNewlineBackspace(t *testing.T) {
	e := newEditor("", 40, 10)
	e = drive(e, typeRunes("SELECT"))
	if e.Value() != "SELECT" || e.cursorOffset() != 6 {
		t.Fatalf("after typing: value=%q off=%d", e.Value(), e.cursorOffset())
	}

	e = drive(e, key(tea.KeyEnter), typeRunes("1"))
	if e.Value() != "SELECT\n1" || e.cursorOffset() != 8 {
		t.Fatalf("after newline+1: value=%q off=%d", e.Value(), e.cursorOffset())
	}

	e = drive(e, key(tea.KeyBackspace)) // delete the "1"
	if e.Value() != "SELECT\n" || e.cursorOffset() != 7 {
		t.Fatalf("after backspace: value=%q off=%d", e.Value(), e.cursorOffset())
	}

	e = drive(e, key(tea.KeyBackspace)) // col 0 -> join lines
	if e.Value() != "SELECT" || e.cursorOffset() != 6 {
		t.Fatalf("after line-join: value=%q off=%d", e.Value(), e.cursorOffset())
	}
}

func TestSQLEditor_InsertMidLine(t *testing.T) {
	e := newEditor("SELECT FROM t", 40, 10)
	e.SetValue("SELECT FROM t")
	e.row, e.col = 0, 7 // between "SELECT " and "FROM"
	e = drive(e, typeRunes("* "))
	if e.Value() != "SELECT * FROM t" {
		t.Fatalf("mid-line insert: %q", e.Value())
	}
}

func TestSQLEditor_PasteMultiline(t *testing.T) {
	e := newEditor("", 40, 10)
	e = drive(e, typeRunes("WITH x AS (\r\n  select 1\r\n)\nSELECT * FROM x;"))
	want := "WITH x AS (\n  select 1\n)\nSELECT * FROM x;"
	if e.Value() != want {
		t.Fatalf("paste value = %q, want %q", e.Value(), want)
	}
	if len(e.lines) != 4 {
		t.Fatalf("paste produced %d logical lines, want 4", len(e.lines))
	}
	if got := e.cursorOffset(); got != len([]rune(want)) {
		t.Fatalf("cursor offset = %d, want %d (end)", got, len([]rune(want)))
	}
}

func TestSQLEditor_PasteMidLineSplice(t *testing.T) {
	e := newEditor("SELECT  FROM t", 40, 10)
	e.row, e.col = 0, 7 // between "SELECT " and "FROM"
	e = drive(e, typeRunes("a,\nb"))
	if e.Value() != "SELECT a,\nb FROM t" {
		t.Fatalf("mid-line paste splice: %q", e.Value())
	}
	if e.row != 1 || e.col != 1 {
		t.Fatalf("cursor at (%d,%d), want (1,1)", e.row, e.col)
	}
}

func TestSQLEditor_DeleteForwardJoins(t *testing.T) {
	e := newEditor("a\nb", 40, 10)
	e.row, e.col = 0, 1 // end of line "a"
	e = drive(e, key(tea.KeyDelete))
	if e.Value() != "ab" {
		t.Fatalf("delete-forward join: %q", e.Value())
	}
}

func TestSQLEditor_CursorEndAndOffset(t *testing.T) {
	e := newEditor("ab\ncd", 40, 10)
	if e.row != 1 || e.col != 2 {
		t.Fatalf("CursorEnd put cursor at (%d,%d), want (1,2)", e.row, e.col)
	}
	if got := e.cursorOffset(); got != 5 {
		t.Fatalf("cursorOffset = %d, want 5", got)
	}
}

func TestSQLEditor_Undo(t *testing.T) {
	e := newEditor("", 40, 10)
	e = drive(e, typeRunes("ab"), key(tea.KeyEnter), typeRunes("c"))
	if e.Value() != "ab\nc" {
		t.Fatalf("setup value = %q", e.Value())
	}
	for _, want := range []string{"ab\n", "ab", ""} {
		e = drive(e, key(tea.KeyCtrlZ))
		if e.Value() != want {
			t.Fatalf("after undo, value = %q, want %q", e.Value(), want)
		}
	}
	e = drive(e, key(tea.KeyCtrlZ)) // empty stack -> no-op
	if e.Value() != "" {
		t.Errorf("undo on empty stack changed value to %q", e.Value())
	}
}

func TestSQLEditor_UndoCoalescesTyping(t *testing.T) {
	e := newEditor("", 40, 10)
	e = drive(e, typeRunes("a"), typeRunes("b"), typeRunes("c"))
	if e.Value() != "abc" {
		t.Fatalf("value = %q", e.Value())
	}
	e = drive(e, key(tea.KeyCtrlZ)) // a typing run collapses to one undo step
	if e.Value() != "" {
		t.Errorf("one undo should revert the whole typing run, got %q", e.Value())
	}
}

func TestSQLEditor_ViewExactHeightAndScroll(t *testing.T) {
	e := newEditor("l0\nl1\nl2\nl3\nl4", 20, 2)
	// CursorEnd is on l4; a 2-row window must have scrolled to show it.
	if e.offset != 3 {
		t.Fatalf("scroll offset = %d, want 3", e.offset)
	}
	v := e.View()
	if got := strings.Count(v, "\n") + 1; got != 2 {
		t.Fatalf("View rendered %d rows, want height 2", got)
	}
	if !strings.Contains(v, "l4") || strings.Contains(v, "l0") {
		t.Fatalf("View should show the scrolled-to tail, not the head:\n%q", v)
	}
}
