package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/bearded-giant/cellar/drivers"
	"github.com/bearded-giant/cellar/internal/state"
	"github.com/bearded-giant/cellar/internal/tui/types"
)

func sidebarModel(t *testing.T) Model {
	t.Helper()
	m := editorModel(t)
	m.Browse.Nodes = []treeNode{
		{Key: "app", Label: "app", Kind: kindDB, DB: "app", HasKids: true, Expanded: true},
		{Key: "app\x1fusers", Label: "users", Kind: kindTable, DB: "app", Table: "users"},
		{Key: "app\x1fOrders", Label: "Orders", Kind: kindTable, DB: "app", Table: "Orders"},
		{Key: "app\x1fv_totals", Label: "v_totals", Kind: kindView, DB: "app", Table: "v_totals"},
	}
	return m
}

func TestSidebar_DefaultShownAndLayout(t *testing.T) {
	m := sidebarModel(t)
	if m.SidebarHidden {
		t.Fatal("sidebar should default to shown")
	}
	sw := m.sidebarWidth()
	if sw < 20 || sw > 44 {
		t.Errorf("sidebar width %d out of clamp", sw)
	}
	ew, _, _ := m.queryLayout()
	if ew != m.Width-sw-1 {
		t.Errorf("editor width %d, want %d", ew, m.Width-sw-1)
	}
}

func TestSidebar_ToggleRestoresFullWidth(t *testing.T) {
	m := sidebarModel(t)
	res, _ := m.handleEditorScreen(tea.KeyPressMsg{Code: 'b', Mod: tea.ModCtrl})
	m = res.(Model)
	if !m.SidebarHidden {
		t.Fatal("ctrl+b should hide the sidebar")
	}
	if ew, _, _ := m.queryLayout(); ew != m.Width {
		t.Errorf("hidden sidebar editor width %d, want %d", ew, m.Width)
	}
	if m.sidebarWidth() != 0 {
		t.Error("hidden sidebar width must be 0")
	}
}

func TestSidebar_ToggleWhileFocusedRefocusesEditor(t *testing.T) {
	m := sidebarModel(t)
	m.Focus = types.FocusTree
	res, _ := m.handleEditorScreen(tea.KeyPressMsg{Code: 'b', Mod: tea.ModCtrl})
	m = res.(Model)
	if m.Focus != types.FocusEditor {
		t.Errorf("focus = %v, want editor", m.Focus)
	}
}

func TestSidebar_FocusCycle(t *testing.T) {
	m := sidebarModel(t)
	m.Focus = types.FocusEditor
	tab := tea.KeyPressMsg{Code: tea.KeyTab}

	res, _ := m.handleEditorScreen(tab) // editor -> results
	m = res.(Model)
	if m.Focus != types.FocusGrid {
		t.Fatalf("editor tab -> %v, want grid", m.Focus)
	}
	res, _ = m.handleEditorScreen(tab) // results -> sidebar
	m = res.(Model)
	if m.Focus != types.FocusTree {
		t.Fatalf("grid tab -> %v, want tree", m.Focus)
	}
	res, _ = m.handleEditorScreen(tab) // sidebar -> editor
	m = res.(Model)
	if m.Focus != types.FocusEditor {
		t.Fatalf("tree tab -> %v, want editor", m.Focus)
	}

	m.SidebarHidden = true
	m.Focus = types.FocusGrid
	res, _ = m.handleEditorScreen(tab) // hidden sidebar: results -> editor
	m = res.(Model)
	if m.Focus != types.FocusEditor {
		t.Errorf("hidden-sidebar grid tab -> %v, want editor", m.Focus)
	}
}

