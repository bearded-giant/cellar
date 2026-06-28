package ui

import (
	"reflect"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jorgerojas26/lazysql/internal/tui/types"
)

func TestQueryExecuted_SelectFillsGrid(t *testing.T) {
	m := browseModel()
	m.Browse.Table = "widgets" // pretend a table was open; query result must clear it
	res, _ := m.handleQueryExecutedMsg(types.QueryExecutedMsg{
		IsSelect: true,
		Rows:     [][]string{{"id", "name"}, {"1", "alpha"}},
		Total:    1,
	})
	m = res.(Model)
	if m.Screen != types.ScreenBrowse || m.Focus != types.FocusGrid {
		t.Error("query result should land on the browse grid")
	}
	if !reflect.DeepEqual(m.Browse.Columns, []string{"id", "name"}) {
		t.Errorf("Columns = %v", m.Browse.Columns)
	}
	if len(m.Browse.Rows) != 1 {
		t.Errorf("data rows = %d, want 1", len(m.Browse.Rows))
	}
	if m.Browse.Table != "" {
		t.Error("query result must blank Table so grid paging stays disabled")
	}
}

func TestQueryExecuted_DMLSetsStatusNoGrid(t *testing.T) {
	m := browseModel()
	res, _ := m.handleQueryExecutedMsg(types.QueryExecutedMsg{IsSelect: false, Info: "2 rows affected"})
	m = res.(Model)
	if m.Browse.Columns != nil {
		t.Error("DML result should not populate grid columns")
	}
	if m.Browse.GridErr != "" {
		t.Errorf("DML success is not an error, got %q", m.Browse.GridErr)
	}
	if m.StatusMsg != "DML: 2 rows affected" {
		t.Errorf("StatusMsg = %q", m.StatusMsg)
	}
}

func TestQueryExecuted_ErrShowsGridErr(t *testing.T) {
	m := browseModel()
	res, _ := m.handleQueryExecutedMsg(types.QueryExecutedMsg{Err: errFake})
	m = res.(Model)
	if m.Browse.GridErr == "" {
		t.Error("query error should surface in GridErr")
	}
	if m.Browse.GridLoading {
		t.Error("GridLoading must clear after a query error")
	}
}

func TestEditor_OpenAndClose(t *testing.T) {
	m := browseModel()
	m.Width, m.Height = 80, 24

	res, cmd := m.openEditor()
	m = res.(Model)
	if m.Screen != types.ScreenEditor || m.Editor == nil {
		t.Fatal("openEditor should switch to ScreenEditor with a live editor")
	}
	if cmd == nil {
		t.Error("openEditor should return the editor Init cmd (cursor blink)")
	}

	res2, _ := m.handleEditorScreen(tea.KeyMsg{Type: tea.KeyCtrlQ})
	if res2.(Model).Screen != types.ScreenBrowse {
		t.Error("ctrl+q should return from the editor to browse")
	}
}

type fakeErr struct{}

func (fakeErr) Error() string { return "boom" }

var errFake error = fakeErr{}
