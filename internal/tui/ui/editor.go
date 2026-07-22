package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/bearded-giant/cellar/internal/tui/commands"
	"github.com/bearded-giant/cellar/internal/tui/sqlmeta"
	"github.com/bearded-giant/cellar/internal/tui/types"
	"github.com/bearded-giant/cellar/lib"
)

// completionAreaRows is the fixed band reserved below the editor for the
// autocomplete popup (kept constant so the layout never jumps).
const completionAreaRows = 5

// editorSplitPct is the editor pane's share of the editor+results rows, fixed
// so the split never jumps as the query grows or results land.
const editorSplitPct = 55

// sidebarWidth is the schema pane width in the editor (0 when hidden); same
// clamps as the browse tree so the two screens line up.
func (m Model) sidebarWidth() int {
	if m.SidebarHidden {
		return 0
	}
	tw := m.Width * 30 / 100
	if tw < 20 {
		tw = 20
	}
	if tw > 44 {
		tw = 44
	}
	return tw
}

// queryLayout splits the query workspace vertically: an editor pane on top and
// a results pane below (both to the right of the schema sidebar when shown).
func (m Model) queryLayout() (editorW, editorH, resultsH int) {
	editorW = m.Width
	if sw := m.sidebarWidth(); sw > 0 {
		editorW = m.Width - sw - 1
	}
	if editorW < 10 {
		editorW = 10
	}
	editorH = m.editorHeight()
	resultsH = m.workspaceRows() - editorH
	if resultsH < 0 {
		resultsH = 0
	}
	return editorW, editorH, resultsH
}

// workspaceRows is what's left for editor+results after viewEditor's fixed
// chrome (blanks, title, tab bar, completion band, rules, status, footer).
func (m Model) workspaceRows() int {
	chrome := 15
	if len(m.QueryTabs) > 0 {
		chrome++ // tab bar row
	}
	rows := m.Height - chrome
	if rows < 7 {
		rows = 7
	}
	return rows
}

// editorHeight gives the editor its fixed editorSplitPct share; ctrl+x zoom
// hands the focused pane the whole split (results zoom hides the editor).
func (m Model) editorHeight() int {
	rows := m.workspaceRows()
	if m.PaneZoomed {
		if m.Focus == types.FocusGrid {
			return 0
		}
		return rows
	}
	eh := rows * editorSplitPct / 100
	if eh < 4 {
		eh = 4
	}
	return eh
}

func (m *Model) syncEditorHeight() {
	m.EditorArea.SetHeight(m.editorHeight())
}

func (m Model) newEditorArea(content string) sqlEditor {
	ew, eh, _ := m.queryLayout()
	return newEditor(content, ew, eh)
}

