package ui

import (
	"testing"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/bearded-giant/cellar/internal/saved"
	"github.com/bearded-giant/cellar/internal/tui/types"
	"github.com/bearded-giant/cellar/models"
)

func TestSavedQuery_RoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := saved.SaveQuery("conn1", "recent orders", "SELECT * FROM orders"); err != nil {
		t.Fatalf("save: %v", err)
	}
	// a duplicate name is rejected (internal/saved errors, does not overwrite)
	if err := saved.SaveQuery("conn1", "recent orders", "SELECT id FROM orders"); err == nil {
		t.Error("duplicate name should error")
	}
	items, err := saved.ReadSavedQueries("conn1")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(items) != 1 || items[0].Name != "recent orders" || items[0].Query != "SELECT * FROM orders" {
		t.Errorf("saved = %+v", items)
	}
}

func TestSavedQueriesScreen_LoadsIntoEditor(t *testing.T) {
	m := browseModel()
	m.Width, m.Height = 100, 30
	m.SavedItems = []models.SavedQuery{{Name: "q", Query: "SELECT 42"}}
	m.Screen = types.ScreenSavedQueries

	res, _ := m.handleSavedQueriesScreen(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = res.(Model)
	if m.Screen != types.ScreenEditor {
		t.Fatal("enter should open the editor")
	}
	if m.EditorArea.Value() != "SELECT 42" {
		t.Errorf("editor seeded with %q, want 'SELECT 42'", m.EditorArea.Value())
	}
	if m.SavedName != "q" {
		t.Errorf("SavedName = %q, want 'q'", m.SavedName)
	}
	if m.editorDirty() {
		t.Error("a freshly loaded query should not be dirty")
	}
}

func TestSaveQuery_ReSaveOverwritesInPlace(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := browseModel()
	m.Width, m.Height = 100, 30
	m.SavedItems = []models.SavedQuery{{Name: "q", Query: "SELECT 42"}}
	m.Screen = types.ScreenSavedQueries
	res, _ := m.handleSavedQueriesScreen(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = res.(Model)

	m.EditorArea.SetValue("SELECT 43")
	if !m.editorDirty() {
		t.Fatal("an edited saved query should be dirty")
	}

	// bound to a name -> ctrl+s re-saves in place, no name prompt
	res, cmd := m.openSaveQuery()
	m = res.(Model)
	if m.Screen != types.ScreenEditor {
		t.Errorf("re-save should stay in the editor, not prompt; screen=%v", m.Screen)
	}
	if cmd == nil {
		t.Fatal("re-save should fire a save command")
	}
	msg, ok := cmd().(types.SavedQuerySavedMsg)
	if !ok || msg.Err != nil {
		t.Fatalf("re-save cmd = %+v, ok=%v", msg, ok)
	}
	res, _ = m.handleSavedQuerySavedMsg(msg)
	m = res.(Model)
	if m.editorDirty() {
		t.Error("after re-save the buffer should be clean")
	}
	if m.SavedBaseline != "SELECT 43" {
		t.Errorf("baseline = %q, want 'SELECT 43'", m.SavedBaseline)
	}
}

// runCmd runs a tea.Cmd synchronously and feeds its msg back through Update.
func runCmd(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a command")
	}
	res, _ := m.Update(cmd())
	return res.(Model)
}

func TestQueryPicker_TabTogglesBetweenSavedAndHistory(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := browseModel()
	m.Width, m.Height = 100, 30

	// ctrl+o from browse opens the saved side, even with nothing saved yet
	res, cmd := m.handleBrowseScreen(tea.KeyPressMsg{Code: 'o', Mod: tea.ModCtrl})
	m = runCmd(t, res.(Model), cmd)
	if m.Screen != types.ScreenSavedQueries {
		t.Fatalf("ctrl+o should open the saved picker, got %v", m.Screen)
	}

	// tab flips to the history side (empty list still opens)
	res, cmd = m.handleSavedQueriesScreen(tea.KeyPressMsg{Code: tea.KeyTab})
	m = runCmd(t, res.(Model), cmd)
	if m.Screen != types.ScreenHistory {
		t.Fatalf("tab should flip to history, got %v", m.Screen)
	}

	// and back again
	res, cmd = m.handleHistoryScreen(tea.KeyPressMsg{Code: tea.KeyTab})
	m = runCmd(t, res.(Model), cmd)
	if m.Screen != types.ScreenSavedQueries {
		t.Fatalf("tab should flip back to saved, got %v", m.Screen)
	}

	// esc closes back to where the picker opened
	res, _ = m.handleSavedQueriesScreen(tea.KeyPressMsg{Code: tea.KeyEsc})
	if got := res.(Model).Screen; got != types.ScreenBrowse {
		t.Errorf("esc should close to browse, got %v", got)
	}
}

func TestQueryPicker_CtrlOFromEditorPreservesBuffer(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := browseModel()
	m.Width, m.Height = 100, 30
	res, _ := m.openEditor()
	m = res.(Model)
	m.EditorArea.SetValue("select wip")

	res, cmd := m.handleEditorScreen(tea.KeyPressMsg{Code: 'o', Mod: tea.ModCtrl})
	m = res.(Model)
	if m.EditorContent != "select wip" {
		t.Errorf("ctrl+o must snapshot the live buffer, got %q", m.EditorContent)
	}
	m = runCmd(t, m, cmd)
	if m.Screen != types.ScreenSavedQueries {
		t.Fatalf("ctrl+o should open the picker from the editor, got %v", m.Screen)
	}
	if m.GridReturnScreen != types.ScreenEditor {
		t.Error("picker opened from the editor should close back to it")
	}
}

func TestQueryPicker_EmptyHistoryStaysOpen(t *testing.T) {
	m := browseModel()
	res, _ := m.handleHistoryLoadedMsg(types.HistoryLoadedMsg{})
	m = res.(Model)
	if m.Screen != types.ScreenHistory {
		t.Errorf("empty history should still open the picker, got %v", m.Screen)
	}
	if m.HistoryCursor != 0 {
		t.Errorf("empty history cursor = %d, want 0", m.HistoryCursor)
	}
	// enter/d on the empty list must be no-ops, not panics
	res, _ = m.handleHistoryScreen(tea.KeyPressMsg{Code: tea.KeyEnter})
	if got := res.(Model).Screen; got != types.ScreenHistory {
		t.Errorf("enter on empty history should be a no-op, got %v", got)
	}
	res, _ = m.handleHistoryScreen(keyMsg('d'))
	_ = res
}

func TestBrowse_HistorySavedShortcutsRemoved(t *testing.T) {
	m := browseModel()
	for _, r := range []rune{'H', 'Y'} {
		res, cmd := m.handleBrowseScreen(keyMsg(r))
		if got := res.(Model).Screen; got != types.ScreenBrowse || cmd != nil {
			t.Errorf("%q should no longer open a modal (screen=%v cmd=%v)", r, got, cmd)
		}
	}
}

func TestSaveQueryScreen_FiresSaveCmd(t *testing.T) {
	m := browseModel()
	m.Width, m.Height = 100, 30
	res, _ := m.openEditor()
	m = res.(Model)
	m.EditorArea.SetValue("SELECT 1")
	res, _ = m.openSaveQuery()
	m = res.(Model)
	if m.Screen != types.ScreenSaveQuery {
		t.Fatal("openSaveQuery should switch to ScreenSaveQuery")
	}
	m.SaveNameInput = textinput.New()
	m.SaveNameInput.SetValue("my query")
	res, cmd := m.handleSaveQueryScreen(tea.KeyPressMsg{Code: tea.KeyEnter})
	if res.(Model).Screen != types.ScreenEditor {
		t.Error("after save, should return to the editor")
	}
	if cmd == nil {
		t.Error("enter with a name should fire the save command")
	}
}
