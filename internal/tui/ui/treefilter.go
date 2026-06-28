package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jorgerojas26/lazysql/internal/tui/types"
)

func (m Model) openTreeFilter() (tea.Model, tea.Cmd) {
	ti := textinput.New()
	ti.SetValue(m.Browse.TreeFilter)
	ti.Placeholder = "filter schema/tables"
	ti.Width = 40
	ti.Focus()
	ti.CursorEnd()
	m.TreeFilterInput = ti
	m.Screen = types.ScreenTreeFilter
	return m, nil
}

func (m Model) handleTreeFilterScreen(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.Screen = types.ScreenBrowse
		return m, nil
	case "enter":
		m.Browse.TreeFilter = strings.TrimSpace(m.TreeFilterInput.Value())
		m.rebuildTree()
		m.Browse.Cursor = 0
		m.Screen = types.ScreenBrowse
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
