package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/bearded-giant/cellar/internal/tui/commands"
	"github.com/bearded-giant/cellar/internal/tui/types"
	"github.com/bearded-giant/cellar/lib"
)

// inspectorTab is one pane of the floating object inspector. Content loads
// lazily on first activation; raw is the unformatted yank payload.
type inspectorTab struct {
	title  string
	loaded bool
	err    string
	lines  []string
	raw    string
}

// tab indices per object kind (InspTabs is built in openInspector).
const (
	inspTableColumns = iota
	inspTableIndexes
	inspTableFKs
	inspTableDDL
)

const (
	inspViewDefinition = iota
	inspViewColumns
)

func (m Model) openInspector(db, target, label string, isView bool) (tea.Model, tea.Cmd) {
	if target == "" || m.ActiveDriver == nil {
		return m, nil
	}
	m.InspOpen = true
	m.InspIsView = isView
	m.InspDB = db
	m.InspTarget = target
	m.InspLabel = label
	m.InspTab = 0
	m.InspScroll = 0
	if isView {
		m.InspTabs = []inspectorTab{{title: "Definition"}, {title: "Columns"}}
	} else {
		m.InspTabs = []inspectorTab{{title: "Columns"}, {title: "Indexes"}, {title: "FKs"}, {title: "DDL"}}
	}
	return m, m.loadInspectorTab()
}

// openInspectorFromGrid inspects the object behind the current grid — the
// loaded browse table/view; query results carry no table context.
func (m Model) openInspectorFromGrid() (tea.Model, tea.Cmd) {
	if m.Browse.Table == "" {
		m.StatusMsg = "Nothing to inspect — results are not a table (open one in browse)"
		return m, nil
	}
	return m.openInspector(m.Browse.TableDB, m.Browse.Table, m.Browse.Label, m.Browse.IsView)
}

func (m *Model) closeInspector() {
	m.InspOpen = false
	m.InspIsView = false
	m.InspDB = ""
	m.InspTarget = ""
	m.InspLabel = ""
	m.InspTabs = nil
	m.InspTab = 0
	m.InspScroll = 0
}

// loadInspectorTab fires the active tab's loader unless its content already
// landed (each tab loads at most once per open).
func (m *Model) loadInspectorTab() tea.Cmd {
	if m.InspTab >= len(m.InspTabs) || m.InspTabs[m.InspTab].loaded {
		return nil
	}
	if m.InspIsView {
		switch m.InspTab {
		case inspViewDefinition:
			return m.Cmds.LoadViewDefinition(m.ActiveDriver, m.InspDB, m.InspTarget)
		case inspViewColumns:
			return m.Cmds.LoadMeta(m.ActiveDriver, m.InspDB, m.InspTarget, commands.MetaColumns)
		}
		return nil
	}
	switch m.InspTab {
	case inspTableColumns:
		return m.Cmds.LoadMeta(m.ActiveDriver, m.InspDB, m.InspTarget, commands.MetaColumns)
	case inspTableIndexes:
		return m.Cmds.LoadMeta(m.ActiveDriver, m.InspDB, m.InspTarget, commands.MetaIndexes)
	case inspTableFKs:
		return m.Cmds.LoadMeta(m.ActiveDriver, m.InspDB, m.InspTarget, commands.MetaForeignKeys)
	case inspTableDDL:
		return m.Cmds.LoadTableDDL(m.ActiveDriver, m.InspDB, m.InspTarget)
	}
	return nil
}

// inspectorMetaTab maps a LoadMeta kind to the tab it fills (-1 = not shown).
func (m Model) inspectorMetaTab(kind commands.MetaKind) int {
	if m.InspIsView {
		if kind == commands.MetaColumns {
			return inspViewColumns
		}
		return -1
	}
	switch kind {
	case commands.MetaColumns:
		return inspTableColumns
	case commands.MetaIndexes:
		return inspTableIndexes
	case commands.MetaForeignKeys:
		return inspTableFKs
	}
	return -1
}