func (m Model) openEditor() (tea.Model, tea.Cmd) {
	m.ensureQueryTabs()
	m.EditorArea = m.newEditorArea(m.EditorContent)
	if m.QueryTabActive < len(m.QueryTabs) {
		t := m.QueryTabs[m.QueryTabActive]
		m.EditorArea.setCursor(t.Row, t.Col)
	}
	m.Completer = m.buildCompleter()
	m.dismissCompletions()
	m.EditorColsLoaded = map[string]bool{}
	m.PaneZoomed = false
	m.Focus = types.FocusEditor
	m.Screen = types.ScreenEditor
	return m, tea.Batch(m.EditorArea.Focus(), m.ensureRefColumns())
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

// focusBackFromEditor is shift+tab from the editor pane: sidebar when shown,
// else straight to results.
func (m Model) focusBackFromEditor() (tea.Model, tea.Cmd) {
	if !m.SidebarHidden {
		m.Focus = types.FocusTree
		return m, nil
	}
	m.Focus = types.FocusGrid
	return m, nil
}

// leaveQueryWorkspace exits the editor+results view back to the tree/grid,
// preserving the query text (re-entered via `e`).
func (m Model) leaveQueryWorkspace() (tea.Model, tea.Cmd) {
	m.ensureQueryTabs()
	m.syncActiveQueryTab() // content + cursor, while the editor is still live
	m.dismissCompletions()
	m.Screen = types.ScreenBrowse
	m.Focus = types.FocusTree
	return m, nil
}

func (m Model) toggleSidebar() (tea.Model, tea.Cmd) {
	m.SidebarHidden = !m.SidebarHidden
	if m.SidebarHidden && m.Focus == types.FocusTree {
		m.Focus = types.FocusEditor
	}
	ew, _, _ := m.queryLayout()
	m.EditorArea.SetWidth(ew)
	return m, m.autosaveQueryState() // the pref rides along with the buffers
}

// handleEditorSidebarKey routes keys while the schema sidebar is focused:
// enter on a table/view inserts its reference at the editor cursor; everything
// else (nav, expand, filter, inspect) is the shared browse tree behavior.
func (m Model) handleEditorSidebarKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab", "esc", "q":
		m.Focus = types.FocusEditor
		return m, nil
	case "shift+tab": // reverse cycle: sidebar -> results
		m.Focus = types.FocusGrid
		return m, nil
	case "enter", " ", "space", "right", "l":
		if n := len(m.Browse.Nodes); n > 0 && m.Browse.Cursor < n {
			node := m.Browse.Nodes[m.Browse.Cursor]
			if node.Kind == kindTable || node.Kind == kindView {
				if s := msg.String(); s == "right" || s == "l" {
					return m, nil
				}
				return m.insertTableRef(node)
			}
		}
	}
	return m.handleBrowseTreeKey(msg)
}

// insertTableRef drops the node's reference into the editor at the cursor,
// quoting each dotted part only when the dialect needs it.
func (m Model) insertTableRef(node treeNode) (tea.Model, tea.Cmd) {
	if node.Table == "" {
		return m, nil
	}
	ref := node.Table
	if m.ActiveDriver != nil {
		parts := strings.Split(node.Table, ".")
		for i, p := range parts {
			if needsQuoting(p) {
				parts[i] = m.ActiveDriver.FormatReference(p)
			}
		}
		ref = strings.Join(parts, ".")
	}
	m.EditorArea.pushUndo()
	m.EditorArea.insert(ref)
	m.Focus = types.FocusEditor
	m.syncEditorHeight()
	return m, nil
}

