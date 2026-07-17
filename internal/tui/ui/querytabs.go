package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/bearded-giant/cellar/internal/state"
	"github.com/bearded-giant/cellar/internal/tui/types"
)

// Query-tab model: the active tab always lives in the EditorArea/EditorContent/
// SavedName/SavedBaseline mirrors (same pattern as m.Browse vs m.Tabs); m.QueryTabs
// holds the snapshots, only as fresh as the last syncActiveQueryTab.

// queryTab is one SQL buffer in the query workspace, distinct from the grid
// browse tabs (m.Tabs).
type queryTab struct {
	Name          string // display name ("untitled" until the save flow names it)
	Content       string
	SavedName     string
	SavedBaseline string
}

// ensureQueryTabs seeds the tab set from the live editor fields on first use.
func (m *Model) ensureQueryTabs() {
	if len(m.QueryTabs) == 0 {
		m.QueryTabs = []queryTab{{
			Name:          "untitled",
			Content:       m.EditorContent,
			SavedName:     m.SavedName,
			SavedBaseline: m.SavedBaseline,
		}}
		m.QueryTabActive = 0
	}
	if m.QueryTabActive >= len(m.QueryTabs) {
		m.QueryTabActive = len(m.QueryTabs) - 1
	}
}

// syncActiveQueryTab writes the live editor fields back into the snapshot slot.
func (m *Model) syncActiveQueryTab() {
	if m.Screen == types.ScreenEditor {
		m.EditorContent = m.EditorArea.Value()
	}
	if m.QueryTabActive < 0 || m.QueryTabActive >= len(m.QueryTabs) {
		return
	}
	t := &m.QueryTabs[m.QueryTabActive]
	t.Content = m.EditorContent
	t.SavedName = m.SavedName
	t.SavedBaseline = m.SavedBaseline
}

// loadQueryTab makes tab i active: live fields + a fresh editor area.
func (m *Model) loadQueryTab(i int) {
	t := m.QueryTabs[i]
	m.QueryTabActive = i
	m.EditorContent = t.Content
	m.SavedName = t.SavedName
	m.SavedBaseline = t.SavedBaseline
	m.EditorArea = m.newEditorArea(t.Content)
	m.EditorArea.Focus()
	m.dismissCompletions()
	m.Focus = types.FocusEditor
}

func (m Model) newQueryTab() (tea.Model, tea.Cmd) {
	m.ensureQueryTabs()
	m.syncActiveQueryTab()
	m.QueryTabs = append(m.QueryTabs, queryTab{Name: "untitled"})
	m.loadQueryTab(len(m.QueryTabs) - 1)
	return m, nil
}

func (m Model) switchQueryTab(delta int) (tea.Model, tea.Cmd) {
	m.ensureQueryTabs()
	if len(m.QueryTabs) <= 1 {
		return m, nil
	}
	m.syncActiveQueryTab()
	n := len(m.QueryTabs)
	m.loadQueryTab(((m.QueryTabActive+delta)%n + n) % n)
	return m, nil
}

func (m Model) closeQueryTab() (tea.Model, tea.Cmd) {
	m.ensureQueryTabs()
	if len(m.QueryTabs) <= 1 {
		return m, nil
	}
	m.QueryTabs = append(m.QueryTabs[:m.QueryTabActive], m.QueryTabs[m.QueryTabActive+1:]...)
	if m.QueryTabActive >= len(m.QueryTabs) {
		m.QueryTabActive = len(m.QueryTabs) - 1
	}
	m.loadQueryTab(m.QueryTabActive)
	return m, nil
}

// resetQueryTabs drops all query-buffer state (connect/disconnect boundary).
func (m *Model) resetQueryTabs() {
	m.QueryTabs = nil
	m.QueryTabActive = 0
	m.EditorContent = ""
	m.SavedName = ""
	m.SavedBaseline = ""
	m.EditorArea = sqlEditor{}
}

// queryStateSnapshot builds the persistable per-connection state; the active
// tab reads from the live editor mirrors.
func (m Model) queryStateSnapshot() state.State {
	m.ensureQueryTabs()
	m.syncActiveQueryTab()
	var st state.State
	for i, t := range m.QueryTabs {
		st.Tabs = append(st.Tabs, state.Tab{
			Name:      t.Name,
			SQL:       t.Content,
			Active:    i == m.QueryTabActive,
			SavedName: t.SavedName,
		})
	}
	return st
}

