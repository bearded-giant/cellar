package commands

import (
	"context"
	"testing"

	"github.com/bearded-giant/cellar/drivers"
	"github.com/bearded-giant/cellar/internal/tui/types"
	"github.com/bearded-giant/cellar/models"
)

// stubDriver records the URL passed to Connect/TestConnection. It satisfies the
// full drivers.Driver interface; only the connect-path methods carry behavior.
type stubDriver struct {
	provider     string
	connectedURL string
	connectErr   error

	databases    []string
	tables       map[string][]string
	useSchemas   bool
	records      [][]string
	recordsTotal int
	recordsErr   error
	lastGetArgs  []string // db, table, where, sort for the last GetRecords

	queryRows  [][]string
	queryTotal int
	queryErr   error
	dmlInfo    string
	dmlErr     error
	ranQuery   string // last ExecuteQuery arg
	ranDML     string // last ExecuteDMLStatement arg
}

func (s *stubDriver) Connect(urlstr string) error {
	s.connectedURL = urlstr
	return s.connectErr
}

func (s *stubDriver) TestConnection(urlstr string) error {
	s.connectedURL = urlstr
	return s.connectErr
}

func (s *stubDriver) GetProvider() string         { return s.provider }
func (s *stubDriver) SetProvider(provider string) { s.provider = provider }

func (s *stubDriver) GetDatabases() ([]string, error)                    { return s.databases, nil }
func (s *stubDriver) GetTables(string) (map[string][]string, error)      { return s.tables, nil }
func (s *stubDriver) GetTableColumns(string, string) ([][]string, error) { return nil, nil }
func (s *stubDriver) GetConstraints(string, string) ([][]string, error)  { return nil, nil }
func (s *stubDriver) GetForeignKeys(string, string) ([][]string, error)  { return nil, nil }
func (s *stubDriver) GetIndexes(string, string) ([][]string, error)      { return nil, nil }
func (s *stubDriver) GetRecords(db, table, where, sort string, _, _ int) ([][]string, int, string, error) {
	s.lastGetArgs = []string{db, table, where, sort}
	return s.records, s.recordsTotal, "", s.recordsErr
}
func (s *stubDriver) ExecuteDMLStatement(q string) (string, error) {
	s.ranDML = q
	return s.dmlInfo, s.dmlErr
}

func (s *stubDriver) ExecuteQuery(q string) ([][]string, int, error) {
	s.ranQuery = q
	return s.queryRows, s.queryTotal, s.queryErr
}
func (s *stubDriver) GetPrimaryKeyColumnNames(string, string) ([]string, error) { return nil, nil }
func (s *stubDriver) SupportsProgramming() bool                                 { return false }
func (s *stubDriver) UseSchemas() bool                                          { return s.useSchemas }
func (s *stubDriver) GetFunctions(string) (map[string][]string, error)          { return nil, nil }
func (s *stubDriver) GetProcedures(string) (map[string][]string, error)         { return nil, nil }
func (s *stubDriver) GetViews(string) (map[string][]string, error)              { return nil, nil }
func (s *stubDriver) GetFunctionDefinition(string, string) (string, error)      { return "", nil }
func (s *stubDriver) GetProcedureDefinition(string, string) (string, error)     { return "", nil }
func (s *stubDriver) GetViewDefinition(string, string) (string, error)          { return "", nil }
func (s *stubDriver) FormatArg(arg any, _ models.CellValueType) any             { return arg }
func (s *stubDriver) FormatArgForQueryString(any) string                        { return "" }
func (s *stubDriver) FormatReference(reference string) string                   { return reference }
func (s *stubDriver) FormatPlaceholder(int) string                              { return "" }

func TestConnect_NonSSHPassesURLToDriver(t *testing.T) {
	stub := &stubDriver{}
	c := &Commands{
		DriverFor: func(string) drivers.Driver { return stub },
	}

	conn := models.Connection{
		Name:     "plain",
		URL:      "mysql://root:pw@127.0.0.1:3306/app",
		Provider: drivers.DriverMySQL,
		UseSSH:   false,
	}

	msg := c.Connect(conn)()
	connected, ok := msg.(types.ConnectedMsg)
	if !ok {
		t.Fatalf("expected ConnectedMsg, got %T", msg)
	}
	if connected.Err != nil {
		t.Fatalf("unexpected error: %v", connected.Err)
	}
	if stub.connectedURL != conn.URL {
		t.Errorf("driver received URL %q, want %q (untunneled passthrough)", stub.connectedURL, conn.URL)
	}
	if connected.URL != conn.URL {
		t.Errorf("ConnectedMsg.URL = %q, want %q", connected.URL, conn.URL)
	}
	if connected.Driver != stub {
		t.Error("ConnectedMsg.Driver should carry the live driver for in-app browse")
	}
}