func TestSidebar_FocusCycleReverse(t *testing.T) {
	m := sidebarModel(t)
	m.Focus = types.FocusEditor
	backtab := tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}

	res, _ := m.handleEditorScreen(backtab) // editor -> sidebar
	m = res.(Model)
	if m.Focus != types.FocusTree {
		t.Fatalf("editor shift+tab -> %v, want tree", m.Focus)
	}
	res, _ = m.handleEditorScreen(backtab) // sidebar -> results
	m = res.(Model)
	if m.Focus != types.FocusGrid {
		t.Fatalf("tree shift+tab -> %v, want grid", m.Focus)
	}
	res, _ = m.handleEditorScreen(backtab) // results -> editor
	m = res.(Model)
	if m.Focus != types.FocusEditor {
		t.Fatalf("grid shift+tab -> %v, want editor", m.Focus)
	}

	m.SidebarHidden = true
	res, _ = m.handleEditorScreen(backtab) // hidden sidebar: editor -> results
	m = res.(Model)
	if m.Focus != types.FocusGrid {
		t.Errorf("hidden-sidebar editor shift+tab -> %v, want grid", m.Focus)
	}
}

func TestSidebar_EnterInsertsQuotedRef(t *testing.T) {
	m := sidebarModel(t)
	m.ActiveDriver = &drivers.Postgres{}
	m.Focus = types.FocusTree
	m.EditorArea.SetValue("select * from ")
	m.EditorArea.CursorEnd()

	m.Browse.Cursor = 2 // Orders — needs quoting
	res, _ := m.handleEditorScreen(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = res.(Model)
	if got := m.EditorArea.Value(); !strings.Contains(got, `"Orders"`) {
		t.Errorf("editor = %q, want quoted Orders", got)
	}
	if m.Focus != types.FocusEditor {
		t.Errorf("focus after insert = %v, want editor", m.Focus)
	}
}

func TestSidebar_EnterInsertsBareSnakeCase(t *testing.T) {
	m := sidebarModel(t)
	m.ActiveDriver = &drivers.Postgres{}
	m.Focus = types.FocusTree
	m.Browse.Cursor = 1 // users
	res, _ := m.handleEditorScreen(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = res.(Model)
	if got := m.EditorArea.Value(); !strings.Contains(got, "users") || strings.Contains(got, `"users"`) {
		t.Errorf("editor = %q, want bare users", got)
	}
}

func TestSidebar_TreeFilterReturnsToEditor(t *testing.T) {
	m := sidebarModel(t)
	m.Focus = types.FocusTree
	res, _ := m.handleEditorScreen(keyMsg('/'))
	m = res.(Model)
	if m.Screen != types.ScreenTreeFilter {
		t.Fatalf("/ should open the tree filter, got %v", m.Screen)
	}
	res, _ = m.handleTreeFilterScreen(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = res.(Model)
	if m.Screen != types.ScreenEditor {
		t.Errorf("filter esc returned to %v, want editor", m.Screen)
	}
}

func TestSidebar_PersistsWithQueryState(t *testing.T) {
	m := sidebarModel(t)
	m.SidebarHidden = true
	if st := m.queryStateSnapshot(); !st.SidebarHidden {
		t.Fatal("snapshot must carry SidebarHidden")
	}

	fresh := browseModel()
	fresh.Width, fresh.Height = 100, 30
	res, _ := fresh.handleQueryStateLoadedMsg(types.QueryStateLoadedMsg{
		State: state.State{SidebarHidden: true, Tabs: []state.Tab{{SQL: "select 1", Active: true}}},
	})
	fresh = res.(Model)
	if !fresh.SidebarHidden {
		t.Error("restore must carry SidebarHidden back")
	}
}

func TestSidebar_ViewComposesTreePane(t *testing.T) {
	m := sidebarModel(t)
	m.Focus = types.FocusTree
	out := stripANSI(m.viewEditor())
	if !strings.Contains(out, "users") || !strings.Contains(out, "│") {
		t.Error("sidebar view should render tree nodes beside the editor")
	}
	m.SidebarHidden = true
	if out := stripANSI(m.viewEditor()); strings.Contains(out, "◇ v_totals") {
		t.Error("hidden sidebar must not render tree nodes")
	}
}
