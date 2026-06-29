package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jorgerojas26/lazysql/internal/tui/types"
)

func TestHelp_OpenCloseRestoresScreen(t *testing.T) {
	m := browseModel() // Screen == ScreenBrowse
	res, _ := m.openHelp()
	m = res.(Model)
	if m.Screen != types.ScreenHelp {
		t.Fatalf("openHelp -> Screen %v, want ScreenHelp", m.Screen)
	}
	res, _ = m.handleHelpScreen(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if got := res.(Model).Screen; got != types.ScreenBrowse {
		t.Errorf("closing help -> Screen %v, want ScreenBrowse", got)
	}
}
