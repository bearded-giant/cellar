package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/bearded-giant/cellar/drivers"
	"github.com/bearded-giant/cellar/internal/tui/types"
)

const browsePageSize = 100

// browseState holds everything the in-app browser needs for one live
// connection: the lazily-loaded schema tree and the current results page.
type browseState struct {
	// UseSchemas mirrors the driver: when true the tree renders a schema tier
	// (db -> schema -> table) and table refs are schema-qualified.
	UseSchemas bool

	Databases  []string
	TablesByDB map[string]map[string][]string // db -> group(schema) -> tables
	Expanded   map[string]bool                // node key -> expanded

	Nodes  []treeNode // flattened visible set, rebuilt on tree change
	Cursor int        // index into Nodes

	TableDB string // database arg for the loaded table
	Table   string // schema-qualified ref passed to the driver
	Label   string // display name of the loaded table

	Columns   []string
	Rows      [][]string // data rows only (header stripped off)
	Total     int
	Offset    int
	Limit     int
	RowCursor int
	ColCursor int

	GridErr     string
	GridLoading bool

	// ViewJSON renders the current result page as JSON instead of a table.
	// JSONLines is the cached split of that render; RowCursor scrolls it.
	ViewJSON  bool
	JSONLines []string

	// WideCells lifts the per-cell truncation cap to the pane width so full
	// hashes/tokens show inline (toggle with `w`).
	WideCells bool

	// PkColumns is the loaded table's primary key (empty -> whole-row fallback).
	PkColumns []string

	Sort     string // ORDER BY arg, e.g. "id DESC" (empty = none)
	Where    string // WHERE clause incl. keyword (empty = none)
	MetaKind int    // metaRecords (editable) or a read-only metadata view

	FKMap  map[string]fkRef // local column -> FK target, for FK jump
	Crumbs []crumb          // FK-jump breadcrumb stack

	TreeFilter string // case-insensitive substring filter on the schema tree

	// QueryRows holds the full in-memory result of a SQL query (ExecuteQuery
	// returns everything at once); Rows is the visible page. nil for tables.
	QueryRows [][]string
}

func (m *Model) initBrowse(driver drivers.Driver) {
	useSchemas := false
	if driver != nil {
		useSchemas = driver.UseSchemas()
	}
	m.Browse = browseState{
		UseSchemas: useSchemas,
		TablesByDB: map[string]map[string][]string{},
		Expanded:   map[string]bool{},
		Limit:      browsePageSize,
	}
	m.Tabs = []browseState{m.Browse}
	m.TabActive = 0
	m.GridReturnScreen = types.ScreenBrowse
	m.resetQueryTabs() // fresh connection: buffers restored via LoadQueryState
}

// resetPending resets the per-table view state (on table switch / FK jump).
func (m *Model) resetPending() {
	m.Browse.ColCursor = 0
	m.Browse.Sort = ""
	m.Browse.Where = ""
	m.Browse.MetaKind = metaRecords
	m.Browse.FKMap = nil
	m.Browse.Crumbs = nil
	m.Browse.QueryRows = nil
}

// connIdent is the per-connection key for query history (the connection name).
func (m Model) connIdent() string {
	if m.CurrentConn != nil {
		return m.CurrentConn.Name
	}
	return ""
}

func (m *Model) rebuildTree() {
	m.Browse.Nodes = flattenTree(m.Browse)
	if m.Browse.Cursor >= len(m.Browse.Nodes) {
		m.Browse.Cursor = max(len(m.Browse.Nodes)-1, 0)
	}
}

// refreshJSONView recomputes the cached JSON lines when in JSON view mode.
func (m *Model) refreshJSONView() {
	if !m.Browse.ViewJSON {
		m.Browse.JSONLines = nil
		return
	}
	m.Browse.JSONLines = strings.Split(recordsToJSON(m.Browse.Columns, m.Browse.Rows), "\n")
}

