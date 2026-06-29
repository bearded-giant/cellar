package commands

import (
	"github.com/bearded-giant/cellar/drivers"
	"github.com/bearded-giant/cellar/internal/tui/config"
	"github.com/bearded-giant/cellar/models"
)

// Commands holds the dependencies for the tea.Cmd factories. The config loader
// and saver back the persistence layer; DriverFor is the injectable provider
// picker so connect-flow tests can substitute a stub driver.
type Commands struct {
	cfg       *config.Config
	DriverFor func(provider string) drivers.Driver
}

func defaultDriverFor(provider string) drivers.Driver {
	switch provider {
	case drivers.DriverMySQL:
		return &drivers.MySQL{}
	case drivers.DriverPostgres:
		return &drivers.Postgres{}
	case drivers.DriverSqlite:
		return &drivers.SQLite{}
	case drivers.DriverMSSQL:
		return &drivers.MSSQL{}
	default:
		return nil
	}
}

func New(cfg *config.Config) *Commands {
	return &Commands{
		cfg:       cfg,
		DriverFor: defaultDriverFor,
	}
}

func (c *Commands) Config() *config.Config { return c.cfg }

func (c *Commands) saveConnections(conns []models.Connection) error {
	if c.cfg == nil {
		return nil
	}
	return c.cfg.SaveConnections(conns)
}
