package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/bearded-giant/cellar/internal/tui/types"
	"github.com/bearded-giant/cellar/models"
)

func filterTestConns() []models.Connection {
	return []models.Connection{
		{Name: "prod-db", URL: "postgres://u:p@prod.internal:5432/app"},
		{Name: "staging", URL: "mysql://u:p@stage.internal:3306/app"},
		{Name: "local", Hostname: "127.0.0.1"},
		{Name: "prod-replica", URL: "postgres://u:p@replica.prod.internal:5432/app"},
	}
}

func TestMatchesConnFilter(t *testing.T) {
	conns := filterTestConns()
	tests := []struct {
		name   string
		conn   models.Connection
		filter string
		want   bool
	}{
		{"empty filter matches all", conns[2], "", true},
		{"whitespace-only filter matches all", conns[2], "   ", true},
		{"name substring", conns[0], "prod", true},
		{"name case-insensitive", conns[0], "PROD", true},
		{"url host substring", conns[1], "stage.internal", true},
		{"url case-insensitive", conns[1], "STAGE.Internal", true},
		{"hostname field substring", conns[2], "127.0", true},
		{"no match", conns[2], "prod", false},
		{"filter trimmed before matching", conns[0], "  prod  ", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesConnFilter(tt.conn, tt.filter); got != tt.want {
				t.Errorf("matchesConnFilter(%q, %q) = %v, want %v", tt.conn.Name, tt.filter, got, tt.want)
			}
		})
	}
}

