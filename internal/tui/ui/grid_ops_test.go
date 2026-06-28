package ui

import (
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jorgerojas26/lazysql/internal/tui/types"
	"github.com/jorgerojas26/lazysql/models"
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
	res, _ := m.handleFilterScreen(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(Model)
	if m.Browse.Where != "WHERE id > 5" {
		t.Errorf("Where = %q, want 'WHERE id > 5'", m.Browse.Where)
	}
	if m.Browse.Offset != 0 {
		t.Error("applying a filter should reset the page offset")
	}
}

func TestSetValue_StagesTypedEdit(t *testing.T) {
	m := gridModel()
	m.Browse.RowCursor, m.Browse.ColCursor = 0, 1
	res, _ := m.openSetValue()
	m = res.(Model)
	if m.Screen != types.ScreenSetValue {
		t.Fatal("openSetValue should switch to ScreenSetValue")
	}
	res, _ = m.handleSetValueScreen(keyMsg('n'))
	m = res.(Model)
	if m.Browse.Edited[[2]int{0, 1}] != dmlNull {
		t.Errorf("n should stage a NULL sentinel, got %q", m.Browse.Edited[[2]int{0, 1}])
	}
}

func TestSetValue_OnInsertRow(t *testing.T) {
	m := gridModel()
	res, _ := m.appendInsertRow()
	m = res.(Model)
	m.Browse.ColCursor = 1

	res, _ = m.openSetValue()
	m = res.(Model)
	if m.Screen != types.ScreenSetValue {
		t.Fatal("SetValue should be allowed on an insert row now")
	}
	res, _ = m.handleSetValueScreen(keyMsg('n'))
	m = res.(Model)
	idx := m.Browse.RowCursor - len(m.Browse.Rows)
	if m.Browse.Inserts[idx][1].typ != models.Null {
		t.Errorf("insert cell should be NULL-typed, got %v", m.Browse.Inserts[idx][1].typ)
	}
}

func TestBuildDMLChanges_TypedNull(t *testing.T) {
	cols := []string{"id", "name"}
	rows := [][]string{{"1", "alpha"}}
	changes := buildDMLChanges("app", "widgets", cols, rows, []string{"id"},
		map[[2]int]string{{0, 1}: dmlNull}, nil, nil)
	if len(changes) != 1 || len(changes[0].Values) != 1 {
		t.Fatalf("expected one update with one value, got %+v", changes)
	}
	v := changes[0].Values[0]
	if v.Type != models.Null {
		t.Errorf("value type = %v, want Null", v.Type)
	}
	if v.Value != nil {
		t.Errorf("NULL value should be nil, got %v", v.Value)
	}
}

func TestMetaLoaded_FillsGridReadOnly(t *testing.T) {
	m := gridModel()
	m.Browse.MetaKind = metaColumns
	res, _ := m.handleMetaLoadedMsg(types.MetaLoadedMsg{
		Kind: int(0),
		Rows: [][]string{{"name", "type"}, {"id", "INTEGER"}},
	})
	m = res.(Model)
	if len(m.Browse.Columns) != 2 || m.Browse.Columns[0] != "name" {
		t.Errorf("meta columns = %v", m.Browse.Columns)
	}
	if m.editable() {
		t.Error("a metadata view must not be editable")
	}
}
