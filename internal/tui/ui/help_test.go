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
