package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/bearded-giant/cellar/internal/tui/sqlmeta"
	"github.com/bearded-giant/cellar/internal/tui/types"
)

// completionAreaRows is the fixed band reserved below the editor for the
// autocomplete popup (kept constant so the layout never jumps).
const completionAreaRows = 5

// queryLayout splits the query workspace vertically: an editor pane on top and
// a results pane below. Fixed chrome = title + completion band + a breather +
// footer + status (see viewEditor); the rest is the results grid.
func (m Model) queryLayout() (editorW, editorH, resultsH int) {
	w, h := m.Width, m.Height
	editorW = w
	if editorW < 10 {
		editorW = 10
	}
	editorH = h / 4
	if editorH < 4 {
		editorH = 4
	}
	if editorH > 10 {
		editorH = 10
	}
	resultsH = h - editorH - completionAreaRows - 4 // title, breather, footer, status
	if resultsH < 3 {
		resultsH = 3
	}
	return editorW, editorH, resultsH
}

func (m Model) newEditorArea(content string) sqlEditor {
	ew, eh, _ := m.queryLayout()
	return newEditor(content, ew, eh)
}

func (m Model) openEditor() (tea.Model, tea.Cmd) {
	m.EditorArea = m.newEditorArea(m.EditorContent)
	m.Completer = m.buildCompleter()
	m.CompVisible = false
	m.Completions = nil
	m.Focus = types.FocusEditor
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

func (m Model) handleEditorScreen(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.GridReturnScreen = types.ScreenEditor // grid modals from the results pane reopen here
	switch msg.String() {
	case "ctrl+q":
		m.EditorContent = m.EditorArea.Value()
		m.Screen = types.ScreenBrowse
		m.Focus = types.FocusTree
		return m, nil
	case "ctrl+s":
		return m.openSaveQuery()
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

	// results pane focused: navigate the grid below the editor; tab/esc returns.
	if m.Focus == types.FocusGrid {
		switch msg.String() {
		case "tab", "esc":
			m.Focus = types.FocusEditor
			return m, nil
		}
		return m.handleBrowseGridKey(msg)
	}

	// editor focused: completion popup nav takes tab/esc first.
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
	// no popup: tab moves down to the results pane.
	if msg.String() == "tab" {
		m.Focus = types.FocusGrid
		return m, nil
	}

	var cmd tea.Cmd
	m.EditorArea, cmd = m.EditorArea.Update(msg)
	m.refreshCompletions()
	return m, cmd
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
	off := m.EditorArea.cursorOffset()
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
	off := m.EditorArea.cursorOffset()
	r := []rune(text)
	if off > len(r) {
		off = len(r)
	}
	start := off - len([]rune(currentPrefix(text, off)))
	newText := string(r[:start]) + item.Text + string(r[off:])
	m.EditorArea.pushUndo()
	m.EditorArea.SetValue(newText)
	m.EditorArea.CursorEnd()
	m.CompVisible = false
	m.Completions = nil
}

func (m Model) handleQueryExecutedMsg(msg types.QueryExecutedMsg) (tea.Model, tea.Cmd) {
	m.Browse.GridLoading = false
	// results render in the query workspace, below the editor (which stays focused
	// so the query can be tweaked and re-run).
	m.Screen = types.ScreenEditor
	m.Focus = types.FocusEditor

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
		m.Browse.Offset = 0
		if len(msg.Rows) > 0 {
			m.Browse.Columns = msg.Rows[0]
			m.Browse.QueryRows = msg.Rows[1:]
		} else {
			m.Browse.Columns = nil
			m.Browse.QueryRows = nil
		}
		m.Browse.Total = len(m.Browse.QueryRows)
		m.Browse.Rows = pageOf(m.Browse.QueryRows, 0, m.Browse.Limit) // paged in-memory
		m.refreshJSONView()
		m.StatusMsg = fmt.Sprintf("Query OK — %d rows", m.Browse.Total)
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

// queryStatusLine reports the last execution status (e.g. "Query OK — 2 rows"),
// shown between the editor and the results in place of a grid title.
func (m Model) queryStatusLine() string {
	msg := m.StatusMsg
	if strings.TrimSpace(msg) == "" {
		msg = "Ready — ctrl+r to run"
	}
	if m.Browse.GridErr != "" {
		return errorStyle.Render(msg)
	}
	return successStyle.Render(msg)
}

func (m Model) viewEditor() string {
	w, h := m.Width, m.Height
	rule := dimStyle.Render(strings.Repeat("─", max(w, 1)))

	title := accentStyle.Render("SQL Query")
	if m.CurrentConn != nil {
		title += dimStyle.Render("  ·  " + m.CurrentConn.Name)
	}
	if m.Focus == types.FocusGrid {
		title += dimStyle.Render("   results — tab to edit")
	} else {
		title += dimStyle.Render("   editing — tab to results")
	}

	// top: blank, header, blank, editor, completion band, rule, blank, status, blank, rule, blank
	top := []string{"", title, ""}
	top = append(top, strings.Split(m.EditorArea.View(), "\n")...)
	top = append(top, m.renderCompletions(w, completionAreaRows)...)
	top = append(top, rule, "", m.queryStatusLine())

	// footer pinned to the bottom: blank, rule, blank, keybinds
	foot := []string{"", rule, "", m.editorFooter()}

	// results fill the gap so the footer sits at the bottom of the window
	resultsH := h - len(top) - len(foot)
	if resultsH < 1 {
		resultsH = 1
	}
	results := m.renderGridLines(w, resultsH, false)
	for len(results) < resultsH {
		results = append(results, "")
	}
	results = results[:resultsH]

	all := append(top, results...)
	all = append(all, foot...)
	return strings.Join(all, "\n")
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
	var kb []struct{ key, desc string }
	if m.Focus == types.FocusGrid {
		kb = []struct{ key, desc string }{
			{"↑/↓", "scroll"}, {"n/p", "page"}, {"v", "cell"}, {"J", "json"},
			{"tab", "editor"}, {"ctrl+r", "run"}, {"ctrl+q", "tree"},
		}
	} else {
		kb = []struct{ key, desc string }{
			{"ctrl+r", "run"}, {"tab", "complete / results"}, {"ctrl+z", "undo"},
			{"ctrl+s", "save"}, {"ctrl+q", "tree"},
		}
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
