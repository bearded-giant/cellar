package ui

import (
	"testing"

	"github.com/jorgerojas26/lazysql/internal/tui/types"
)

func TestGridHitTest(t *testing.T) {
	widths := []int{5, 8} // col0 x[0,5), sep, col1 x[8,16)
	// y<3 are chrome (title/header/rule)
	if _, _, ok := gridHitTest(2, 0, widths, 0, 2, 0, 10); ok {
		t.Error("clicks above row 3 should miss (chrome)")
	}
	// first data row (y=3), x in col0
	row, col, ok := gridHitTest(3, 2, widths, 0, 2, 0, 10)
	if !ok || row != 0 || col != 0 {
		t.Errorf("hit = (%d,%d,%v), want row0 col0", row, col, ok)
	}
	// x in col1 (>= 8)
	_, col, ok = gridHitTest(3, 9, widths, 0, 2, 0, 10)
	if !ok || col != 1 {
		t.Errorf("col = %d (ok=%v), want col1", col, ok)
	}
	// y beyond data count -> miss
	if _, _, ok := gridHitTest(20, 0, widths, 0, 2, 0, 3); ok {
		t.Error("click past data rows should miss")
	}
	// dataStart offset applied
	row, _, ok = gridHitTest(3, 0, widths, 0, 2, 50, 10)
	if !ok || row != 50 {
		t.Errorf("row with offset = %d, want 50", row)
	}
}

func TestMouseScroll(t *testing.T) {
	m := gridModel()
	m.Focus = types.FocusGrid
	m.Browse.RowCursor = 0
	res, _ := m.mouseScroll(+1)
	if res.(Model).Browse.RowCursor != 1 {
		t.Error("wheel down should advance the grid row cursor")
	}
}
