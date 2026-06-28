# LazySQL Fork Implementation Plan

This document outlines the planned improvements for the lazysql fork to enhance usability and add new features.

## Overview

Fork repository: https://github.com/bearded-giant/lazysql
Local path: ~/dev/golang/lazysql

## Priority Features

### 1. Fix Connection Switching Keybinding [HIGH PRIORITY - QUICK WIN]

**Problem**: Backspace key switches connections, breaking normal text editing in SQL editor
**Solution**: Move connection switching from Backspace to Ctrl+Q

**Implementation**:
- File: `app/keymap.go`
- Find in HomeGroup section: `tcell.KeyBackspace2: SwitchToConnectionsView,`
- Remove that line
- Add: `tcell.NewEventKey(tcell.KeyCtrlQ, 0, tcell.ModNone): SwitchToConnectionsView,`

**Testing**:
- Verify Backspace works normally in SQL editor
- Verify Ctrl+Q switches connections

### 2. TSV Copy Functionality [HIGH VALUE]

**Goal**: Copy table data as TSV without borders for clean pasting

**Features**:
1. Copy current row as TSV: `Ctrl+Y`
2. Copy selected rows as TSV: `Ctrl+Shift+C`
3. Toggle minimal view mode: `Ctrl+T`

**Implementation**:

#### Add to `commands/commands.go`:
```go
const (
    // Add to existing commands
    CopyRowAsTSV Command = "copy_row_tsv"
    CopySelectionAsTSV Command = "copy_selection_tsv"
    ToggleTSVView Command = "toggle_tsv_view"
)
```

#### Add to `components/results_table.go`:
```go
func (rt *ResultsTable) CopyRowAsTSV() error {
    row, _ := rt.table.GetSelection()
    var values []string
    
    for col := 0; col < rt.columnCount; col++ {
        cell := rt.table.GetCell(row, col)
        values = append(values, cell.Text)
    }
    
    tsvData := strings.Join(values, "\t")
    return clipboard.WriteAll(tsvData)
}

func (rt *ResultsTable) CopySelectionAsTSV() error {
    // Implementation for multi-row selection
    // Get selected rows, format as TSV, copy to clipboard
}
```

#### Update keybindings in `app/keymap.go`:
- Add to TableGroup:
  - `tcell.NewEventKey(tcell.KeyCtrlY, 0, tcell.ModNone): CopyRowAsTSV`
  - `tcell.NewEventKey(tcell.KeyCtrlC, 0, tcell.ModShift): CopySelectionAsTSV`

### 3. Configurable Keybindings [MEDIUM PRIORITY]

**Goal**: Allow users to customize keybindings via config file

**Implementation**:

#### Update `app/config.go`:
```go
type Config struct {
    // Existing fields...
    KeyBindings KeyBindings `toml:"keybindings,omitempty"`
}

type KeyBindings struct {
    SwitchConnections string `toml:"switch_connections,omitempty"`
    ExecuteQuery      string `toml:"execute_query,omitempty"`
    ClearEditor       string `toml:"clear_editor,omitempty"`
    CopyRowTSV        string `toml:"copy_row_tsv,omitempty"`
    // Add more as needed
}

// Default keybindings
func DefaultKeyBindings() KeyBindings {
    return KeyBindings{
        SwitchConnections: "ctrl+q",
        ExecuteQuery:      "ctrl+space",
        ClearEditor:       "ctrl+d",
        CopyRowTSV:        "ctrl+y",
    }
}
```

#### Create `app/keyparser.go`:
```go
func ParseKeyString(keyStr string) tcell.Key {
    // Parse strings like "ctrl+q", "f10", "backspace"
    // Return corresponding tcell.Key
}
```

#### Update `app/keymap.go`:
- Load custom bindings from config
- Override defaults with user preferences

### 4. Query Save/Load [NICE TO HAVE]

**Goal**: Save and load SQL queries for reuse

**Features**:
1. Save current query: `Ctrl+S`
2. Load saved query: `Ctrl+L`
3. Auto-save query history

**Implementation**:

#### Create `models/query.go`:
```go
type SavedQuery struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`
    SQL       string    `json:"sql"`
    Database  string    `json:"database"`
    CreatedAt time.Time `json:"created_at"`
}
```

#### Create `components/query_manager.go`:
- Save queries to `~/.config/lazysql/saved_queries.json`
- Provide save dialog (enter name)
- Provide load picker (list saved queries)

#### Update keybindings:
- Add to EditorGroup:
  - `tcell.NewEventKey(tcell.KeyCtrlS, 0, tcell.ModNone): SaveQuery`
  - `tcell.NewEventKey(tcell.KeyCtrlL, 0, tcell.ModNone): LoadQuery`

## Implementation Order

1. **Day 1**: Backspace fix (5 minutes) - Immediate relief
2. **Day 2**: TSV copy functionality (2-3 hours) - High value feature
3. **Day 3**: Configurable keybindings (3-4 hours) - Flexibility
4. **Day 4**: Query save/load (4-5 hours) - Nice to have

## Testing Plan

### Manual Testing:
1. Connection switching with new keybinding
2. SQL editor text manipulation
3. TSV copy with various data types
4. Config file parsing and keybinding overrides
5. Query save/load workflow

### Test Scenarios:
- Large result sets
- Special characters in data
- Multiple connections
- Config file errors

## Build Instructions

```bash
cd ~/dev/golang/lazysql
go mod download
go build -o lazysql main.go
./lazysql
```

## Dependencies to Add

```bash
# For clipboard support
go get golang.design/x/clipboard

# May already be present
go get github.com/gdamore/tcell/v2
```

## Configuration Example

After implementing configurable keybindings, users can customize via `~/.config/lazysql/config.toml`:

```toml
[keybindings]
switch_connections = "ctrl+\\"
execute_query = "f5"
copy_row_tsv = "ctrl+y"
clear_editor = "ctrl+l"
```

## Notes

- All keybinding changes should preserve existing functionality
- TSV copy should handle NULL values appropriately
- Config parsing should have sensible defaults
- Error messages should be user-friendly

## References

- Original repo: https://github.com/jorgerojas26/lazysql
- tcell key constants: https://github.com/gdamore/tcell/blob/main/key.go
- tview documentation: https://github.com/rivo/tview/wiki