package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/bearded-giant/cellar/helpers"
	"github.com/bearded-giant/cellar/internal/tui/types"
)

// exportRows returns the bounded result set to export. A query result holds its
// full set in QueryRows; otherwise fall back to the loaded page.
func (m Model) exportRows() [][]string {
	if m.Browse.QueryRows != nil {
		return m.Browse.QueryRows
	}
	return m.Browse.Rows
}

func (m Model) openExport() (tea.Model, tea.Cmd) {
	// Export is for query results only — a table preview can be millions of rows
	// and reading them all would hammer the DB. Run a query (with a LIMIT) instead.
	if m.Browse.Table != "" {
		m.StatusMsg = "Export is for query results — run a query with a LIMIT (e)"
		return m, nil
	}
	if len(m.Browse.Columns) == 0 {
		m.StatusMsg = "Nothing to export"
		return m, nil
	}
	ti := textinput.New()
	ti.SetValue(defaultExportPath(m.Browse.Label))
	ti.Width = 60
	ti.Focus()
	ti.CursorEnd()
	m.ExportInput = ti
	m.Screen = types.ScreenExport
	return m, nil
}

func (m Model) handleExportScreen(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.Screen = m.GridReturnScreen
		return m, nil
	case "enter":
		path := strings.TrimSpace(m.ExportInput.Value())
		if path == "" {
			return m, nil
		}
		m.Screen = m.GridReturnScreen
		m.StatusMsg = "Exporting..."
		return m, exportCmd(path, m.Browse.Columns, m.exportRows())
	}
	var cmd tea.Cmd
	m.ExportInput, cmd = m.ExportInput.Update(msg)
	return m, cmd
}

func (m Model) handleExportDoneMsg(msg types.ExportDoneMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.StatusMsg = "Export failed: " + msg.Err.Error()
		return m, nil
	}
	m.StatusMsg = fmt.Sprintf("Exported %d rows to %s", msg.Rows, msg.Path)
	return m, nil
}

func (m Model) viewExport() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Export Results"))
	b.WriteString("\n\n")
	b.WriteString(keyStyle.Render("File path:"))
	b.WriteString("\n")
	b.WriteString(m.ExportInput.View())
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render(".csv or .json extension picks the format"))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("enter:export  esc:cancel"))
	return m.renderModal(b.String())
}

// exportCmd writes the given rows to a file. Format is chosen by the path
// extension: .json -> JSON array (NULL/EMPTY preserved as null/""), otherwise
// CSV via helpers.CSVWriter (which collapses NULL/EMPTY/DEFAULT to "").
func exportCmd(path string, columns []string, rows [][]string) tea.Cmd {
	return func() tea.Msg {
		if strings.HasSuffix(strings.ToLower(path), ".json") {
			data := recordsToJSON(columns, rows) + "\n"
			if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
				return types.ExportDoneMsg{Path: path, Err: err}
			}
			return types.ExportDoneMsg{Path: path, Rows: len(rows)}
		}

		writer, err := helpers.NewCSVWriter(path)
		if err != nil {
			return types.ExportDoneMsg{Path: path, Err: err}
		}
		records := append([][]string{columns}, rows...)
		if err := writer.WriteRecords(records, true); err != nil {
			writer.Abort()
			return types.ExportDoneMsg{Path: path, Err: err}
		}
		if err := writer.Commit(); err != nil {
			return types.ExportDoneMsg{Path: path, Err: err}
		}
		return types.ExportDoneMsg{Path: path, Rows: writer.RowCount()}
	}
}

func defaultExportDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	dl := filepath.Join(home, "Downloads")
	if fi, err := os.Stat(dl); err == nil && fi.IsDir() {
		return dl
	}
	return home
}

func defaultExportPath(label string) string {
	name := sanitizeExportName(label)
	if name == "" {
		name = "export"
	}
	stamp := time.Now().Format("20060102_150405")
	return filepath.Join(defaultExportDir(), name+"_"+stamp+".csv")
}

func sanitizeExportName(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return strings.Trim(b.String(), "_")
}
