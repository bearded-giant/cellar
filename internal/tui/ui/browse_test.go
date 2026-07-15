package ui

import (
	"reflect"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/bearded-giant/cellar/drivers"
	"github.com/bearded-giant/cellar/internal/tui/commands"
	"github.com/bearded-giant/cellar/internal/tui/types"
)

func browseModel() Model {
	m := Model{Cmds: &commands.Commands{}, Screen: types.ScreenBrowse}
	m.initBrowse(nil)
	return m
}

func tableRefs(nodes []treeNode) []string {
	var out []string
	for _, n := range nodes {
		if n.Kind == kindTable {
			out = append(out, n.Table)
		}
	}
	return out
}

func TestFlattenTree_FlatDriver(t *testing.T) {
	b := browseState{
		Databases:  []string{"app"},
		Expanded:   map[string]bool{"app": true},
		TablesByDB: map[string]map[string][]string{"app": {"app": {"orders", "users"}}},
	}
	nodes := flattenTree(b)
	if len(nodes) != 3 {
		t.Fatalf("want db + 2 tables = 3 nodes, got %d", len(nodes))
	}
	if nodes[0].Kind != kindDB || !nodes[0].Expanded {
		t.Errorf("node0 should be expanded db, got %+v", nodes[0])
	}
	// flat driver: table refs are bare and alphabetically sorted, depth 1
	if got := tableRefs(nodes); !reflect.DeepEqual(got, []string{"orders", "users"}) {
		t.Errorf("table refs = %v, want [orders users]", got)
	}
	if nodes[1].Depth != 1 {
		t.Errorf("flat table depth = %d, want 1", nodes[1].Depth)
	}
}

func TestFlattenTree_SchemaDriver(t *testing.T) {
	b := browseState{
		UseSchemas: true,
		Databases:  []string{"app"},
		Expanded:   map[string]bool{"app": true, "app" + treeKeySep + "public": true},
		TablesByDB: map[string]map[string][]string{
			"app": {"public": {"users"}, "audit": {"log"}},
		},
	}
	nodes := flattenTree(b)
	// db + group audit (collapsed) + group public (expanded) + table public.users
	refs := tableRefs(nodes)
	if !reflect.DeepEqual(refs, []string{"public.users"}) {
		t.Errorf("table refs = %v, want [public.users] (audit collapsed, schema-qualified)", refs)
	}
	// the collapsed audit schema must still appear as a group node
	var sawAudit bool
	for _, n := range nodes {
		if n.Kind == kindGroup && n.Label == "audit" {
			sawAudit = true
		}
	}
	if !sawAudit {
		t.Error("collapsed audit schema should still render as a group node")
	}
}

func TestFlattenTree_CollapsedAndLazy(t *testing.T) {
	collapsed := flattenTree(browseState{
		Databases:  []string{"app"},
		Expanded:   map[string]bool{},
		TablesByDB: map[string]map[string][]string{"app": {"app": {"x"}}},
	})
	if len(collapsed) != 1 {
		t.Errorf("collapsed db should show only itself, got %d nodes", len(collapsed))
	}

	lazy := flattenTree(browseState{
		Databases:  []string{"app"},
		Expanded:   map[string]bool{"app": true},
		TablesByDB: map[string]map[string][]string{}, // tables not loaded yet
	})
	if len(lazy) != 1 {
		t.Errorf("expanded-but-unloaded db should show only itself, got %d nodes", len(lazy))
	}
}

func TestVisibleWindow(t *testing.T) {
	cases := []struct {
		total, cursor, height, wantStart, wantEnd int
	}{
		{5, 0, 10, 0, 5}, // everything fits
		{100, 0, 10, 0, 10},
		{100, 50, 10, 41, 51}, // cursor kept in view
		{100, 99, 10, 90, 100},
	}
	for _, c := range cases {
		s, e := visibleWindow(c.total, c.cursor, c.height)
		if s != c.wantStart || e != c.wantEnd {
			t.Errorf("visibleWindow(%d,%d,%d) = (%d,%d), want (%d,%d)",
				c.total, c.cursor, c.height, s, e, c.wantStart, c.wantEnd)
		}
	}
}