func (m Model) handleBrowseScreen(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.GridReturnScreen = types.ScreenBrowse // grid modals reopen here
	switch msg.String() {
	case "esc", "q":
		// back one level: exit an inspect/meta view to the table first; a
		// grid-focused table backs out to the tree; only the tree confirms disconnect
		if m.Focus == types.FocusGrid && m.Browse.MetaKind != metaRecords {
			m.Browse.MetaKind = metaRecords
			m.Browse.GridLoading = true
			return m.reloadRecords()
		}
		if m.Focus == types.FocusGrid {
			m.Focus = types.FocusTree
			return m, nil
		}
		return m.confirmDisconnect()
	case "e":
		return m.openEditor()
	case "x":
		return m.openExport()
	case "y", "ctrl+y":
		return m.openYank()
	case "H":
		return m.openHistory()
	case "Y":
		return m.openSavedQueries()
	case "?":
		return m.openHelp()
	case "T":
		return m.openSelectedInNewTab()
	case "]":
		return m.switchTab(+1)
	case "[":
		return m.switchTab(-1)
	case "X":
		return m.closeTab()
	case "tab":
		if m.Focus == types.FocusTree {
			m.Focus = types.FocusGrid
		} else {
			m.Focus = types.FocusTree
		}
		return m, nil
	}
	if m.Focus == types.FocusGrid {
		return m.handleBrowseGridKey(msg)
	}
	return m.handleBrowseTreeKey(msg)
}

// confirmDisconnect opens a confirm modal before tearing down the connection,
// so an accidental back-out doesn't drop you to the connections list.
func (m Model) confirmDisconnect() (tea.Model, tea.Cmd) {
	if m.ActiveDriver == nil && m.ActiveTunnel == nil {
		return m, nil // not connected; nothing to confirm
	}
	m.ConfirmType = "disconnect"
	m.ConfirmReturnScreen = m.Screen
	m.Screen = types.ScreenConfirmDelete
	return m, nil
}

// disconnectBrowse closes the live tunnel (which severs the driver pool's
// transport — the Driver interface has no Close) and returns to the list.
// ponytail: pool object lingers until GC; add Driver.Close() when a real
// disconnect/reconnect leak shows up.
func (m Model) disconnectBrowse() (tea.Model, tea.Cmd) {
	m.PersistQueryState() // backstop; autosave-on-run covers the common path
	if m.ActiveTunnel != nil {
		_ = m.ActiveTunnel.Close()
		m.ActiveTunnel = nil
	}
	m.ActiveDriver = nil
	m.CurrentConn = nil
	m.Browse = browseState{}
	m.Tabs = nil
	m.TabActive = 0
	m.resetQueryTabs()
	m.Focus = types.FocusTree
	m.Screen = types.ScreenConnections
	m.StatusMsg = "Disconnected"
	return m, nil
}

func (m Model) handleDatabasesLoadedMsg(msg types.DatabasesLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.Browse.GridErr = "Error loading databases: " + msg.Err.Error()
		return m, nil
	}
	m.Browse.Databases = msg.Databases

	// Land the user on tables: auto-expand the connection's database (or the
	// sole database) and kick its table load.
	target := ""
	if m.CurrentConn != nil && contains(msg.Databases, m.CurrentConn.DBName) {
		target = m.CurrentConn.DBName
	} else if len(msg.Databases) == 1 {
		target = msg.Databases[0]
	}
	if target != "" {
		m.Browse.Expanded[target] = true
		m.rebuildTree()
		return m, m.Cmds.LoadTables(m.ActiveDriver, target)
	}
	m.rebuildTree()
	return m, nil
}

func (m Model) handleTablesLoadedMsg(msg types.TablesLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.Browse.GridErr = "Error loading tables: " + msg.Err.Error()
		return m, nil
	}
	if m.Browse.TablesByDB == nil {
		m.Browse.TablesByDB = map[string]map[string][]string{}
	}
	m.Browse.TablesByDB[msg.DB] = msg.Tables
	m.rebuildTree()
	m.expandDefaultSchema(msg.DB)
	return m, nil
}

// expandDefaultSchema auto-expands and selects the connection's DefaultSchema
// within db (postgres schema tier) so the user lands in its tables. Fires once
// per table load; a no-op when unset, not a schema driver, or already expanded.
func (m *Model) expandDefaultSchema(db string) {
	if m.CurrentConn == nil || m.CurrentConn.DefaultSchema == "" || !m.Browse.UseSchemas {
		return
	}
	gKey := db + treeKeySep + m.CurrentConn.DefaultSchema
	if _, ok := m.Browse.Expanded[gKey]; ok {
		return // already toggled by the user; don't fight them
	}
	m.Browse.Expanded[gKey] = true
	m.rebuildTree()
	for i, n := range m.Browse.Nodes {
		if n.Key == gKey {
			m.Browse.Cursor = i
			break
		}
	}
}

