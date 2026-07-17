package drivers

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	_ "github.com/lib/pq"
)

const (
	DBNamePostgres    = "postgres"
	schemaPostgres    = "public"
	tableNamePostgres = "test_table"
)

var schemaAndTablePostgres = fmt.Sprintf("%s.%s", schemaPostgres, tableNamePostgres)

func TestPostgres_FormatArg_SpecialCharacters(t *testing.T) {
	db := &Postgres{}

	testCases := []struct {
		name     string
		arg      any
		expected string
	}{
		{
			name:     "String with single quote",
			arg:      "O'Reilly",
			expected: "'O''Reilly'",
		},
		{
			name:     "String with backslash",
			arg:      "C:\\Program Files",
			expected: "'C:\\Program Files'",
		},
		{
			name:     "String with double quotes",
			arg:      `"quoted"`,
			expected: `'"quoted"'`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			formattedArg := db.FormatArgForQueryString(tc.arg)
			if formattedArg != tc.expected {
				t.Fatalf("expected %q, but got %q", tc.expected, formattedArg)
			}
		})
	}
}

func TestPostgres_FormatArg(t *testing.T) {
	db := &Postgres{}

	testCases := []struct {
		name     string
		arg      any
		expected string
	}{
		{
			name:     "Integer argument",
			arg:      123,
			expected: "123",
		},
		{
			name:     "String argument",
			arg:      "test string",
			expected: "'test string'",
		},
		{
			name:     "Byte array argument",
			arg:      []byte("byte array"),
			expected: "[98 121 116 101 32 97 114 114 97 121]",
		},
		{
			name:     "Float argument",
			arg:      123.45,
			expected: "123.45",
		},
		{
			name:     "Boolean true",
			arg:      true,
			expected: "true",
		},
		{
			name:     "Boolean false",
			arg:      false,
			expected: "false",
		},
		{
			name:     "NULL value",
			arg:      nil,
			expected: "<nil>",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			formattedArg := db.FormatArgForQueryString(tc.arg)
			if formattedArg != tc.expected {
				t.Fatalf("expected %q, but got %q", tc.expected, formattedArg)
			}
		})
	}
}

func TestPostgres_Connect_Mock(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock: %v", err)
	}
	defer db.Close()

	pg := &Postgres{Connection: db}
	mock.ExpectPing()

	err = pg.Connection.Ping()
	if err != nil {
		t.Fatalf("Ping failed: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

func TestPostgres_ErrorScenarios(t *testing.T) {
	testCases := []struct {
		name      string
		setupMock func(mock sqlmock.Sqlmock)
		testFunc  func(db *Postgres) error
	}{
		{
			name: "GetTables error",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT table_name, table_schema FROM information_schema.tables WHERE table_catalog = \\$1").WithArgs(schemaPostgres).
					WillReturnError(errors.New("query error"))
			},
			testFunc: func(db *Postgres) error {
				_, err := db.GetTables(schemaPostgres)
				return err
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("Error creating mock: %v", err)
			}
			defer db.Close()

			tc.setupMock(mock)
			pg := &Postgres{Connection: db, CurrentDatabase: schemaPostgres}

			err = tc.testFunc(pg)
			if err == nil {
				t.Error("Expected error but got nil")
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("Unfulfilled expectations: %s", err)
			}
		})
	}
}