func TestColWidths(t *testing.T) {
	cols := []string{"id", "name"}
	rows := [][]string{{"1", "alpha"}, {"1000", "x"}}
	w := colWidths(cols, rows, maxCellWidth)
	if w[0] != 4 { // "1000"
		t.Errorf("col0 width = %d, want 4", w[0])
	}
	if w[1] != 5 { // "alpha"
		t.Errorf("col1 width = %d, want 5", w[1])
	}

	// cap is enforced
	wide := colWidths([]string{"c"}, [][]string{{"this-is-a-very-long-cell-value-exceeding-the-cap"}}, 10)
	if wide[0] != 10 {
		t.Errorf("capped width = %d, want 10", wide[0])
	}
}

func TestCellCap(t *testing.T) {
	m := Model{Width: 200}
	if got := m.cellCap(); got != maxCellWidth {
		t.Errorf("default cap = %d, want %d", got, maxCellWidth)
	}
	m.Browse.WideCells = true
	if got := m.cellCap(); got != 200 {
		t.Errorf("wide cap = %d, want 200 (pane width)", got)
	}
	m.Width = 20 // narrower than the default cap: don't shrink below it
	if got := m.cellCap(); got != maxCellWidth {
		t.Errorf("wide cap on narrow pane = %d, want %d", got, maxCellWidth)
	}
}

func TestVisibleColsForCursor(t *testing.T) {
	widths := []int{10, 10, 10} // each needs 10 (+3 sep)
	s, e := visibleColsForCursor(widths, 0, 30)
	if s != 0 || e != 2 {
		t.Errorf("cursor 0 = (%d,%d), want (0,2)", s, e)
	}
	// a cursor at the end pulls earlier columns into view
	s, e = visibleColsForCursor(widths, 2, 30)
	if s != 1 || e != 3 {
		t.Errorf("cursor 2 = (%d,%d), want (1,3)", s, e)
	}
	// always at least one column, even when it overflows the width
	s, e = visibleColsForCursor([]int{100}, 0, 5)
	if s != 0 || e != 1 {
		t.Errorf("single overflowing col = (%d,%d), want (0,1)", s, e)
	}
	// cursor is always inside the returned window
	s, e = visibleColsForCursor(widths, 2, 5)
	if !(s <= 2 && 2 < e) {
		t.Errorf("cursor 2 not in window (%d,%d)", s, e)
	}
}

func TestDisplayCell(t *testing.T) {
	if displayCell("NULL&") != "NULL" {
		t.Error("NULL& should render as NULL")
	}
	if displayCell("EMPTY&") != "" {
		t.Error("EMPTY& should render as empty")
	}
	if displayCell("plain") != "plain" {
		t.Error("plain value should pass through")
	}
}

func TestPadTruncateRunes(t *testing.T) {
	if got := padRunes("ab", 5); got != "ab   " {
		t.Errorf("padRunes = %q, want 'ab   '", got)
	}
	if got := truncateRunes("abcdef", 4); got != "abc…" {
		t.Errorf("truncateRunes = %q, want 'abc…'", got)
	}
	if got := padRunes("abcdef", 4); got != "abc…" {
		t.Errorf("padRunes over-width should truncate, got %q", got)
	}
}

func TestTreeKey_NavAndExpand(t *testing.T) {
	m := browseModel()
	m.Browse.Databases = []string{"app", "metrics"}
	m.rebuildTree() // 2 collapsed db nodes

	// j moves the cursor down
	res, _ := m.handleBrowseTreeKey(keyMsg('j'))
	m = res.(Model)
	if m.Browse.Cursor != 1 {
		t.Fatalf("cursor after j = %d, want 1", m.Browse.Cursor)
	}

	// enter on a collapsed, unloaded db expands it and fires a table load
	res2, cmd := m.handleBrowseTreeKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = res2.(Model)
	if !m.Browse.Expanded["metrics"] {
		t.Error("enter should expand the db node")
	}
	if cmd == nil {
		t.Error("expanding an unloaded db should fire a LoadTables cmd")
	}
}

