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

const defaultQueryRowLimit = 5000

// queryRowLimit resolves the editor fetch cap: 0/unset = default, -1 = unlimited.
func (c *Commands) queryRowLimit() int {
	if c.cfg == nil || c.cfg.AppConfig == nil || c.cfg.AppConfig.QueryRowLimit == 0 {
		return defaultQueryRowLimit
	}
	if c.cfg.AppConfig.QueryRowLimit < 0 {
		return 0
	}
	return c.cfg.AppConfig.QueryRowLimit
}

// capQueryRows trims the limit+1 truncation probe row (see Driver.ExecuteQuery).
func capQueryRows(rows [][]string, total, limit int) ([][]string, int, bool) {
	if limit <= 0 || total <= limit {
		return rows, total, false
	}
	return rows[:limit+1], limit, true // header + limit data rows
}

func (c *Commands) saveConnections(conns []models.Connection) error {
	if c.cfg == nil {
		return nil
	}
	return c.cfg.SaveConnections(conns)
}
