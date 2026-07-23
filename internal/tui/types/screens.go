package types

type Screen int

const (
	ScreenConnections Screen = iota
	ScreenAddConnection
	ScreenEditConnection
	ScreenSSHTunnel
	ScreenTestConnection
	ScreenConfirmDelete
	ScreenBrowse
	ScreenEditor
	ScreenExport
	ScreenHistory
	ScreenFilter
	ScreenSaveQuery
	ScreenSavedQueries
	ScreenYank
	ScreenCellView
	ScreenTreeFilter
	ScreenHelp
	ScreenConnFilter
	ScreenSettings
	ScreenCommand
)

func (s Screen) String() string {
	names := map[Screen]string{
		ScreenConnections:    "Connections",
		ScreenAddConnection:  "Add Connection",
		ScreenEditConnection: "Edit Connection",
		ScreenSSHTunnel:      "SSH Tunnel",
		ScreenTestConnection: "Test Connection",
		ScreenConfirmDelete:  "Confirm Delete",
		ScreenBrowse:         "Browse",
		ScreenEditor:         "SQL Editor",
		ScreenExport:         "Export",
		ScreenHistory:        "Query History",
		ScreenFilter:         "Filter",
		ScreenSaveQuery:      "Save Query",
		ScreenSavedQueries:   "Saved Queries",
		ScreenYank:           "Copy",
		ScreenCellView:       "Cell",
		ScreenTreeFilter:     "Filter Tree",
		ScreenHelp:           "Help",
		ScreenConnFilter:     "Filter Connections",
		ScreenSettings:       "Settings",
		ScreenCommand:        "Command",
	}
	if name, ok := names[s]; ok {
		return name
	}
	return "Unknown"
}
