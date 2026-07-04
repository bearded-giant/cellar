package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/bearded-giant/cellar/internal/state"
	"github.com/bearded-giant/cellar/internal/tui/types"
	"github.com/bearded-giant/cellar/models"
)

func altKey(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}, Alt: true}
}

func editorModel(t *testing.T) Model {
	t.Helper()
	m := browseModel()
	m.Width, m.Height = 100, 30
	res, _ := m.openEditor()
	return res.(Model)
}

func TestQueryTabs_NewSwitchClosePreservesBuffers(t *testing.T) {
	m := editorModel(t)
	m.EditorArea.SetValue("select 1")

	// alt+t: new blank tab
	res, _ := m.handleEditorScreen(altKey('t'))
	m = res.(Model)
	if len(m.QueryTabs) != 2 || m.QueryTabActive != 1 {
		t.Fatalf("want 2 tabs active=1, got %d active=%d", len(m.QueryTabs), m.QueryTabActive)
	}
	if m.EditorArea.Value() != "" {
		t.Errorf("new tab should be blank, got %q", m.EditorArea.Value())
	}
	m.EditorArea.SetValue("select 2")

	// alt+[: back to tab 0 with its content intact
	res, _ = m.handleEditorScreen(altKey('['))
	m = res.(Model)
	if m.QueryTabActive != 0 || m.EditorArea.Value() != "select 1" {
		t.Fatalf("tab 0 should hold 'select 1', got active=%d %q", m.QueryTabActive, m.EditorArea.Value())
	}

	// alt+]: forward again, buffer 2 intact
	res, _ = m.handleEditorScreen(altKey(']'))
	m = res.(Model)
	if m.EditorArea.Value() != "select 2" {
		t.Fatalf("tab 1 should hold 'select 2', got %q", m.EditorArea.Value())
	}

	// alt+w: close active, land on the survivor
	res, _ = m.handleEditorScreen(altKey('w'))
	m = res.(Model)
	if len(m.QueryTabs) != 1 || m.EditorArea.Value() != "select 1" {
		t.Fatalf("close should leave tab 0, got %d tabs %q", len(m.QueryTabs), m.EditorArea.Value())
	}

	// alt+w on the last tab is a no-op
	res, _ = m.handleEditorScreen(altKey('w'))
	m = res.(Model)
	if len(m.QueryTabs) != 1 {
		t.Error("closing the last tab must be a no-op")
	}
}

func TestQueryTabs_SavedBindingIsPerTab(t *testing.T) {
	m := editorModel(t)
	m.EditorArea.SetValue("select 1")
	m.SavedName, m.SavedBaseline = "one", "select 1"

	res, _ := m.handleEditorScreen(altKey('t'))
	m = res.(Model)
	if m.SavedName != "" {
		t.Fatalf("new tab must be unbound scratch, got %q", m.SavedName)
	}

	res, _ = m.handleEditorScreen(altKey('['))
	m = res.(Model)
	if m.SavedName != "one" || m.SavedBaseline != "select 1" {
		t.Fatalf("binding should travel with the tab, got %q/%q", m.SavedName, m.SavedBaseline)
	}
}

func TestQueryStateLoaded_RestoresTabsAndActive(t *testing.T) {
	m := browseModel()
	res, _ := m.handleQueryStateLoadedMsg(types.QueryStateLoadedMsg{State: state.State{Tabs: []state.Tab{
		{SQL: "select a"},
		{SQL: "select b", Active: true, SavedName: "bee"},
	}}})
	m = res.(Model)
	if len(m.QueryTabs) != 2 || m.QueryTabActive != 1 {
		t.Fatalf("want 2 tabs active=1, got %d active=%d", len(m.QueryTabs), m.QueryTabActive)
	}
	if m.EditorContent != "select b" || m.SavedName != "bee" {
		t.Fatalf("live mirrors should hold the active tab, got %q/%q", m.EditorContent, m.SavedName)
	}

	// opening the editor lands on the restored active buffer
	m.Width, m.Height = 100, 30
	res, _ = m.openEditor()
	m = res.(Model)
	if m.EditorArea.Value() != "select b" {
		t.Errorf("editor should open on restored SQL, got %q", m.EditorArea.Value())
	}
}

func TestQueryStateLoaded_ErrOrEmptyOrEditorOpenIsNoop(t *testing.T) {
	m := browseModel()
	res, _ := m.handleQueryStateLoadedMsg(types.QueryStateLoadedMsg{Err: errFake})
	if len(res.(Model).QueryTabs) != 0 {
		t.Error("load error must start blank")
	}

	// editor already open: restore must not clobber the live buffer
	m = editorModel(t)
	m.EditorArea.SetValue("live typing")
	res, _ = m.handleQueryStateLoadedMsg(types.QueryStateLoadedMsg{State: state.State{Tabs: []state.Tab{{SQL: "stale"}}}})
	m = res.(Model)
	if m.EditorArea.Value() != "live typing" {
		t.Error("restore clobbered an open editor")
	}
}

func TestPersistQueryState_RoundTripAcrossReconnect(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	conn := &models.Connection{Name: "prod"}

	m := editorModel(t)
	m.CurrentConn = conn
	m.EditorArea.SetValue("select 1")
	res, _ := m.handleEditorScreen(altKey('t'))
	m = res.(Model)
	m.EditorArea.SetValue("select 2 -- wip")

	m.PersistQueryState() // quit/disconnect backstop path

	st, err := state.Load("prod")
	if err != nil || len(st.Tabs) != 2 {
		t.Fatalf("want 2 persisted tabs, got %d (err %v)", len(st.Tabs), err)
	}
	if st.Tabs[1].SQL != "select 2 -- wip" || !st.Tabs[1].Active {
		t.Fatalf("active tab not persisted from live editor: %+v", st.Tabs[1])
	}

	// reconnect: restore lands the same workspace
	m2 := browseModel()
	res, _ = m2.handleQueryStateLoadedMsg(types.QueryStateLoadedMsg{State: st})
	m2 = res.(Model)
	if len(m2.QueryTabs) != 2 || m2.QueryTabActive != 1 || m2.EditorContent != "select 2 -- wip" {
		t.Fatalf("restore mismatch: %d tabs active=%d %q", len(m2.QueryTabs), m2.QueryTabActive, m2.EditorContent)
	}
}

func TestPersistQueryState_NeverHadBuffersLeavesFileAlone(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := state.Save("prod", state.State{Tabs: []state.Tab{{SQL: "keep me"}}}); err != nil {
		t.Fatal(err)
	}

	m := browseModel() // connected but editor never touched, restore never landed
	m.CurrentConn = &models.Connection{Name: "prod"}
	m.PersistQueryState()

	st, err := state.Load("prod")
	if err != nil || len(st.Tabs) != 1 || st.Tabs[0].SQL != "keep me" {
		t.Fatalf("prior state must survive an idle session, got %+v (err %v)", st, err)
	}
}