func (m Model) handleEditorScreen(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.PeekOpen { // the floating peek owns input until closed
		return m.handlePeekKey(msg)
	}
	if m.InspOpen { // so does the floating inspector
		return m.handleInspectorKey(msg)
	}
	m.GridReturnScreen = types.ScreenEditor // grid modals from the results pane reopen here

	if msg.String() == "esc" && m.QueryRunning {
		if commands.CancelRunningQuery() {
			m.StatusMsg = "Cancelling query…"
		}
		return m, nil
	}

	// workspace-wide actions (both panes)
	switch msg.String() {
	case "ctrl+s":
		return m.openSaveQuery()
	case "ctrl+o": // saved + history picker
		m.EditorContent = m.EditorArea.Value() // preserve the buffer if cancelled
		return m.openSavedQueries()
	case "ctrl+t":
		return m.newQueryTab()
	// ctrl+]/ctrl+[ need kitty disambiguation (legacy encodes ctrl+[ as esc);
	// the pg chords stay as aliases for full-size keyboards.
	case "ctrl+]", "ctrl+pgdown":
		return m.switchQueryTab(+1)
	case "ctrl+[", "ctrl+pgup":
		return m.switchQueryTab(-1)
	case "ctrl+w":
		return m.closeQueryTab()
	case "ctrl+shift+s": // save-as/rename: always prompts, even on a bound buffer
		return m.openSaveQueryAs()
	// ctrl+enter needs the kitty keyboard protocol (wezterm et al.); legacy
	// terminals deliver it as plain enter, so ctrl+r stays the fallback bind.
	case "ctrl+enter", "ctrl+r":
		if m.QueryRunning { // single global query slot — a second run would orphan the first's cancel handle
			m.StatusMsg = "Query already running — esc cancels"
			return m, nil
		}
		m.EditorContent = m.EditorArea.Value()
		// run only the statement under the cursor (';'-delimited); single-statement
		// buffers return the whole text unchanged.
		query := sqlmeta.StatementAt(m.EditorContent, m.EditorArea.cursorOffset())
		if strings.TrimSpace(query) == "" {
			return m, nil
		}
		m.dismissCompletions() // else a lingering popup eats the tab-to-results
		m.Browse.GridLoading = true
		m.QueryRunning = true
		m.StatusMsg = "Running query… esc cancels"
		readOnly := m.CurrentConn != nil && m.CurrentConn.ReadOnly
		return m, tea.Batch(
			m.Cmds.RunQuery(m.ActiveDriver, query, readOnly, m.connIdent()),
			m.autosaveQueryState(),
			m.Spinner.Tick,
		)
	case "ctrl+shift+enter": // run every ';'-delimited statement in order (notebook)
		if m.QueryRunning {
			m.StatusMsg = "Query already running — esc cancels"
			return m, nil
		}
		m.EditorContent = m.EditorArea.Value()
		stmts := sqlmeta.SplitStatements(m.EditorContent)
		if len(stmts) == 0 {
			return m, nil
		}
		m.dismissCompletions() // else a lingering popup eats the tab-to-results
		m.Browse.GridLoading = true
		m.QueryRunning = true
		m.StatusMsg = "Running all statements… esc cancels"
		readOnly := m.CurrentConn != nil && m.CurrentConn.ReadOnly
		return m, tea.Batch(
			m.Cmds.RunQueries(m.ActiveDriver, stmts, readOnly, m.connIdent()),
			m.autosaveQueryState(),
			m.Spinner.Tick,
		)
	case "ctrl+1", "ctrl+2", "ctrl+3", "ctrl+4", "ctrl+5",
		"ctrl+6", "ctrl+7", "ctrl+8", "ctrl+9":
		return m.jumpQueryTab(int(msg.Key().Code - '1'))
	case "ctrl+b":
		return m.toggleSidebar()
	case "ctrl+x": // tmux-style zoom; follows focus, so tab flips which pane is full
		m.PaneZoomed = !m.PaneZoomed
		m.syncEditorHeight()
		return m, nil
	}

	// schema sidebar focused: tree nav + insert-at-cursor.
	if m.Focus == types.FocusTree {
		return m.handleEditorSidebarKey(msg)
	}

	// results pane focused: single-key grid affordances + back/cycle.
	if m.Focus == types.FocusGrid {
		switch msg.String() {
		case "tab": // cycle onward: results -> sidebar (when shown) -> editor
			if !m.SidebarHidden {
				m.Focus = types.FocusTree
				return m, nil
			}
			m.Focus = types.FocusEditor
			return m, nil
		case "shift+tab": // reverse cycle: results -> editor
			m.Focus = types.FocusEditor
			return m, nil
		case "esc": // esc backs up one level: results -> editor text
			m.Focus = types.FocusEditor
			return m, nil
		case "q":
			return m.leaveQueryWorkspace()
		case "]":
			return m.switchQueryTab(+1)
		case "[":
			return m.switchQueryTab(-1)
		case "x":
			return m.openExport()
		case "y":
			return m.openYank()
		case "?":
			return m.openHelp()
		}
		return m.handleBrowseGridKey(msg)
	}

	// editor (text) focused.
	switch msg.String() {
	case "ctrl+space":
		m.showCompletionsManual()
		return m, nil
	}

	// popup keys: ctrl+n/ctrl+p always engage; ↑/↓, tab and esc only belong to
	// the popup once engaged (a passive popup leaves them to the editor).
	if m.CompVisible {
		switch msg.String() {
		case "ctrl+p":
			m.CompEngaged = true
			if m.CompCursor > 0 {
				m.CompCursor--
			}
			return m, nil
		case "ctrl+n":
			m.CompEngaged = true
			if m.CompCursor < len(m.Completions)-1 {
				m.CompCursor++
			}
			return m, nil
		case "up":
			if m.CompEngaged {
				if m.CompCursor > 0 {
					m.CompCursor--
				}
				return m, nil
			}
		case "down":
			if m.CompEngaged {
				if m.CompCursor < len(m.Completions)-1 {
					m.CompCursor++
				}
				return m, nil
			}
		case "tab":
			if m.CompEngaged {
				m.acceptCompletion()
				return m, nil
			}
			m.dismissCompletions() // passive: tab keeps its pane-cycle job
			m.Focus = types.FocusGrid
			return m, nil
		case "shift+tab": // never accepts — always dismiss + reverse cycle
			m.dismissCompletions()
			return m.focusBackFromEditor()
		case "esc":
			m.suppressCompletions()
			return m, nil
		}
	}
	switch msg.String() {
	case "tab": // no popup: move down to the results pane
		m.Focus = types.FocusGrid
		return m, nil
	case "shift+tab": // reverse cycle: editor -> sidebar (results when hidden)
		return m.focusBackFromEditor()
	case "esc": // back one level: leave the workspace to the tree
		return m.leaveQueryWorkspace()
	// ctrl+/ arrives as the ctrl+_ byte in legacy terminals (kitty sends the name)
	case "ctrl+_", "ctrl+/": // toggle "-- " comment across the statement under the cursor
		start, end := sqlmeta.StatementBoundsAt(m.EditorArea.Value(), m.EditorArea.cursorOffset())
		m.EditorArea.toggleCommentSpan(start, end)
		m.refreshCompletions()
		return m, nil
	case "ctrl+y": // yank the cursor line to the clipboard
		if err := lib.NewClipboard().Write(m.EditorArea.currentLine()); err != nil {
			m.StatusMsg = "Copy failed: " + err.Error()
		} else {
			m.StatusMsg = "Yanked line to clipboard"
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.EditorArea, cmd = m.EditorArea.Update(msg)
	m.syncEditorHeight()
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
	m.CompEngaged = false
	m.CompDismissed = false
}

// suppressCompletions hides the popup and remembers the word under the cursor
// so auto-show stays off until that word changes (esc-dismiss memory).
func (m *Model) suppressCompletions() {
	prefix := currentPrefix(m.EditorArea.Value(), m.EditorArea.cursorOffset())
	m.CompVisible = false
	m.Completions = nil
	m.CompEngaged = false
	m.CompDismissed = true
	m.CompDismissedAt = m.EditorArea.cursorOffset() - len([]rune(prefix))
	m.CompDismissedPrefix = prefix
}

// showCompletionsManual (ctrl+space) bypasses the min-prefix gate and any
// esc-dismiss suppression, e.g. to list columns right after "table.".
func (m *Model) showCompletionsManual() {
	if m.Completer == nil {
		return
	}
	m.CompDismissed = false
	m.Completions = m.Completer.Complete(m.EditorArea.Value(), m.EditorArea.cursorOffset())
	m.CompCursor = 0
	m.CompEngaged = false
	m.CompVisible = len(m.Completions) > 0
}

// completionMinPrefix is the auto-show threshold: a visible popup survives
// narrowing back to a 1-rune prefix, but never opens below 2.
func completionMinPrefix(visible bool) int {
	if visible {
		return 1
	}
	return 2
}

// completionSuppressed reports whether an esc-dismissed popup still covers the
// word at wordStart: same start and the prefix still extends the dismissed one.
func completionSuppressed(wordStart int, prefix string, dismissedAt int, dismissedPrefix string) bool {
	return wordStart == dismissedAt && strings.HasPrefix(prefix, dismissedPrefix)
}

func (m *Model) refreshCompletions() {
	if m.Completer == nil {
		m.CompVisible = false
		return
	}
	text := m.EditorArea.Value()
	off := m.EditorArea.cursorOffset()
	prefix := currentPrefix(text, off)
	if m.CompDismissed &&
		!completionSuppressed(off-len([]rune(prefix)), prefix, m.CompDismissedAt, m.CompDismissedPrefix) {
		m.CompDismissed = false
	}
	if m.CompDismissed || len([]rune(prefix)) < completionMinPrefix(m.CompVisible) {
		m.CompVisible = false
		m.Completions = nil
		m.CompEngaged = false
		return
	}
	m.Completions = m.Completer.Complete(text, off)
	m.CompCursor = 0
	m.CompEngaged = false
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
	// a typed opening quote ("Cu<tab>) belongs to the word being replaced —
	// consume it, the insert below re-quotes for the dialect when needed
	if start > 0 && (r[start-1] == '"' || r[start-1] == '`' || r[start-1] == '[') {
		start--
	}
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
	m.EditorArea.setCursorOffset(start + len([]rune(insert)))
	m.dismissCompletions()
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
	wasRunning := m.QueryRunning
	m.QueryRunning = false
	// a result landing after disconnect/reconnect belongs to a dead session —
	// applying it would drop the user into a ghost editor (reset EditorArea
	// panics on the first keystroke) or clobber the new session's state
	if !wasRunning || m.ActiveDriver == nil {
		return m, nil
	}
	// only claim the display from inside the query workspace or a modal opened
	// over it; hijacking Screen/Focus from anywhere else kills open prompts or
	// yanks the user out of a browse table
	inEditor := m.Screen == types.ScreenEditor
	overEditor := m.GridReturnScreen == types.ScreenEditor &&
		m.Screen != types.ScreenBrowse && m.Screen != types.ScreenConnections
	if !inEditor && !overEditor {
		m.StatusMsg = "Query finished — result discarded (left the query workspace)"
		return m, nil
	}
	m.Browse.GridLoading = false
	if inEditor {
		m.Focus = types.FocusEditor
	}

	m.Browse.Table = ""
	m.Browse.TableDB = ""
	m.Browse.Offset = 0
	m.Browse.RowCursor = 0
	m.resetPending()

	if isCancelledErr(msg.Err) {
		m.Browse.GridErr = ""
		m.Browse.Columns = nil
		m.Browse.Rows = nil
		m.Browse.Label = "query"
		m.StatusMsg = "Query cancelled"
		return m, nil
	}
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
		if msg.Truncated {
			m.StatusMsg = fmt.Sprintf("Query OK — first %d rows (capped; QueryRowLimit in config raises it)", m.Browse.Total)
		} else {
			m.StatusMsg = fmt.Sprintf("Query OK — %d rows", m.Browse.Total)
		}
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
		msg = "Ready — ctrl+enter to run"
	}
	if m.QueryRunning {
		return successStyle.Render(m.Spinner.View() + " " + msg)
	}
	if m.Browse.GridErr != "" {
		return errorStyle.Render(msg)
	}
	return successStyle.Render(msg)
}

// isCancelledErr matches a user-cancelled query across drivers: context
// cancellation, or sqlite/modernc's "interrupted" when killed mid-execution.
func isCancelledErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return true
	}
	s := err.Error()
	// "interrupted (9)" is modernc/sqlite's SQLITE_INTERRUPT; a bare
	// "interrupted" substring would match user identifiers in error text
	return strings.Contains(s, "context canceled") || strings.Contains(s, "interrupted (9)")
}

