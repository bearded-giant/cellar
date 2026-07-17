package ui

import (
	"reflect"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/bearded-giant/cellar/internal/tui/types"
)

func TestQueryExecuted_SelectFillsGrid(t *testing.T) {
	m := browseModel()
	m.Browse.Table = "widgets" // pretend a table was open; query result must clear it
	res, _ := m.handleQueryExecutedMsg(types.QueryExecutedMsg{
		IsSelect: true,
		Rows:     [][]string{{"id", "name"}, {"1", "alpha"}},
		Total:    1,
	})
	m = res.(Model)
	if m.Screen != types.ScreenEditor || m.Focus != types.FocusEditor {
		t.Error("query result should stay in the editor workspace (editor focused)")
	}
	if !reflect.DeepEqual(m.Browse.Columns, []string{"id", "name"}) {
		t.Errorf("Columns = %v", m.Browse.Columns)
	}
	if len(m.Browse.Rows) != 1 {
		t.Errorf("data rows = %d, want 1", len(m.Browse.Rows))
	}
	if m.Browse.Table != "" {
		t.Error("query result must blank Table so grid paging stays disabled")
	}
}

func TestQueryExecuted_DMLSetsStatusNoGrid(t *testing.T) {
	m := browseModel()
	res, _ := m.handleQueryExecutedMsg(types.QueryExecutedMsg{IsSelect: false, Info: "2 rows affected"})
	m = res.(Model)
	if m.Browse.Columns != nil {
		t.Error("DML result should not populate grid columns")
	}
	if m.Browse.GridErr != "" {
		t.Errorf("DML success is not an error, got %q", m.Browse.GridErr)
	}
	if m.StatusMsg != "DML: 2 rows affected" {
		t.Errorf("StatusMsg = %q", m.StatusMsg)
	}
}

func TestQueryExecuted_ErrShowsGridErr(t *testing.T) {
	m := browseModel()
	res, _ := m.handleQueryExecutedMsg(types.QueryExecutedMsg{Err: errFake})
	m = res.(Model)
	if m.Browse.GridErr == "" {
		t.Error("query error should surface in GridErr")
	}
	if m.Browse.GridLoading {
		t.Error("GridLoading must clear after a query error")
	}
}

func TestEditor_OpenAndClose(t *testing.T) {
	m := browseModel()
	m.Width, m.Height = 80, 24

	res, _ := m.openEditor()
	m = res.(Model)
	if m.Screen != types.ScreenEditor {
		t.Fatal("openEditor should switch to ScreenEditor")
	}
	if !m.EditorArea.focused {
		t.Error("openEditor should focus the editor")
	}

	res2, _ := m.handleEditorScreen(tea.KeyMsg{Type: tea.KeyEsc})
	if res2.(Model).Screen != types.ScreenBrowse {
		t.Error("esc should return from the editor to the tree/grid")
	}
}

func TestEditor_AutocompleteKeyword(t *testing.T) {
	m := browseModel()
	m.Width, m.Height = 100, 30
	res, _ := m.openEditor()
	m = res.(Model)

	m.EditorArea.SetValue("SEL")
	m.EditorArea.CursorEnd()
	m.refreshCompletions()

	if !m.CompVisible || len(m.Completions) == 0 {
		t.Fatal("typing 'SEL' should surface completions")
	}
	var sawSelect bool
	for _, it := range m.Completions {
		if it.Text == "SELECT" {
			sawSelect = true
		}
	}
	if !sawSelect {
		t.Errorf("completions should include SELECT, got %+v", m.Completions)
	}
}

func TestEditor_AcceptCompletion(t *testing.T) {
	m := browseModel()
	m.Width, m.Height = 100, 30
	res, _ := m.openEditor()
	m = res.(Model)

	m.EditorArea.SetValue("SEL")
	m.EditorArea.CursorEnd()
	m.refreshCompletions()
	if !m.CompVisible {
		t.Fatal("expected completions for 'SEL'")
	}
	want := m.Completions[m.CompCursor].Text
	m.acceptCompletion()
	if m.EditorArea.Value() != want {
		t.Errorf("after accept, value = %q, want %q", m.EditorArea.Value(), want)
	}
	if m.CompVisible {
		t.Error("popup should hide after accepting")
	}
}

