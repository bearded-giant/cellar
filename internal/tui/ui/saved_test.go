package ui

import (
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

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

	res, _ := m.handleSavedQueriesScreen(tea.KeyMsg{Type: tea.KeyEnter})
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
	res, _ := m.handleSavedQueriesScreen(tea.KeyMsg{Type: tea.KeyEnter})
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
	res, cmd := m.handleSaveQueryScreen(tea.KeyMsg{Type: tea.KeyEnter})
	if res.(Model).Screen != types.ScreenEditor {
		t.Error("after save, should return to the editor")
	}
	if cmd == nil {
		t.Error("enter with a name should fire the save command")
	}
}
