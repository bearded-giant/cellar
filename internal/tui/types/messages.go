package types

import (
	"time"

	"github.com/bearded-giant/cellar/drivers"
	"github.com/bearded-giant/cellar/helpers"
	"github.com/bearded-giant/cellar/models"
)

type ConnectionsLoadedMsg struct {
	Connections []models.Connection
	Err         error
}

type ConnectionSavedMsg struct {
	Connection models.Connection
	IsEdit     bool
	Err        error
}

type ConnectionsReorderedMsg struct {
	Err error
}

type ConnectionDeletedMsg struct {
	Name string
	Err  error
}

type ConnectedMsg struct {
	Connection models.Connection
	URL        string
	Tunnel     *helpers.Tunnel
	// Driver is the live, validated connection; the in-app browser keeps it and
	// the tunnel open for the session.
	Driver drivers.Driver
	Err    error
}

type TestResultMsg struct {
	Success bool
	Latency time.Duration
	Err     error
}

type SSHTestMsg struct {
	Err error
}

// ViewsLoadedMsg mirrors TablesLoadedMsg for views: group -> view names
// (schema-grouped on schema drivers, database-grouped otherwise).
type ViewsLoadedMsg struct {
	DB    string
	Views map[string][]string
	Err   error
}

// ViewDefinitionLoadedMsg carries a view's SQL definition. View echoes the
// requested name (schema-qualified on schema drivers).
type ViewDefinitionLoadedMsg struct {
	View       string
	Definition string
	Err        error
}

// TableDDLLoadedMsg carries a table's CREATE DDL (plus index/constraint
// statements where the driver returns them).
type TableDDLLoadedMsg struct {
	Table string
	DDL   string
	Err   error
}
