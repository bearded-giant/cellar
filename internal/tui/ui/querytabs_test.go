package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/bearded-giant/cellar/internal/state"
	"github.com/bearded-giant/cellar/internal/tui/types"
	"github.com/bearded-giant/cellar/models"
)

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

	// ctrl+t: new blank tab
	res, _ := m.handleEditorScreen(tea.KeyPressMsg{Code: 't', Mod: tea.ModCtrl})
	m = res.(Model)
	if len(m.QueryTabs) != 2 || m.QueryTabActive != 1 {
		t.Fatalf("want 2 tabs active=1, got %d active=%d", len(m.QueryTabs), m.QueryTabActive)
	}
	if m.EditorArea.Value() != "" {
		t.Errorf("new tab should be blank, got %q", m.EditorArea.Value())
	}
	m.EditorArea.SetValue("select 2")

	// ctrl+pgup: back to tab 0 with its content intact
	res, _ = m.handleEditorScreen(tea.KeyPressMsg{Code: tea.KeyPgUp, Mod: tea.ModCtrl})
	m = res.(Model)
	if m.QueryTabActive != 0 || m.EditorArea.Value() != "select 1" {
		t.Fatalf("tab 0 should hold 'select 1', got active=%d %q", m.QueryTabActive, m.EditorArea.Value())
	}

	// ctrl+pgdown: forward again, buffer 2 intact
	res, _ = m.handleEditorScreen(tea.KeyPressMsg{Code: tea.KeyPgDown, Mod: tea.ModCtrl})
	m = res.(Model)
	if m.EditorArea.Value() != "select 2" {
		t.Fatalf("tab 1 should hold 'select 2', got %q", m.EditorArea.Value())
	}

	// ctrl+w: close active, land on the survivor
	res, _ = m.handleEditorScreen(tea.KeyPressMsg{Code: 'w', Mod: tea.ModCtrl})
	m = res.(Model)
	if len(m.QueryTabs) != 1 || m.EditorArea.Value() != "select 1" {
		t.Fatalf("close should leave tab 0, got %d tabs %q", len(m.QueryTabs), m.EditorArea.Value())
	}

	// ctrl+w on the last tab is a no-op
	res, _ = m.handleEditorScreen(tea.KeyPressMsg{Code: 'w', Mod: tea.ModCtrl})
	m = res.(Model)
	if len(m.QueryTabs) != 1 {
		t.Error("closing the last tab must be a no-op")
	}
}

