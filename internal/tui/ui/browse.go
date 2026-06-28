package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jorgerojas26/lazysql/drivers"
	"github.com/jorgerojas26/lazysql/internal/tui/types"
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

	// PkColumns is the loaded table's primary key (empty -> whole-row fallback).
	PkColumns []string

	// Pending DML state. Maps auto-merge edits; []DBDMLChange is synthesized at
	// commit. Editable only for a real table (Table != "", not read-only).
	EditCol int               // column being edited via ScreenCellEdit (row = RowCursor)
	Edited  map[[2]int]string // (rowIndex,colIndex) -> new value, existing rows
	Deleted map[int]bool      // existing-row index -> staged delete
	Inserts [][]insertCell    // appended new rows (rendered after Rows)
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
		Edited:     map[[2]int]string{},
		Deleted:    map[int]bool{},
	}
}

// resetPending clears staged DML state (on table switch or after commit).
func (m *Model) resetPending() {
	m.Browse.Edited = map[[2]int]string{}
	m.Browse.Deleted = map[int]bool{}
	m.Browse.Inserts = nil
	m.Browse.ColCursor = 0
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
	switch msg.String() {
	case "esc", "q":
		return m.disconnectBrowse()
	case "e":
		return m.openEditor()
	case "x":
		return m.openExport()
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

// disconnectBrowse closes the live tunnel (which severs the driver pool's
// transport — the Driver interface has no Close) and returns to the list.
// ponytail: pool object lingers until GC; add Driver.Close() when a real
// disconnect/reconnect leak shows up.
func (m Model) disconnectBrowse() (tea.Model, tea.Cmd) {
	if m.ActiveTunnel != nil {
		_ = m.ActiveTunnel.Close()
		m.ActiveTunnel = nil
	}
	m.ActiveDriver = nil
	m.CurrentConn = nil
	m.Browse = browseState{}
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
	return m, nil
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

func (m Model) viewBrowse() string {
	w, h := m.Width, m.Height
	if w < 20 || h < 8 {
		// tiny terminal / test path (Width==0): skip the split layout
		return strings.Join(m.renderTreeLines(max(w, 20), max(h, 4)), "\n")
	}

	bodyH := h - 3 // body + footer + status
	if bodyH < 1 {
		bodyH = 1
	}
	treeW := w * 30 / 100
	if treeW < 20 {
		treeW = 20
	}
	if treeW > 44 {
		treeW = 44
	}
	gridW := w - treeW - 1
	if gridW < 10 {
		gridW = 10
	}

	tree := fitHeight(m.renderTreeLines(treeW, bodyH), treeW, bodyH)
	grid := fitHeight(m.renderGridLines(gridW, bodyH), gridW, bodyH)
	sep := dimStyle.Render("│")

	rows := make([]string, 0, bodyH)
	for i := 0; i < bodyH; i++ {
		rows = append(rows, tree[i]+sep+grid[i])
	}
	return strings.Join(rows, "\n") + "\n" + m.browseFooter() + "\n" + m.getStatusBar()
}

func (m Model) browseFooter() string {
	var kb []struct{ key, desc string }
	if m.Focus == types.FocusTree {
		kb = []struct{ key, desc string }{
			{"↑/↓", "nav"}, {"enter", "open"}, {"→", "expand"}, {"←", "collapse"},
			{"e", "sql"}, {"tab", "grid"}, {"q", "disconnect"},
		}
	} else {
		kb = []struct{ key, desc string }{
			{"c", "edit"}, {"o", "add"}, {"d", "del"}, {"ctrl+s", "commit"},
			{"J", "json"}, {"x", "export"}, {"e", "sql"}, {"tab", "tree"}, {"q", "disconnect"},
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
