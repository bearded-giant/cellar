package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/bearded-giant/cellar/internal/tui/types"
)

// Multi-tab model: m.Browse is always the ACTIVE tab's live state (every
// handler reads/writes it unchanged); m.Tabs holds the snapshots. Inactive tabs
// are only as fresh as the last saveActiveTab, which runs before any switch.
// Tabs share the schema-tree maps (Databases/TablesByDB/Expanded) by reference
// so an expand/table-load in one tab is visible in all; grid + DML state is
// per-tab.

func (m *Model) saveActiveTab() {
	if m.TabActive >= 0 && m.TabActive < len(m.Tabs) {
		m.Tabs[m.TabActive] = m.Browse
	}
}

// freshTab clones the active tab's shared tree but resets all grid/DML state.
func (m Model) freshTab() browseState {
	b := m.Browse
	return browseState{
		UseSchemas: b.UseSchemas,
		Databases:  b.Databases,
		TablesByDB: b.TablesByDB,
		ViewsByDB:  b.ViewsByDB,
		Expanded:   b.Expanded,
		Nodes:      b.Nodes,
		Cursor:     b.Cursor,
		TreeFilter: b.TreeFilter,
		Limit:      browsePageSize,
	}
}

func (m Model) openSelectedInNewTab() (tea.Model, tea.Cmd) {
	if len(m.Tabs) == 0 {
		m.Tabs = []browseState{m.Browse}
		m.TabActive = 0
	}
	m.saveActiveTab()
	nb := m.freshTab()
	m.Tabs = append(m.Tabs, nb)
	m.TabActive = len(m.Tabs) - 1
	m.Browse = nb

	if m.Browse.Cursor < len(m.Browse.Nodes) {
		if node := m.Browse.Nodes[m.Browse.Cursor]; node.Kind == kindTable || node.Kind == kindView {
			return m.openTable(node)
		}
	}
	m.Focus = types.FocusTree
	return m, nil
}

func (m Model) switchTab(delta int) (tea.Model, tea.Cmd) {
	if len(m.Tabs) <= 1 {
		return m, nil
	}
	m.saveActiveTab()
	n := len(m.Tabs)
	m.TabActive = ((m.TabActive+delta)%n + n) % n
	m.Browse = m.Tabs[m.TabActive]
	m.rebuildTree()
	return m, nil
}

func (m Model) closeTab() (tea.Model, tea.Cmd) {
	if len(m.Tabs) <= 1 {
		return m, nil
	}
	m.saveActiveTab()
	m.Tabs = append(m.Tabs[:m.TabActive], m.Tabs[m.TabActive+1:]...)
	if m.TabActive >= len(m.Tabs) {
		m.TabActive = len(m.Tabs) - 1
	}
	m.Browse = m.Tabs[m.TabActive]
	m.rebuildTree()
	return m, nil
}

func (m Model) tabBarHeight() int {
	if len(m.Tabs) > 1 {
		return 1
	}
	return 0
}

func (m Model) tabBar(width int) string {
	if len(m.Tabs) <= 1 {
		return "" // single tab: no bar (height stays 0)
	}
	var b strings.Builder
	used := 0
	for i := range m.Tabs {
		label := m.Tabs[i].Label
		if i == m.TabActive {
			label = m.Browse.Label // active tab's snapshot is stale until a switch
		}
		if strings.TrimSpace(label) == "" {
			label = "untitled"
		}
		seg := fmt.Sprintf(" %d %s ", i+1, truncateRunes(label, 18))
		if used+len([]rune(seg)) > width {
			b.WriteString(dimStyle.Render("…"))
			break
		}
		used += len([]rune(seg))
		if i == m.TabActive {
			b.WriteString(selectedRowStyle.Render(seg))
		} else {
			b.WriteString(dimStyle.Render(seg))
		}
	}
	return b.String()
}
