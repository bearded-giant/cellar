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
	ScreenCellEdit
	ScreenHistory
	ScreenFilter
	ScreenSetValue
	ScreenSaveQuery
	ScreenSavedQueries
	ScreenCommitPreview
	ScreenYank
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
		ScreenCellEdit:       "Edit Cell",
		ScreenHistory:        "Query History",
		ScreenFilter:         "Filter",
		ScreenSetValue:       "Set Value",
		ScreenSaveQuery:      "Save Query",
		ScreenSavedQueries:   "Saved Queries",
		ScreenCommitPreview:  "Commit Preview",
		ScreenYank:           "Copy",
	}
	if name, ok := names[s]; ok {
		return name
	}
	return "Unknown"
}
