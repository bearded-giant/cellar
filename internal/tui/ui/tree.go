package ui

import (
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/bearded-giant/cellar/internal/tui/types"
)

type nodeKind int

const (
	kindDB nodeKind = iota
	kindGroup
	kindTable
)

type treeNode struct {
	Key      string // unique expand-state key
	Label    string
	Kind     nodeKind
	Depth    int
	DB       string
	Group    string // schema, for schema drivers
	Table    string // schema-qualified ref for table nodes
	Expanded bool
	HasKids  bool
}

const treeKeySep = "\x1f"

// flattenTree turns the loaded database/table maps into the visible, ordered
// node slice the tree renders and the cursor indexes. Pure — unit tested.
// When TreeFilter is set, only matching nodes (and their ancestors) appear and
// matched ancestors are force-expanded.
func flattenTree(b browseState) []treeNode {
	filter := strings.ToLower(strings.TrimSpace(b.TreeFilter))
	filtering := filter != ""
	match := func(s string) bool { return strings.Contains(strings.ToLower(s), filter) }

	var nodes []treeNode
	for _, db := range b.Databases {
		groups, loaded := b.TablesByDB[db]
		dbMatches := match(db)
		if filtering && !dbMatches && !loadedHasMatch(groups, match) {
			continue
		}
		dbExpanded := b.Expanded[db] || (filtering && loaded)
		nodes = append(nodes, treeNode{
			Key: db, Label: db, Kind: kindDB, Depth: 0,
			DB: db, Expanded: dbExpanded, HasKids: true,
		})
		if !dbExpanded || !loaded {
			continue
		}

		if b.UseSchemas {
			for _, g := range sortedKeys(groups) {
				gMatches := match(g)
				tableMatch := anyMatch(groups[g], match)
				if filtering && !dbMatches && !gMatches && !tableMatch {
					continue
				}
				gKey := db + treeKeySep + g
				gExpanded := b.Expanded[gKey] || (filtering && (dbMatches || gMatches || tableMatch))
				nodes = append(nodes, treeNode{
					Key: gKey, Label: g, Kind: kindGroup, Depth: 1,
					DB: db, Group: g, Expanded: gExpanded, HasKids: true,
				})
				if !gExpanded {
					continue
				}
				for _, t := range sortedCopy(groups[g]) {
					if filtering && !dbMatches && !gMatches && !match(t) {
						continue
					}
					nodes = append(nodes, treeNode{
						Key: gKey + treeKeySep + t, Label: t, Kind: kindTable, Depth: 2,
						DB: db, Group: g, Table: g + "." + t,
					})
				}
			}
			continue
		}

		// flat drivers: collapse all groups (usually one) into a table list
		var tables []string
		for _, g := range sortedKeys(groups) {
			tables = append(tables, groups[g]...)
		}
		for _, t := range sortedCopy(tables) {
			if filtering && !dbMatches && !match(t) {
				continue
			}
			nodes = append(nodes, treeNode{
				Key: db + treeKeySep + t, Label: t, Kind: kindTable, Depth: 1,
				DB: db, Table: t,
			})
		}
	}
	return nodes
}

func anyMatch(ss []string, match func(string) bool) bool {
	for _, s := range ss {
		if match(s) {
			return true
		}
	}
	return false
}

// loadedHasMatch reports whether any group name or table in the loaded map
// matches (used to decide whether a db should appear under a filter).
func loadedHasMatch(groups map[string][]string, match func(string) bool) bool {
	for g, tables := range groups {
		if match(g) || anyMatch(tables, match) {
			return true
		}
	}
	return false
}