// editorDirty reports whether the bound saved query has unsaved edits.
func (m Model) editorDirty() bool {
	return m.SavedName != "" && m.EditorArea.Value() != m.SavedBaseline
}

func (m Model) viewEditor() string {
	w, h := m.Width, m.Height
	m.syncEditorHeight() // grow/shrink the pane to the query + results state
	sw := m.sidebarWidth()
	rw := w
	if sw > 0 {
		rw = max(w-sw-1, 10)
	}
	rule := dimStyle.Render(strings.Repeat("─", max(rw, 1)))

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
	switch m.Focus {
	case types.FocusGrid:
		title += dimStyle.Render("   results — tab to schema/editor")
	case types.FocusTree:
		title += dimStyle.Render("   schema — enter inserts, tab to edit")
	default:
		title += dimStyle.Render("   editing — tab to results")
	}
	if m.PaneZoomed {
		title += dimStyle.Render("  ·  zoomed (ctrl+x)")
	}
	zoomResults := m.PaneZoomed && m.Focus == types.FocusGrid

	// top: blank, header, [query tab bar], blank, editor, completion band, rule,
	// blank, status, blank, rule, blank
	top := []string{"", title}
	if tb := m.queryTabBar(rw); tb != "" {
		top = append(top, tb)
	}
	top = append(top, "")
	if !zoomResults {
		edLines := strings.Split(m.EditorArea.View(), "\n")
		if m.Focus == types.FocusEditor {
			edLines = tintBG(edLines, rw)
		}
		top = append(top, edLines...)
		top = append(top, m.renderCompletions(rw, completionAreaRows)...)
	}
	top = append(top, rule, "", m.queryStatusLine())

	// footer pinned to the bottom: blank, rule, blank, keybinds
	foot := []string{"", rule, "", m.editorFooter(rw)}

	// results fill the gap so the footer sits at the bottom of the window
	resultsH := h - len(top) - len(foot)
	if resultsH < 0 {
		resultsH = 0
	}
	var results []string
	if resultsH > 0 {
		results = m.renderGridLines(rw, resultsH, false)
		for len(results) < resultsH {
			results = append(results, "")
		}
		results = results[:resultsH]
		if m.Focus == types.FocusGrid {
			results = tintBG(results, rw)
		}
	}

	right := append(top, results...)
	right = append(right, foot...)
	if sw == 0 {
		return strings.Join(right, "\n")
	}

	tree := fitHeight(m.renderTreeLines(sw, len(right)), sw, len(right))
	if m.Focus == types.FocusTree {
		tree = tintBG(tree, sw)
	}
	sep := dimStyle.Render("│")
	rows := make([]string, len(right))
	for i := range right {
		rows[i] = tree[i] + sep + right[i]
	}
	return strings.Join(rows, "\n")
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

func (m Model) editorFooter(width int) string {
	// help leads so it survives a narrow-terminal clip; ctrl+g works in all panes.
	kb := []kbd{{"ctrl+g", "help"}}
	switch m.Focus {
	case types.FocusGrid:
		kb = append(kb,
			kbd{"↑/↓", "scroll"}, kbd{"n/p", "page"}, kbd{"v/V", "peek/cell"}, kbd{"w", "wide"}, kbd{"J", "json"},
			kbd{"x", "export"}, kbd{"y", "copy"}, kbd{"ctrl+x", "zoom"}, kbd{"]/[", "query tab"}, kbd{"tab/esc", "editor"}, kbd{"q", "back"},
		)
	case types.FocusTree:
		kb = append(kb,
			kbd{"↑/↓", "nav"}, kbd{"enter", "insert name"}, kbd{"→/←", "expand/collapse"},
			kbd{"/", "search"}, kbd{"i", "inspect"}, kbd{"ctrl+b", "hide schema"},
			kbd{"tab/esc", "editor"},
		)
	default:
		kb = append(kb,
			kbd{"ctrl+enter", "run stmt (ctrl+r)"}, kbd{"ctrl+shift+enter", "run all"},
			kbd{"ctrl+/", "comment"}, kbd{"ctrl+y", "yank"},
			kbd{"ctrl+t/w", "tabs"}, kbd{"ctrl+]/[", "switch tab"}, kbd{"ctrl+1..9", "tab N"},
			kbd{"ctrl+space", "complete"}, kbd{"tab", "results"}, kbd{"ctrl+x", "zoom"}, kbd{"ctrl+b", "schema"}, kbd{"ctrl+z", "undo"},
			kbd{"ctrl+s", "save"}, kbd{"ctrl+shift+s", "save as"},
			kbd{"ctrl+o", "queries"}, kbd{"esc", "back"},
		)
	}
	return renderKeyHints(kb, width)
}