func (m Model) handleRecordsLoadedMsg(msg types.RecordsLoadedMsg) (tea.Model, tea.Cmd) {
	// Drop results for a table the user has since navigated away from.
	if msg.Table != m.Browse.Table {
		return m, nil
	}
	m.Browse.GridLoading = false
	if msg.Err != nil {
		m.Browse.GridErr = "Error: " + msg.Err.Error()
		m.Browse.Columns = nil
		m.Browse.Rows = nil
		return m, nil
	}
	m.Browse.GridErr = ""
	m.Browse.QueryRows = nil // a real table page, not an in-memory query result
	m.Browse.Offset = msg.Offset
	if len(msg.Rows) > 0 {
		m.Browse.Columns = msg.Rows[0]
		m.Browse.Rows = msg.Rows[1:]
	} else {
		m.Browse.Columns = nil
		m.Browse.Rows = nil
	}
	m.Browse.Total = msg.Total
	m.Browse.RowCursor = 0
	m.refreshJSONView()
	return m, nil
}

// browseLayout computes the split-pane geometry shared by viewBrowse and the
// mouse hit-test, so clicks stay aligned with what is rendered.
func (m Model) browseLayout() (treeW, gridW, bodyH int) {
	w, h := m.Width, m.Height
	bodyH = h - 3 - m.tabBarHeight() // tab bar + body + footer + status
	if bodyH < 1 {
		bodyH = 1
	}
	treeW = w * 30 / 100
	if treeW < 20 {
		treeW = 20
	}
	if treeW > 44 {
		treeW = 44
	}
	gridW = w - treeW - 1
	if gridW < 10 {
		gridW = 10
	}
	return treeW, gridW, bodyH
}

func (m Model) viewBrowse() string {
	w, h := m.Width, m.Height
	if w < 20 || h < 8 {
		// tiny terminal / test path (Width==0): skip the split layout
		return strings.Join(m.renderTreeLines(max(w, 20), max(h, 4)), "\n")
	}

	treeW, gridW, bodyH := m.browseLayout()

	tree := fitHeight(m.renderTreeLines(treeW, bodyH), treeW, bodyH)
	grid := fitHeight(m.renderGridLines(gridW, bodyH, true), gridW, bodyH)
	sep := dimStyle.Render("│")

	rows := make([]string, 0, bodyH)
	for i := 0; i < bodyH; i++ {
		rows = append(rows, tree[i]+sep+grid[i])
	}
	var b strings.Builder
	if tb := m.tabBar(w); tb != "" {
		b.WriteString(tb + "\n")
	}
	b.WriteString(strings.Join(rows, "\n") + "\n" + m.browseFooter() + "\n" + m.getStatusBar())
	return b.String()
}

func (m Model) browseFooter() string {
	type kbd = struct{ key, desc string }
	var kb []kbd
	switch {
	case m.Focus == types.FocusTree:
		kb = []kbd{
			{"↑/↓", "nav"}, {"→/enter", "open"}, {"←", "collapse"},
			{"/", "search"}, {"e", "sql"}, {"H/Y", "hist/saved"}, {"tab", "grid"}, {"?", "help"}, {"q", "disconnect"},
		}
	case len(m.Browse.Columns) == 0:
		// grid focused but nothing loaded — only the cross-pane actions apply
		kb = []kbd{
			{"←/tab", "tree"}, {"e", "sql"}, {"H/Y", "hist/saved"}, {"?", "help"}, {"q", "tree"},
		}
	default:
		// table preview: read + navigate. d/o generate DELETE/INSERT SQL into the
		// editor (run there); no in-grid editing. export/json are query-only.
		kb = []kbd{
			{"↑/↓/←/→", "move"}, {"enter", "fk"}, {"s", "sort"}, {"/", "filter"}, {"i", "inspect"},
			{"v", "cell"}, {"w", "wide"}, {"y", "copy"}, {"d/o", "del/insert SQL"}, {"e", "sql"}, {"tab/←", "tree"}, {"q", "tree"},
		}
		if len(m.Browse.Crumbs) > 0 {
			kb = append(kb, kbd{"⌫", "fk-back"})
		}
	}
	kb = append(kb, kbd{"T", "new tab"})
	if len(m.Tabs) > 1 {
		kb = append(kb, kbd{"]/[", "switch"}, kbd{"X", "close tab"})
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

// fitHeight forces lines to exactly height entries, padding short blocks with
// width-wide blanks. It must NOT truncate existing lines — they already carry
// ANSI styling sized to width.
func fitHeight(lines []string, width, height int) []string {
	if len(lines) > height {
		lines = lines[:height]
	}
	blank := strings.Repeat(" ", width)
	for len(lines) < height {
		lines = append(lines, blank)
	}
	return lines
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
