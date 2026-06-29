package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/bearded-giant/cellar/drivers"
	"github.com/bearded-giant/cellar/helpers"
	"github.com/bearded-giant/cellar/internal/tui/types"
)

func (m Model) openExport() (tea.Model, tea.Cmd) {
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
	m.ExportAll = false
	m.Screen = types.ScreenExport
	return m, nil
}

// exportTableScope reports whether the "all rows" scope is available (a real
// table is loaded, not a query result or metadata view).
func (m Model) exportTableScope() bool {
	return m.Browse.Table != "" && m.Browse.MetaKind == metaRecords
}

func (m Model) handleExportScreen(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.Screen = m.GridReturnScreen
		return m, nil
	case "tab":
		if m.exportTableScope() {
			m.ExportAll = !m.ExportAll
		}
		return m, nil
	case "enter":
		path := strings.TrimSpace(m.ExportInput.Value())
		if path == "" {
			return m, nil
		}
		m.Screen = m.GridReturnScreen
		m.StatusMsg = "Exporting..."
		if m.ExportAll && m.exportTableScope() {
			return m, exportAllCmd(m.ActiveDriver, m.Browse.TableDB, m.Browse.Table, m.Browse.Where, m.exportSort(), path)
		}
		return m, exportCmd(path, m.Browse.Columns, m.Browse.Rows)
	}
	var cmd tea.Cmd
	m.ExportInput, cmd = m.ExportInput.Update(msg)
	return m, cmd
}

// exportSort picks a stable ORDER BY for batched export: the active sort, else
// the primary key, else the first column.
func (m Model) exportSort() string {
	if m.Browse.Sort != "" {
		return m.Browse.Sort
	}
	if len(m.Browse.PkColumns) > 0 {
		return m.Browse.PkColumns[0] + " ASC"
	}
	if len(m.Browse.Columns) > 0 {
		return m.Browse.Columns[0] + " ASC"
	}
	return ""
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
	scope := "current page"
	if m.ExportAll {
		scope = "ALL rows (batched)"
	}
	if m.exportTableScope() {
		b.WriteString(keyStyle.Render("scope: ") + normalStyle.Render(scope) + dimStyle.Render("  (tab toggles)"))
		b.WriteString("\n")
	}
	b.WriteString(helpStyle.Render(".csv or .json extension picks the format"))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("enter:export  esc:cancel"))
	return m.renderModal(b.String())
}

// exportCmd writes the current result page to a file. Format is chosen by the
// path extension: .json -> JSON array (NULL/EMPTY preserved as null/""),
// otherwise CSV via helpers.CSVWriter (which collapses NULL/EMPTY/DEFAULT to "").
// Exports the loaded page only (full-table batch export is a follow-up).
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

// exportAllCmd exports an entire table by paging GetRecords in batches (needs a
// stable sort). CSV streams via CSVWriter; JSON accumulates then marshals.
func exportAllCmd(driver drivers.Driver, db, table, where, sort, path string) tea.Cmd {
	return func() tea.Msg {
		if driver == nil {
			return types.ExportDoneMsg{Path: path}
		}
		const batch = 1000
		isJSON := strings.HasSuffix(strings.ToLower(path), ".json")

		var columns []string
		var jsonRows [][]string
		var writer *helpers.CSVWriter
		offset, total := 0, 0

		for {
			rows, _, _, err := driver.GetRecords(db, table, where, sort, offset, batch)
			if err != nil {
				if writer != nil {
					writer.Abort()
				}
				return types.ExportDoneMsg{Path: path, Err: err}
			}
			if len(rows) == 0 {
				break
			}
			header, data := rows[0], rows[1:]
			if offset == 0 {
				columns = header
			}
			if isJSON {
				jsonRows = append(jsonRows, data...)
			} else {
				if writer == nil {
					if writer, err = helpers.NewCSVWriter(path); err != nil {
						return types.ExportDoneMsg{Path: path, Err: err}
					}
				}
				if err := writer.WriteRecords(rows, offset == 0); err != nil {
					writer.Abort()
					return types.ExportDoneMsg{Path: path, Err: err}
				}
			}
			total += len(data)
			if len(data) < batch {
				break
			}
			offset += batch
		}

		if isJSON {
			if err := os.WriteFile(path, []byte(recordsToJSON(columns, jsonRows)+"\n"), 0o644); err != nil {
				return types.ExportDoneMsg{Path: path, Err: err}
			}
			return types.ExportDoneMsg{Path: path, Rows: len(jsonRows)}
		}
		if writer == nil { // empty table
			var err error
			if writer, err = helpers.NewCSVWriter(path); err != nil {
				return types.ExportDoneMsg{Path: path, Err: err}
			}
			_ = writer.WriteRecords([][]string{columns}, true)
		}
		if err := writer.Commit(); err != nil {
			return types.ExportDoneMsg{Path: path, Err: err}
		}
		return types.ExportDoneMsg{Path: path, Rows: total}
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
