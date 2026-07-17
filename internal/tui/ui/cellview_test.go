package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

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

func peekModel() Model {
	m := gridModel()
	m.Width, m.Height = 80, 24
	return m
}

func TestPeek_OpenFromBrowseGrid(t *testing.T) {
	m := peekModel()
	m.Browse.RowCursor, m.Browse.ColCursor = 0, 1

	res, _ := m.handleBrowseScreen(keyMsg('v'))
	m = res.(Model)
	if !m.PeekOpen {
		t.Fatal("v should open the peek popup")
	}
	if m.Screen != types.ScreenBrowse {
		t.Errorf("peek must not swap screens, got %v", m.Screen)
	}
	if m.PeekCol != "name" {
		t.Errorf("peek col = %q, want name", m.PeekCol)
	}
	if m.PeekRaw != "alpha" {
		t.Errorf("peek raw = %q, want alpha", m.PeekRaw)
	}
}

func TestPeek_ShiftVOpensFullCellView(t *testing.T) {
	m := peekModel()
	res, _ := m.handleBrowseScreen(keyMsg('V'))
	m = res.(Model)
	if m.PeekOpen {
		t.Error("V must not open the peek")
	}
	if m.Screen != types.ScreenCellView {
		t.Errorf("V should open the full cell view, got %v", m.Screen)
	}
}

func TestPeek_ScrollAndClose(t *testing.T) {
	m := peekModel()
	res, _ := m.openPeek()
	m = res.(Model)
	m.PeekLines = []string{strings.Repeat("a", 500), "tail"}

	res, _ = m.handleBrowseScreen(keyMsg('j'))
	m = res.(Model)
	if m.PeekScroll != 1 {
		t.Errorf("j should scroll to 1, got %d", m.PeekScroll)
	}
	res, _ = m.handleBrowseScreen(keyMsg('G'))
	m = res.(Model)
	if want := len(m.peekDisplayLines()) - 1; m.PeekScroll != want {
		t.Errorf("G should scroll to %d, got %d", want, m.PeekScroll)
	}
	res, _ = m.handleBrowseScreen(keyMsg('g'))
	m = res.(Model)
	if m.PeekScroll != 0 {
		t.Errorf("g should scroll to top, got %d", m.PeekScroll)
	}
	res, _ = m.handleBrowseScreen(keyMsg('v'))
	m = res.(Model)
	if m.PeekOpen {
		t.Error("v should close an open peek")
	}
	if m.Screen != types.ScreenBrowse {
		t.Errorf("closing the peek must stay on browse, got %v", m.Screen)
	}
}

func TestPeek_CopyClosesWithStatus(t *testing.T) {
	m := peekModel()
	res, _ := m.openPeek()
	m = res.(Model)

	res, _ = m.handleBrowseScreen(keyMsg('y'))
	m = res.(Model)
	if m.PeekOpen {
		t.Error("y should close the peek")
	}
	if !strings.HasPrefix(m.StatusMsg, "Copied cell") && !strings.HasPrefix(m.StatusMsg, "Copy failed") {
		t.Errorf("y should surface a copy status, got %q", m.StatusMsg)
	}
}

func TestPeek_BlocksEditorPaneKeysUntilClosed(t *testing.T) {
	m := peekModel()
	m.Screen = types.ScreenEditor
	m.Focus = types.FocusGrid
	m.EditorArea = m.newEditorArea("select 1")

	res, _ := m.handleEditorScreen(keyMsg('v'))
	m = res.(Model)
	if !m.PeekOpen {
		t.Fatal("v should open the peek over the editor results")
	}
	res, _ = m.handleEditorScreen(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	m = res.(Model)
	if m.Screen != types.ScreenEditor || !m.PeekOpen {
		t.Error("workspace chords must not fire while the peek is open")
	}
	res, _ = m.handleEditorScreen(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = res.(Model)
	if m.PeekOpen {
		t.Error("esc should close the peek")
	}
	if m.Screen != types.ScreenEditor || m.Focus != types.FocusGrid {
		t.Errorf("closing the peek must stay in the editor results, got %v/%v", m.Screen, m.Focus)
	}
}

func TestPeek_ResizeRewrapsAndClampsScroll(t *testing.T) {
	m := peekModel()
	res, _ := m.openPeek()
	m = res.(Model)
	m.PeekLines = []string{strings.Repeat("a", 300)}

	res, _ = m.handleBrowseScreen(keyMsg('G'))
	m = res.(Model)
	before := m.PeekScroll
	if before == 0 {
		t.Fatal("long value should wrap into multiple display lines")
	}

	m.Width = 200 // re-wrap: wider box, fewer wrapped lines
	res, _ = m.handleBrowseScreen(keyMsg('j'))
	m = res.(Model)
	if last := len(m.peekDisplayLines()) - 1; m.PeekScroll > last {
		t.Errorf("scroll %d should clamp to new last line %d after resize", m.PeekScroll, last)
	}
	if m.PeekScroll >= before {
		t.Errorf("wider wrap should shrink the scroll range: %d -> %d", before, m.PeekScroll)
	}
}

func TestPeek_OverlayRendersOverBrowse(t *testing.T) {
	m := peekModel()
	m.Browse.Rows = [][]string{{"1", `{"k":"peekvalue"}`}}
	m.Browse.RowCursor, m.Browse.ColCursor = 0, 1
	res, _ := m.handleBrowseScreen(keyMsg('v'))
	m = res.(Model)

	out := m.viewContent()
	lines := strings.Split(out, "\n")
	if len(lines) != m.Height {
		t.Fatalf("composed view height = %d, want %d", len(lines), m.Height)
	}
	plain := stripANSI(out)
	if !strings.Contains(plain, "Cell: name") {
		t.Error("peek title should render in the composition")
	}
	if !strings.Contains(plain, "peekvalue") {
		t.Error("peek body should render the cell value")
	}
	if !strings.Contains(plain, "esc close") {
		t.Error("peek footer hint should render")
	}
	if !strings.Contains(plain, "widgets") {
		t.Error("base browse view should stay visible around the peek")
	}

	m.closePeek()
	if strings.Contains(stripANSI(m.viewContent()), "Cell: name") {
		t.Error("closed peek must not render")
	}
}