func (m Model) handleMetaLoadedMsg(msg types.MetaLoadedMsg) (tea.Model, tea.Cmd) {
	if !m.InspOpen {
		return m, nil
	}
	idx := m.inspectorMetaTab(commands.MetaKind(msg.Kind))
	if idx < 0 || idx >= len(m.InspTabs) {
		return m, nil
	}
	t := &m.InspTabs[idx]
	t.loaded = true
	if msg.Err != nil {
		t.err = msg.Err.Error()
		t.lines, t.raw = nil, ""
		return m, nil
	}
	t.err = ""
	t.lines = metaLines(msg.Rows)
	t.raw = metaTSV(msg.Rows)
	return m, nil
}

func (m Model) handleTableDDLLoadedMsg(msg types.TableDDLLoadedMsg) (tea.Model, tea.Cmd) {
	if !m.InspOpen || m.InspIsView || msg.Table != m.InspTarget {
		return m, nil
	}
	fillInspectorText(&m.InspTabs[inspTableDDL], msg.DDL, msg.Err)
	return m, nil
}

func (m Model) handleViewDefinitionLoadedMsg(msg types.ViewDefinitionLoadedMsg) (tea.Model, tea.Cmd) {
	if !m.InspOpen || !m.InspIsView || msg.View != m.InspTarget {
		return m, nil
	}
	fillInspectorText(&m.InspTabs[inspViewDefinition], msg.Definition, msg.Err)
	return m, nil
}

func fillInspectorText(t *inspectorTab, text string, err error) {
	t.loaded = true
	if err != nil {
		t.err = err.Error()
		t.lines, t.raw = nil, ""
		return
	}
	t.err = ""
	t.raw = text
	if strings.TrimSpace(text) == "" {
		t.lines = []string{"(empty)"}
		return
	}
	t.lines = strings.Split(text, "\n")
}

// metaLines renders LoadMeta rows (row 0 = header) as aligned text lines.
func metaLines(rows [][]string) []string {
	if len(rows) == 0 || len(rows[0]) == 0 {
		return []string{"(none)"}
	}
	widths := colWidths(rows[0], rows[1:], maxCellWidth)
	lines := []string{gridRowText(rows[0], widths, 0, len(widths))}
	lines = append(lines, strings.Repeat("─", len([]rune(lines[0]))))
	if len(rows) == 1 {
		return append(lines, "(none)")
	}
	for _, r := range rows[1:] {
		lines = append(lines, gridRowText(r, widths, 0, len(widths)))
	}
	return lines
}

// metaTSV is the yank shape for meta rows: tab-separated, one row per line.
func metaTSV(rows [][]string) string {
	var b strings.Builder
	for i, r := range rows {
		if i > 0 {
			b.WriteByte('\n')
		}
		for j, c := range r {
			if j > 0 {
				b.WriteByte('\t')
			}
			b.WriteString(displayCell(c))
		}
	}
	return b.String()
}

func (m Model) switchInspectorTab(idx int) (tea.Model, tea.Cmd) {
	if idx < 0 || idx >= len(m.InspTabs) || idx == m.InspTab {
		return m, nil
	}
	m.InspTab = idx
	m.InspScroll = 0
	return m, m.loadInspectorTab()
}

func (m Model) handleInspectorKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	last := len(m.inspectorDisplayLines()) - 1
	if m.InspScroll > last { // resize may have re-wrapped to fewer lines
		m.InspScroll = max(last, 0)
	}
	n := len(m.InspTabs)
	switch msg.String() {
	case "esc", "q", "i":
		m.closeInspector()
	case "tab", "l", "right":
		return m.switchInspectorTab((m.InspTab + 1) % n)
	case "shift+tab", "h", "left":
		return m.switchInspectorTab((m.InspTab + n - 1) % n)
	case "1", "2", "3", "4":
		return m.switchInspectorTab(int(msg.String()[0] - '1'))
	case "y": // copy the current tab raw and close (status shows underneath)
		t := m.InspTabs[m.InspTab]
		m.closeInspector()
		switch {
		case !t.loaded || t.raw == "":
			m.StatusMsg = "Nothing to copy yet"
		default:
			if err := lib.NewClipboard().Write(t.raw); err != nil {
				m.StatusMsg = "Copy failed: " + err.Error()
			} else {
				m.StatusMsg = "Copied " + strings.ToLower(t.title) + " to clipboard"
			}
		}
	case "up", "k":
		if m.InspScroll > 0 {
			m.InspScroll--
		}
	case "down", "j":
		if m.InspScroll < last {
			m.InspScroll++
		}
	case "g", "home":
		m.InspScroll = 0
	case "G", "end":
		m.InspScroll = max(last, 0)
	}
	return m, nil
}

