package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

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
	ti := textinput.New()
	ti.Placeholder = "query name"
	ti.Width = 40
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
		return m, m.Cmds.SaveQuery(m.connIdent(), name, m.EditorContent)
	}
	var cmd tea.Cmd
	m.SaveNameInput, cmd = m.SaveNameInput.Update(msg)
	return m, cmd
}

func (m Model) handleSavedQuerySavedMsg(msg types.SavedQuerySavedMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.StatusMsg = "Save failed: " + msg.Err.Error()
		return m, nil
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

// openSavedQueries (from browse) loads the saved-query list.
func (m Model) openSavedQueries() (tea.Model, tea.Cmd) {
	return m, m.Cmds.LoadSavedQueries(m.connIdent())
}

func (m Model) handleSavedQueriesLoadedMsg(msg types.SavedQueriesLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.StatusMsg = "Saved queries error: " + msg.Err.Error()
		return m, nil
	}
	if len(msg.Items) == 0 {
		m.StatusMsg = "No saved queries"
		return m, nil
	}
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
			m.EditorContent = m.SavedItems[m.SavedCursor].Query
			return m.openEditor()
		}
	}
	return m, nil
}

func (m Model) viewSavedQueries() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Saved Queries"))
	b.WriteString("\n\n")
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
	b.WriteString(helpStyle.Render("↑/↓ select  enter:load into editor  esc:cancel"))
	return m.renderModal(b.String())
}