func sortedKeys(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedCopy(ss []string) []string {
	out := append([]string(nil), ss...)
	sort.Strings(out)
	return out
}

// openTable loads a table node into the active tab's grid (shared by the tree
// Enter path and "open in new tab").
func (m Model) openTable(node treeNode) (tea.Model, tea.Cmd) {
	m.Browse.TableDB = node.DB
	m.Browse.Table = node.Table
	m.Browse.Label = node.Label
	m.Browse.Offset = 0
	m.Browse.RowCursor = 0
	m.Browse.PkColumns = nil
	m.Browse.ViewJSON = false
	m.Browse.FKMap = nil
	m.Browse.Crumbs = nil
	m.resetPending()
	m.Browse.GridErr = ""
	m.Browse.GridLoading = true
	m.Focus = types.FocusGrid
	return m, tea.Batch(
		m.Cmds.LoadRecords(m.ActiveDriver, node.DB, node.Table, "", "", 0, m.Browse.Limit),
		m.Cmds.LoadPrimaryKey(m.ActiveDriver, node.DB, node.Table),
		m.Cmds.LoadForeignKeys(m.ActiveDriver, node.DB, node.Table),
	)
}

func (m Model) handleBrowseTreeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	n := len(m.Browse.Nodes)
	switch msg.String() {
	case "/":
		return m.openTreeFilter()
	case "up", "k":
		if m.Browse.Cursor > 0 {
			m.Browse.Cursor--
		}
	case "down", "j":
		if m.Browse.Cursor < n-1 {
			m.Browse.Cursor++
		}
	case "g", "home":
		m.Browse.Cursor = 0
	case "G", "end":
		m.Browse.Cursor = max(n-1, 0)
	case "enter", " ", "right", "l":
		if n == 0 {
			return m, nil
		}
		node := m.Browse.Nodes[m.Browse.Cursor]
		switch node.Kind {
		case kindTable:
			return m.openTable(node)
		case kindDB:
			if m.Browse.Expanded[node.Key] {
				m.Browse.Expanded[node.Key] = false
				m.rebuildTree()
				return m, nil
			}
			m.Browse.Expanded[node.Key] = true
			m.rebuildTree()
			if _, loaded := m.Browse.TablesByDB[node.DB]; !loaded {
				return m, m.Cmds.LoadTables(m.ActiveDriver, node.DB)
			}
		case kindGroup:
			m.Browse.Expanded[node.Key] = !m.Browse.Expanded[node.Key]
			m.rebuildTree()
		}
	case "left", "h":
		if n == 0 {
			return m, nil
		}
		node := m.Browse.Nodes[m.Browse.Cursor]
		if node.HasKids && m.Browse.Expanded[node.Key] {
			m.Browse.Expanded[node.Key] = false
			m.rebuildTree()
		}
	}
	if m.Browse.Cursor >= len(m.Browse.Nodes) {
		m.Browse.Cursor = max(len(m.Browse.Nodes)-1, 0)
	}
	return m, nil
}

func treeIcon(n treeNode) string {
	if n.Kind == kindTable {
		return "• "
	}
	if n.Expanded {
		return "▾ "
	}
	return "▸ "
}

func (m Model) renderTreeLines(width, height int) []string {
	var lines []string
	add := func(plain string, style func(string) string) {
		lines = append(lines, style(padRunes(plain, width)))
	}
	schemaTitle := "Schema"
	if m.Browse.TreeFilter != "" {
		schemaTitle += " /" + m.Browse.TreeFilter
	}
	add(schemaTitle, func(s string) string { return accentStyle.Render(s) })

	if len(m.Browse.Nodes) == 0 {
		add("(loading…)", func(s string) string { return dimStyle.Render(s) })
		return lines
	}

	bodyH := height - 1
	if bodyH < 1 {
		bodyH = 1
	}
	start, end := visibleWindow(len(m.Browse.Nodes), m.Browse.Cursor, bodyH)
	for i := start; i < end; i++ {
		node := m.Browse.Nodes[i]
		txt := "  " + strings.Repeat("  ", node.Depth) + treeIcon(node) + node.Label
		switch {
		case i == m.Browse.Cursor && m.Focus == types.FocusTree:
			lines = append(lines, selectedRowStyle.Render(padRunes(txt, width)))
		case i == m.Browse.Cursor:
			lines = append(lines, accentStyle.Render(padRunes(txt, width)))
		default:
			lines = append(lines, normalStyle.Render(padRunes(txt, width)))
		}
	}
	return lines
}
