package types

import (
	"time"

	"github.com/jorgerojas26/lazysql/helpers"
	"github.com/jorgerojas26/lazysql/models"
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
	Err        error
}

type TestResultMsg struct {
	Success bool
	Latency time.Duration
	Err     error
}

type SSHTestMsg struct {
	Err error
}