func TestEditor_NoPopupWithoutPrefix(t *testing.T) {
	m := browseModel()
	m.Width, m.Height = 100, 30
	res, _ := m.openEditor()
	m = res.(Model)

	m.EditorArea.SetValue("SELECT * FROM ")
	m.EditorArea.CursorEnd()
	m.refreshCompletions()
	if m.CompVisible {
		t.Error("no trailing word -> no popup")
	}
}

func TestEditor_RunClearsPopupSoTabReachesResults(t *testing.T) {
	m := browseModel()
	m.Width, m.Height = 100, 30
	res, _ := m.openEditor()
	m = res.(Model)

	m.EditorArea.SetValue("select token fro") // "fro" -> FROM popup
	m.EditorArea.CursorEnd()
	m.refreshCompletions()
	if !m.CompVisible {
		t.Fatal("expected completion popup for 'fro'")
	}

	res2, _ := m.handleEditorScreen(tea.KeyMsg{Type: tea.KeyCtrlR})
	m = res2.(Model)
	if m.CompVisible {
		t.Error("running a query must dismiss the completion popup (else tab is eaten)")
	}

	res3, _ := m.handleQueryExecutedMsg(types.QueryExecutedMsg{
		IsSelect: true,
		Rows:     [][]string{{"token"}, {"abc"}},
		Total:    1,
	})
	m = res3.(Model)

	res4, _ := m.handleEditorScreen(tea.KeyMsg{Type: tea.KeyTab})
	if got := res4.(Model).Focus; got != types.FocusGrid {
		t.Errorf("after run, tab should reach the results pane; Focus=%v", got)
	}
}

// press runs a key through the editor-screen handler and returns the new model.
func press(t *testing.T, m Model, msg tea.KeyMsg) Model {
	t.Helper()
	res, _ := m.handleEditorScreen(msg)
	return res.(Model)
}

func TestCompletion_AutoShowNeedsTwoRunes(t *testing.T) {
	m := editorModel(t)

	m = press(t, m, keyMsg('S'))
	if m.CompVisible {
		t.Error("1-rune prefix must not auto-show the popup")
	}
	m = press(t, m, keyMsg('E'))
	if !m.CompVisible {
		t.Error("2-rune prefix should auto-show the popup")
	}
	if m.CompEngaged {
		t.Error("a fresh popup must start passive")
	}
}

func TestCompletion_VisiblePopupSurvivesOneRune(t *testing.T) {
	m := editorModel(t)
	m = press(t, m, keyMsg('S'))
	m = press(t, m, keyMsg('E'))
	if !m.CompVisible {
		t.Fatal("expected popup at 2 runes")
	}
	m = press(t, m, tea.KeyMsg{Type: tea.KeyBackspace})
	if !m.CompVisible {
		t.Error("narrowing back to 1 rune should keep an open popup")
	}
	m = press(t, m, tea.KeyMsg{Type: tea.KeyBackspace})
	if m.CompVisible {
		t.Error("empty prefix must hide the popup")
	}
}

func TestCompletion_ManualTriggerAtOneRune(t *testing.T) {
	m := editorModel(t)
	m = press(t, m, keyMsg('S'))
	if m.CompVisible {
		t.Fatal("precondition: no popup at 1 rune")
	}
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlAt}) // ctrl+space
	if !m.CompVisible || len(m.Completions) == 0 {
		t.Error("ctrl+space should show completions for a 1-rune prefix")
	}
}

