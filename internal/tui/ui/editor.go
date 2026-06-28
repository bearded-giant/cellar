package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jorgerojas26/lazysql/internal/tui/sqlmeta"
	"github.com/jorgerojas26/lazysql/internal/tui/types"
)

// completionAreaRows is the fixed band reserved below the editor for the
// autocomplete popup (kept constant so the editor height never jumps).
const completionAreaRows = 5

// editorSize reserves rows for the title, footer, status bar, and completion
// band around the editor body.
func editorSize(w, h int) (int, int) {
	bw := w
	if bw < 10 {
		bw = 10
	}
	bh := h - 3 - completionAreaRows
	if bh < 3 {
		bh = 3
	}
	return bw, bh
}

func newEditorArea(content string, w, h int) textarea.Model {
	ta := textarea.New()
	ta.Prompt = "│ "
	ta.ShowLineNumbers = true
	ta.CharLimit = 0
	ta.SetValue(content)
	bw, bh := editorSize(w, h)
	ta.SetWidth(bw)
	ta.SetHeight(bh)
	ta.CursorEnd()
	return ta
}

func (m Model) openEditor() (tea.Model, tea.Cmd) {
	m.EditorArea = newEditorArea(m.EditorContent, m.Width, m.Height)
	m.Completer = m.buildCompleter()
	m.CompVisible = false
	m.Completions = nil
	m.Screen = types.ScreenEditor
	return m, m.EditorArea.Focus()
}

// buildCompleter seeds the autocompleter with the loaded schema: every table
// name in the tree, plus the current table's columns.
func (m Model) buildCompleter() *sqlmeta.Autocompleter {
	ac := sqlmeta.NewAutocompleter()
	var tables []string
	for _, groups := range m.Browse.TablesByDB {
		for _, names := range groups {
			tables = append(tables, names...)
		}
	}
	ac.SetTables(tables)
	if m.Browse.Label != "" && len(m.Browse.Columns) > 0 {
		ac.SetColumns(m.Browse.Label, m.Browse.Columns)
	}
	return ac
}

func (m Model) forwardToEditor(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.EditorArea, cmd = m.EditorArea.Update(msg)
	return m, cmd
}

func (m Model) handleEditorScreen(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+q":
		m.EditorContent = m.EditorArea.Value()
		m.Screen = types.ScreenBrowse
		return m, nil
	case "ctrl+r":
		query := m.EditorArea.Value()
		m.EditorContent = query
		if strings.TrimSpace(query) == "" {
			return m, nil
		}
		m.Browse.GridLoading = true
		m.StatusMsg = "Running query..."
		readOnly := m.CurrentConn != nil && m.CurrentConn.ReadOnly
		return m, m.Cmds.RunQuery(m.ActiveDriver, query, readOnly, m.connIdent())
	}

	if m.CompVisible {
		switch msg.String() {
		case "up", "ctrl+p":
			if m.CompCursor > 0 {
				m.CompCursor--
			}
			return m, nil
		case "down", "ctrl+n":
			if m.CompCursor < len(m.Completions)-1 {
				m.CompCursor++
			}
			return m, nil
		case "tab":
			m.acceptCompletion()
			return m, nil
		case "esc":
			m.CompVisible = false
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.EditorArea, cmd = m.EditorArea.Update(msg)
	m.refreshCompletions()
	return m, cmd
}

// textareaCursorOffset returns the rune offset of the cursor within the full
// editor text (logical column = StartColumn+ColumnOffset; rows split on \n).
func textareaCursorOffset(ta textarea.Model) int {
	lines := strings.Split(ta.Value(), "\n")
	row := ta.Line()
	li := ta.LineInfo()
	col := li.StartColumn + li.ColumnOffset
	off := 0
	for i := 0; i < row && i < len(lines); i++ {
		off += len([]rune(lines[i])) + 1 // +1 for the newline
	}
	return off + col
}

func currentPrefix(text string, off int) string {
	r := []rune(text)
	if off > len(r) {
		off = len(r)
	}
	i := off
	for i > 0 {
		c := r[i-1]
		if c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			i--
			continue
		}
		break
	}
	return string(r[i:off])
}

