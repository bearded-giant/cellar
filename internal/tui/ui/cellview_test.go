package ui

import (
	"strings"
	"testing"

	"github.com/bearded-giant/cellar/internal/tui/types"
)

func TestPrettyCellLines_JSON(t *testing.T) {
	lines := prettyCellLines(`{"a":1,"b":[2,3]}`)
	if len(lines) < 3 {
		t.Fatalf("JSON should pretty-print to multiple lines, got %d: %v", len(lines), lines)
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "\n  ") {
		t.Errorf("expected 2-space indentation:\n%s", joined)
	}
}

func TestPrettyCellLines_PlainAndNull(t *testing.T) {
	if got := prettyCellLines("hello"); len(got) != 1 || got[0] != "hello" {
		t.Errorf("plain = %v", got)
	}
	if got := prettyCellLines("NULL&"); got[0] != "NULL" {
		t.Errorf("NULL& should display as NULL, got %v", got)
	}
}

func TestOpenCellView(t *testing.T) {
	m := gridModel()
	m.Browse.Rows = [][]string{{"1", `{"k":"v"}`}}
	m.Browse.RowCursor, m.Browse.ColCursor = 0, 1
	res, _ := m.openCellView()
	m = res.(Model)
	if m.Screen != types.ScreenCellView {
		t.Fatal("should open the cell view")
	}
	if m.CellViewCol != "name" {
		t.Errorf("col title = %q, want name", m.CellViewCol)
	}
	if len(m.CellViewLines) < 3 {
		t.Errorf("JSON cell should pretty-print, got %v", m.CellViewLines)
	}
}
