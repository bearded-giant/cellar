package ui

import (
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

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
	res, _ := m.handleFilterScreen(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(Model)
	if m.Browse.Where != "WHERE id > 5" {
		t.Errorf("Where = %q, want 'WHERE id > 5'", m.Browse.Where)
	}
	if m.Browse.Offset != 0 {
		t.Error("applying a filter should reset the page offset")
	}
}

func TestMetaLoaded_FillsGrid(t *testing.T) {
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
}
