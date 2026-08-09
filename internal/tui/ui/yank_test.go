package ui

import (
	"strings"
	"testing"
)

func TestYankBuilders(t *testing.T) {
	m := gridModel() // cols [id name], rows [[1 alpha][2 beta]]
	m.Browse.RowCursor, m.Browse.ColCursor = 0, 0

	if got := m.yankCell(); got != "1" {
		t.Errorf("yankCell = %q, want 1", got)
	}
	if got := m.yankRow(); got != "1\talpha" {
		t.Errorf("yankRow = %q, want '1\\talpha'", got)
	}
}

func TestYankStatement_MultiLine(t *testing.T) {
	multi := "select id,\n       name\nfrom widgets\nwhere id > 3"
	buf := "select 1;\n\n" + multi + ";\n\nselect 2;"

	m := gridModel()
	m.EditorArea = newEditor(buf, 80, 20)

	cases := []struct {
		name string
		row  int
		want string
	}{
		{"first line of statement", 2, multi},
		{"middle line of statement", 3, multi},
		{"last line of statement", 5, multi},
		{"single-line statement", 0, "select 1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m.EditorArea.setCursor(tc.row, 0)
			got := m.yankStatement()
			if got != tc.want {
				t.Errorf("yankStatement = %q, want %q", got, tc.want)
			}
			if tc.want == multi && strings.Count(got, "\n") != 3 {
				t.Errorf("expected all 4 lines, got %d", strings.Count(got, "\n")+1)
			}
		})
	}

	empty := gridModel()
	empty.EditorArea = newEditor("   \n\n", 80, 20)
	if got := empty.yankStatement(); got != "" {
		t.Errorf("blank buffer should yank nothing, got %q", got)
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
