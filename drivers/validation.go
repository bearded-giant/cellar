package drivers

import (
	"errors"
	"regexp"
	"strings"
)

// sql keywords that are blocked in read-only mode
var readOnlyBlockedKeywords = []string{
	"INSERT", "UPDATE", "DELETE", "DROP", "ALTER",
	"TRUNCATE", "REPLACE", "MERGE",
	"GRANT", "REVOKE", "RENAME",
	"CREATE TABLE", "CREATE INDEX", "CREATE DATABASE",
	"CREATE SCHEMA", "CREATE VIEW", "CREATE FUNCTION",
	"CREATE PROCEDURE", "CREATE TRIGGER",
}

// sql keywords that are allowed even in read-only mode
var readOnlyAllowedKeywords = []string{
	"CREATE TEMPORARY TABLE", "CREATE TEMP TABLE",
	"CREATE TEMP VIEW", "CREATE TEMPORARY VIEW",
}

var (
	sqlLineComment  = regexp.MustCompile(`--[^\n]*`)
	sqlBlockComment = regexp.MustCompile(`/\*[\s\S]*?\*/`)
	// literals and quoted identifiers are blanked so their contents can't trip
	// (or hide from) the keyword scan
	sqlSingleQuoted = regexp.MustCompile(`'(?:[^']|'')*'`)
	sqlDoubleQuoted = regexp.MustCompile(`"(?:[^"]|"")*"`)
	sqlBacktickID   = regexp.MustCompile("`(?:[^`]|``)*`")
)

// IsQueryMutation checks if a SQL query is a mutation operation. Blocked
// keywords are matched anywhere in the statement (word-bounded, outside
// strings/comments) — a prefix check alone misses WITH-wrapped DML like
// "WITH d AS (DELETE ...) SELECT ..." and "EXPLAIN ANALYZE DELETE ...",
// which drivers execute for real.
func IsQueryMutation(query string) bool {
	upperQuery := strings.TrimSpace(strings.ToUpper(query))
	upperQuery = sqlLineComment.ReplaceAllString(upperQuery, "")
	upperQuery = sqlBlockComment.ReplaceAllString(upperQuery, "")
	upperQuery = sqlSingleQuoted.ReplaceAllString(upperQuery, "''")
	upperQuery = sqlDoubleQuoted.ReplaceAllString(upperQuery, `""`)
	upperQuery = sqlBacktickID.ReplaceAllString(upperQuery, "``")
	upperQuery = strings.TrimSpace(upperQuery)

	// check if query starts with allowed keywords
	for _, allowedKeyword := range readOnlyAllowedKeywords {
		if strings.HasPrefix(upperQuery, allowedKeyword) {
			return false
		}
	}

	for _, blockedKeyword := range readOnlyBlockedKeywords {
		pattern := `\b` + strings.ReplaceAll(regexp.QuoteMeta(blockedKeyword), ` `, `\s+`) + `\b`
		if matched, _ := regexp.MatchString(pattern, upperQuery); matched {
			return true
		}
	}

	return false
}

func ValidateQueryForReadOnly(query string) error {
	if IsQueryMutation(query) {
		return errors.New("mutation queries are not allowed in read-only mode")
	}
	return nil
}
