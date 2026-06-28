package ui

import "testing"

func TestYankBuilders(t *testing.T) {
	m := gridModel() // cols [id name], rows [[1 alpha][2 beta]]
	m.Browse.RowCursor, m.Browse.ColCursor = 0, 0

	if got := m.yankCell(); got != "1" {
		t.Errorf("yankCell = %q, want 1", got)
	}
	if got := m.yankRow(); got != "1\talpha" {
		t.Errorf("yankRow = %q, want '1\\talpha'", got)
	}
	want := "id\tname\n1\talpha\n2\tbeta\n"
	if got := m.yankAll(); got != want {
		t.Errorf("yankAll = %q, want %q", got, want)
	}
}

func TestYankCell_HandlesSentinel(t *testing.T) {
	m := gridModel()
	m.Browse.Rows = [][]string{{"1", "NULL&"}}
	m.Browse.RowCursor, m.Browse.ColCursor = 0, 1
	if got := m.yankCell(); got != "NULL" {
		t.Errorf("yankCell sentinel = %q, want NULL", got)
	}
}