func TestPostgres_GetTables_FiltersToBaseTables(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("Error creating mock: %v", err)
	}
	defer db.Close()

	pg := &Postgres{Connection: db, CurrentDatabase: DBNamePostgres}

	rows := sqlmock.NewRows([]string{"table_name", "table_schema"}).
		AddRow("users", "public").
		AddRow("events", "audit")

	mock.ExpectQuery("SELECT table_name, table_schema FROM information_schema.tables WHERE table_catalog = $1 AND table_type = 'BASE TABLE'").
		WithArgs(DBNamePostgres).
		WillReturnRows(rows)

	tables, err := pg.GetTables(DBNamePostgres)
	if err != nil {
		t.Fatalf("GetTables failed: %v", err)
	}

	expected := map[string][]string{
		"public": {"users"},
		"audit":  {"events"},
	}
	if !reflect.DeepEqual(tables, expected) {
		t.Fatalf("Expected %v, got %v", expected, tables)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

func TestPostgres_GetViews(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("Error creating mock: %v", err)
	}
	defer db.Close()

	pg := &Postgres{Connection: db, CurrentDatabase: DBNamePostgres}

	rows := sqlmock.NewRows([]string{"table_name", "table_schema"}).
		AddRow("v_users", "public").
		AddRow("mv_daily_totals", "public")

	mock.ExpectQuery("SELECT table_name, table_schema FROM information_schema.tables WHERE table_catalog = $1 AND table_type = 'VIEW' UNION ALL SELECT matviewname, schemaname FROM pg_matviews").
		WithArgs(DBNamePostgres).
		WillReturnRows(rows)

	views, err := pg.GetViews(DBNamePostgres)
	if err != nil {
		t.Fatalf("GetViews failed: %v", err)
	}

	expected := map[string][]string{
		"public": {"v_users", "mv_daily_totals"},
	}
	if !reflect.DeepEqual(views, expected) {
		t.Fatalf("Expected %v, got %v", expected, views)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

func TestPostgres_GetViewDefinition_View(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("Error creating mock: %v", err)
	}
	defer db.Close()

	pg := &Postgres{Connection: db, CurrentDatabase: DBNamePostgres}

	// not a matview -> falls through to pg_get_viewdef
	mock.ExpectQuery("SELECT definition FROM pg_matviews WHERE schemaname = $1 AND matviewname = $2").
		WithArgs(schemaPostgres, "v_users").
		WillReturnRows(sqlmock.NewRows([]string{"definition"}))

	mock.ExpectQuery("SELECT pg_get_viewdef(format('%I.%I', $1::text, $2::text)::regclass, true)").
		WithArgs(schemaPostgres, "v_users").
		WillReturnRows(sqlmock.NewRows([]string{"pg_get_viewdef"}).AddRow(" SELECT id, name FROM users;"))

	def, err := pg.GetViewDefinition(DBNamePostgres, "public.v_users")
	if err != nil {
		t.Fatalf("GetViewDefinition failed: %v", err)
	}

	if def != "SELECT id, name FROM users;" {
		t.Fatalf("definition = %q", def)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

func TestPostgres_GetViewDefinition_MaterializedView(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("Error creating mock: %v", err)
	}
	defer db.Close()

	pg := &Postgres{Connection: db, CurrentDatabase: DBNamePostgres}

	mock.ExpectQuery("SELECT definition FROM pg_matviews WHERE schemaname = $1 AND matviewname = $2").
		WithArgs(schemaPostgres, "mv_daily_totals").
		WillReturnRows(sqlmock.NewRows([]string{"definition"}).AddRow(" SELECT day, sum(amount) FROM orders GROUP BY day;"))

	def, err := pg.GetViewDefinition(DBNamePostgres, "public.mv_daily_totals")
	if err != nil {
		t.Fatalf("GetViewDefinition failed: %v", err)
	}

	if def != "SELECT day, sum(amount) FROM orders GROUP BY day;" {
		t.Fatalf("definition = %q", def)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

func TestPostgres_GetViewDefinition_RequiresSchemaQualifiedName(t *testing.T) {
	pg := &Postgres{}
	if _, err := pg.GetViewDefinition(DBNamePostgres, "bare_view"); err == nil {
		t.Fatal("expected error for unqualified view name")
	}
}

func TestPostgres_GetTableDDL(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("Error creating mock: %v", err)
	}
	defer db.Close()

	pg := &Postgres{Connection: db, CurrentDatabase: DBNamePostgres}

	mock.ExpectQuery("SELECT a.attname, format_type(a.atttypid, a.atttypmod), a.attnotnull, COALESCE(pg_get_expr(ad.adbin, ad.adrelid), '') FROM pg_attribute a JOIN pg_class c ON c.oid = a.attrelid JOIN pg_namespace n ON n.oid = c.relnamespace LEFT JOIN pg_attrdef ad ON ad.adrelid = a.attrelid AND ad.adnum = a.attnum WHERE n.nspname = $1 AND c.relname = $2 AND a.attnum > 0 AND NOT a.attisdropped ORDER BY a.attnum").
		WithArgs(schemaPostgres, tableNamePostgres).
		WillReturnRows(sqlmock.NewRows([]string{"attname", "format_type", "attnotnull", "default"}).
			AddRow("id", "integer", true, "nextval('test_table_id_seq'::regclass)").
			AddRow("name", "character varying(255)", false, ""))

	mock.ExpectQuery("SELECT con.conname, pg_get_constraintdef(con.oid) FROM pg_constraint con JOIN pg_class c ON c.oid = con.conrelid JOIN pg_namespace n ON n.oid = c.relnamespace WHERE con.contype = 'p' AND n.nspname = $1 AND c.relname = $2").
		WithArgs(schemaPostgres, tableNamePostgres).
		WillReturnRows(sqlmock.NewRows([]string{"conname", "condef"}).
			AddRow("test_table_pkey", "PRIMARY KEY (id)"))

	mock.ExpectQuery("SELECT con.conname, pg_get_constraintdef(con.oid) FROM pg_constraint con JOIN pg_class c ON c.oid = con.conrelid JOIN pg_namespace n ON n.oid = c.relnamespace WHERE con.contype = 'f' AND n.nspname = $1 AND c.relname = $2 ORDER BY con.conname").
		WithArgs(schemaPostgres, tableNamePostgres).
		WillReturnRows(sqlmock.NewRows([]string{"conname", "condef"}).
			AddRow("fk_user", "FOREIGN KEY (user_id) REFERENCES users(id)"))

	mock.ExpectQuery("SELECT indexname, indexdef FROM pg_indexes WHERE schemaname = $1 AND tablename = $2 ORDER BY indexname").
		WithArgs(schemaPostgres, tableNamePostgres).
		WillReturnRows(sqlmock.NewRows([]string{"indexname", "indexdef"}).
			AddRow("idx_name", "CREATE INDEX idx_name ON public.test_table USING btree (name)").
			AddRow("test_table_pkey", "CREATE UNIQUE INDEX test_table_pkey ON public.test_table USING btree (id)"))

	ddl, err := pg.GetTableDDL(DBNamePostgres, schemaAndTablePostgres)
	if err != nil {
		t.Fatalf("GetTableDDL failed: %v", err)
	}

	expected := `CREATE TABLE "public"."test_table" (
    "id" integer NOT NULL DEFAULT nextval('test_table_id_seq'::regclass),
    "name" character varying(255),
    CONSTRAINT "test_table_pkey" PRIMARY KEY (id)
);

ALTER TABLE "public"."test_table" ADD CONSTRAINT "fk_user" FOREIGN KEY (user_id) REFERENCES users(id);

CREATE INDEX idx_name ON public.test_table USING btree (name);`

	if ddl != expected {
		t.Fatalf("DDL mismatch.\nExpected:\n%s\nGot:\n%s", expected, ddl)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

func TestPostgres_GetTableDDL_TableNotFound(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("Error creating mock: %v", err)
	}
	defer db.Close()

	pg := &Postgres{Connection: db, CurrentDatabase: DBNamePostgres}

	mock.ExpectQuery("SELECT a.attname, format_type(a.atttypid, a.atttypmod), a.attnotnull, COALESCE(pg_get_expr(ad.adbin, ad.adrelid), '') FROM pg_attribute a JOIN pg_class c ON c.oid = a.attrelid JOIN pg_namespace n ON n.oid = c.relnamespace LEFT JOIN pg_attrdef ad ON ad.adrelid = a.attrelid AND ad.adnum = a.attnum WHERE n.nspname = $1 AND c.relname = $2 AND a.attnum > 0 AND NOT a.attisdropped ORDER BY a.attnum").
		WithArgs(schemaPostgres, "missing").
		WillReturnRows(sqlmock.NewRows([]string{"attname", "format_type", "attnotnull", "default"}))

	if _, err := pg.GetTableDDL(DBNamePostgres, "public.missing"); err == nil {
		t.Fatal("expected error for missing table")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

func TestPostgres_GetTableColumns(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("Error creating mock: %v", err)
	}
	defer db.Close()

	pg := &Postgres{Connection: db, CurrentDatabase: DBNamePostgres}

	// Mocks expected query with all 5 columns including the new "comment" field
	rows := sqlmock.NewRows([]string{
		"column_name",
		"data_type",
		"is_nullable",
		"column_default",
		"comment",
	}).AddRow(
		"id",
		"integer",
		"NO",
		"nextval('test_table_id_seq'::regclass)",
		"Primary key identifier",
	).AddRow(
		"name",
		"character varying",
		"YES",
		"",
		"User name field",
	).AddRow(
		"email",
		"character varying",
		"YES",
		"",
		"", // Empty comment
	)

	mock.ExpectQuery("SELECT c.column_name, c.data_type, c.is_nullable, c.column_default, COALESCE(pd.description, '') as comment FROM information_schema.columns c LEFT JOIN pg_class pc ON pc.relname = c.table_name AND pc.relnamespace = (SELECT oid FROM pg_namespace WHERE nspname = c.table_schema) LEFT JOIN pg_namespace pn ON pn.nspname = c.table_schema AND pn.oid = pc.relnamespace LEFT JOIN pg_description pd ON pd.objoid = pc.oid AND pd.objsubid = c.ordinal_position WHERE c.table_catalog = $1 AND c.table_schema = $2 AND c.table_name = $3 ORDER by c.ordinal_position").
		WithArgs(DBNamePostgres, schemaPostgres, tableNamePostgres).
		WillReturnRows(rows)

	columns, err := pg.GetTableColumns(DBNamePostgres, schemaAndTablePostgres)
	if err != nil {
		t.Fatalf("GetTableColumns failed: %v", err)
	}

	expected := [][]string{
		{"column_name", "data_type", "is_nullable", "column_default", "comment"},
		{"id", "integer", "NO", "nextval('test_table_id_seq'::regclass)", "Primary key identifier"},
		{"name", "character varying", "YES", "", "User name field"},
		{"email", "character varying", "YES", "", ""},
	}

	if !reflect.DeepEqual(columns, expected) {
		t.Fatalf("Expected %v, got %v", expected, columns)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

func TestPostgres_GetTableColumns_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock: %v", err)
	}
	defer db.Close()

	pg := &Postgres{Connection: db, CurrentDatabase: DBNamePostgres}
	mock.ExpectQuery("SELECT c.column_name, c.data_type, c.is_nullable, c.column_default, COALESCE\\(pd.description, ''\\) as comment FROM information_schema.columns c LEFT JOIN pg_class pc ON pc.relname = c.table_name AND pc.relnamespace = \\(SELECT oid FROM pg_namespace WHERE nspname = c.table_schema\\) LEFT JOIN pg_namespace pn ON pn.nspname = c.table_schema AND pn.oid = pc.relnamespace LEFT JOIN pg_description pd ON pd.objoid = pc.oid AND pd.objsubid = c.ordinal_position WHERE c.table_catalog = \\$1 AND c.table_schema = \\$2 AND c.table_name = \\$3 ORDER by c.ordinal_position").WithArgs(DBNamePostgres, schemaPostgres, tableNamePostgres).
		WillReturnError(errors.New("query error"))

	_, err = pg.GetTableColumns(DBNamePostgres, schemaAndTablePostgres)
	if err == nil {
		t.Fatal("Expected error but got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

func TestPostgres_GetRecords(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock: %v", err)
	}
	defer db.Close()

	pg := &Postgres{Connection: db, CurrentDatabase: DBNamePostgres}

	columns := []string{"id", "name"}
	rows := sqlmock.NewRows(columns).
		AddRow(1, "Alice").
		AddRow(2, "Bob")

	mock.ExpectQuery(fmt.Sprintf(`SELECT \* FROM "%s"."%s" LIMIT \$1 OFFSET \$2`, schemaPostgres, tableNamePostgres)).WithArgs(DefaultRowLimit, 0).WillReturnRows(rows)

	mock.ExpectQuery(fmt.Sprintf(`SELECT COUNT\(\*\) FROM "%s"."%s"`, schemaPostgres, tableNamePostgres)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	records, total, _, err := pg.GetRecords(DBNamePostgres, schemaAndTablePostgres, "", "", 0, DefaultRowLimit)
	if err != nil {
		t.Fatalf("GetRecords failed: %v", err)
	}

	expected := [][]string{
		{"id", "name"},
		{"1", "Alice"},
		{"2", "Bob"},
	}

	if !reflect.DeepEqual(records, expected) {
		t.Fatalf("Expected %v, got %v", expected, records)
	}

	if total != 2 {
		t.Fatalf("Expected total 2, got %d", total)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

func TestPostgres_GetForeignKeys(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("Error creating mock: %v", err)
	}
	defer db.Close()

	pg := &Postgres{Connection: db, CurrentDatabase: DBNamePostgres}

	rows := sqlmock.NewRows([]string{
		"constraint_name", "column_name",
		"foreign_table_schema", "foreign_table_name", "foreign_column_name",
		"update_rule", "delete_rule",
	}).AddRow(
		"fk_test", "user_id",
		"public", "users", "id",
		"CASCADE", "SET NULL",
	)

	mock.ExpectQuery(fmt.Sprintf(`
        SELECT
            con.conname AS constraint_name,
            src_att.attname AS column_name,
            ref_ns.nspname AS foreign_table_schema,
            ref_cls.relname AS foreign_table_name,
            ref_att.attname AS foreign_column_name
        FROM pg_constraint con
        JOIN pg_class src_cls ON src_cls.oid = con.conrelid
        JOIN pg_namespace src_ns ON src_ns.oid = src_cls.relnamespace
        JOIN pg_class ref_cls ON ref_cls.oid = con.confrelid
        JOIN pg_namespace ref_ns ON ref_ns.oid = ref_cls.relnamespace
        JOIN LATERAL unnest(con.conkey, con.confkey) AS fk(src_attnum, ref_attnum) ON true
        JOIN pg_attribute src_att ON src_att.attrelid = con.conrelid AND src_att.attnum = fk.src_attnum
        JOIN pg_attribute ref_att ON ref_att.attrelid = con.confrelid AND ref_att.attnum = fk.ref_attnum
        WHERE con.contype = 'f'
          AND src_ns.nspname = '%s'
          AND src_cls.relname = '%s'
        ORDER BY con.conname, src_att.attnum
  `, schemaPostgres, tableNamePostgres)).WillReturnRows(rows)

	constraints, err := pg.GetForeignKeys(DBNamePostgres, schemaAndTablePostgres)
	if err != nil {
		t.Fatalf("GetForeignKeys failed: %v", err)
	}

	expected := [][]string{
		{"constraint_name", "column_name", "foreign_table_schema", "foreign_table_name", "foreign_column_name", "update_rule", "delete_rule"},
		{"fk_test", "user_id", "public", "users", "id", "CASCADE", "SET NULL"},
	}

	if !reflect.DeepEqual(constraints, expected) {
		t.Fatalf("Expected %v, got %v", expected, constraints)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

func TestPostgres_GetIndexes(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock: %v", err)
	}
	defer db.Close()

	pg := &Postgres{Connection: db, CurrentDatabase: DBNamePostgres}

	rows := sqlmock.NewRows([]string{"indexname", "indexdef"}).
		AddRow("idx_name", "CREATE INDEX idx_name ON test_table USING btree (name)")

	mock.ExpectQuery(fmt.Sprintf(`
        SELECT
            i.relname AS index_name,
            a.attname AS column_name,
            am.amname AS type
        FROM
            pg_namespace n,
            pg_class t,
            pg_class i,
            pg_index ix,
            pg_attribute a,
            pg_am am
        WHERE
            t.oid = ix.indrelid
            and i.oid = ix.indexrelid
            and a.attrelid = t.oid
            and a.attnum = ANY\(ix.indkey\)
            and t.relkind = 'r'
            and am.oid = i.relam
          	and n.oid = t.relnamespace
            and n.nspname = '%s'
            and t.relname = '%s'
        ORDER BY
            t.relname,
            i.relname
  `, schemaPostgres, tableNamePostgres)).WillReturnRows(rows)

	indexes, err := pg.GetIndexes(DBNamePostgres, schemaAndTablePostgres)
	if err != nil {
		t.Fatalf("GetIndexes failed: %v", err)
	}

	expected := [][]string{
		{"indexname", "indexdef"},
		{"idx_name", "CREATE INDEX idx_name ON test_table USING btree (name)"},
	}

	if !reflect.DeepEqual(indexes, expected) {
		t.Fatalf("Expected %v, got %v", expected, indexes)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

func TestPostgres_Transactions(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock: %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectRollback()
	mock.ExpectBegin()
	mock.ExpectCommit()

	// Test Begin and Rollback
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}

	// Test Begin and Commit
	tx, err = db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

func TestPostgres_GetPrimaryKeyColumnNames(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock: %v", err)
	}
	defer db.Close()

	pg := &Postgres{Connection: db, CurrentDatabase: DBNamePostgres}

	rows := sqlmock.NewRows([]string{"column_name"}).
		AddRow("id")

	mock.ExpectQuery(`
		SELECT
			a.attname AS column_name
		FROM
			pg_index i
			JOIN pg_class c ON c.oid = i.indrelid
			JOIN pg_attribute a ON a.attrelid = c.oid
				AND a.attnum = ANY \(i.indkey\)
			JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE
			relname = \$2 AND nspname = \$1 AND indisprimary
	`).WithArgs(schemaPostgres, tableNamePostgres).WillReturnRows(rows)

	keys, err := pg.GetPrimaryKeyColumnNames(DBNamePostgres, schemaAndTablePostgres)
	if err != nil {
		t.Fatalf("GetPrimaryKeyColumnNames failed: %v", err)
	}

	expected := []string{"id"}
	if !reflect.DeepEqual(keys, expected) {
		t.Fatalf("Expected %v, got %v", expected, keys)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

func TestPostgres_SetGetProvider(t *testing.T) {
	db := &Postgres{}
	db.SetProvider(DriverPostgres)

	if db.GetProvider() != DriverPostgres {
		t.Fatalf("Provider mismatch: got %s, expected %s", db.GetProvider(), DriverPostgres)
	}
}

func TestPostgres_formatTableName(t *testing.T) {
	db := &Postgres{}

	splitTableString := strings.Split(schemaAndTablePostgres, ".")

	tableSchema := splitTableString[0]
	name := splitTableString[1]

	tableName, err := db.formatTableName(schemaAndTablePostgres)
	if err != nil {
		t.Fatalf("formatTableName failed: %v", err)
	}

	expected := fmt.Sprintf("\"%s\".\"%s\"", tableSchema, name)

	if tableName != expected {
		t.Fatalf("formatTableName failed: got %s, expected %s", tableName, expected)
	}
}
