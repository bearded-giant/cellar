package ui

import (
	"encoding/json"
	"strings"
)

// recordsToJSON renders a result page as a pretty JSON array of objects,
// preserving column order. Driver sentinels map to JSON: NULL& -> null,
// EMPTY& -> "". All other cells are emitted as JSON strings (driver rows are
// stringified), so this is faithful, not type-inferred.
func recordsToJSON(columns []string, rows [][]string) string {
	if len(columns) == 0 {
		return "[]"
	}
	var b strings.Builder
	b.WriteString("[\n")
	for ri, row := range rows {
		b.WriteString("  {\n")
		for ci, col := range columns {
			val := ""
			if ci < len(row) {
				val = row[ci]
			}
			key, _ := json.Marshal(col)
			b.WriteString("    ")
			b.Write(key)
			b.WriteString(": ")
			b.WriteString(cellToJSONValue(val))
			if ci < len(columns)-1 {
				b.WriteByte(',')
			}
			b.WriteByte('\n')
		}
		b.WriteString("  }")
		if ri < len(rows)-1 {
			b.WriteByte(',')
		}
		b.WriteByte('\n')
	}
	b.WriteString("]")
	return b.String()
}

func cellToJSONValue(cell string) string {
	switch cell {
	case "NULL&":
		return "null"
	case "EMPTY&":
		return `""`
	}
	v, _ := json.Marshal(cell)
	return string(v)
}
