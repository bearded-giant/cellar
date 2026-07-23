package ui

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/bearded-giant/cellar/internal/tui/types"
)

func (m Model) openTreeFilter() (tea.Model, tea.Cmd) {
	ti := textinput.New()
	ti.Placeholder = "filter schema/tables"
	ti.SetWidth(40)
	ti.Focus()
	m.TreeFilterInput = ti
	m.TreeFilterReturn = m.Screen // the sidebar opens this from the editor too
	m.Screen = types.ScreenTreeFilter
	return m, nil
}

func (m Model) handleTreeFilterScreen(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.Screen = m.TreeFilterReturn
		return m, nil
	case "enter":
		m.Browse.TreeFilter = strings.TrimSpace(m.TreeFilterInput.Value())
		m.Browse.Cursor = 0 // reset before rebuildTree so its clamp resets TreeTop too
		m.rebuildTree()
		m.Screen = m.TreeFilterReturn
		m.Focus = types.FocusTree
		return m, nil
	}
	var cmd tea.Cmd
	m.TreeFilterInput, cmd = m.TreeFilterInput.Update(msg)
	return m, cmd
}

func (m Model) viewTreeFilter() string {
	body := titleStyle.Render("Filter Tree") + "\n\n" +
		keyStyle.Render("Match (blank clears):") + "\n" +
		m.TreeFilterInput.View() + "\n\n" +
		helpStyle.Render("matched tables auto-expand · enter:apply · esc:cancel")
	return m.renderModal(body)
}
