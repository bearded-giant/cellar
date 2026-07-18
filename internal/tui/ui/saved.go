package ui

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/bearded-giant/cellar/internal/tui/types"
)

// openSaveQuery (from the editor) prompts for a name to save the current query.
func (m Model) openSaveQuery() (tea.Model, tea.Cmd) {
	q := m.EditorArea.Value()
	if strings.TrimSpace(q) == "" {
		m.StatusMsg = "Nothing to save"
		return m, nil
	}
	m.EditorContent = q
	// bound to a saved query already: re-save in place, no name prompt
	if m.SavedName != "" {
		m.SavePendingTab = m.QueryTabActive
		return m, m.Cmds.UpdateSavedQuery(m.connIdent(), m.SavedName, q)
	}
	return m.promptSaveQuery()
}

// openSaveQueryAs (ctrl+shift+s) always prompts, so a bound buffer can be
// saved under a new name; the save flow then rebinds the tab to that name.
func (m Model) openSaveQueryAs() (tea.Model, tea.Cmd) {
	q := m.EditorArea.Value()
	if strings.TrimSpace(q) == "" {
		m.StatusMsg = "Nothing to save"
		return m, nil
	}
	m.EditorContent = q
	return m.promptSaveQuery()
}

// promptSaveQuery opens the name prompt pre-filled with the current tab name.
func (m Model) promptSaveQuery() (tea.Model, tea.Cmd) {
	ti := textinput.New()
	ti.Placeholder = "query name"
	ti.SetWidth(40)
	if i := m.QueryTabActive; i >= 0 && i < len(m.QueryTabs) {
		if name := m.queryTabLabel(i); name != "" && name != "untitled" {
			ti.SetValue(name)
			ti.CursorEnd()
		}
	}
	ti.Focus()
	m.SaveNameInput = ti
	m.Screen = types.ScreenSaveQuery
	return m, nil
}

func (m Model) handleSaveQueryScreen(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.Screen = types.ScreenEditor
		return m, nil
	case "enter":
		name := strings.TrimSpace(m.SaveNameInput.Value())
		if name == "" {
			return m, nil
		}
		m.Screen = types.ScreenEditor
		m.SavePendingTab = m.QueryTabActive
		return m, m.Cmds.SaveQuery(m.connIdent(), name, m.EditorContent)
	}
	var cmd tea.Cmd
	m.SaveNameInput, cmd = m.SaveNameInput.Update(msg)
	return m, cmd
}

func (m Model) handleSavedQuerySavedMsg(msg types.SavedQuerySavedMsg) (tea.Model, tea.Cmd) {
	target := m.SavePendingTab
	m.SavePendingTab = -1
	if msg.Err != nil {
		m.StatusMsg = "Save failed: " + msg.Err.Error()
		return m, nil
	}
	// bind the tab that REQUESTED the save — the user may have switched tabs
	// while the write was in flight
	if target >= 0 && target < len(m.QueryTabs) {
		m.QueryTabs[target].Name = msg.Name
		m.QueryTabs[target].SavedName = msg.Name
		m.QueryTabs[target].SavedBaseline = msg.Query
	}
	if target == m.QueryTabActive {
		m.SavedName = msg.Name      // bind the live mirrors so ctrl+s re-saves in place
		m.SavedBaseline = msg.Query // clean point for the dirty (*) marker
	}
	m.StatusMsg = "Saved query: " + msg.Name
	return m, nil
}

func (m Model) viewSaveQuery() string {
	body := titleStyle.Render("Save Query") + "\n\n" +
		keyStyle.Render("Name:") + "\n" +
		m.SaveNameInput.View() + "\n\n" +
		helpStyle.Render("enter:save  esc:cancel")
	return m.renderModal(body)
}

// openSavedQueries opens the saved side of the query picker (ctrl+o); tab/h/l
// toggles over to history.
func (m Model) openSavedQueries() (tea.Model, tea.Cmd) {
	return m, m.Cmds.LoadSavedQueries(m.connIdent())
}

func (m Model) handleSavedQueriesLoadedMsg(msg types.SavedQueriesLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.StatusMsg = "Saved queries error: " + msg.Err.Error()
		return m, nil
	}
	// an empty list still opens so the history side stays reachable via toggle
	m.SavedItems = msg.Items
	m.SavedCursor = 0
	m.Screen = types.ScreenSavedQueries
	return m, nil
}

func (m Model) handleSavedQueriesScreen(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.Screen = m.GridReturnScreen // back to browse or the editor, wherever it opened
		return m, nil
	case "tab", "h", "l":
		return m.openHistory()
	case "up", "k":
		if m.SavedCursor > 0 {
			m.SavedCursor--
		}
	case "down", "j":
		if m.SavedCursor < len(m.SavedItems)-1 {
			m.SavedCursor++
		}
	case "enter":
		if m.SavedCursor < len(m.SavedItems) {
			it := m.SavedItems[m.SavedCursor]
			return m.openQueryInEditor(it.Query, it.Name, it.Query)
		}
	}
	return m, nil
}

// pickerHeader titles the two-list query picker with the active side lit.
func pickerHeader(historyActive bool) string {
	saved, hist := titleStyle.Render("Saved Queries"), dimStyle.Render("History")
	if historyActive {
		saved, hist = dimStyle.Render("Saved Queries"), titleStyle.Render("History")
	}
	return saved + dimStyle.Render("  │  ") + hist
}

func (m Model) viewSavedQueries() string {
	var b strings.Builder
	b.WriteString(pickerHeader(false))
	b.WriteString("\n\n")
	if len(m.SavedItems) == 0 {
		b.WriteString(dimStyle.Render("(empty)"))
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render("tab:history  esc:close"))
		return m.renderModal(b.String())
	}
	innerW := min(80, max(m.Width-14, 30))
	for i, it := range m.SavedItems {
		q := strings.ReplaceAll(it.Query, "\n", " ")
		line := truncateRunes(it.Name+"  —  "+q, innerW)
		if i == m.SavedCursor {
			b.WriteString(selectedRowStyle.Render("▶ " + line))
		} else {
			b.WriteString(normalStyle.Render("  " + line))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("↑/↓ select  enter:load into editor  tab:history  esc:close"))
	return m.renderModal(b.String())
}
