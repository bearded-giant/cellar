package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/bearded-giant/cellar/internal/tui/types"
)

func TestSplit_FixedPercentIgnoresLineCount(t *testing.T) {
	m := editorModel(t)
	_, eh, rh := m.queryLayout()
	rows := m.workspaceRows()
	if want := rows * editorSplitPct / 100; eh != want {
		t.Fatalf("editor height %d, want %d (%d%% of %d)", eh, want, editorSplitPct, rows)
	}
	if eh+rh != rows {
		t.Fatalf("split %d+%d != workspace %d", eh, rh, rows)
	}

	// growing the buffer must not move the split
	m.EditorArea.SetValue("select 1\nfrom x\nwhere a\nand b\nand c\nand d\nand e\nand f\nand g\nand h")
	m.syncEditorHeight()
	if _, eh2, _ := m.queryLayout(); eh2 != eh {
		t.Errorf("editor height moved %d -> %d on content growth", eh, eh2)
	}
}

func TestSplit_ZoomFollowsFocus(t *testing.T) {
	m := editorModel(t)
	ctrlX := tea.KeyPressMsg{Code: 'x', Mod: tea.ModCtrl}

	res, _ := m.handleEditorScreen(ctrlX)
	m = res.(Model)
	if !m.PaneZoomed {
		t.Fatal("ctrl+x should zoom")
	}
	if _, eh, rh := m.queryLayout(); eh != m.workspaceRows() || rh != 0 {
		t.Fatalf("zoomed editor got %d/%d, want %d/0", eh, rh, m.workspaceRows())
	}

	m.Focus = types.FocusGrid // zoom follows focus: results now full
	if _, eh, rh := m.queryLayout(); eh != 0 || rh != m.workspaceRows() {
		t.Fatalf("zoomed results got %d/%d, want 0/%d", eh, rh, m.workspaceRows())
	}

	res, _ = m.handleEditorScreen(ctrlX)
	m = res.(Model)
	if m.PaneZoomed {
		t.Fatal("second ctrl+x should unzoom")
	}
}

func TestSplit_ViewHeightStableAcrossZoom(t *testing.T) {
	m := editorModel(t)
	base := lineCount(m.viewEditor())
	if base != m.Height {
		t.Fatalf("view is %d lines, want %d", base, m.Height)
	}
	m.PaneZoomed = true
	if got := lineCount(m.viewEditor()); got != base {
		t.Errorf("editor-zoom view %d lines, want %d", got, base)
	}
	m.Focus = types.FocusGrid
	if got := lineCount(m.viewEditor()); got != base {
		t.Errorf("results-zoom view %d lines, want %d", got, base)
	}
}

func lineCount(s string) int {
	n := 1
	for _, r := range s {
		if r == '\n' {
			n++
		}
	}
	return n
}
