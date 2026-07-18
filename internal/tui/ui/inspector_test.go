package ui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/bearded-giant/cellar/internal/tui/types"
)

func inspectorModel() Model {
	m := gridModel()
	m.Width, m.Height = 100, 30
	res, _ := m.openInspector("app", "widgets", "widgets", false)
	return res.(Model)
}

func TestInspector_OpenFromGrid(t *testing.T) {
	m := gridModel()
	res, cmd := m.openInspectorFromGrid()
	m = res.(Model)
	if !m.InspOpen || m.InspTarget != "widgets" || m.InspIsView {
		t.Fatalf("inspector state: %+v", m)
	}
	if cmd == nil {
		t.Error("first tab should lazy-load via cmd")
	}
	if len(m.InspTabs) != 4 || m.InspTabs[inspTableDDL].title != "DDL" {
		t.Errorf("table tabs = %+v", m.InspTabs)
	}
}

func TestInspector_GridNoTableHints(t *testing.T) {
	m := gridModel()
	m.Browse.Table = ""
	res, cmd := m.openInspectorFromGrid()
	m = res.(Model)
	if m.InspOpen || cmd != nil {
		t.Fatal("no table context must not open the inspector")
	}
	if m.StatusMsg == "" {
		t.Error("expected a status hint")
	}
}

func TestInspector_ViewTabs(t *testing.T) {
	m := gridModel()
	res, _ := m.openInspector("app", "v_totals", "v_totals", true)
	m = res.(Model)
	if len(m.InspTabs) != 2 || m.InspTabs[inspViewDefinition].title != "Definition" {
		t.Errorf("view tabs = %+v", m.InspTabs)
	}
}

func TestInspector_TreeIKey(t *testing.T) {
	m := gridModel()
	m.Focus = types.FocusTree
	m.Browse.Nodes = []treeNode{
		{Key: "app", Label: "app", Kind: kindDB, DB: "app"},
		{Key: "app\x1fwidgets", Label: "widgets", Kind: kindTable, DB: "app", Table: "widgets"},
	}
	m.Browse.Cursor = 1
	res, cmd := m.handleBrowseTreeKey(keyMsg('i'))
	m = res.(Model)
	if !m.InspOpen || m.InspTarget != "widgets" || cmd == nil {
		t.Fatalf("tree i should open inspector: open=%v target=%q", m.InspOpen, m.InspTarget)
	}

	m.closeInspector()
	m.Browse.Cursor = 0
	res, _ = m.handleBrowseTreeKey(keyMsg('i'))
	m = res.(Model)
	if m.InspOpen {
		t.Error("i on a db node must not open the inspector")
	}
}

func TestInspector_TabSwitchLazyLoads(t *testing.T) {
	m := inspectorModel()
	res, cmd := m.switchInspectorTab(inspTableDDL)
	m = res.(Model)
	if m.InspTab != inspTableDDL || cmd == nil {
		t.Fatal("switching to DDL should fire its loader")
	}
	res, _ = m.handleTableDDLLoadedMsg(types.TableDDLLoadedMsg{Table: "widgets", DDL: "CREATE TABLE widgets (id);"})
	m = res.(Model)
	res, cmd = m.switchInspectorTab(inspTableColumns)
	m = res.(Model)
	res, cmd = m.switchInspectorTab(inspTableDDL)
	m = res.(Model)
	if cmd != nil {
		t.Error("loaded tab must not re-fire its loader")
	}
	if got := strings.Join(m.inspectorDisplayLines(), "\n"); !strings.Contains(got, "CREATE TABLE widgets") {
		t.Errorf("DDL body = %q", got)
	}
}

func TestInspector_DDLMsgGuards(t *testing.T) {
	m := inspectorModel()
	res, _ := m.handleTableDDLLoadedMsg(types.TableDDLLoadedMsg{Table: "other", DDL: "nope"})
	m = res.(Model)
	if m.InspTabs[inspTableDDL].loaded {
		t.Error("mismatched table must not fill the DDL tab")
	}
	res, _ = m.handleTableDDLLoadedMsg(types.TableDDLLoadedMsg{Table: "widgets", Err: errors.New("boom")})
	m = res.(Model)
	tab := m.InspTabs[inspTableDDL]
	if !tab.loaded || tab.err != "boom" {
		t.Errorf("error fill = %+v", tab)
	}
}

func TestInspector_ViewDefinitionMsg(t *testing.T) {
	m := gridModel()
	res, _ := m.openInspector("app", "v_totals", "v_totals", true)
	m = res.(Model)
	res, _ = m.handleViewDefinitionLoadedMsg(types.ViewDefinitionLoadedMsg{View: "v_totals", Definition: "SELECT 1"})
	m = res.(Model)
	tab := m.InspTabs[inspViewDefinition]
	if !tab.loaded || tab.raw != "SELECT 1" {
		t.Errorf("definition tab = %+v", tab)
	}
}

func TestInspector_KeyRouting(t *testing.T) {
	m := inspectorModel()
	res, _ := m.handleInspectorKey(tea.KeyPressMsg{Code: tea.KeyTab})
	m = res.(Model)
	if m.InspTab != inspTableIndexes {
		t.Errorf("tab -> %d, want indexes", m.InspTab)
	}
	res, _ = m.handleInspectorKey(keyMsg('4'))
	m = res.(Model)
	if m.InspTab != inspTableDDL {
		t.Errorf("4 -> %d, want DDL", m.InspTab)
	}
	res, _ = m.handleInspectorKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = res.(Model)
	if m.InspOpen {
		t.Error("esc must close")
	}
}

func TestInspector_BrowseKeyGuard(t *testing.T) {
	m := inspectorModel()
	m.Screen = types.ScreenBrowse
	res, _ := m.handleBrowseScreen(keyMsg('q'))
	m = res.(Model)
	if m.InspOpen {
		t.Error("q while inspector open should close it, not leave browse")
	}
	if m.Screen != types.ScreenBrowse {
		t.Errorf("screen changed to %v", m.Screen)
	}
}

func TestInspector_ScrollClamp(t *testing.T) {
	m := inspectorModel()
	res, _ := m.handleMetaLoadedMsg(types.MetaLoadedMsg{
		Kind: int(0),
		Rows: [][]string{{"name"}, {"a"}, {"b"}, {"c"}},
	})
	m = res.(Model)
	m.InspScroll = 999
	res, _ = m.handleInspectorKey(keyMsg('j'))
	m = res.(Model)
	if last := len(m.inspectorDisplayLines()) - 1; m.InspScroll > last {
		t.Errorf("scroll %d beyond last line %d", m.InspScroll, last)
	}
}

func TestInspector_ComposeOverBase(t *testing.T) {
	m := inspectorModel()
	m.Width, m.Height = 100, 30
	out := m.composeInspector(strings.TrimRight(strings.Repeat(strings.Repeat("x", 100)+"\n", 30), "\n"))
	if lines := strings.Split(out, "\n"); len(lines) != 30 {
		t.Fatalf("composited height = %d, want 30", len(lines))
	}
	if !strings.Contains(stripANSI(out), "widgets") {
		t.Error("inspector title missing from composite")
	}
}
