package ui

import "testing"

func TestBareIdent(t *testing.T) {
	cases := map[string]string{
		"Product":          "Product",
		`"Product"`:        "Product",
		"public.Product":   "Product",
		`"public"."Order"`: "Order",
		"`db`.`widgets`":   "widgets",
	}
	for in, want := range cases {
		if got := bareIdent(in); got != want {
			t.Errorf("bareIdent(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestResolveTableRef(t *testing.T) {
	// schema driver: db -> schema -> tables
	m := browseModel()
	m.Browse.UseSchemas = true
	m.Browse.TablesByDB = map[string]map[string][]string{
		"appdb": {"public": {"Product", "orders"}},
	}
	db, qualified, ok := m.resolveTableRef("product") // case-insensitive match
	if !ok || db != "appdb" || qualified != "public.Product" {
		t.Errorf("schema resolve = %q/%q/%v, want appdb/public.Product/true", db, qualified, ok)
	}

	// flat driver (mysql/sqlite): bare table, no schema qualification
	m.Browse.UseSchemas = false
	m.Browse.TablesByDB = map[string]map[string][]string{
		"shop": {"shop": {"widgets"}},
	}
	db, qualified, ok = m.resolveTableRef("widgets")
	if !ok || db != "shop" || qualified != "widgets" {
		t.Errorf("flat resolve = %q/%q/%v, want shop/widgets/true", db, qualified, ok)
	}

	if _, _, ok := m.resolveTableRef("nope"); ok {
		t.Error("unknown table should not resolve")
	}
}