// autosaveQueryState is the primary persistence trigger, fired on every run
// (ctrl+r / alt+r) so anything executed is already on disk.
func (m *Model) autosaveQueryState() tea.Cmd {
	m.ensureQueryTabs()
	m.syncActiveQueryTab()
	return m.Cmds.SaveQueryState(m.connIdent(), m.queryStateSnapshot())
}

// PersistQueryState synchronously writes the query buffers — the disconnect and
// quit backstop (autosave-on-run covers the common path). Never touches the
// file when this session held no buffers at all.
func (m Model) PersistQueryState() {
	if m.CurrentConn == nil {
		return
	}
	if len(m.QueryTabs) == 0 &&
		strings.TrimSpace(m.EditorContent) == "" &&
		strings.TrimSpace(m.EditorArea.Value()) == "" {
		return
	}
	_ = state.Save(m.CurrentConn.Name, m.queryStateSnapshot())
}

// openQueryInEditor loads a saved query / history entry into the workspace.
// A clean or blank active buffer is replaced in place; unsaved scratch gets a
// new tab instead so a load never clobbers it (dirty-buffer guard).
func (m Model) openQueryInEditor(content, savedName, baseline string) (tea.Model, tea.Cmd) {
	m.ensureQueryTabs()
	dirty := strings.TrimSpace(m.EditorContent) != "" &&
		m.EditorContent != content &&
		(m.SavedName == "" || m.EditorContent != m.SavedBaseline)
	if dirty {
		m.syncActiveQueryTab()
		m.QueryTabs = append(m.QueryTabs, queryTab{Name: "untitled"})
		m.QueryTabActive = len(m.QueryTabs) - 1
	}
	m.EditorContent = content
	m.SavedName = savedName
	m.SavedBaseline = baseline
	mdl, cmd := m.openEditor()
	out := mdl.(Model)
	if dirty {
		out.StatusMsg = "Opened in new tab — unsaved buffer kept"
	}
	return out, cmd
}

// handleQueryStateLoadedMsg restores the persisted buffers on connect. Errors
// and empty state are non-fatal: start blank.
func (m Model) handleQueryStateLoadedMsg(msg types.QueryStateLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil || len(msg.State.Tabs) == 0 {
		return m, nil
	}
	if m.Screen == types.ScreenEditor {
		return m, nil // editor already open: don't clobber live buffers
	}
	m.QueryTabs = nil
	active := 0
	for i, t := range msg.State.Tabs {
		if t.Active {
			active = i
		}
		m.QueryTabs = append(m.QueryTabs, queryTab{
			Name:      t.Name,
			Content:   t.SQL,
			SavedName: t.SavedName,
			// restored buffer starts clean against its binding
			SavedBaseline: t.SQL,
		})
	}
	m.QueryTabActive = active
	t := m.QueryTabs[active]
	m.EditorContent = t.Content
	m.SavedName = t.SavedName
	m.SavedBaseline = t.SavedBaseline
	return m, nil
}

func (m Model) handleQueryStateSavedMsg(msg types.QueryStateSavedMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.StatusMsg = "Query state save failed: " + msg.Err.Error()
	}
	return m, nil
}

// queryTabLabel is the tab's saved-query name when bound (Name and SavedName
// converge after a save), else its display name.
func (m Model) queryTabLabel(i int) string {
	t := m.QueryTabs[i]
	if i == m.QueryTabActive {
		t.SavedName = m.SavedName
	}
	switch {
	case t.SavedName != "":
		return t.SavedName
	case t.Name != "":
		return t.Name
	}
	return "untitled" // pre-naming tabs restored from older state files
}

func (m Model) queryTabBar(width int) string {
	if len(m.QueryTabs) == 0 {
		return ""
	}
	var b strings.Builder
	used := 0
	for i := range m.QueryTabs {
		seg := fmt.Sprintf(" %d:%s ", i+1, truncateRunes(m.queryTabLabel(i), 14))
		w := len([]rune(seg))
		if i > 0 {
			w++ // segment gap
		}
		if used+w > width {
			b.WriteString(dimStyle.Render("…"))
			break
		}
		if i > 0 {
			b.WriteString(" ")
		}
		used += w
		if i == m.QueryTabActive {
			b.WriteString(queryTabActiveStyle.Render(seg))
		} else {
			b.WriteString(queryTabInactiveStyle.Render(seg))
		}
	}
	return b.String()
}
