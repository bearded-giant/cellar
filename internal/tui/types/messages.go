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
