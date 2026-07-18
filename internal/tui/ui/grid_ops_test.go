package ui

import (
	"testing"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/bearded-giant/cellar/internal/tui/types"
)

func TestCycleSort(t *testing.T) {
	m := gridModel()
	m.Browse.ColCursor = 0 // id

	res, _ := m.cycleSort()
	m = res.(Model)
	if m.Browse.Sort != "id ASC" {
		t.Errorf("first sort = %q, want 'id ASC'", m.Browse.Sort)
	}
	res, _ = m.cycleSort()
	m = res.(Model)
	if m.Browse.Sort != "id DESC" {
		t.Errorf("second sort = %q, want 'id DESC'", m.Browse.Sort)
	}
	res, _ = m.cycleSort()
	m = res.(Model)
	if m.Browse.Sort != "" {
		t.Errorf("third sort = %q, want '' (cleared)", m.Browse.Sort)
	}
}

func TestFilter_PrependsWhere(t *testing.T) {
	m := gridModel()
	m.FilterInput = textinput.New()
	m.FilterInput.SetValue("id > 5")
	res, _ := m.handleFilterScreen(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = res.(Model)
	if m.Browse.Where != "WHERE id > 5" {
		t.Errorf("Where = %q, want 'WHERE id > 5'", m.Browse.Where)
	}
	if m.Browse.Offset != 0 {
		t.Error("applying a filter should reset the page offset")
	}
}

func TestMetaLoaded_FillsInspectorTab(t *testing.T) {
	m := gridModel()
	res, _ := m.openInspector("db", "users", "users", false)
	m = res.(Model)
	res, _ = m.handleMetaLoadedMsg(types.MetaLoadedMsg{
		Kind: int(0), // columns
		Rows: [][]string{{"name", "type"}, {"id", "INTEGER"}},
	})
	m = res.(Model)
	tab := m.InspTabs[inspTableColumns]
	if !tab.loaded || tab.err != "" {
		t.Fatalf("columns tab not loaded cleanly: %+v", tab)
	}
	if len(tab.lines) < 3 { // header + rule + 1 row
		t.Errorf("tab lines = %v", tab.lines)
	}
	if tab.raw != "name\ttype\nid\tINTEGER" {
		t.Errorf("tab raw = %q", tab.raw)
	}
}

func TestMetaLoaded_IgnoredWhenInspectorClosed(t *testing.T) {
	m := gridModel()
	res, _ := m.handleMetaLoadedMsg(types.MetaLoadedMsg{
		Kind: int(0),
		Rows: [][]string{{"name"}, {"id"}},
	})
	m = res.(Model)
	if m.InspOpen || len(m.Browse.Columns) != 2 {
		t.Errorf("closed-inspector meta msg must not disturb state; cols=%v", m.Browse.Columns)
	}
}