func TestFilterConnIndices(t *testing.T) {
	conns := filterTestConns()
	tests := []struct {
		name   string
		filter string
		want   []int
	}{
		{"empty filter returns all indices", "", []int{0, 1, 2, 3}},
		{"name match keeps real indices", "prod", []int{0, 3}},
		{"host match via url", "stage.internal", []int{1}},
		{"hostname field match", "127", []int{2}},
		{"no matches", "zzz", []int{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterConnIndices(conns, tt.filter)
			if len(got) != len(tt.want) {
				t.Fatalf("filterConnIndices(%q) = %v, want %v", tt.filter, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("filterConnIndices(%q)[%d] = %d, want %d", tt.filter, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestClampIndex(t *testing.T) {
	tests := []struct {
		i, n, want int
	}{
		{0, 5, 0},
		{4, 5, 4},
		{5, 5, 4},
		{10, 3, 2},
		{-1, 5, 0},
		{0, 0, 0},
		{3, 0, 0},
	}
	for _, tt := range tests {
		if got := clampIndex(tt.i, tt.n); got != tt.want {
			t.Errorf("clampIndex(%d, %d) = %d, want %d", tt.i, tt.n, got, tt.want)
		}
	}
}

func TestScrollWindow(t *testing.T) {
	tests := []struct {
		name                  string
		selected, total, maxV int
		wantStart, wantEnd    int
	}{
		{"all fit", 0, 3, 5, 0, 3},
		{"window follows selection", 7, 10, 3, 5, 8},
		{"top of list", 0, 10, 3, 0, 3},
		{"bottom of list", 9, 10, 3, 7, 10},
		{"empty list", 0, 0, 3, 0, 0},
		{"short filtered list", 1, 2, 3, 0, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end := scrollWindow(tt.selected, tt.total, tt.maxV)
			if start != tt.wantStart || end != tt.wantEnd {
				t.Errorf("scrollWindow(%d, %d, %d) = %d,%d want %d,%d",
					tt.selected, tt.total, tt.maxV, start, end, tt.wantStart, tt.wantEnd)
			}
		})
	}
}

func TestConnFilterFlow(t *testing.T) {
	m := newTestModel()
	m.Connections = filterTestConns()
	m.SelectedConnIdx = 2

	// '/' opens the filter input
	res, _ := m.handleConnectionsScreen(keyMsg('/'))
	m = res.(Model)
	if m.Screen != types.ScreenConnFilter {
		t.Fatalf("after / Screen = %v, want ConnFilter", m.Screen)
	}

	// typing live-filters and clamps the selection to the visible set
	for _, r := range "prod" {
		res, _ = m.handleConnFilterScreen(keyMsg(r))
		m = res.(Model)
	}
	if m.ConnFilter != "prod" {
		t.Fatalf("live ConnFilter = %q, want prod", m.ConnFilter)
	}
	vis := m.visibleConnIndices()
	if len(vis) != 2 || vis[0] != 0 || vis[1] != 3 {
		t.Fatalf("visible indices = %v, want [0 3]", vis)
	}
	if m.SelectedConnIdx != 1 {
		t.Errorf("SelectedConnIdx = %d, want 1 (clamped from 2 into 2-item set)", m.SelectedConnIdx)
	}

	// enter applies and returns to the list
	res, _ = m.handleConnFilterScreen(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(Model)
	if m.Screen != types.ScreenConnections {
		t.Fatalf("after enter Screen = %v, want Connections", m.Screen)
	}
	if m.ConnFilter != "prod" {
		t.Errorf("applied ConnFilter = %q, want prod", m.ConnFilter)
	}

	// esc inside the input cancels: full list restored
	res, _ = m.handleConnectionsScreen(keyMsg('/'))
	m = res.(Model)
	res, _ = m.handleConnFilterScreen(tea.KeyMsg{Type: tea.KeyEsc})
	m = res.(Model)
	if m.Screen != types.ScreenConnections {
		t.Fatalf("after esc Screen = %v, want Connections", m.Screen)
	}
	if m.ConnFilter != "" {
		t.Errorf("esc should clear the filter, got %q", m.ConnFilter)
	}
	if got := len(m.visibleConnIndices()); got != 4 {
		t.Errorf("full list not restored, visible = %d", got)
	}
}

func TestConnFilter_ReopensWithAppliedValue(t *testing.T) {
	m := newTestModel()
	m.Connections = filterTestConns()
	m.ConnFilter = "prod"

	res, _ := m.handleConnectionsScreen(keyMsg('/'))
	m = res.(Model)
	if got := m.ConnFilterInput.Value(); got != "prod" {
		t.Errorf("reopened input = %q, want prod", got)
	}
}

func TestHandleConnectionsScreen_ActionsUseFilteredSet(t *testing.T) {
	t.Run("navigation bounded by visible items", func(t *testing.T) {
		m := newTestModel()
		m.Connections = filterTestConns()
		m.ConnFilter = "prod" // vis = [0, 3]
		m.SelectedConnIdx = 1
		res, _ := m.handleConnectionsScreen(keyMsg('j'))
		if got := res.(Model).SelectedConnIdx; got != 1 {
			t.Errorf("j past last visible: SelectedConnIdx = %d, want 1", got)
		}
	})

	t.Run("edit resolves the real connection behind the visible index", func(t *testing.T) {
		m := newTestModel()
		m.Connections = filterTestConns()
		m.ConnFilter = "prod"
		m.SelectedConnIdx = 1 // second visible = real idx 3
		res, _ := m.handleConnectionsScreen(keyMsg('e'))
		got := res.(Model)
		if got.Screen != types.ScreenEditConnection {
			t.Fatalf("Screen = %v, want EditConnection", got.Screen)
		}
		if got.EditingConnection == nil || got.EditingConnection.Name != "prod-replica" {
			t.Errorf("EditingConnection = %+v, want prod-replica", got.EditingConnection)
		}
	})

	t.Run("delete targets the real connection", func(t *testing.T) {
		m := newTestModel()
		m.Connections = filterTestConns()
		m.ConnFilter = "127"
		m.SelectedConnIdx = 0 // only visible = real idx 2
		res, _ := m.handleConnectionsScreen(keyMsg('d'))
		got := res.(Model)
		conn, ok := got.ConfirmData.(models.Connection)
		if !ok || conn.Name != "local" {
			t.Errorf("ConfirmData = %+v, want local", got.ConfirmData)
		}
	})

	t.Run("stale real index self-heals via clamp", func(t *testing.T) {
		m := newTestModel()
		m.Connections = filterTestConns()
		m.ConnFilter = "prod" // vis len 2
		m.SelectedConnIdx = 3 // stale index from before the filter
		res, _ := m.handleConnectionsScreen(keyMsg('j'))
		if got := res.(Model).SelectedConnIdx; got != 1 {
			t.Errorf("SelectedConnIdx = %d, want 1 (clamped)", got)
		}
	})

	t.Run("no visible matches disables selection actions", func(t *testing.T) {
		m := newTestModel()
		m.Connections = filterTestConns()
		m.ConnFilter = "zzz"
		res, _ := m.handleConnectionsScreen(keyMsg('e'))
		if got := res.(Model).Screen; got != types.ScreenConnections {
			t.Errorf("e with no matches: Screen = %v, want Connections", got)
		}
	})

	t.Run("esc on the list clears an applied filter", func(t *testing.T) {
		m := newTestModel()
		m.Connections = filterTestConns()
		m.ConnFilter = "prod"
		res, _ := m.handleConnectionsScreen(tea.KeyMsg{Type: tea.KeyEsc})
		if got := res.(Model).ConnFilter; got != "" {
			t.Errorf("ConnFilter = %q, want cleared", got)
		}
	})
}

func TestMoveConnection_NoOpWhileFiltered(t *testing.T) {
	m := newTestModel()
	m.Connections = filterTestConns()
	m.ConnFilter = "prod"
	m.SelectedConnIdx = 0

	res, cmd := m.handleConnectionsScreen(keyMsg('J'))
	if cmd != nil {
		t.Error("J while filtered should not fire a persist command")
	}
	got := res.(Model)
	if names(got.Connections) != names(filterTestConns()) {
		t.Errorf("J while filtered reordered: %v", names(got.Connections))
	}

	res, cmd = m.handleConnectionsScreen(keyMsg('K'))
	if cmd != nil {
		t.Error("K while filtered should not fire a persist command")
	}
	if names(res.(Model).Connections) != names(filterTestConns()) {
		t.Errorf("K while filtered reordered: %v", names(res.(Model).Connections))
	}
}