func (m *Model) refreshCompletions() {
	if m.Completer == nil {
		m.CompVisible = false
		return
	}
	text := m.EditorArea.Value()
	off := textareaCursorOffset(m.EditorArea)
	if currentPrefix(text, off) == "" {
		m.CompVisible = false
		m.Completions = nil
		return
	}
	m.Completions = m.Completer.Complete(text, off)
	m.CompCursor = 0
	m.CompVisible = len(m.Completions) > 0
}

func (m *Model) acceptCompletion() {
	if m.CompCursor >= len(m.Completions) {
		return
	}
	item := m.Completions[m.CompCursor]
	text := m.EditorArea.Value()
	off := textareaCursorOffset(m.EditorArea)
	r := []rune(text)
	if off > len(r) {
		off = len(r)
	}
	start := off - len([]rune(currentPrefix(text, off)))
	newText := string(r[:start]) + item.Text + string(r[off:])
	m.EditorArea.SetValue(newText)
	m.EditorArea.CursorEnd()
	m.CompVisible = false
	m.Completions = nil
}

func (m Model) handleQueryExecutedMsg(msg types.QueryExecutedMsg) (tea.Model, tea.Cmd) {
	m.Browse.GridLoading = false
	m.Screen = types.ScreenBrowse
	m.Focus = types.FocusGrid

	m.Browse.Table = ""
	m.Browse.TableDB = ""
	m.Browse.Offset = 0
	m.Browse.RowCursor = 0
	m.resetPending()

	if msg.Err != nil {
		m.Browse.GridErr = "Query error: " + msg.Err.Error()
		m.Browse.Columns = nil
		m.Browse.Rows = nil
		m.Browse.Label = "query"
		m.StatusMsg = "Query failed"
		return m, nil
	}
	m.Browse.GridErr = ""

	if msg.IsSelect {
		m.Browse.Label = "query result"
		if len(msg.Rows) > 0 {
			m.Browse.Columns = msg.Rows[0]
			m.Browse.Rows = msg.Rows[1:]
		} else {
			m.Browse.Columns = nil
			m.Browse.Rows = nil
		}
		m.Browse.Total = msg.Total
		m.refreshJSONView()
		m.StatusMsg = fmt.Sprintf("Query OK — %d rows", msg.Total)
		return m, nil
	}

	m.Browse.Label = "query"
	m.Browse.Columns = nil
	m.Browse.Rows = nil
	m.Browse.Total = 0
	info := msg.Info
	if info == "" {
		info = "OK"
	}
	m.StatusMsg = "DML: " + info
	return m, nil
}

func (m Model) viewEditor() string {
	title := accentStyle.Render("SQL Query")
	if m.CurrentConn != nil {
		title += dimStyle.Render("  ·  " + m.CurrentConn.Name)
	}
	parts := []string{
		title,
		m.EditorArea.View(),
		strings.Join(m.renderCompletions(m.EditorArea.Width(), completionAreaRows), "\n"),
		m.editorFooter(),
		m.getStatusBar(),
	}
	return strings.Join(parts, "\n")
}

func (m Model) renderCompletions(width, rows int) []string {
	out := make([]string, 0, rows)
	if m.CompVisible && len(m.Completions) > 0 {
		start := 0
		if m.CompCursor >= rows {
			start = m.CompCursor - rows + 1
		}
		end := min(start+rows, len(m.Completions))
		for i := start; i < end; i++ {
			it := m.Completions[i]
			label := it.Text
			if it.Description != "" {
				label += "  " + it.Description
			}
			label = truncateRunes(label, max(width-2, 4))
			if i == m.CompCursor {
				out = append(out, selectedRowStyle.Render("▶ "+label))
			} else {
				out = append(out, dimStyle.Render("  "+label))
			}
		}
	}
	for len(out) < rows {
		out = append(out, "")
	}
	return out
}

func (m Model) editorFooter() string {
	kb := []struct{ key, desc string }{
		{"ctrl+r", "run"}, {"tab", "complete"}, {"ctrl+q", "back"},
	}
	var b strings.Builder
	for i, k := range kb {
		b.WriteString(badge(k.key, "236", "255"))
		b.WriteString(" ")
		b.WriteString(dimStyle.Render(k.desc))
		if i < len(kb)-1 {
			b.WriteString("  ")
		}
	}
	return b.String()
}
