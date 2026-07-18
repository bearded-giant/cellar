package drivers

import (
	"context"

	"github.com/bearded-giant/cellar/models"
)

type Driver interface {
	Connect(urlstr string) error
	TestConnection(urlstr string) error
	GetDatabases() ([]string, error)
	GetTables(database string) (map[string][]string, error)
	GetTableColumns(database, table string) ([][]string, error)
	GetConstraints(database, table string) ([][]string, error)
	GetForeignKeys(database, table string) ([][]string, error)
	GetIndexes(database, table string) ([][]string, error)
	// the long-running paths take a context so an in-flight query can be
	// cancelled; metadata getters stay ctx-less (fast)
	GetRecords(ctx context.Context, database, table, where, sort string, offset, limit int) ([][]string, int, string, error)
	ExecuteDMLStatement(ctx context.Context, query string) (string, error)
	// ExecuteQuery fetches at most limit+1 data rows when limit > 0 (the extra
	// row lets callers detect truncation); limit <= 0 fetches everything.
	ExecuteQuery(ctx context.Context, query string, limit int) ([][]string, int, error)
	GetProvider() string
	GetPrimaryKeyColumnNames(database, table string) ([]string, error)

	SupportsProgramming() bool
	UseSchemas() bool
	GetFunctions(database string) (map[string][]string, error)
	GetProcedures(database string) (map[string][]string, error)
	GetViews(database string) (map[string][]string, error)
	GetFunctionDefinition(database string, name string) (string, error)
	GetProcedureDefinition(database string, name string) (string, error)
	GetViewDefinition(database string, name string) (string, error)
	GetTableDDL(database, table string) (string, error)

	FormatArg(arg any, colype models.CellValueType) any
	FormatArgForQueryString(arg any) string
	FormatReference(reference string) string
	FormatPlaceholder(index int) string

	// NOTE: This is used to get the primary key from the database table until I
	// find a better way to do it. See *ResultsTable.GetPrimaryKeyValue()
	SetProvider(provider string)
}
