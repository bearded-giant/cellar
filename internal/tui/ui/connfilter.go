package ui

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/bearded-giant/cellar/internal/tui/types"
	"github.com/bearded-giant/cellar/models"
)

func matchesConnFilter(conn models.Connection, filter string) bool {
	f := strings.ToLower(strings.TrimSpace(filter))
	if f == "" {
		return true
	}
	return strings.Contains(strings.ToLower(conn.Name), f) ||
		strings.Contains(strings.ToLower(conn.URL), f) ||
		strings.Contains(strings.ToLower(conn.Hostname), f)
}

// filterConnIndices returns the real m.Connections indices that survive the
// filter, so actions on a filtered selection resolve to the right connection.
func filterConnIndices(conns []models.Connection, filter string) []int {
	idx := make([]int, 0, len(conns))
	for i := range conns {
		if matchesConnFilter(conns[i], filter) {
			idx = append(idx, i)
		}
	}
	return idx
}

func clampIndex(i, n int) int {
	if i >= n {
		i = n - 1
	}
	if i < 0 {
		i = 0
	}
	return i
}

// scrollWindow computes the [start, end) slice of a windowed list that keeps
// the selection in view.
func scrollWindow(selected, total, maxVisible int) (int, int) {
	start := 0
	if selected >= maxVisible {
		start = selected - maxVisible + 1
	}
	end := start + maxVisible
	if end > total {
		end = total
		if end-start < maxVisible {
			start = max(end-maxVisible, 0)
		}
	}
	return start, end
}

func (m Model) visibleConnIndices() []int {
	return filterConnIndices(m.Connections, m.ConnFilter)
}

func (m Model) visibleSelectedConn(vis []int) (models.Connection, bool) {
	if len(vis) == 0 || m.SelectedConnIdx < 0 || m.SelectedConnIdx >= len(vis) {
		return models.Connection{}, false
	}
	return m.Connections[vis[m.SelectedConnIdx]], true
}

func (m Model) openConnFilter() (tea.Model, tea.Cmd) {
	ti := textinput.New()
	ti.SetValue(m.ConnFilter)
	ti.Placeholder = "filter name/host"
	ti.SetWidth(40)
	ti.Focus()
	ti.CursorEnd()
	m.ConnFilterInput = ti
	m.Screen = types.ScreenConnFilter
	return m, nil
}

func (m Model) handleConnFilterScreen(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.ConnFilter = ""
		m.SelectedConnIdx = clampIndex(m.SelectedConnIdx, len(m.Connections))
		m.Screen = types.ScreenConnections
		return m, nil
	case "enter":
		m.ConnFilter = strings.TrimSpace(m.ConnFilterInput.Value())
		m.SelectedConnIdx = clampIndex(m.SelectedConnIdx, len(m.visibleConnIndices()))
		m.Screen = types.ScreenConnections
		return m, nil
	}
	var cmd tea.Cmd
	m.ConnFilterInput, cmd = m.ConnFilterInput.Update(msg)
	m.ConnFilter = strings.TrimSpace(m.ConnFilterInput.Value())
	m.SelectedConnIdx = clampIndex(m.SelectedConnIdx, len(m.visibleConnIndices()))
	return m, cmd
}

// viewConnFilter reuses the connections view so matches stay visible while
// typing; the input row renders in place of the footer help.
func (m Model) viewConnFilter() string {
	return m.viewConnections()
}
