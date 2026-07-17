package ui

import (
	"reflect"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/bearded-giant/cellar/drivers"
	"github.com/bearded-giant/cellar/internal/tui/types"
)

func viewRefs(nodes []treeNode) []string {
	var out []string
	for _, n := range nodes {
		if n.Kind == kindView {
			out = append(out, n.Table)
		}
	}
	return out
}

func findNode(nodes []treeNode, label string, kind nodeKind) (treeNode, bool) {
	for _, n := range nodes {
		if n.Label == label && n.Kind == kind {
			return n, true
		}
	}
	return treeNode{}, false
}

func TestFlattenTree_ViewsGroupFlat(t *testing.T) {
	b := browseState{
		Databases:  []string{"app"},
		Expanded:   map[string]bool{"app": true},
		TablesByDB: map[string]map[string][]string{"app": {"app": {"orders"}}},
		ViewsByDB:  map[string]map[string][]string{"app": {"app": {"v_orders", "v_users"}}},
	}
	nodes := flattenTree(b)
	group, ok := findNode(nodes, "views", kindGroup)
	if !ok {
		t.Fatal("expanded flat db with views should show a views group")
	}
	if group.Depth != 1 || group.Expanded {
		t.Errorf("views group should be depth 1 and collapsed by default, got %+v", group)
	}
	if refs := viewRefs(nodes); refs != nil {
		t.Errorf("collapsed views group should hide view nodes, got %v", refs)
	}

	b.Expanded[group.Key] = true
	nodes = flattenTree(b)
	if refs := viewRefs(nodes); !reflect.DeepEqual(refs, []string{"v_orders", "v_users"}) {
		t.Errorf("view refs = %v, want bare sorted [v_orders v_users]", refs)
	}
	if v, _ := findNode(nodes, "v_orders", kindView); v.Depth != 2 || v.DB != "app" {
		t.Errorf("flat view node = %+v, want depth 2 in db app", v)
	}
}

func TestFlattenTree_ViewsGroupSchema(t *testing.T) {
	gKey := "app" + treeKeySep + "public"
	b := browseState{
		UseSchemas: true,
		Databases:  []string{"app"},
		Expanded:   map[string]bool{"app": true, gKey: true},
		TablesByDB: map[string]map[string][]string{"app": {"public": {"users"}}},
		ViewsByDB:  map[string]map[string][]string{"app": {"public": {"v_users"}}},
	}
	nodes := flattenTree(b)
	group, ok := findNode(nodes, "views", kindGroup)
	if !ok {
		t.Fatal("expanded schema with views should show a views group")
	}
	if group.Depth != 2 || group.Group != "public" {
		t.Errorf("schema views group = %+v, want depth 2 under public", group)
	}
	// the views group must never share a key with a table named "views"
	if group.Key == gKey+treeKeySep+"views" {
		t.Error("views group key collides with a table key")
	}

	b.Expanded[group.Key] = true
	nodes = flattenTree(b)
	if refs := viewRefs(nodes); !reflect.DeepEqual(refs, []string{"public.v_users"}) {
		t.Errorf("view refs = %v, want schema-qualified [public.v_users]", refs)
	}
	if v, _ := findNode(nodes, "v_users", kindView); v.Depth != 3 {
		t.Errorf("schema view node depth = %d, want 3", v.Depth)
	}
}

func TestFlattenTree_ViewsOnlySchemaAppears(t *testing.T) {
	// a schema holding only views must still get its group tier
	b := browseState{
		UseSchemas: true,
		Databases:  []string{"app"},
		Expanded:   map[string]bool{"app": true},
		TablesByDB: map[string]map[string][]string{"app": {"public": {"users"}}},
		ViewsByDB:  map[string]map[string][]string{"app": {"reporting": {"v_daily"}}},
	}
	nodes := flattenTree(b)
	if _, ok := findNode(nodes, "reporting", kindGroup); !ok {
		t.Error("schema with only views should still render as a group node")
	}
}

