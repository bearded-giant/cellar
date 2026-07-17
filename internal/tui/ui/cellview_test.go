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

func TestWrapLine(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		width int
		want  []string
	}{
		{"exact width", "abcde", 5, []string{"abcde"}},
		{"width plus one", "abcdef", 5, []string{"abcde", "f"}},
		{"multi-rune unicode", "héllö wörld→", 5, []string{"héllö", " wörl", "d→"}},
		{"empty line", "", 5, []string{""}},
		{"single long token", "aaaaaaaaaaaa", 4, []string{"aaaa", "aaaa", "aaaa"}},
		{"shorter than width", "ab", 5, []string{"ab"}},
		{"width zero disables wrap", "abcdef", 0, []string{"abcdef"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wrapLine(tt.in, tt.width)
			if len(got) != len(tt.want) {
				t.Fatalf("wrapLine(%q, %d) = %v, want %v", tt.in, tt.width, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("line %d = %q, want %q", i, got[i], tt.want[i])
				}
			}
			if strings.Join(got, "") != tt.in {
				t.Errorf("wrapped chunks should concatenate back to input, got %q", strings.Join(got, ""))
			}
		})
	}
}

func TestWrapLines_JSONMultiLine(t *testing.T) {
	lines := prettyCellLines(`{"key":"` + strings.Repeat("x", 30) + `","b":[1,2]}`)
	width := 10
	wrapped := wrapLines(lines, width)
	if len(wrapped) <= len(lines) {
		t.Fatalf("long JSON value should wrap into more display lines: %d -> %d", len(lines), len(wrapped))
	}
	for i, l := range wrapped {
		if n := len([]rune(l)); n > width {
			t.Errorf("wrapped line %d has %d runes, exceeds width %d: %q", i, n, width, l)
		}
	}
	if strings.Join(wrapped, "") != strings.Join(lines, "") {
		t.Error("wrapping should preserve content exactly")
	}
}

func TestCellViewScroll_WrappedLines(t *testing.T) {
	m := gridModel()
	m.Width, m.Height = 20, 10
	m.Screen = types.ScreenCellView
	m.CellViewLines = []string{strings.Repeat("a", 50), "short"}

	wrapped := m.cellViewDisplayLines()
	if want := 4; len(wrapped) != want { // 50 runes at width 20 = 3 lines + "short"
		t.Fatalf("display lines = %d, want %d: %v", len(wrapped), want, wrapped)
	}

	res, _ := m.handleCellViewScreen(keyMsg('G'))
	m = res.(Model)
	if m.CellViewScroll != len(wrapped)-1 {
		t.Errorf("G should scroll to last wrapped line %d, got %d", len(wrapped)-1, m.CellViewScroll)
	}

	m.Width = 60 // re-wrap: 50-rune line now fits, so fewer display lines
	res, _ = m.handleCellViewScreen(keyMsg('j'))
	m = res.(Model)
	if last := len(m.cellViewDisplayLines()) - 1; m.CellViewScroll > last {
		t.Errorf("scroll %d should clamp to new last wrapped line %d after resize", m.CellViewScroll, last)
	}

	res, _ = m.handleCellViewScreen(keyMsg('g'))
	m = res.(Model)
	if m.CellViewScroll != 0 {
		t.Errorf("g should scroll to top, got %d", m.CellViewScroll)
	}
}

func TestViewCellView_WrapsLongLine(t *testing.T) {
	m := gridModel()
	m.Width, m.Height = 24, 12
	m.Screen = types.ScreenCellView
	m.CellViewCol = "name"
	m.CellViewLines = []string{strings.Repeat("z", 40), ""}

	out := m.viewCellView()
	if !strings.Contains(out, strings.Repeat("z", 24)) {
		t.Error("first wrapped segment should fill the pane width")
	}
	if !strings.Contains(out, strings.Repeat("z", 16)+" ") {
		t.Error("continuation segment should carry the overflow runes")
	}
	if strings.Contains(out, "…") {
		t.Error("soft-wrap should not truncate with an ellipsis")
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