func TestCompletion_EscSuppressesUntilWordChanges(t *testing.T) {
	m := editorModel(t)
	m = press(t, m, keyMsg('S'))
	m = press(t, m, keyMsg('E'))
	if !m.CompVisible {
		t.Fatal("expected popup for SE")
	}

	m = press(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.CompVisible {
		t.Fatal("esc should dismiss the popup")
	}
	if m.Screen != types.ScreenEditor {
		t.Fatal("first esc must only dismiss, not leave the workspace")
	}

	// extending the dismissed word stays suppressed
	m = press(t, m, keyMsg('L'))
	if m.CompVisible {
		t.Error("typing into the dismissed word must not resurface the popup")
	}

	// a new word auto-shows again
	m = press(t, m, tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
	m = press(t, m, keyMsg('F'))
	m = press(t, m, keyMsg('R'))
	if !m.CompVisible {
		t.Error("a different word should clear the suppression")
	}
}

func TestCompletion_SuppressionClearsWhenPrefixShrinks(t *testing.T) {
	m := editorModel(t)
	m = press(t, m, keyMsg('S'))
	m = press(t, m, keyMsg('E'))
	m = press(t, m, keyMsg('L'))
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.CompVisible || !m.CompDismissed {
		t.Fatal("esc should dismiss and remember the word")
	}

	m = press(t, m, tea.KeyMsg{Type: tea.KeyBackspace}) // "SE" no longer extends "SEL"
	if m.CompDismissed {
		t.Error("shrinking below the dismissed prefix should clear suppression")
	}
	if !m.CompVisible {
		t.Error("with suppression cleared, the 2-rune prefix should show again")
	}
}

func TestCompletion_CtrlSpaceOverridesSuppression(t *testing.T) {
	m := editorModel(t)
	m = press(t, m, keyMsg('S'))
	m = press(t, m, keyMsg('E'))
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlAt})
	if !m.CompVisible {
		t.Error("ctrl+space must override esc-dismiss suppression")
	}
}

func TestCompletion_PassiveKeysStayWithEditor(t *testing.T) {
	m := editorModel(t)
	m.EditorArea.SetValue("SELECT 1\nSE")
	m.EditorArea.CursorEnd()
	m.refreshCompletions()
	if !m.CompVisible || m.CompEngaged {
		t.Fatal("expected a passive popup")
	}

	m = press(t, m, tea.KeyMsg{Type: tea.KeyUp})
	if m.EditorArea.row != 0 {
		t.Error("passive ↑ should move the editor cursor, not the popup")
	}

	m.EditorArea.CursorEnd()
	m.refreshCompletions()
	if !m.CompVisible {
		t.Fatal("expected popup back on the SE word")
	}
	m = press(t, m, tea.KeyMsg{Type: tea.KeyTab})
	if m.Focus != types.FocusGrid {
		t.Error("passive tab should keep its pane-cycle job")
	}
	if m.CompVisible {
		t.Error("moving to the results pane should drop the popup")
	}
}

func TestCompletion_EngagedNavAcceptAndEsc(t *testing.T) {
	m := editorModel(t)
	m = press(t, m, keyMsg('S'))
	m = press(t, m, keyMsg('E'))
	if !m.CompVisible || len(m.Completions) < 2 {
		t.Fatalf("expected 2+ completions for SE, got %d", len(m.Completions))
	}

	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlN})
	if !m.CompEngaged || m.CompCursor != 1 {
		t.Fatalf("ctrl+n should engage and advance, engaged=%v cursor=%d", m.CompEngaged, m.CompCursor)
	}
	m = press(t, m, tea.KeyMsg{Type: tea.KeyUp})
	if m.CompCursor != 0 {
		t.Errorf("engaged ↑ should move the popup cursor back to 0, got %d", m.CompCursor)
	}
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlN})
	want := m.Completions[m.CompCursor].Text

	m = press(t, m, tea.KeyMsg{Type: tea.KeyTab})
	if m.EditorArea.Value() != want {
		t.Errorf("engaged tab should accept %q, buffer = %q", want, m.EditorArea.Value())
	}
	if m.CompVisible || m.CompEngaged {
		t.Error("accept should clear popup + engagement")
	}
}

