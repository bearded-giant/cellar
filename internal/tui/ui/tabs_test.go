package ui

import "testing"

func TestTabs_NewTabIsolatesGridSharesTree(t *testing.T) {
	m := browseModel()
	m.Browse.Label = "t0"
	m.Browse.Where = "WHERE id = 1" // per-tab grid state on tab 0

	res, _ := m.openSelectedInNewTab() // empty tree -> blank new tab
	m = res.(Model)

	if len(m.Tabs) != 2 || m.TabActive != 1 {
		t.Fatalf("after new tab: len=%d active=%d, want 2/1", len(m.Tabs), m.TabActive)
	}
	if m.Browse.Where != "" {
		t.Errorf("new tab must start with fresh grid state, got Where=%q", m.Browse.Where)
	}

	// tree maps are shared by reference across tabs
	m.Browse.Expanded["db1"] = true
	m.Browse.Label = "t1"

	res, _ = m.switchTab(-1) // back to tab 0
	m = res.(Model)
	if m.TabActive != 0 || m.Browse.Label != "t0" {
		t.Fatalf("switch back: active=%d label=%q, want 0/t0", m.TabActive, m.Browse.Label)
	}
	if m.Browse.Where != "WHERE id = 1" {
		t.Error("tab 0 per-tab grid state lost across switch")
	}
	if !m.Browse.Expanded["db1"] {
		t.Error("tree expansion should be shared across tabs")
	}
}

func TestTabs_SwitchWrapsAround(t *testing.T) {
	m := browseModel()
	res, _ := m.openSelectedInNewTab()
	m = res.(Model) // 2 tabs, active 1

	res, _ = m.switchTab(+1) // 1 -> wraps to 0
	m = res.(Model)
	if m.TabActive != 0 {
		t.Errorf("switch +1 from last tab should wrap to 0, got %d", m.TabActive)
	}
}

func TestTabs_CloseKeepsLast(t *testing.T) {
	m := browseModel()
	res, _ := m.openSelectedInNewTab()
	m = res.(Model) // 2 tabs, active 1

	res, _ = m.closeTab()
	m = res.(Model)
	if len(m.Tabs) != 1 || m.TabActive != 0 {
		t.Fatalf("after close: len=%d active=%d, want 1/0", len(m.Tabs), m.TabActive)
	}

	res, _ = m.closeTab() // closing the last tab is a no-op
	m = res.(Model)
	if len(m.Tabs) != 1 {
		t.Errorf("closing the final tab must be a no-op, len=%d", len(m.Tabs))
	}
}
