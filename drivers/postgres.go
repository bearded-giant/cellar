package drivers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	// import postgresql driver
	_ "github.com/lib/pq"
	"github.com/xo/dburl"

	"github.com/bearded-giant/cellar/helpers/logger"
	"github.com/bearded-giant/cellar/models"
)

type Postgres struct {
	Connection       *sql.DB
	Provider         string
	CurrentDatabase  string
	PreviousDatabase string
	Urlstr           string
}

const (
	defaultPort = "5432"
)

func (db *Postgres) TestConnection(urlstr string) error {
	return db.Connect(urlstr)
}

func (db *Postgres) Connect(urlstr string) error {
	db.SetProvider(DriverPostgres)

	connection, err := dburl.Open(urlstr)
	if err != nil {
		return err
	}

	db.Connection = connection

	err = db.Connection.Ping()
	if err != nil {
		return err
	}

	db.Urlstr = urlstr

	// Get the current database.
	rows := db.Connection.QueryRow("SELECT current_database();")

	database := ""
	err = rows.Scan(&database)
	if err != nil {
		return err
	}

	db.CurrentDatabase = database
	db.PreviousDatabase = database

	return nil
}

func (db *Postgres) GetDatabases() ([]string, error) {
	rows, err := db.Connection.Query("SELECT datname FROM pg_database WHERE datallowconn AND has_database_privilege(current_user, datname, 'CONNECT');")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var databases []string
	for rows.Next() {
		var database string
		err := rows.Scan(&database)
		if err != nil {
			return nil, err
		}
		databases = append(databases, database)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return databases, nil
}

func (db *Postgres) GetTables(database string) (map[string][]string, error) {
	if database == "" {
		return nil, errors.New("database name is required")
	}

	conn, needsClose, err := db.connectionFor(database)
	if err != nil {
		return nil, err
	}
	if needsClose {
		defer conn.Close()
	}

	query := "SELECT table_name, table_schema FROM information_schema.tables WHERE table_catalog = $1 AND table_type = 'BASE TABLE'"
	rows, err := conn.Query(query, database)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tables := make(map[string][]string)
	for rows.Next() {
		var (
			tableName   string
			tableSchema string
		)
		if err := rows.Scan(&tableName, &tableSchema); err != nil {
			return nil, err
		}

		tables[tableSchema] = append(tables[tableSchema], tableName)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tables, nil
}

func (db *Postgres) GetTableColumns(database, table string) ([][]string, error) {
	if database == "" {
		return nil, errors.New("database name is required")
	}
	if table == "" {
		return nil, errors.New("table name is required")
	}

	splitTableString := strings.Split(table, ".")

	if len(splitTableString) == 1 {
		return nil, errors.New("table must be in the format schema.table")
	}

	conn, needsClose, err := db.connectionFor(database)
	if err != nil {
		return nil, err
	}
	if needsClose {
		defer conn.Close()
	}

	tableSchema := splitTableString[0]
	tableName := splitTableString[1]

	query := "SELECT c.column_name, c.data_type, c.is_nullable, c.column_default, COALESCE(pd.description, '') as comment FROM information_schema.columns c LEFT JOIN pg_class pc ON pc.relname = c.table_name AND pc.relnamespace = (SELECT oid FROM pg_namespace WHERE nspname = c.table_schema) LEFT JOIN pg_namespace pn ON pn.nspname = c.table_schema AND pn.oid = pc.relnamespace LEFT JOIN pg_description pd ON pd.objoid = pc.oid AND pd.objsubid = c.ordinal_position WHERE c.table_catalog = $1 AND c.table_schema = $2 AND c.table_name = $3 ORDER by c.ordinal_position"

	rows, err := conn.Query(query, database, tableSchema, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	results := [][]string{columns}
	for rows.Next() {
		rowValues := make([]any, len(columns))

		for i := range columns {
			rowValues[i] = new(sql.RawBytes)
		}

		if err := rows.Scan(rowValues...); err != nil {
			return nil, err
		}

		var row []string
		for _, col := range rowValues {
			row = append(row, string(*col.(*sql.RawBytes)))
		}

		results = append(results, row)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

func (db *Postgres) GetConstraints(database, table string) ([][]string, error) {
	if database == "" {
		return nil, errors.New("database name is required")
	}
	if table == "" {
		return nil, errors.New("table name is required")
	}

	splitTableString := strings.Split(table, ".")
	if len(splitTableString) == 1 {
		return nil, errors.New("table must be in the format schema.table")
	}

	conn, needsClose, err := db.connectionFor(database)
	if err != nil {
		return nil, err
	}
	if needsClose {
		defer conn.Close()
	}

	tableSchema := splitTableString[0]
	tableName := splitTableString[1]

	rows, err := conn.Query(fmt.Sprintf(`
        SELECT
            tc.constraint_name,
            kcu.column_name,
            tc.constraint_type
        FROM
            information_schema.table_constraints AS tc
            JOIN information_schema.key_column_usage AS kcu ON tc.constraint_name = kcu.constraint_name
            AND tc.table_schema = kcu.table_schema
            JOIN information_schema.constraint_column_usage AS ccu ON ccu.constraint_name = tc.constraint_name
            AND ccu.table_schema = tc.table_schema
        WHERE
            NOT tc.constraint_type = 'FOREIGN KEY'
			AND tc.table_schema = '%s'
            AND tc.table_name = '%s'
            `, tableSchema, tableName))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	constraints := [][]string{columns}
	for rows.Next() {
		rowValues := make([]any, len(columns))
		for i := range columns {
			rowValues[i] = new(sql.RawBytes)
		}

		if err := rows.Scan(rowValues...); err != nil {
			return nil, err
		}

		var row []string
		for _, col := range rowValues {
			row = append(row, string(*col.(*sql.RawBytes)))
		}

		constraints = append(constraints, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return constraints, nil
}

func (db *Postgres) GetForeignKeys(database, table string) ([][]string, error) {
	if database == "" {
		return nil, errors.New("database name is required")
	}
	if table == "" {
		return nil, errors.New("table name is required")
	}

	splitTableString := strings.Split(table, ".")
	if len(splitTableString) == 1 {
		return nil, errors.New("table must be in the format schema.table")
	}

	conn, needsClose, err := db.connectionFor(database)
	if err != nil {
		return nil, err
	}
	if needsClose {
		defer conn.Close()
	}

	tableSchema := splitTableString[0]
	tableName := splitTableString[1]

	rows, err := conn.Query(fmt.Sprintf(`
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
  `, tableSchema, tableName))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	foreignKeys := [][]string{columns}
	for rows.Next() {
		rowValues := make([]any, len(columns))
		for i := range columns {
			rowValues[i] = new(sql.RawBytes)
		}

		if err := rows.Scan(rowValues...); err != nil {
			return nil, err
		}

		var row []string
		for _, col := range rowValues {
			row = append(row, string(*col.(*sql.RawBytes)))
		}

		foreignKeys = append(foreignKeys, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return foreignKeys, nil
}

func (db *Postgres) GetIndexes(database, table string) ([][]string, error) {
	if database == "" {
		return nil, errors.New("database name is required")
	}
	if table == "" {
		return nil, errors.New("table name is required")
	}

	splitTableString := strings.Split(table, ".")
	if len(splitTableString) == 1 {
		return nil, errors.New("table must be in the format schema.table")
	}

	conn, needsClose, err := db.connectionFor(database)
	if err != nil {
		return nil, err
	}
	if needsClose {
		defer conn.Close()
	}

	tableSchema := splitTableString[0]
	tableName := splitTableString[1]

	rows, err := conn.Query(fmt.Sprintf(`
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
            and a.attnum = ANY(ix.indkey)
            and t.relkind = 'r'
            and am.oid = i.relam
          	and n.oid = t.relnamespace
            and n.nspname = '%s'
            and t.relname = '%s'
        ORDER BY
            t.relname,
            i.relname
  `, tableSchema, tableName))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	indexes := [][]string{columns}
	for rows.Next() {
		rowValues := make([]any, len(columns))
		for i := range columns {
			rowValues[i] = new(sql.RawBytes)
		}

		if err := rows.Scan(rowValues...); err != nil {
			return nil, err
		}

		var row []string
		for _, col := range rowValues {
			row = append(row, string(*col.(*sql.RawBytes)))
		}

		indexes = append(indexes, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return indexes, nil
}

func (db *Postgres) GetRecords(ctx context.Context, database, table, where, sort string, offset, limit int) (records [][]string, totalRecords int, queryString string, err error) {
	if database == "" {
		return nil, 0, "", errors.New("database name is required")
	}
	if table == "" {
		return nil, 0, "", errors.New("table name is required")
	}

	formattedTableName, err := db.formatTableName(table)
	if err != nil {
		return nil, 0, "", err
	}

	conn, needsClose, err := db.connectionFor(database)
	if err != nil {
		return nil, 0, "", err
	}
	if needsClose {
		defer conn.Close()
	}

	queryString = "SELECT * FROM "
	queryString += formattedTableName

	if where != "" {
		queryString += fmt.Sprintf(" %s", where)
	}

	if sort != "" {
		queryString += fmt.Sprintf(" ORDER BY %s", sort)
	}

	queryString += " LIMIT $1 OFFSET $2"

	if limit == 0 {
		limit = DefaultRowLimit
	}

	paginatedRows, err := conn.QueryContext(ctx, queryString, limit, offset)
	if err != nil {
		return nil, 0, queryString, err
	}
	defer paginatedRows.Close()

	columns, columnsError := paginatedRows.Columns()
	if columnsError != nil {
		return nil, 0, queryString, columnsError
	}

	records = [][]string{columns}
	for paginatedRows.Next() {
		nullStringSlice := make([]sql.NullString, len(columns))

		rowValues := make([]any, len(columns))
		for i := range nullStringSlice {
			rowValues[i] = &nullStringSlice[i]
		}

		if err := paginatedRows.Scan(rowValues...); err != nil {
			return nil, 0, queryString, err
		}

		var row []string
		for _, col := range nullStringSlice {
			if col.Valid {
				if col.String == "" {
					row = append(row, "EMPTY&")
				} else {
					row = append(row, col.String)
				}
			} else {
				row = append(row, "NULL&")
			}
		}

		records = append(records, row)
	}

	if err := paginatedRows.Err(); err != nil {
		return nil, 0, queryString, err
	}
	// close to release the connection
	if err := paginatedRows.Close(); err != nil {
		return nil, 0, queryString, err
	}

	countQuery := "SELECT COUNT(*) FROM "
	countQuery += formattedTableName

	if where != "" {
		countQuery += fmt.Sprintf(" %s", where)
	}

	countRow := conn.QueryRowContext(ctx, countQuery)

	if err := countRow.Scan(&totalRecords); err != nil {
		return records, 0, queryString, err
	}

	// Replace the limit and offset with actual values in the query string
	queryString = strings.Replace(queryString, "$1", strconv.Itoa(limit), 1)
	queryString = strings.Replace(queryString, "$2", strconv.Itoa(offset), 1)

	return records, totalRecords, queryString, nil
}

func (db *Postgres) ExecuteDMLStatement(ctx context.Context, query string) (result string, err error) {
	res, err := db.Connection.ExecContext(ctx, query)
	if err != nil {
		return result, err
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return result, err
	}
	return fmt.Sprintf("%d rows affected", rowsAffected), nil
}

func (db *Postgres) ExecuteQuery(ctx context.Context, query string) ([][]string, int, error) {
	rows, err := db.Connection.QueryContext(ctx, query)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, 0, err
	}

	records := make([][]string, 0)
	for rows.Next() {
		rowValues := make([]any, len(columns))
		for i := range columns {
			rowValues[i] = new(sql.RawBytes)
		}

		err = rows.Scan(rowValues...)
		if err != nil {
			return nil, 0, err
		}

		var row []string
		for _, col := range rowValues {
			row = append(row, string(*col.(*sql.RawBytes)))
		}

		records = append(records, row)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	// Prepend the columns to the records.
	results := append([][]string{columns}, records...)

	return results, len(records), nil
}

func (db *Postgres) GetPrimaryKeyColumnNames(database, table string) ([]string, error) {
	if database == "" {
		return nil, errors.New("database name is required")
	}
	if table == "" {
		return nil, errors.New("table name is required")
	}

	splitTableString := strings.Split(table, ".")
	if len(splitTableString) != 2 {
		return nil, errors.New("table must be in the format schema.table")
	}

	schemaName := splitTableString[0]
	tableName := splitTableString[1]

	conn, needsClose, err := db.connectionFor(database)
	if err != nil {
		return nil, err
	}
	if needsClose {
		defer conn.Close()
	}

	row, err := conn.Query(`
		SELECT
			a.attname AS column_name
		FROM
			pg_index i
			JOIN pg_class c ON c.oid = i.indrelid
			JOIN pg_attribute a ON a.attrelid = c.oid
				AND a.attnum = ANY (i.indkey)
			JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE
			relname = $2 AND nspname = $1 AND indisprimary
	`, schemaName, tableName)
	if err != nil {
		logger.Error("GetPrimaryKeyColumnNames", map[string]any{"error": err.Error()})
		return nil, err
	}

	defer row.Close()

	var primaryKeyColumnName []string
	for row.Next() {
		var colName string
		err = row.Scan(&colName)
		if err != nil {
			return nil, err
		}

		if row.Err() != nil {
			return nil, row.Err()
		}

		primaryKeyColumnName = append(primaryKeyColumnName, colName)
	}

	if row.Err() != nil {
		return nil, row.Err()
	}

	return primaryKeyColumnName, nil
}

func (db *Postgres) SetProvider(provider string) {
	db.Provider = provider
}

func (db *Postgres) GetProvider() string {
	return db.Provider
}

// connectToDatabase opens a new connection to the given database without
// mutating the receiver. The caller must close the returned connection.
func (db *Postgres) connectToDatabase(database string) (*sql.DB, error) {
	parsedConn, err := dburl.Parse(db.Urlstr)
	if err != nil {
		return nil, err
	}

	user := parsedConn.User.Username()
	password, hasPassword := parsedConn.User.Password()
	host := parsedConn.Hostname()
	port := parsedConn.Port()
	if port == "" {
		port = defaultPort
	}

	dsn := fmt.Sprintf("host=%s port=%s user=%s dbname=%s sslmode=disable", host, port, user, database)
	if hasPassword {
		dsn += fmt.Sprintf(" password=%s", password)
	}

	conn, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

// connectionFor returns a connection to the given database. If it matches
// the current database, the existing connection is returned (caller must NOT
// close it). Otherwise a new temporary connection is opened and returned
// (caller MUST close it).
func (db *Postgres) connectionFor(database string) (conn *sql.DB, needsClose bool, err error) {
	if database == db.CurrentDatabase {
		return db.Connection, false, nil
	}
	conn, err = db.connectToDatabase(database)
	if err != nil {
		return nil, false, err
	}
	return conn, true, nil
}

func (db *Postgres) SwitchDatabase(database string) error {
	conn, err := db.connectToDatabase(database)
	if err != nil {
		return err
	}

	err = db.Connection.Close()
	if err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			logger.Error("Failed to close postgres connection", map[string]any{"error": closeErr})
		}
		return err
	}

	db.Connection = conn
	db.PreviousDatabase = db.CurrentDatabase
	db.CurrentDatabase = database

	return nil
}

func (db *Postgres) formatTableName(table string) (string, error) {
	splitTableString := strings.Split(table, ".")

	if len(splitTableString) == 1 {
		return "", errors.New("table must be in the format schema.table")
	}

	tableSchema := splitTableString[0]
	tableName := splitTableString[1]

	return fmt.Sprintf("\"%s\".\"%s\"", tableSchema, tableName), nil
}

func (db *Postgres) FormatArg(arg any, colType models.CellValueType) any {
	if colType == models.Null {
		return sql.NullString{
			String: "",
			Valid:  false,
		}
	}

	if colType == models.Empty {
		return ""
	}

	if colType == models.String {
		switch v := arg.(type) {
		case int, int64:
			return fmt.Sprintf("%d", v)
		case float64, float32:
			s := fmt.Sprintf("%f", v)
			trimmed := strings.TrimRight(s, "0")
			if strings.HasSuffix(trimmed, ".") {
				trimmed += "0"
			}
			return trimmed
		case string:
			return v
		case []byte:
			return string(v)
		case nil:
			return sql.NullString{
				String: "",
				Valid:  false,
			}
		default:
			return fmt.Sprintf("%v", v)
		}
	}

	return fmt.Sprintf("%v", arg)
}

func (db *Postgres) FormatArgForQueryString(arg any) string {
	switch v := arg.(type) {
	case string:
		if v == "NULL" || v == "DEFAULT" {
			return v
		}
		escaped := strings.ReplaceAll(v, "'", "''")
		return "'" + escaped + "'"
	case sql.NullString:
		if !v.Valid {
			return "NULL"
		}
		escaped := strings.ReplaceAll(v.String, "'", "''")
		return "'" + escaped + "'"
	default:
		return fmt.Sprintf("%v", v)
	}
}

func (db *Postgres) FormatReference(reference string) string {
	return fmt.Sprintf("\"%s\"", reference)
}

func (db *Postgres) FormatPlaceholder(index int) string {
	return fmt.Sprintf("$%d", index)
}

func (db *Postgres) GetFunctions(_ string) (map[string][]string, error) {
	return nil, errors.New("not implemented")
}

func (db *Postgres) GetProcedures(_ string) (map[string][]string, error) {
	return nil, errors.New("not implemented")
}

func (db *Postgres) GetViews(database string) (map[string][]string, error) {
	if database == "" {
		return nil, errors.New("database name is required")
	}

	conn, needsClose, err := db.connectionFor(database)
	if err != nil {
		return nil, err
	}
	if needsClose {
		defer conn.Close()
	}

	// matviews are absent from information_schema; pg_matviews fills them in
	// unmarked so name-based definition lookup works for both kinds
	query := "SELECT table_name, table_schema FROM information_schema.tables WHERE table_catalog = $1 AND table_type = 'VIEW' UNION ALL SELECT matviewname, schemaname FROM pg_matviews"
	rows, err := conn.Query(query, database)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	views := make(map[string][]string)
	for rows.Next() {
		var (
			viewName   string
			viewSchema string
		)
		if err := rows.Scan(&viewName, &viewSchema); err != nil {
			return nil, err
		}

		views[viewSchema] = append(views[viewSchema], viewName)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return views, nil
}

func (db *Postgres) SupportsProgramming() bool {
	return false
}

func (db *Postgres) UseSchemas() bool {
	return true
}

func (db *Postgres) GetFunctionDefinition(_ string, _ string) (string, error) {
	return "", errors.New("not implemented")
}

func (db *Postgres) GetProcedureDefinition(_ string, _ string) (string, error) {
	return "", errors.New("not implemented")
}

func (db *Postgres) GetViewDefinition(database string, name string) (string, error) {
	if database == "" {
		return "", errors.New("database name is required")
	}
	if name == "" {
		return "", errors.New("view name is required")
	}

	splitViewString := strings.Split(name, ".")
	if len(splitViewString) != 2 {
		return "", errors.New("view must be in the format schema.view")
	}

	conn, needsClose, err := db.connectionFor(database)
	if err != nil {
		return "", err
	}
	if needsClose {
		defer conn.Close()
	}

	viewSchema := splitViewString[0]
	viewName := splitViewString[1]

	var definition string
	err = conn.QueryRow("SELECT definition FROM pg_matviews WHERE schemaname = $1 AND matviewname = $2", viewSchema, viewName).Scan(&definition)
	if err == nil {
		return strings.TrimSpace(definition), nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}

	err = conn.QueryRow("SELECT pg_get_viewdef(format('%I.%I', $1::text, $2::text)::regclass, true)", viewSchema, viewName).Scan(&definition)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(definition), nil
}

func (db *Postgres) GetTableDDL(database, table string) (string, error) {
	if database == "" {
		return "", errors.New("database name is required")
	}
	if table == "" {
		return "", errors.New("table name is required")
	}

	splitTableString := strings.Split(table, ".")
	if len(splitTableString) != 2 {
		return "", errors.New("table must be in the format schema.table")
	}

	conn, needsClose, err := db.connectionFor(database)
	if err != nil {
		return "", err
	}
	if needsClose {
		defer conn.Close()
	}

	tableSchema := splitTableString[0]
	tableName := splitTableString[1]

	colQuery := "SELECT a.attname, format_type(a.atttypid, a.atttypmod), a.attnotnull, COALESCE(pg_get_expr(ad.adbin, ad.adrelid), '') FROM pg_attribute a JOIN pg_class c ON c.oid = a.attrelid JOIN pg_namespace n ON n.oid = c.relnamespace LEFT JOIN pg_attrdef ad ON ad.adrelid = a.attrelid AND ad.adnum = a.attnum WHERE n.nspname = $1 AND c.relname = $2 AND a.attnum > 0 AND NOT a.attisdropped ORDER BY a.attnum"
	colRows, err := conn.Query(colQuery, tableSchema, tableName)
	if err != nil {
		return "", err
	}
	defer colRows.Close()

	var defs []string
	for colRows.Next() {
		var (
			colName    string
			colType    string
			notNull    bool
			defaultVal string
		)
		if err := colRows.Scan(&colName, &colType, &notNull, &defaultVal); err != nil {
			return "", err
		}
		line := fmt.Sprintf("    %q %s", colName, colType)
		if notNull {
			line += " NOT NULL"
		}
		if defaultVal != "" {
			line += " DEFAULT " + defaultVal
		}
		defs = append(defs, line)
	}
	if err := colRows.Err(); err != nil {
		return "", err
	}
	if len(defs) == 0 {
		return "", fmt.Errorf("table %s.%s not found", tableSchema, tableName)
	}

	pkQuery := "SELECT con.conname, pg_get_constraintdef(con.oid) FROM pg_constraint con JOIN pg_class c ON c.oid = con.conrelid JOIN pg_namespace n ON n.oid = c.relnamespace WHERE con.contype = 'p' AND n.nspname = $1 AND c.relname = $2"
	var pkName, pkDef string
	err = conn.QueryRow(pkQuery, tableSchema, tableName).Scan(&pkName, &pkDef)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	if err == nil {
		defs = append(defs, fmt.Sprintf("    CONSTRAINT %q %s", pkName, pkDef))
	}

	stmts := []string{fmt.Sprintf("CREATE TABLE %q.%q (\n%s\n);", tableSchema, tableName, strings.Join(defs, ",\n"))}

	fkQuery := "SELECT con.conname, pg_get_constraintdef(con.oid) FROM pg_constraint con JOIN pg_class c ON c.oid = con.conrelid JOIN pg_namespace n ON n.oid = c.relnamespace WHERE con.contype = 'f' AND n.nspname = $1 AND c.relname = $2 ORDER BY con.conname"
	fkRows, err := conn.Query(fkQuery, tableSchema, tableName)
	if err != nil {
		return "", err
	}
	defer fkRows.Close()

	for fkRows.Next() {
		var fkName, fkDef string
		if err := fkRows.Scan(&fkName, &fkDef); err != nil {
			return "", err
		}
		stmts = append(stmts, fmt.Sprintf("ALTER TABLE %q.%q ADD CONSTRAINT %q %s;", tableSchema, tableName, fkName, fkDef))
	}
	if err := fkRows.Err(); err != nil {
		return "", err
	}

	idxQuery := "SELECT indexname, indexdef FROM pg_indexes WHERE schemaname = $1 AND tablename = $2 ORDER BY indexname"
	idxRows, err := conn.Query(idxQuery, tableSchema, tableName)
	if err != nil {
		return "", err
	}
	defer idxRows.Close()

	for idxRows.Next() {
		var idxName, idxDef string
		if err := idxRows.Scan(&idxName, &idxDef); err != nil {
			return "", err
		}
		// the pk's backing index is already covered by the CONSTRAINT line
		if idxName == pkName {
			continue
		}
		stmts = append(stmts, idxDef+";")
	}
	if err := idxRows.Err(); err != nil {
		return "", err
	}

	return strings.Join(stmts, "\n\n"), nil
}
