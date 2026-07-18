package drivers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"reflect"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	_ "github.com/go-sql-driver/mysql"
)

const (
	testDBNameMySQL      = "test_db"
	testDBTableNameMySQL = "test_table"
)

func TestMySQL_FormatArg_SpecialCharacters(t *testing.T) {
	db := &MySQL{}

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

func TestMySQL_FormatArgForQueryString(t *testing.T) {
	db := &MySQL{}

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
			expected: "'byte array'",
		},
		{
			name:     "Float argument",
			arg:      123.45,
			expected: "123.45",
		},
		{
			name:     "Integer-looking float",
			arg:      5.0,
			expected: "5.0",
		},
		{
			name:     "Simple decimal",
			arg:      2.5,
			expected: "2.5",
		},
		{
			name:     "Float with trailing zeros",
			arg:      3.0000,
			expected: "3.0",
		},
		{
			name:     "Float with multiple decimal places",
			arg:      123.456789,
			expected: "123.456789",
		},
		{
			name:     "Float with zero value",
			arg:      0.0,
			expected: "0.0",
		},
		{
			name:     "Float with mixed decimal places",
			arg:      98.6000,
			expected: "98.6",
		},
		{
			name:     "Float32 value",
			arg:      float32(3.5),
			expected: "3.5",
		},
		{
			name:     "Float with small decimal",
			arg:      0.00001,
			expected: "0.00001",
		},
		{
			name:     "Float with no decimal part",
			arg:      100.0,
			expected: "100.0",
		},
		{
			name:     "Float with exact precision",
			arg:      2.50000,
			expected: "2.5",
		},
		{
			name:     "Default argument",
			arg:      true,
			expected: "true",
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

func TestMySQL_Connect_Mock(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	mysql := &MySQL{Connection: db}

	mock.ExpectPing()

	err = mysql.Connection.Ping()
	if err != nil {
		t.Fatalf("Ping failed: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestMySQL_ErrorScenarios(t *testing.T) {
	testCases := []struct {
		name      string
		setupMock func(mock sqlmock.Sqlmock)
		testFunc  func(db *MySQL) error
	}{
		{
			name: "GetDatabases error",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SHOW DATABASES").WillReturnError(errors.New("query error"))
			},
			testFunc: func(db *MySQL) error {
				_, err := db.GetDatabases()
				return err
			},
		},
		{
			name: "GetTables error",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(fmt.Sprintf("SHOW FULL TABLES FROM `%s`", testDBNameMySQL)).WillReturnError(errors.New("query error"))
			},
			testFunc: func(db *MySQL) error {
				_, err := db.GetTables("test_db")
				return err
			},
		},
		{
			name: "Empty database name",
			setupMock: func(_ sqlmock.Sqlmock) {
				// No expectations needed for this case
			},
			testFunc: func(db *MySQL) error {
				_, err := db.GetTables("")
				return err
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer db.Close()

			tc.setupMock(mock)

			mysql := &MySQL{Connection: db}

			err = tc.testFunc(mysql)
			if err == nil {
				t.Error("Expected error, but got nil")
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("there were unfulfilled expectations: %s", err)
			}
		})
	}
}

func TestMySQL_GetTableColumns_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	mysql := &MySQL{Connection: db}

	mock.ExpectQuery(fmt.Sprintf("SHOW FULL COLUMNS FROM %s", mysql.formatTableName(testDBNameMySQL, testDBTableNameMySQL))).WillReturnError(errors.New("query error"))

	_, err = mysql.GetTableColumns(testDBNameMySQL, testDBTableNameMySQL)

	if err == nil {
		t.Fatalf("Expected error, but got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestMySQL_GetConstraints_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}

	defer db.Close()

	mysql := &MySQL{Connection: db}

	mock.ExpectQuery("SELECT CONSTRAINT_NAME, COLUMN_NAME, REFERENCED_TABLE_NAME, REFERENCED_COLUMN_NAME FROM information_schema.KEY_COLUMN_USAGE WHERE TABLE_SCHEMA = \\? AND TABLE_NAME = \\?").WithArgs(testDBNameMySQL, testDBTableNameMySQL).WillReturnError(errors.New("query error"))

	_, err = mysql.GetConstraints(testDBNameMySQL, testDBTableNameMySQL)

	log.Println("errrorrrrrr", err.Error())

	if err == nil {
		t.Fatalf("Expected error, but got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestMySQL_GetForeignKeys_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	mysql := &MySQL{Connection: db}

	mock.ExpectQuery("SELECT TABLE_NAME, COLUMN_NAME, CONSTRAINT_NAME, REFERENCED_COLUMN_NAME, REFERENCED_TABLE_NAME FROM information_schema.KEY_COLUMN_USAGE WHERE REFERENCED_TABLE_SCHEMA = \\? AND REFERENCED_TABLE_NAME = \\?").WithArgs(testDBNameMySQL, testDBTableNameMySQL).WillReturnError(errors.New("query error"))

	_, err = mysql.GetForeignKeys(testDBNameMySQL, testDBTableNameMySQL)

	if err == nil {
		t.Fatalf("Expected error, but got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestMySQL_GetIndexes_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	mysql := &MySQL{Connection: db}

	mock.ExpectQuery(fmt.Sprintf("SHOW INDEX FROM %s", mysql.formatTableName(testDBNameMySQL, testDBTableNameMySQL))).WillReturnError(errors.New("query error"))

	_, err = mysql.GetIndexes(testDBNameMySQL, testDBTableNameMySQL)

	if err == nil {
		t.Fatalf("Expected error, but got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestMySQL_GetRecords_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	mysql := &MySQL{Connection: db}

	testCases := []struct {
		name      string
		setupMock func(mock sqlmock.Sqlmock)
		testFunc  func(db *MySQL) error
	}{
		{
			name: "GetRecords error",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(fmt.Sprintf("SELECT \\* FROM %s LIMIT \\?, \\?", mysql.formatTableName(testDBNameMySQL, testDBTableNameMySQL))).WithArgs(0, DefaultRowLimit).WillReturnError(errors.New("query error"))
			},
			testFunc: func(db *MySQL) error {
				_, _, _, err := db.GetRecords(context.Background(), "test_db", "test_table", "", "", 0, DefaultRowLimit)
				return err
			},
		},
		{
			name: "GetRecords with where error",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(fmt.Sprintf("SELECT \\* FROM %s WHERE id = 1 LIMIT \\?, \\?", mysql.formatTableName(testDBNameMySQL, testDBTableNameMySQL))).WithArgs(0, DefaultRowLimit).WillReturnError(errors.New("query error"))
			},
			testFunc: func(db *MySQL) error {
				_, _, _, err := db.GetRecords(context.Background(), "test_db", "test_table", "WHERE id = 1", "", 0, DefaultRowLimit)
				return err
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setupMock(mock)
			err = tc.testFunc(mysql)
			if err == nil {
				t.Error("Expected error, but got nil")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("there were unfulfilled expectations: %s", err)
			}
		})
	}
}

func TestMySQL_ExecuteQuery_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	mysql := &MySQL{Connection: db}

	mock.ExpectQuery(fmt.Sprintf("SELECT \\* FROM %s", mysql.formatTableName(testDBNameMySQL, testDBTableNameMySQL))).WillReturnError(errors.New("query error"))

	_, _, err = mysql.ExecuteQuery(context.Background(), fmt.Sprintf("SELECT * FROM %s", mysql.formatTableName(testDBNameMySQL, testDBTableNameMySQL)), 0)

	if err == nil {
		t.Fatalf("Expected error, but got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestMySQL_ExecuteDMLStatement_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	mysql := &MySQL{Connection: db}

	mock.ExpectExec("UPDATE test_table SET value = 3 WHERE name = 'test1'").WillReturnError(errors.New("query error"))

	_, err = mysql.ExecuteDMLStatement(context.Background(), fmt.Sprintf("UPDATE %s SET value = 3 WHERE name = 'test1'", testDBTableNameMySQL))

	if err == nil {
		t.Fatalf("Expected error, but got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestMySQL_ExecuteQuery_PreCancelledContext(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error creating mock: %s", err)
	}
	defer db.Close()

	mysql := &MySQL{Connection: db}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, _, err := mysql.ExecuteQuery(ctx, "SELECT 1", 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if _, err := mysql.ExecuteDMLStatement(ctx, "UPDATE t SET x = 1"); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestMySQL_GetPrimaryKeyColumnNames_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	mysql := &MySQL{Connection: db}

	mock.ExpectQuery("SELECT column_name FROM information_schema.key_column_usage WHERE table_schema = \\? AND table_name = \\? AND constraint_name = \\?").WithArgs(testDBNameMySQL, testDBTableNameMySQL, "PRIMARY").WillReturnError(errors.New("query error"))

	_, err = mysql.GetPrimaryKeyColumnNames(testDBNameMySQL, testDBTableNameMySQL)

	log.Println(err.Error())

	if err == nil {
		t.Fatalf("Expected error, but got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestMySQL_TestConnection(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error creating mock database: %v", err)
	}
	defer db.Close()

	mysql := &MySQL{Connection: db}

	mock.ExpectPing()

	err = mysql.Connection.Ping()
	if err != nil {
		t.Fatalf("Connection test failed: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestMySQL_Transactions(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected", err)
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
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

// func TestMySQL_ConnectionPooling(t *testing.T) {
// 	db, mock, err := sqlmock.New()
// 	if err != nil {
// 		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
// 	}
// 	defer db.Close()
//
// 	mock.ExpectQuery("PRAGMA index_list\\(test_table\\)").WillReturnError(errors.New("query error"))
//
// 	mysql := &MySQL{Connection: db}
//
// 	// Test multiple connections
// 	err = mysql.Connect(testDBNameMySQL)
// 	if err != nil {
// 		t.Fatalf("Failed to connect: %v", err)
// 	}
// 	defer cleanupMySQLTestDB(t, mysql)
//
// 	// Verify connection is reusable
// 	for range 3 {
// 		_, err := mysql.GetDatabases()
// 		if err != nil {
// 			t.Fatalf("Failed to use connection: %v", err)
// 		}
// 	}
// }

// func TestMySQL_Connect(t *testing.T) {
// 	db, mock, err := sqlmock.New()
// 	if err != nil {
// 		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
// 	}
// 	defer db.Close()
//
// 	mock.ExpectQuery("PRAGMA index_list\\(test_table\\)").WillReturnError(errors.New("query error"))
//
// 	mysql := &MySQL{Connection: db}
//
// 	err = mysql.Connect(testDBNameMySQL)
// 	if err != nil {
// 		t.Fatalf("Connect failed: %v", err)
// 	}
//
// 	defer cleanupMySQLTestDB(t, mysql)
// }

func TestMySQL_GetDatabases(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected", err)
	}
	defer db.Close()

	mysql := &MySQL{Connection: db}

	rows := sqlmock.NewRows([]string{"Database"}).
		AddRow(testDBNameMySQL)

	mock.ExpectQuery("SHOW DATABASES").WillReturnRows(rows)

	databases, err := mysql.GetDatabases()
	if err != nil {
		t.Fatalf("GetDatabases failed: %v", err)
	}

	expected := []string{"test_db"}
	if !reflect.DeepEqual(databases, expected) {
		t.Fatalf("Expected %v, got %v", expected, databases)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestMySQL_GetTables(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error creating mock: %s", err)
	}
	defer db.Close()

	mysql := &MySQL{Connection: db}

	// Set up mock expectations
	rows := sqlmock.NewRows([]string{"Tables_in_test_db", "Table_type"}).
		AddRow("test_table", "BASE TABLE").
		AddRow("another_table", "BASE TABLE")

	mock.ExpectQuery("SHOW FULL TABLES FROM `test_db` WHERE Table_type = 'BASE TABLE'").WillReturnRows(rows)

	tables, err := mysql.GetTables("test_db")
	if err != nil {
		t.Fatalf("GetTables failed: %v", err)
	}

	expected := map[string][]string{
		"test_db": {"test_table", "another_table"},
	}

	if !reflect.DeepEqual(tables, expected) {
		t.Fatalf("Expected %v, got %v", expected, tables)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestMySQL_GetViews(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error creating mock: %s", err)
	}
	defer db.Close()

	mysql := &MySQL{Connection: db}

	rows := sqlmock.NewRows([]string{"Tables_in_test_db", "Table_type"}).
		AddRow("v_users", "VIEW").
		AddRow("v_orders", "VIEW")

	mock.ExpectQuery("SHOW FULL TABLES FROM `test_db` WHERE Table_type = 'VIEW'").WillReturnRows(rows)

	views, err := mysql.GetViews("test_db")
	if err != nil {
		t.Fatalf("GetViews failed: %v", err)
	}

	expected := map[string][]string{
		"test_db": {"v_users", "v_orders"},
	}

	if !reflect.DeepEqual(views, expected) {
		t.Fatalf("Expected %v, got %v", expected, views)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestMySQL_GetViews_EmptyDatabase(t *testing.T) {
	mysql := &MySQL{}
	if _, err := mysql.GetViews(""); err == nil {
		t.Fatal("expected error for empty database name")
	}
}

func TestMySQL_GetViewDefinition(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error creating mock: %s", err)
	}
	defer db.Close()

	mysql := &MySQL{Connection: db}

	createStmt := "CREATE ALGORITHM=UNDEFINED DEFINER=`root`@`%` SQL SECURITY DEFINER VIEW `v_users` AS select `id`,`name` from `users`"
	rows := sqlmock.NewRows([]string{"View", "Create View", "character_set_client", "collation_connection"}).
		AddRow("v_users", createStmt, "utf8mb4", "utf8mb4_unicode_ci")

	mock.ExpectQuery("SHOW CREATE VIEW `test_db`.`v_users`").WillReturnRows(rows)

	def, err := mysql.GetViewDefinition("test_db", "v_users")
	if err != nil {
		t.Fatalf("GetViewDefinition failed: %v", err)
	}

	if def != createStmt {
		t.Fatalf("Expected %q, got %q", createStmt, def)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestMySQL_GetViewDefinition_NoRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error creating mock: %s", err)
	}
	defer db.Close()

	mysql := &MySQL{Connection: db}

	mock.ExpectQuery("SHOW CREATE VIEW `test_db`.`missing`").
		WillReturnRows(sqlmock.NewRows([]string{"View", "Create View", "character_set_client", "collation_connection"}))

	if _, err := mysql.GetViewDefinition("test_db", "missing"); err == nil {
		t.Fatal("expected error for missing view")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestMySQL_GetTableDDL(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error creating mock: %s", err)
	}
	defer db.Close()

	mysql := &MySQL{Connection: db}

	createStmt := "CREATE TABLE `test_table` (\n  `id` int NOT NULL AUTO_INCREMENT,\n  `name` varchar(255) DEFAULT NULL,\n  PRIMARY KEY (`id`)\n) ENGINE=InnoDB"
	rows := sqlmock.NewRows([]string{"Table", "Create Table"}).
		AddRow("test_table", createStmt)

	mock.ExpectQuery("SHOW CREATE TABLE `test_db`.`test_table`").WillReturnRows(rows)

	ddl, err := mysql.GetTableDDL(testDBNameMySQL, testDBTableNameMySQL)
	if err != nil {
		t.Fatalf("GetTableDDL failed: %v", err)
	}

	if ddl != createStmt {
		t.Fatalf("Expected %q, got %q", createStmt, ddl)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestMySQL_GetTableDDL_EmptyArgs(t *testing.T) {
	mysql := &MySQL{}
	if _, err := mysql.GetTableDDL("", "t"); err == nil {
		t.Fatal("expected error for empty database name")
	}
	if _, err := mysql.GetTableDDL("db", ""); err == nil {
		t.Fatal("expected error for empty table name")
	}
}

func TestMySQL_GetTableColumns(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error creating mock: %s", err)
	}
	defer db.Close()

	mysql := &MySQL{Connection: db}

	// Set up mock expectations for SHOW FULL COLUMNS FROM
	rows := sqlmock.NewRows([]string{"Field", "Type", "Collation", "Null", "Key", "Default", "Extra", "Privileges", "Comment"}).
		AddRow("id", "int(11)", nil, "NO", "PRI", nil, "auto_increment", "select,insert,update,references", "").
		AddRow("name", "varchar(255)", "utf8mb4_unicode_ci", "YES", "", nil, "", "select,insert,update,references", "User name")

	mock.ExpectQuery(fmt.Sprintf("SHOW FULL COLUMNS FROM %s", mysql.formatTableName(testDBNameMySQL, testDBTableNameMySQL))).WillReturnRows(rows)

	columns, err := mysql.GetTableColumns(testDBNameMySQL, testDBTableNameMySQL)
	if err != nil {
		t.Fatalf("GetTableColumns failed: %v", err)
	}

	expected := [][]string{
		{"Field", "Type", "Collation", "Null", "Key", "Default", "Extra", "Privileges", "Comment"},
		{"id", "int(11)", "", "NO", "PRI", "", "auto_increment", "select,insert,update,references", ""},
		{"name", "varchar(255)", "utf8mb4_unicode_ci", "YES", "", "", "", "select,insert,update,references", "User name"},
	}

	if !reflect.DeepEqual(columns, expected) {
		t.Fatalf("Expected %v, got %v", expected, columns)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestMySQL_GetConstraints(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error creating mock: %s", err)
	}
	defer db.Close()

	mysql := &MySQL{Connection: db}

	// Set up mock expectations
	rows := sqlmock.NewRows([]string{"CONSTRAINT_NAME", "COLUMN_NAME", "REFERENCED_TABLE_NAME", "REFERENCED_COLUMN_NAME"}).
		AddRow("fk_test", "user_id", "users", "id")

	mock.ExpectQuery("SELECT CONSTRAINT_NAME, COLUMN_NAME, REFERENCED_TABLE_NAME, REFERENCED_COLUMN_NAME FROM information_schema.KEY_COLUMN_USAGE WHERE TABLE_SCHEMA = \\? AND TABLE_NAME = \\?").
		WithArgs(testDBNameMySQL, testDBTableNameMySQL).
		WillReturnRows(rows)

	constraints, err := mysql.GetConstraints(testDBNameMySQL, testDBTableNameMySQL)
	if err != nil {
		t.Fatalf("GetConstraints failed: %v", err)
	}

	expected := [][]string{
		{"CONSTRAINT_NAME", "COLUMN_NAME", "REFERENCED_TABLE_NAME", "REFERENCED_COLUMN_NAME"},
		{"fk_test", "user_id", "users", "id"},
	}

	if !reflect.DeepEqual(constraints, expected) {
		t.Fatalf("Expected %v, got %v", expected, constraints)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestMySQL_GetForeignKeys(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error creating mock: %s", err)
	}
	defer db.Close()

	mysql := &MySQL{Connection: db}

	// Set up mock expectations
	rows := sqlmock.NewRows([]string{
		"TABLE_NAME",
		"COLUMN_NAME",
		"CONSTRAINT_NAME",
		"REFERENCED_COLUMN_NAME",
		"REFERENCED_TABLE_NAME",
	}).AddRow("orders", "user_id", "fk_user", "id", "users")

	mock.ExpectQuery(
		"SELECT TABLE_NAME, COLUMN_NAME, CONSTRAINT_NAME, REFERENCED_COLUMN_NAME, REFERENCED_TABLE_NAME "+
			"FROM information_schema.KEY_COLUMN_USAGE "+
			"WHERE REFERENCED_TABLE_SCHEMA = \\? AND REFERENCED_TABLE_NAME = \\?").
		WithArgs(testDBNameMySQL, testDBTableNameMySQL).
		WillReturnRows(rows)

	foreignKeys, err := mysql.GetForeignKeys(testDBNameMySQL, testDBTableNameMySQL)
	if err != nil {
		t.Fatalf("GetForeignKeys failed: %v", err)
	}

	expected := [][]string{
		{"TABLE_NAME", "COLUMN_NAME", "CONSTRAINT_NAME", "REFERENCED_COLUMN_NAME", "REFERENCED_TABLE_NAME"},
		{"orders", "user_id", "fk_user", "id", "users"},
	}

	if !reflect.DeepEqual(foreignKeys, expected) {
		t.Fatalf("Expected:\n%v\nGot:\n%v", expected, foreignKeys)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestMySQL_GetIndexes(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error creating mock: %s", err)
	}
	defer db.Close()

	mysql := &MySQL{Connection: db}

	// Set up mock expectations
	rows := sqlmock.NewRows([]string{
		"Seq", "Key_name", "Non_unique", "Index_type", "Column_name",
	}).AddRow(1, "idx_name", 0, "BTREE", "name").
		AddRow(2, "PRIMARY", 0, "BTREE", "id")

	mock.ExpectQuery(fmt.Sprintf("SHOW INDEX FROM %s", mysql.formatTableName(testDBNameMySQL, testDBTableNameMySQL))).
		WillReturnRows(rows)

	indexes, err := mysql.GetIndexes(testDBNameMySQL, testDBTableNameMySQL)
	if err != nil {
		t.Fatalf("GetIndexes failed: %v", err)
	}

	expected := [][]string{
		{"Seq", "Key_name", "Non_unique", "Index_type", "Column_name"},
		{"1", "idx_name", "0", "BTREE", "name"},
		{"2", "PRIMARY", "0", "BTREE", "id"},
	}

	if !reflect.DeepEqual(indexes, expected) {
		t.Fatalf("Expected:\n%v\nGot:\n%v", expected, indexes)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestMySQL_GetRecords(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error creating mock: %s", err)
	}
	defer db.Close()

	mysql := &MySQL{Connection: db}

	// Set up mock expectations
	columns := []string{"id", "name", "value"}
	rows := sqlmock.NewRows(columns).
		AddRow(1, "test1", 100).
		AddRow(2, "test2", 200)

	mock.ExpectQuery(fmt.Sprintf("SELECT \\* FROM %s LIMIT \\?, \\?", mysql.formatTableName(testDBNameMySQL, testDBTableNameMySQL))).
		WithArgs(0, DefaultRowLimit).
		WillReturnRows(rows)

	mock.ExpectQuery(fmt.Sprintf("SELECT COUNT\\(\\*\\) FROM %s", mysql.formatTableName(testDBNameMySQL, testDBTableNameMySQL))).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	records, total, _, err := mysql.GetRecords(context.Background(), testDBNameMySQL, testDBTableNameMySQL, "", "", 0, DefaultRowLimit)
	if err != nil {
		t.Fatalf("GetRecords failed: %v", err)
	}

	if total != 2 {
		t.Fatalf("Expected total 2, got %d", total)
	}

	expectedRecords := [][]string{
		{"id", "name", "value"},
		{"1", "test1", "100"},
		{"2", "test2", "200"},
	}

	if !reflect.DeepEqual(records, expectedRecords) {
		t.Fatalf("Expected %v, got %v", expectedRecords, records)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestMySQL_ExecuteQuery(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error creating mock: %s", err)
	}
	defer db.Close()

	mysql := &MySQL{Connection: db}

	// Set up mock expectations
	columns := []string{"id", "name"}
	rows := sqlmock.NewRows(columns).
		AddRow(1, "Alice").
		AddRow(2, "Bob")

	mock.ExpectQuery(fmt.Sprintf("SELECT \\* FROM %s", mysql.formatTableName(testDBNameMySQL, testDBTableNameMySQL))).
		WillReturnRows(rows)

	results, _, err := mysql.ExecuteQuery(context.Background(), fmt.Sprintf("SELECT * FROM %s", mysql.formatTableName(testDBNameMySQL, testDBTableNameMySQL)), 0)
	if err != nil {
		t.Fatalf("ExecuteQuery failed: %v", err)
	}

	expectedResults := [][]string{
		{"id", "name"},
		{"1", "Alice"},
		{"2", "Bob"},
	}

	if !reflect.DeepEqual(results, expectedResults) {
		t.Fatalf("Expected results:\n%v\nGot:\n%v", expectedResults, results)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestMySQL_ExecuteDMLStatement(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error creating mock: %s", err)
	}
	defer db.Close()

	mysql := &MySQL{Connection: db}

	// Set up mock expectations
	mock.ExpectExec(fmt.Sprintf("UPDATE %s SET value = 3 WHERE name = 'test1'", mysql.formatTableName(testDBNameMySQL, testDBTableNameMySQL))).
		WillReturnResult(sqlmock.NewResult(0, 2))

	result, err := mysql.ExecuteDMLStatement(context.Background(), fmt.Sprintf("UPDATE %s SET value = 3 WHERE name = 'test1'", mysql.formatTableName(testDBNameMySQL, testDBTableNameMySQL)))
	if err != nil {
		t.Fatalf("ExecuteDMLStatement failed: %v", err)
	}

	expected := "2 rows affected"
	if result != expected {
		t.Fatalf("Expected %q, got %q", expected, result)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestMySQL_GetPrimaryKeyColumnNames(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error creating mock: %s", err)
	}
	defer db.Close()

	mysql := &MySQL{Connection: db}

	rows := sqlmock.NewRows([]string{"column_name"}).
		AddRow("id").
		AddRow("uuid")

	mock.ExpectQuery("SELECT column_name FROM information_schema.key_column_usage WHERE table_schema = \\? AND table_name = \\? AND constraint_name = \\?").
		WithArgs(testDBNameMySQL, testDBTableNameMySQL, "PRIMARY").
		WillReturnRows(rows)

	keys, err := mysql.GetPrimaryKeyColumnNames(testDBNameMySQL, testDBTableNameMySQL)
	if err != nil {
		t.Fatalf("GetPrimaryKeyColumnNames failed: %v", err)
	}

	expected := []string{"id", "uuid"}
	if !reflect.DeepEqual(keys, expected) {
		t.Fatalf("Expected %v, got %v", expected, keys)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestMySQL_SetProvider(t *testing.T) {
	db := &MySQL{}
	db.SetProvider(DriverMySQL)

	if db.Provider != DriverMySQL {
		t.Fatalf("SetProvider failed: got %q, expected %q", db.Provider, DriverMySQL)
	}
}

func TestMySQL_GetProvider(t *testing.T) {
	db := &MySQL{Provider: DriverMySQL}

	provider := db.GetProvider()
	if provider != DriverMySQL {
		t.Fatalf("GetProvider failed: got %q, expected %q", provider, DriverMySQL)
	}
}

func TestMySQL_formatTableName(t *testing.T) {
	db := &MySQL{}

	tableName := db.formatTableName(testDBNameMySQL, testDBTableNameMySQL)
	expectedTableName := fmt.Sprintf("`%s`.`%s`", testDBNameMySQL, testDBTableNameMySQL)

	if tableName != expectedTableName {
		t.Fatalf("formatTableName failed: got %q, expected %q", tableName, expectedTableName)
	}
}
