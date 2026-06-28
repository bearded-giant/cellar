# Implementation Status

Track progress on lazysql fork improvements.

## Status Key
- 🔴 Not Started
- 🟡 In Progress  
- 🟢 Complete
- 🔵 Testing

## Features

### 1. Fix Connection Switching Keybinding 🔴
- [ ] Remove Backspace binding from HomeGroup in `app/keymap.go`
- [ ] Add Ctrl+Q binding for SwitchToConnectionsView
- [ ] Test backspace works in SQL editor
- [ ] Test Ctrl+Q switches connections

### 2. TSV Copy Functionality 🔴
- [ ] Add new commands to `commands/commands.go`
- [ ] Implement CopyRowAsTSV in `components/results_table.go`
- [ ] Implement CopySelectionAsTSV 
- [ ] Add clipboard dependency
- [ ] Add keybindings to TableGroup
- [ ] Test with various data types
- [ ] Test with NULL values

### 3. Configurable Keybindings 🔴
- [ ] Add KeyBindings struct to `app/config.go`
- [ ] Create `app/keyparser.go` for string parsing
- [ ] Update keymap.go to use config
- [ ] Add defaults function
- [ ] Test config loading
- [ ] Document in README

### 4. Query Save/Load 🔴
- [ ] Create `models/query.go`
- [ ] Create `components/query_manager.go`
- [ ] Add save dialog component
- [ ] Add load picker component
- [ ] Update EditorGroup keybindings
- [ ] Test save/load workflow

## Quick Reference

### File Locations
- Keybindings: `app/keymap.go`
- Commands: `commands/commands.go`
- Config: `app/config.go`
- Results Table: `components/results_table.go`
- SQL Editor: `components/sql_editor.go`

### Key Testing Commands
```bash
# Build
go build -o lazysql main.go

# Run with specific connection
./lazysql --url "mysql://user:pass@host/db"

# Run with config
./lazysql --config
```

### Current Blockers
- None identified yet

### Notes
- Remember to update README.md with new keybindings
- Consider adding a help screen showing all keybindings
- May need to handle key conflicts