func TestCompletion_EngagedEscDismissesOnly(t *testing.T) {
	m := editorModel(t)
	m = press(t, m, keyMsg('S'))
	m = press(t, m, keyMsg('E'))
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlN})
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.CompVisible || m.CompEngaged {
		t.Error("esc while engaged should dismiss + disengage")
	}
	if m.Screen != types.ScreenEditor {
		t.Error("esc while engaged must not leave the workspace")
	}
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.Screen != types.ScreenBrowse {
		t.Error("second esc should leave to the tree")
	}
}

func TestEditorResults_EscReturnsToEditorNotTree(t *testing.T) {
	m := editorModel(t)
	m.Focus = types.FocusGrid

	m = press(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.Screen != types.ScreenEditor || m.Focus != types.FocusEditor {
		t.Fatalf("results esc should refocus the editor, got screen=%v focus=%v", m.Screen, m.Focus)
	}

	m.Focus = types.FocusGrid
	m = press(t, m, keyMsg('q'))
	if m.Screen != types.ScreenBrowse {
		t.Errorf("results q should still leave the workspace, got %v", m.Screen)
	}
}

func TestEditorResults_BracketsSwitchQueryTabs(t *testing.T) {
	m := editorModel(t)
	m.EditorArea.SetValue("select 1")
	res, _ := m.newQueryTab()
	m = res.(Model)
	m.EditorArea.SetValue("select 2")
	m.Focus = types.FocusGrid

	m = press(t, m, keyMsg('['))
	if m.QueryTabActive != 0 || m.EditorArea.Value() != "select 1" {
		t.Fatalf("[ in results should switch to the prev query tab, got active=%d %q",
			m.QueryTabActive, m.EditorArea.Value())
	}
	m.Focus = types.FocusGrid
	m = press(t, m, keyMsg(']'))
	if m.QueryTabActive != 1 || m.EditorArea.Value() != "select 2" {
		t.Fatalf("] in results should switch to the next query tab, got active=%d %q",
			m.QueryTabActive, m.EditorArea.Value())
	}
}

func TestEditor_CommentToggleOnCtrlUnderscore(t *testing.T) {
	m := editorModel(t)
	m.EditorArea.SetValue("select 1")
	m.EditorArea.CursorEnd()

	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlUnderscore})
	if m.EditorArea.Value() != "-- select 1" {
		t.Fatalf("ctrl+_ should comment the statement, got %q", m.EditorArea.Value())
	}
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlUnderscore})
	if m.EditorArea.Value() != "select 1" {
		t.Errorf("second ctrl+_ should uncomment, got %q", m.EditorArea.Value())
	}
}

func TestCompletionMinPrefix(t *testing.T) {
	if got := completionMinPrefix(false); got != 2 {
		t.Errorf("hidden popup threshold = %d, want 2", got)
	}
	if got := completionMinPrefix(true); got != 1 {
		t.Errorf("visible popup threshold = %d, want 1", got)
	}
}

func TestCompletionSuppressed(t *testing.T) {
	cases := []struct {
		name            string
		start           int
		prefix          string
		dismissedAt     int
		dismissedPrefix string
		want            bool
	}{
		{"same word extended", 4, "sel", 4, "se", true},
		{"same word exact", 4, "se", 4, "se", true},
		{"word start moved", 9, "se", 4, "se", false},
		{"prefix shrank", 4, "s", 4, "se", false},
		{"different word same start", 4, "fro", 4, "se", false},
	}
	for _, c := range cases {
		if got := completionSuppressed(c.start, c.prefix, c.dismissedAt, c.dismissedPrefix); got != c.want {
			t.Errorf("%s: suppressed = %v, want %v", c.name, got, c.want)
		}
	}
}

type fakeErr struct{}

func (fakeErr) Error() string { return "boom" }

var errFake error = fakeErr{}
