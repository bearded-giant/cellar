package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/bearded-giant/cellar/internal/tui/sqlmeta"
	"github.com/bearded-giant/cellar/internal/tui/types"
	"github.com/bearded-giant/cellar/lib"
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
	m.ensureQueryTabs()
	m.EditorArea = m.newEditorArea(m.EditorContent)
	m.Completer = m.buildCompleter()
	m.CompVisible = false
	m.Completions = nil
	m.EditorColsLoaded = map[string]bool{}
	m.Focus = types.FocusEditor
	m.Screen = types.ScreenEditor
	// drop mouse reporting so the terminal does native drag-to-select/copy in the
	// editor; nothing here consumes mouse events (handleMouse only acts on Browse).
	return m, tea.Batch(m.EditorArea.Focus(), m.ensureRefColumns(), tea.DisableMouse)
}

// ensureRefColumns fires column-load commands for tables referenced in the
// editor whose columns aren't in the completer yet (each table loads once), so
// `select … from Foo where <tab>` completes Foo's columns without opening it.
func (m *Model) ensureRefColumns() tea.Cmd {
	if m.Completer == nil || m.ActiveDriver == nil || m.EditorColsLoaded == nil {
		return nil
	}
	var cmds []tea.Cmd
	for _, raw := range sqlmeta.ReferencedTables(m.EditorArea.Value()) {
		bare := bareIdent(raw)
		key := strings.ToLower(bare)
		if key == "" || m.EditorColsLoaded[key] {
			continue
		}
		db, qualified, ok := m.resolveTableRef(bare)
		if !ok {
			continue
		}
		m.EditorColsLoaded[key] = true // optimistic: don't refetch (best-effort)
		cmds = append(cmds, m.Cmds.LoadColumns(m.ActiveDriver, db, qualified, bare))
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

// bareIdent strips quoting and any schema/db qualifier from a referenced name.
func bareIdent(raw string) string {
	if i := strings.LastIndex(raw, "."); i >= 0 {
		raw = raw[i+1:]
	}
	return strings.Trim(raw, "\"`[]")
}

// resolveTableRef locates a referenced (bare) table in the loaded schema tree and
// returns its database + the driver-qualified table arg for GetTableColumns
// (schema.table for schema drivers, bare table for MySQL/SQLite).
func (m Model) resolveTableRef(bare string) (db, qualified string, ok bool) {
	want := strings.ToLower(bare)
	for dbName, groups := range m.Browse.TablesByDB {
		for schema, tables := range groups {
			for _, t := range tables {
				if strings.ToLower(t) == want {
					if m.Browse.UseSchemas {
						return dbName, schema + "." + t, true
					}
					return dbName, t, true
				}
			}
		}
	}
	return "", "", false
}

func (m Model) handleColumnsLoadedMsg(msg types.ColumnsLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil || m.Completer == nil || len(msg.Columns) == 0 {
		return m, nil // best-effort autocomplete: ignore load failures
	}
	m.Completer.SetColumns(msg.Table, msg.Columns)
	m.refreshCompletions()
	return m, nil
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

// leaveQueryWorkspace exits the editor+results view back to the tree/grid,
// preserving the query text (re-entered via `e`).
func (m Model) leaveQueryWorkspace() (tea.Model, tea.Cmd) {
	m.EditorContent = m.EditorArea.Value()
	m.Screen = types.ScreenBrowse
	m.Focus = types.FocusTree
	return m, tea.EnableMouseCellMotion // re-arm wheel-scroll/click in the browse grid
}

func (m Model) handleEditorScreen(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.GridReturnScreen = types.ScreenEditor // grid modals from the results pane reopen here

	// workspace-wide actions (both panes)
	switch msg.String() {
	case "ctrl+s":
		return m.openSaveQuery()
	case "ctrl+y":
		m.EditorContent = m.EditorArea.Value() // preserve the buffer if recall is cancelled
		return m.openHistory()
	case "ctrl+o":
		m.EditorContent = m.EditorArea.Value() // preserve the buffer if cancelled
		return m.openSavedQueries()
	case "alt+t":
		return m.newQueryTab()
	case "alt+]":
		return m.switchQueryTab(+1)
	case "alt+[":
		return m.switchQueryTab(-1)
	case "alt+w":
		return m.closeQueryTab()
	// ctrl+enter is a shadow for run; most terminals can't distinguish it from
	// plain enter (no kitty keyboard protocol in this bubbletea), so ctrl+r stays
	// the reliable bind.
	case "ctrl+r", "ctrl+enter":
		m.EditorContent = m.EditorArea.Value()
		// run only the statement under the cursor (';'-delimited); single-statement
		// buffers return the whole text unchanged.
		query := sqlmeta.StatementAt(m.EditorContent, m.EditorArea.cursorOffset())
		if strings.TrimSpace(query) == "" {
			return m, nil
		}
		m.dismissCompletions() // else a lingering popup eats the tab-to-results
		m.Browse.GridLoading = true
		m.StatusMsg = "Running query..."
		readOnly := m.CurrentConn != nil && m.CurrentConn.ReadOnly
		return m, tea.Batch(
			m.Cmds.RunQuery(m.ActiveDriver, query, readOnly, m.connIdent()),
			m.autosaveQueryState(),
		)
	case "alt+r": // run every ';'-delimited statement in order (notebook)
		m.EditorContent = m.EditorArea.Value()
		stmts := sqlmeta.SplitStatements(m.EditorContent)
		if len(stmts) == 0 {
			return m, nil
		}
		m.dismissCompletions() // else a lingering popup eats the tab-to-results
		m.Browse.GridLoading = true
		m.StatusMsg = "Running all statements..."
		readOnly := m.CurrentConn != nil && m.CurrentConn.ReadOnly
		return m, tea.Batch(
			m.Cmds.RunQueries(m.ActiveDriver, stmts, readOnly, m.connIdent()),
			m.autosaveQueryState(),
		)
	}

	// results pane focused: single-key grid affordances + back/cycle.
	if m.Focus == types.FocusGrid {
		switch msg.String() {
		case "tab":
			m.Focus = types.FocusEditor
			return m, nil
		case "esc", "q":
			return m.leaveQueryWorkspace()
		case "x":
			return m.openExport()
		case "y":
			return m.openYank()
		case "?":
			return m.openHelp()
		}
		return m.handleBrowseGridKey(msg)
	}

	// editor (text) focused: completion popup nav takes tab/esc first.
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
	switch msg.String() {
	case "tab": // no popup: move down to the results pane
		m.Focus = types.FocusGrid
		return m, nil
	case "esc": // back one level: leave the workspace to the tree
		return m.leaveQueryWorkspace()
	case "alt+c": // toggle "-- " comment across the statement under the cursor
		start, end := sqlmeta.StatementBoundsAt(m.EditorArea.Value(), m.EditorArea.cursorOffset())
		m.EditorArea.toggleCommentSpan(start, end)
		m.refreshCompletions()
		return m, nil
	case "alt+y": // yank the cursor line to the clipboard
		if err := lib.NewClipboard().Write(m.EditorArea.currentLine()); err != nil {
			m.StatusMsg = "Copy failed: " + err.Error()
		} else {
			m.StatusMsg = "Yanked line to clipboard"
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.EditorArea, cmd = m.EditorArea.Update(msg)
	m.refreshCompletions()
	return m, tea.Batch(cmd, m.ensureRefColumns())
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

func (m *Model) dismissCompletions() {
	m.CompVisible = false
	m.Completions = nil
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
	insert := item.Text
	// quote case-sensitive / special identifiers for the dialect (Postgres
	// "Product", MySQL `Product`); plain snake_case stays bare.
	if (item.Description == "table" || item.Description == "column") &&
		m.ActiveDriver != nil && needsQuoting(item.Text) {
		insert = m.ActiveDriver.FormatReference(item.Text)
	}
	newText := string(r[:start]) + insert + string(r[off:])
	m.EditorArea.pushUndo()
	m.EditorArea.SetValue(newText)
	m.EditorArea.CursorEnd()
	m.CompVisible = false
	m.Completions = nil
}

// needsQuoting reports whether an identifier must be quoted to survive a
// case-folding dialect: anything with an uppercase letter, a leading digit, or
// a character outside [a-z0-9_].
func needsQuoting(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r == '_':
			continue
		case r >= '0' && r <= '9':
			if i == 0 {
				return true
			}
		default:
			return true
		}
	}
	return false
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

// editorDirty reports whether the bound saved query has unsaved edits.
func (m Model) editorDirty() bool {
	return m.SavedName != "" && m.EditorArea.Value() != m.SavedBaseline
}

func (m Model) viewEditor() string {
	w, h := m.Width, m.Height
	rule := dimStyle.Render(strings.Repeat("─", max(w, 1)))

	title := accentStyle.Render("SQL Query")
	if m.SavedName != "" {
		name := m.SavedName
		if m.editorDirty() {
			name += "*"
		}
		title += accentStyle.Render("  ·  " + name)
	}
	if m.CurrentConn != nil {
		title += dimStyle.Render("  ·  " + m.CurrentConn.Name)
	}
	if m.Focus == types.FocusGrid {
		title += dimStyle.Render("   results — tab to edit")
	} else {
		title += dimStyle.Render("   editing — tab to results")
	}

	// top: blank, header, [query tab bar], blank, editor, completion band, rule,
	// blank, status, blank, rule, blank
	top := []string{"", title}
	if tb := m.queryTabBar(w); tb != "" {
		top = append(top, tb)
	}
	top = append(top, "")
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
	// help leads so it survives a narrow-terminal clip; ctrl+g works in both panes.
	kb := []kbd{{"ctrl+g", "help"}}
	if m.Focus == types.FocusGrid {
		kb = append(kb,
			kbd{"↑/↓", "scroll"}, kbd{"n/p", "page"}, kbd{"v", "cell"}, kbd{"w", "wide"}, kbd{"J", "json"},
			kbd{"x", "export"}, kbd{"y", "copy"}, kbd{"tab", "editor"}, kbd{"esc", "back"},
		)
	} else {
		kb = append(kb,
			kbd{"ctrl+r", "run stmt"}, kbd{"alt+r", "run all"}, kbd{"alt+c", "comment"}, kbd{"alt+y", "yank"},
			kbd{"alt+t/]/[/w", "tabs"},
			kbd{"tab", "complete / results"}, kbd{"ctrl+z", "undo"}, kbd{"ctrl+s", "save"}, kbd{"ctrl+o", "saved"},
			kbd{"ctrl+y", "history"}, kbd{"esc", "back"},
		)
	}
	return renderKeyHints(kb, m.Width)
}
