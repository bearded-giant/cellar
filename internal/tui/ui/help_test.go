package ui

import (
	"testing"

	"github.com/bearded-giant/cellar/internal/tui/types"
)

func TestHelp_OpenCloseRestoresScreen(t *testing.T) {
	m := browseModel() // Screen == ScreenBrowse
	res, _ := m.openHelp()
	m = res.(Model)
	if m.Screen != types.ScreenHelp {
		t.Fatalf("openHelp -> Screen %v, want ScreenHelp", m.Screen)
	}
	res, _ = m.handleHelpScreen(keyMsg('x'))
	if got := res.(Model).Screen; got != types.ScreenBrowse {
		t.Errorf("closing help -> Screen %v, want ScreenBrowse", got)
	}
}

func TestHelp_ScrollClampsAndStaysOpen(t *testing.T) {
	m := browseModel()
	m.Height = 20 // force a viewport shorter than the content
	res, _ := m.openHelp()
	m = res.(Model)

	res, _ = m.handleHelpScreen(keyMsg('k')) // up at top: no underflow
	m = res.(Model)
	if m.Screen != types.ScreenHelp || m.HelpScroll != 0 {
		t.Fatalf("up at top -> screen=%v scroll=%d, want ScreenHelp 0", m.Screen, m.HelpScroll)
	}

	res, _ = m.handleHelpScreen(keyMsg('G')) // jump to bottom
	m = res.(Model)
	maxScroll := max(0, len(m.helpLines())-m.helpViewportHeight())
	if m.HelpScroll != maxScroll || maxScroll == 0 {
		t.Fatalf("G -> scroll=%d, want maxScroll=%d (>0)", m.HelpScroll, maxScroll)
	}

	res, _ = m.handleHelpScreen(keyMsg('j')) // down at bottom: no overflow
	if got := res.(Model).HelpScroll; got != maxScroll {
		t.Errorf("down at bottom -> scroll=%d, want %d", got, maxScroll)
	}
}