func TestQueryTabs_SavedBindingIsPerTab(t *testing.T) {
	m := editorModel(t)
	m.EditorArea.SetValue("select 1")
	m.SavedName, m.SavedBaseline = "one", "select 1"

	res, _ := m.handleEditorScreen(tea.KeyPressMsg{Code: 't', Mod: tea.ModCtrl})
	m = res.(Model)
	if m.SavedName != "" {
		t.Fatalf("new tab must be unbound scratch, got %q", m.SavedName)
	}

	res, _ = m.handleEditorScreen(tea.KeyPressMsg{Code: tea.KeyPgUp, Mod: tea.ModCtrl})
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
	res, _ := m.handleEditorScreen(tea.KeyPressMsg{Code: 't', Mod: tea.ModCtrl})
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

func TestOpenQueryInEditor_DirtyScratchGetsNewTab(t *testing.T) {
	m := editorModel(t)
	m.EditorArea.SetValue("select wip")
	res, _ := m.leaveQueryWorkspace() // back to browse, EditorContent synced
	m = res.(Model)

	res, _ = m.openQueryInEditor("select saved", "mysaved", "select saved")
	m = res.(Model)
	if len(m.QueryTabs) != 2 || m.QueryTabActive != 1 {
		t.Fatalf("dirty buffer should push a new tab, got %d tabs active=%d", len(m.QueryTabs), m.QueryTabActive)
	}
	if m.EditorArea.Value() != "select saved" || m.SavedName != "mysaved" {
		t.Fatalf("new tab should hold the loaded query, got %q/%q", m.EditorArea.Value(), m.SavedName)
	}
	if m.QueryTabs[0].Content != "select wip" {
		t.Fatalf("original scratch must survive in tab 0, got %q", m.QueryTabs[0].Content)
	}
}

func TestOpenQueryInEditor_BlankBufferReplacedInPlace(t *testing.T) {
	m := editorModel(t)
	res, _ := m.openQueryInEditor("select 1", "", "")
	m = res.(Model)
	if len(m.QueryTabs) != 1 || m.EditorArea.Value() != "select 1" {
		t.Fatalf("blank buffer should be replaced in place, got %d tabs %q", len(m.QueryTabs), m.EditorArea.Value())
	}
}

func TestOpenQueryInEditor_CleanBoundBufferReplacedInPlace(t *testing.T) {
	m := editorModel(t)
	m.EditorArea.SetValue("select a")
	m.SavedName, m.SavedBaseline = "aye", "select a" // bound + clean: recoverable
	res, _ := m.leaveQueryWorkspace()
	m = res.(Model)

	res, _ = m.openQueryInEditor("select b", "bee", "select b")
	m = res.(Model)
	if len(m.QueryTabs) != 1 {
		t.Fatalf("clean bound buffer should be replaced, not tabbed, got %d tabs", len(m.QueryTabs))
	}
	if m.EditorArea.Value() != "select b" || m.SavedName != "bee" {
		t.Fatalf("loaded query should be live, got %q/%q", m.EditorArea.Value(), m.SavedName)
	}
}

func TestQueryTabs_DefaultNameUntitled(t *testing.T) {
	m := editorModel(t)
	if m.QueryTabs[0].Name != "untitled" {
		t.Errorf("seed tab Name = %q, want untitled", m.QueryTabs[0].Name)
	}
	res, _ := m.handleEditorScreen(tea.KeyPressMsg{Code: 't', Mod: tea.ModCtrl})
	m = res.(Model)
	if m.QueryTabs[1].Name != "untitled" {
		t.Errorf("new tab Name = %q, want untitled", m.QueryTabs[1].Name)
	}
}

func TestQueryTabLabel_SavedNameWinsOverName(t *testing.T) {
	m := editorModel(t)
	m.QueryTabs = []queryTab{
		{Name: "untitled"},
		{Name: "renamed"},
		{Name: "renamed", SavedName: "bound"},
		{}, // restored from an older state file
	}
	m.QueryTabActive = 0
	for i, want := range []string{"untitled", "renamed", "bound", "untitled"} {
		if got := m.queryTabLabel(i); got != want {
			t.Errorf("label(%d) = %q, want %q", i, got, want)
		}
	}
}

func TestQueryTabLabel_ActiveTabReadsLiveSavedName(t *testing.T) {
	m := editorModel(t)
	m.SavedName = "just saved"
	if got := m.queryTabLabel(0); got != "just saved" {
		t.Errorf("active label = %q, want live SavedName", got)
	}
}

func TestQueryTabBar_RendersWithSingleTab(t *testing.T) {
	m := editorModel(t)
	bar := m.queryTabBar(80)
	if bar == "" {
		t.Fatal("tab bar should render with one tab")
	}
	if !strings.Contains(bar, "1:untitled") {
		t.Errorf("bar = %q, want a ' 1:untitled ' segment", bar)
	}
}

func TestQueryTabBar_TruncatesLongLabels(t *testing.T) {
	m := editorModel(t)
	m.QueryTabs[0].Name = "a really long query name"
	bar := m.queryTabBar(80)
	if !strings.Contains(bar, "…") {
		t.Errorf("bar = %q, want a 14-rune truncated label", bar)
	}
	if strings.Contains(bar, "a really long query name") {
		t.Errorf("bar = %q, label should be truncated", bar)
	}
}

func TestSaveFlow_NamesTabOnSave(t *testing.T) {
	m := editorModel(t)
	m.EditorArea.SetValue("select 1")
	res, _ := m.openSaveQuery()
	m = res.(Model)
	if m.SaveNameInput.Value() != "" {
		t.Errorf("untitled tab must not pre-fill the prompt, got %q", m.SaveNameInput.Value())
	}

	m.SavePendingTab = m.QueryTabActive
	res, _ = m.handleSavedQuerySavedMsg(types.SavedQuerySavedMsg{Name: "daily", Query: "select 1"})
	m = res.(Model)
	if m.QueryTabs[0].Name != "daily" || m.SavedName != "daily" {
		t.Errorf("after save, tab Name/SavedName = %q/%q, want daily/daily", m.QueryTabs[0].Name, m.SavedName)
	}
}

func TestSaveFlow_PrefillsNonUntitledTabName(t *testing.T) {
	m := editorModel(t)
	m.EditorArea.SetValue("select 1")
	m.QueryTabs[0].Name = "report"
	res, _ := m.openSaveQuery()
	m = res.(Model)
	if m.SaveNameInput.Value() != "report" {
		t.Errorf("prompt should pre-fill with tab name, got %q", m.SaveNameInput.Value())
	}
}

func TestQueryState_TabNameRoundTrips(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := editorModel(t)
	m.CurrentConn = &models.Connection{Name: "prod"}
	m.EditorArea.SetValue("select 1")
	m.QueryTabs[0].Name = "daily"
	m.PersistQueryState()

	st, err := state.Load("prod")
	if err != nil || len(st.Tabs) != 1 || st.Tabs[0].Name != "daily" {
		t.Fatalf("persisted Name = %+v (err %v), want daily", st.Tabs, err)
	}

	m2 := browseModel()
	res, _ := m2.handleQueryStateLoadedMsg(types.QueryStateLoadedMsg{State: st})
	m2 = res.(Model)
	if m2.QueryTabs[0].Name != "daily" {
		t.Errorf("restored Name = %q, want daily", m2.QueryTabs[0].Name)
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

func TestQueryTabs_CtrlDigitJumps(t *testing.T) {
	m := editorModel(t)
	m.EditorArea.SetValue("select 1")
	res, _ := m.handleEditorScreen(tea.KeyPressMsg{Code: 't', Mod: tea.ModCtrl})
	m = res.(Model)
	m.EditorArea.SetValue("select 2")
	res, _ = m.handleEditorScreen(tea.KeyPressMsg{Code: 't', Mod: tea.ModCtrl})
	m = res.(Model)
	m.EditorArea.SetValue("select 3")

	res, _ = m.handleEditorScreen(tea.KeyPressMsg{Code: '1', Mod: tea.ModCtrl})
	m = res.(Model)
	if m.QueryTabActive != 0 || m.EditorArea.Value() != "select 1" {
		t.Fatalf("ctrl+1 -> active=%d %q, want tab 0 'select 1'", m.QueryTabActive, m.EditorArea.Value())
	}

	res, _ = m.handleEditorScreen(tea.KeyPressMsg{Code: '3', Mod: tea.ModCtrl})
	m = res.(Model)
	if m.QueryTabActive != 2 || m.EditorArea.Value() != "select 3" {
		t.Fatalf("ctrl+3 -> active=%d %q, want tab 2 'select 3'", m.QueryTabActive, m.EditorArea.Value())
	}

	// absent tab: no-op, stay put
	res, _ = m.handleEditorScreen(tea.KeyPressMsg{Code: '9', Mod: tea.ModCtrl})
	m = res.(Model)
	if m.QueryTabActive != 2 {
		t.Errorf("ctrl+9 with 3 tabs should be a no-op, active=%d", m.QueryTabActive)
	}
}

func TestQueryTabs_CtrlBracketSwitchesTabs(t *testing.T) {
	m := editorModel(t)
	m.EditorArea.SetValue("select 1")
	res, _ := m.handleEditorScreen(tea.KeyPressMsg{Code: 't', Mod: tea.ModCtrl})
	m = res.(Model)

	res, _ = m.handleEditorScreen(tea.KeyPressMsg{Code: '[', Mod: tea.ModCtrl})
	m = res.(Model)
	if m.QueryTabActive != 0 {
		t.Fatalf("ctrl+[ should go to prev tab, active=%d", m.QueryTabActive)
	}
	res, _ = m.handleEditorScreen(tea.KeyPressMsg{Code: ']', Mod: tea.ModCtrl})
	m = res.(Model)
	if m.QueryTabActive != 1 {
		t.Fatalf("ctrl+] should go to next tab, active=%d", m.QueryTabActive)
	}
}

func TestQueryTabs_CursorRestoredOnSwitch(t *testing.T) {
	m := editorModel(t)
	m.EditorArea.SetValue("select 1\nfrom t\nwhere x")
	m.EditorArea.setCursor(1, 3)

	res, _ := m.handleEditorScreen(tea.KeyPressMsg{Code: 't', Mod: tea.ModCtrl}) // new tab
	m = res.(Model)
	if m.EditorArea.row != 0 || m.EditorArea.col != 0 {
		t.Fatalf("new tab cursor = %d,%d, want 0,0", m.EditorArea.row, m.EditorArea.col)
	}

	res, _ = m.handleEditorScreen(tea.KeyPressMsg{Code: tea.KeyPgUp, Mod: tea.ModCtrl}) // back to tab 0
	m = res.(Model)
	if m.EditorArea.row != 1 || m.EditorArea.col != 3 {
		t.Errorf("restored cursor = %d,%d, want 1,3", m.EditorArea.row, m.EditorArea.col)
	}

	if st := m.queryStateSnapshot(); st.Tabs[0].CursorRow != 1 || st.Tabs[0].CursorCol != 3 {
		t.Errorf("snapshot cursor = %d,%d, want 1,3", st.Tabs[0].CursorRow, st.Tabs[0].CursorCol)
	}
}

func TestQueryTabs_CursorSurvivesLeaveAndReopen(t *testing.T) {
	m := editorModel(t)
	m.EditorArea.SetValue("a\nbb\nccc")
	m.EditorArea.setCursor(2, 1)

	res, _ := m.handleEditorScreen(tea.KeyPressMsg{Code: tea.KeyEsc}) // leave workspace
	m = res.(Model)
	res, _ = m.openEditor()
	m = res.(Model)
	if m.EditorArea.row != 2 || m.EditorArea.col != 1 {
		t.Errorf("reopen cursor = %d,%d, want 2,1", m.EditorArea.row, m.EditorArea.col)
	}
}