func TestConnect_StubDriverErrorPropagates(t *testing.T) {
	stub := &stubDriver{connectErr: errStub}
	c := &Commands{DriverFor: func(string) drivers.Driver { return stub }}

	conn := models.Connection{URL: "mysql://h/db", Provider: drivers.DriverMySQL}
	msg := c.Connect(conn)()
	connected := msg.(types.ConnectedMsg)
	if connected.Err != errStub {
		t.Errorf("Err = %v, want errStub", connected.Err)
	}
}

func TestConnect_UnsupportedProvider(t *testing.T) {
	c := &Commands{DriverFor: func(string) drivers.Driver { return nil }}
	conn := models.Connection{URL: "x://y", Provider: "bogus"}
	msg := c.Connect(conn)()
	connected := msg.(types.ConnectedMsg)
	if connected.Err == nil {
		t.Fatal("expected error for unsupported provider")
	}
}

func TestOpenDial_NonSSHSkipsTunnel(t *testing.T) {
	c := &Commands{}
	conn := models.Connection{
		URL:      "postgres://u:p@db:5432/app",
		Provider: drivers.DriverPostgres,
		UseSSH:   false,
	}
	url, tunnel, err := c.openDial(context.Background(), conn)
	if err != nil {
		t.Fatalf("openDial error: %v", err)
	}
	if tunnel != nil {
		t.Error("tunnel should be nil for a non-SSH connection")
	}
	if url != conn.URL {
		t.Errorf("url = %q, want %q (unmodified)", url, conn.URL)
	}
}

func TestOpenDial_SQLiteNeverTunnels(t *testing.T) {
	c := &Commands{}
	conn := models.Connection{
		URL:      "sqlite:///tmp/app.db",
		Provider: drivers.DriverSqlite,
		UseSSH:   true, // even with UseSSH, SQLite has no host to tunnel
		SSHHost:  "bastion",
		SSHUser:  "deploy",
	}
	url, tunnel, err := c.openDial(context.Background(), conn)
	if err != nil {
		t.Fatalf("openDial error: %v", err)
	}
	if tunnel != nil {
		t.Error("SQLite must never tunnel even when UseSSH is set")
	}
	if url != conn.URL {
		t.Errorf("url = %q, want %q (unmodified)", url, conn.URL)
	}
}

func TestSSHConfigFromConnection(t *testing.T) {
	t.Run("port parsed string to int", func(t *testing.T) {
		cfg, err := sshConfigFromConnection(models.Connection{
			SSHHost: "bastion",
			SSHPort: "2222",
			SSHUser: "deploy",
		})
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if cfg.Port != 2222 {
			t.Errorf("Port = %d, want 2222", cfg.Port)
		}
		if cfg.Host != "bastion" || cfg.User != "deploy" {
			t.Errorf("Host/User = %q/%q", cfg.Host, cfg.User)
		}
	})
	t.Run("empty port maps to 0", func(t *testing.T) {
		cfg, err := sshConfigFromConnection(models.Connection{SSHHost: "bastion", SSHPort: ""})
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if cfg.Port != 0 {
			t.Errorf("Port = %d, want 0 (defaults to 22 in dialSSH)", cfg.Port)
		}
	})
	t.Run("invalid port errors", func(t *testing.T) {
		_, err := sshConfigFromConnection(models.Connection{SSHHost: "b", SSHPort: "notanum"})
		if err == nil {
			t.Error("expected error for non-numeric port")
		}
	})
}

func TestDefaultDBPort(t *testing.T) {
	cases := map[string]string{
		drivers.DriverPostgres: "5432",
		drivers.DriverMySQL:    "3306",
		drivers.DriverSqlite:   "3306", // falls into default branch
	}
	for provider, want := range cases {
		if got := defaultDBPort(provider); got != want {
			t.Errorf("defaultDBPort(%q) = %q, want %q", provider, got, want)
		}
	}
}

type stubErr struct{}

func (stubErr) Error() string { return "stub connect failure" }

var errStub error = stubErr{}