func TestTreeKey_SelectTableLoadsGrid(t *testing.T) {
	m := browseModel()
	m.Browse.Databases = []string{"app"}
	m.Browse.Expanded["app"] = true
	m.Browse.TablesByDB["app"] = map[string][]string{"app": {"widgets"}}
	m.rebuildTree() // [app, widgets]
	m.Browse.Cursor = 1

	res, cmd := m.handleBrowseTreeKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(Model)
	if m.Focus != types.FocusGrid {
		t.Error("selecting a table should move focus to the grid")
	}
	if m.Browse.Table != "widgets" {
		t.Errorf("Table = %q, want widgets", m.Browse.Table)
	}
	if !m.Browse.GridLoading || cmd == nil {
		t.Error("selecting a table should set GridLoading and fire LoadRecords")
	}
}

func TestRecordsLoadedMsg_SplitsHeaderAndStaleDrop(t *testing.T) {
	m := browseModel()
	m.Browse.Table = "widgets"
	m.Browse.GridLoading = true

	// stale result for a different table is dropped
	stale, _ := m.handleRecordsLoadedMsg(types.RecordsLoadedMsg{Table: "other", Rows: [][]string{{"x"}}})
	if cols := stale.(Model).Browse.Columns; cols != nil {
		t.Errorf("stale records should be dropped, got columns %v", cols)
	}

	res, _ := m.handleRecordsLoadedMsg(types.RecordsLoadedMsg{
		Table: "widgets",
		Rows:  [][]string{{"id", "name"}, {"1", "alpha"}},
		Total: 1,
	})
	m = res.(Model)
	if !reflect.DeepEqual(m.Browse.Columns, []string{"id", "name"}) {
		t.Errorf("Columns = %v, want [id name]", m.Browse.Columns)
	}
	if len(m.Browse.Rows) != 1 || m.Browse.Rows[0][1] != "alpha" {
		t.Errorf("data Rows = %v, want one row [1 alpha]", m.Browse.Rows)
	}
	if m.Browse.GridLoading {
		t.Error("GridLoading should clear on result")
	}
}

func TestBrowseScreen_TabAndDisconnect(t *testing.T) {
	m := browseModel()
	m.ActiveDriver = &drivers.SQLite{} // a live connection, so disconnect confirms
	m.Focus = types.FocusTree

	res, _ := m.handleBrowseScreen(tea.KeyMsg{Type: tea.KeyTab})
	m = res.(Model)
	if m.Focus != types.FocusGrid {
		t.Error("tab should toggle focus tree -> grid")
	}

	// q from a grid-focused table backs out to the tree (does not disconnect)
	res, _ = m.handleBrowseScreen(keyMsg('q'))
	m = res.(Model)
	if m.Focus != types.FocusTree || m.Screen != types.ScreenBrowse {
		t.Errorf("q from grid should return to the tree, got focus=%v screen=%v", m.Focus, m.Screen)
	}

	// q from the tree opens a disconnect confirmation, not an immediate disconnect
	res, _ = m.handleBrowseScreen(keyMsg('q'))
	m = res.(Model)
	if m.Screen != types.ScreenConfirmDelete || m.ConfirmType != "disconnect" {
		t.Fatalf("q from tree should confirm disconnect, got screen=%v type=%q", m.Screen, m.ConfirmType)
	}

	// confirming tears the connection down
	res, _ = m.handleConfirmDeleteScreen(keyMsg('y'))
	m = res.(Model)
	if m.Screen != types.ScreenConnections || m.ActiveDriver != nil {
		t.Errorf("confirming should disconnect to the list, got screen=%v driver=%v", m.Screen, m.ActiveDriver)
	}
}

func TestDatabasesLoaded_AutoExpandSingle(t *testing.T) {
	m := browseModel()
	res, cmd := m.handleDatabasesLoadedMsg(types.DatabasesLoadedMsg{Databases: []string{"only"}})
	m = res.(Model)
	if !m.Browse.Expanded["only"] {
		t.Error("a sole database should auto-expand")
	}
	if cmd == nil {
		t.Error("auto-expand should fire LoadTables")
	}
}