// inspSize is ~85% of the terminal, floored for small windows and clamped to
// the terminal itself.
func (m Model) inspSize() (w, h int) {
	w = m.Width * 17 / 20
	if w < 30 {
		w = 30
	}
	if w > m.Width {
		w = max(m.Width, 1)
	}
	h = m.Height * 17 / 20
	if h < 8 {
		h = 8
	}
	if h > m.Height {
		h = max(m.Height, 1)
	}
	return w, h
}

// inspWrapWidth is the box's inner content width: border (2) + padding (2).
func (m Model) inspWrapWidth() int {
	w, _ := m.inspSize()
	return max(w-4, 1)
}

func (m Model) inspectorDisplayLines() []string {
	if m.InspTab >= len(m.InspTabs) {
		return nil
	}
	t := m.InspTabs[m.InspTab]
	switch {
	case t.err != "":
		return wrapLines([]string{"Error: " + t.err}, m.inspWrapWidth())
	case !t.loaded:
		return []string{"loading…"}
	}
	return wrapLines(t.lines, m.inspWrapWidth())
}

func (m Model) inspectorTabBar(innerW int) string {
	var b strings.Builder
	used := 0
	for i, t := range m.InspTabs {
		seg := " " + t.title + " "
		if used+len([]rune(seg)) > innerW {
			break
		}
		used += len([]rune(seg))
		if i == m.InspTab {
			b.WriteString(selectedRowStyle.Render(seg))
		} else {
			b.WriteString(dimStyle.Render(seg))
		}
	}
	if used < innerW {
		b.WriteString(strings.Repeat(" ", innerW-used))
	}
	return b.String()
}

func (m Model) renderInspector() string {
	w, h := m.inspSize()
	innerW := max(w-4, 1)
	bodyH := max(h-5, 1) // border (2) + title + tab bar + footer
	kind := "table"
	if m.InspIsView {
		kind = "view"
	}
	lines := m.inspectorDisplayLines()
	scroll := m.InspScroll
	if last := len(lines) - 1; scroll > last {
		scroll = max(last, 0)
	}
	start, end := visibleWindow(len(lines), scroll, bodyH)
	rows := make([]string, 0, bodyH+3)
	rows = append(rows, accentStyle.Render(truncateRunes(m.InspLabel+"  ·  "+kind, innerW)))
	rows = append(rows, m.inspectorTabBar(innerW))
	for i := start; i < end; i++ {
		rows = append(rows, normalStyle.Render(padRunes(lines[i], innerW)))
	}
	for len(rows) < bodyH+2 {
		rows = append(rows, strings.Repeat(" ", innerW))
	}
	hint := "tab/h/l tabs  1..4 jump  j/k scroll  y copy  esc close"
	if len(lines) > bodyH {
		hint = fmt.Sprintf("%d-%d/%d  ", start+1, end, len(lines)) + hint
	}
	rows = append(rows, dimStyle.Render(truncateRunes(hint, innerW)))
	return peekBoxStyle.Render(strings.Join(rows, "\n"))
}

// composeInspector floats the open inspector over the current screen's render.
func (m Model) composeInspector(base string) string {
	if !m.InspOpen {
		return base
	}
	return overlayCenter(base, m.renderInspector(), m.Width, m.Height)
}