func TestFlattenTree_NoViewsNoGroup(t *testing.T) {
	b := browseState{
		Databases:  []string{"app"},
		Expanded:   map[string]bool{"app": true},
		TablesByDB: map[string]map[string][]string{"app": {"app": {"orders"}}},
	}
	if _, ok := findNode(flattenTree(b), "views", kindGroup); ok {
		t.Error("db without views should not render a views group")
	}
}

func TestFlattenTree_FilterMatchesViews(t *testing.T) {
	b := browseState{
		Databases:  []string{"app"},
		Expanded:   map[string]bool{},
		TablesByDB: map[string]map[string][]string{"app": {"app": {"orders"}}},
		ViewsByDB:  map[string]map[string][]string{"app": {"app": {"v_daily", "v_weekly"}}},
		TreeFilter: "daily",
	}
	nodes := flattenTree(b)
	if refs := viewRefs(nodes); !reflect.DeepEqual(refs, []string{"v_daily"}) {
		t.Fatalf("filtered view refs = %v, want [v_daily]", refs)
	}
	if refs := tableRefs(nodes); refs != nil {
		t.Errorf("non-matching tables should be hidden, got %v", refs)
	}
	group, ok := findNode(nodes, "views", kindGroup)
	if !ok || !group.Expanded {
		t.Error("views group should be present and force-expanded under a matching filter")
	}
}

func TestViewsLoadedMsg_StoresAndRebuilds(t *testing.T) {
	m := browseModel()
	m.Browse.Databases = []string{"app"}
	m.Browse.Expanded["app"] = true
	m.Browse.TablesByDB["app"] = map[string][]string{"app": {"orders"}}

	res, _ := m.handleViewsLoadedMsg(types.ViewsLoadedMsg{
		DB:    "app",
		Views: map[string][]string{"app": {"v_orders"}},
	})
	m = res.(Model)
	if _, ok := findNode(m.Browse.Nodes, "views", kindGroup); !ok {
		t.Error("ViewsLoadedMsg should rebuild the tree with the views group")
	}

	errRes, _ := m.handleViewsLoadedMsg(types.ViewsLoadedMsg{DB: "app", Err: errFake})
	if got := errRes.(Model).Browse.GridErr; got == "" {
		t.Error("a views load error should surface in GridErr")
	}
}

func TestExpandDBLoadsViews(t *testing.T) {
	m := browseModel()
	m.Browse.Databases = []string{"app"}
	m.rebuildTree()

	res, cmd := m.handleBrowseTreeKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = res.(Model)
	if !m.Browse.Expanded["app"] || cmd == nil {
		t.Fatal("expanding an unloaded db should fire table + view loads")
	}
}

func TestOpenView_SkipsMetaAndBlocksDML(t *testing.T) {
	m := browseModel()
	m.ActiveDriver = &drivers.SQLite{}
	m.Browse.Databases = []string{"app"}
	m.Browse.Expanded["app"] = true
	m.Browse.TablesByDB["app"] = map[string][]string{"app": {}}
	m.Browse.ViewsByDB["app"] = map[string][]string{"app": {"v_orders"}}
	m.Browse.Expanded["app"+treeKeySep+treeKeySep+"views"] = true
	m.rebuildTree()

	// cursor onto the view node
	for i, n := range m.Browse.Nodes {
		if n.Kind == kindView {
			m.Browse.Cursor = i
		}
	}
	res, cmd := m.handleBrowseTreeKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = res.(Model)
	if !m.Browse.IsView || m.Browse.Table != "v_orders" {
		t.Fatalf("opening a view should set IsView, got IsView=%v table=%q", m.Browse.IsView, m.Browse.Table)
	}
	if cmd == nil {
		t.Fatal("opening a view should still load records")
	}

	m.Browse.Columns = []string{"id"}
	m.Browse.Rows = [][]string{{"1"}}
	res, _ = m.generateDelete()
	m = res.(Model)
	if m.Screen == types.ScreenEditor {
		t.Error("d on a view must not open the editor with a DELETE")
	}
	if m.StatusMsg == "" {
		t.Error("d on a view should explain why nothing happened")
	}
	res, _ = m.generateInsert()
	m = res.(Model)
	if m.Screen == types.ScreenEditor {
		t.Error("o on a view must not open the editor with an INSERT")
	}
}
