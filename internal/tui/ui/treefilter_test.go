package ui

import "testing"

func TestFlattenTree_Filter(t *testing.T) {
	b := browseState{
		Databases:  []string{"app"},
		Expanded:   map[string]bool{}, // not manually expanded
		TablesByDB: map[string]map[string][]string{"app": {"app": {"users", "orders", "order_items"}}},
		TreeFilter: "order",
	}
	nodes := flattenTree(b)
	// db force-expands under a filter; only "order"-matching tables show
	got := tableRefs(nodes)
	want := map[string]bool{"orders": true, "order_items": true}
	if len(got) != 2 {
		t.Fatalf("filtered tables = %v, want orders + order_items", got)
	}
	for _, r := range got {
		if !want[r] {
			t.Errorf("unexpected table %q in filtered tree", r)
		}
	}
	// db node present + expanded
	if nodes[0].Kind != kindDB || !nodes[0].Expanded {
		t.Errorf("db should be force-expanded under filter: %+v", nodes[0])
	}
}

func TestFlattenTree_FilterDropsNonMatchingDB(t *testing.T) {
	b := browseState{
		Databases:  []string{"app", "metrics"},
		Expanded:   map[string]bool{},
		TablesByDB: map[string]map[string][]string{"app": {"app": {"users"}}}, // metrics not loaded
		TreeFilter: "zzz",
	}
	if nodes := flattenTree(b); len(nodes) != 0 {
		t.Errorf("no matches -> empty tree, got %d nodes", len(nodes))
	}
}

func TestFlattenTree_NoFilterUnchanged(t *testing.T) {
	// empty filter must preserve the original collapsed-by-default behavior
	b := browseState{
		Databases:  []string{"app"},
		Expanded:   map[string]bool{},
		TablesByDB: map[string]map[string][]string{"app": {"app": {"users"}}},
	}
	nodes := flattenTree(b)
	if len(nodes) != 1 || nodes[0].Expanded {
		t.Errorf("no filter: db should be collapsed, got %d nodes (expanded=%v)", len(nodes), nodes[0].Expanded)
	}
}
