package sqlmeta

import (
	"strings"
	"unicode"
)

// TokenType classifies a span of SQL text for syntax highlighting.
type TokenType int

const (
	Whitespace TokenType = iota
	Keyword
	String
	Number
	Comment
	Function
	Operator
	Identifier
	Punctuation
	Parameter // $1, ?
	TypeDef   // INT, VARCHAR, TEXT, etc.
	Boolean   // TRUE, FALSE, NULL
)

// Token represents a single token in SQL source. Start and End are RUNE
// offsets into the input (Start inclusive, End exclusive).
type Token struct {
	Type  TokenType
	Start int
	End   int
}

type sqlLexer struct {
	input []rune
	pos   int
}

// Tokenize splits SQL input into tokens with rune offsets.
func Tokenize(input string) []Token {
	l := &sqlLexer{
		input: []rune(input),
		pos:   0,
	}
	var tokens []Token

	for l.pos < len(l.input) {
		ch := l.input[l.pos]

		switch {
		case ch == '-' && l.peek() == '-':
			tokens = append(tokens, l.readComment())
		case ch == '/' && l.peek() == '*':
			tokens = append(tokens, l.readBlockComment())
		case ch == '\'':
			tokens = append(tokens, l.readString())
		case ch == '"':
			tokens = append(tokens, l.readQuotedIdentifier())
		case ch == '`':
			tokens = append(tokens, l.readBacktickIdentifier())
		case ch == '$' && l.peekRune(1) != '(' && l.peekRune(1) != '\'':
			tokens = append(tokens, l.readParameter())
		case ch == '?':
			tokens = append(tokens, Token{Type: Parameter, Start: l.pos, End: l.pos + 1})
			l.pos++
		case ch == ':' && l.pos+1 < len(l.input) && !unicode.IsSpace(l.input[l.pos+1]) && l.input[l.pos+1] != ':' && l.input[l.pos+1] != '=':
			tokens = append(tokens, l.readNamedParameter())
		case isDigit(ch) || (ch == '.' && l.pos+1 < len(l.input) && isDigit(l.input[l.pos+1])):
			tokens = append(tokens, l.readNumber())
		case isLetter(ch) || ch == '_':
			tokens = append(tokens, l.readIdentOrKeyword())
		case isOperator(ch):
			tokens = append(tokens, l.readOperator())
		case isPunctuation(ch):
			tokens = append(tokens, Token{Type: Punctuation, Start: l.pos, End: l.pos + 1})
			l.pos++
		default:
			start := l.pos
			for l.pos < len(l.input) && (unicode.IsSpace(l.input[l.pos]) || l.input[l.pos] == 0) {
				l.pos++
			}
			if l.pos > start {
				tokens = append(tokens, Token{Type: Whitespace, Start: start, End: l.pos})
			} else {
				l.pos++
			}
		}
	}

	return tokens
}

func (l *sqlLexer) peek() rune {
	if l.pos+1 < len(l.input) {
		return l.input[l.pos+1]
	}
	return 0
}

func (l *sqlLexer) peekRune(n int) rune {
	if l.pos+n < len(l.input) {
		return l.input[l.pos+n]
	}
	return 0
}

func (l *sqlLexer) readComment() Token {
	start := l.pos
	for l.pos < len(l.input) && l.input[l.pos] != '\n' {
		l.pos++
	}
	return Token{Type: Comment, Start: start, End: l.pos}
}

func (l *sqlLexer) readBlockComment() Token {
	start := l.pos
	l.pos += 2 // skip /*
	for l.pos < len(l.input) {
		if l.input[l.pos] == '*' && l.peek() == '/' {
			l.pos += 2
			break
		}
		l.pos++
	}
	return Token{Type: Comment, Start: start, End: l.pos}
}

func (l *sqlLexer) readString() Token {
	start := l.pos
	quote := l.input[l.pos]
	l.pos++ // skip opening quote
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if ch == quote {
			l.pos++ // skip closing quote
			if l.pos < len(l.input) && l.input[l.pos] == quote {
				l.pos++
				continue
			}
			break
		}
		l.pos++
	}
	return Token{Type: String, Start: start, End: l.pos}
}

func (l *sqlLexer) readQuotedIdentifier() Token {
	start := l.pos
	l.pos++ // skip opening "
	for l.pos < len(l.input) {
		if l.input[l.pos] == '"' {
			l.pos++ // skip closing "
			if l.pos < len(l.input) && l.input[l.pos] == '"' {
				l.pos++
				continue
			}
			break
		}
		l.pos++
	}
	return Token{Type: Identifier, Start: start, End: l.pos}
}

func (l *sqlLexer) readBacktickIdentifier() Token {
	start := l.pos
	l.pos++ // skip opening `
	for l.pos < len(l.input) {
		if l.input[l.pos] == '`' {
			l.pos++ // skip closing `
			break
		}
		l.pos++
	}
	return Token{Type: Identifier, Start: start, End: l.pos}
}

func (l *sqlLexer) readParameter() Token {
	start := l.pos
	l.pos++ // skip $
	for l.pos < len(l.input) && isDigit(l.input[l.pos]) {
		l.pos++
	}
	return Token{Type: Parameter, Start: start, End: l.pos}
}

func (l *sqlLexer) readNamedParameter() Token {
	start := l.pos
	l.pos++ // skip :
	for l.pos < len(l.input) && (isLetter(l.input[l.pos]) || isDigit(l.input[l.pos]) || l.input[l.pos] == '_') {
		l.pos++
	}
	return Token{Type: Parameter, Start: start, End: l.pos}
}

func (l *sqlLexer) readNumber() Token {
	start := l.pos
	if l.input[l.pos] == '.' {
		l.pos++
	}
	for l.pos < len(l.input) && isDigit(l.input[l.pos]) {
		l.pos++
	}
	if l.pos < len(l.input) && l.input[l.pos] == '.' {
		l.pos++
		for l.pos < len(l.input) && isDigit(l.input[l.pos]) {
			l.pos++
		}
	}
	if l.pos < len(l.input) && (l.input[l.pos] == 'e' || l.input[l.pos] == 'E') {
		l.pos++
		if l.pos < len(l.input) && (l.input[l.pos] == '+' || l.input[l.pos] == '-') {
			l.pos++
		}
		for l.pos < len(l.input) && isDigit(l.input[l.pos]) {
			l.pos++
		}
	}
	return Token{Type: Number, Start: start, End: l.pos}
}

func (l *sqlLexer) readIdentOrKeyword() Token {
	start := l.pos
	for l.pos < len(l.input) && (isLetter(l.input[l.pos]) || isDigit(l.input[l.pos]) || l.input[l.pos] == '_') {
		l.pos++
	}
	word := string(l.input[start:l.pos])
	upper := strings.ToUpper(word)

	if isKeyword(upper) {
		return Token{Type: Keyword, Start: start, End: l.pos}
	}
	if isType(upper) {
		return Token{Type: TypeDef, Start: start, End: l.pos}
	}
	if isBoolean(upper) {
		return Token{Type: Boolean, Start: start, End: l.pos}
	}
	if l.pos < len(l.input) && l.input[l.pos] == '(' {
		return Token{Type: Function, Start: start, End: l.pos}
	}

	return Token{Type: Identifier, Start: start, End: l.pos}
}

func (l *sqlLexer) readOperator() Token {
	start := l.pos
	ch := l.input[l.pos]
	l.pos++

	if ch == '<' && l.pos < len(l.input) {
		if l.input[l.pos] == '=' || l.input[l.pos] == '>' {
			l.pos++
		}
	} else if ch == '>' && l.pos < len(l.input) && l.input[l.pos] == '=' {
		l.pos++
	} else if ch == '!' && l.pos < len(l.input) && l.input[l.pos] == '=' {
		l.pos++
	} else if ch == ':' && l.pos < len(l.input) && l.input[l.pos] == '=' {
		l.pos++
	} else if ch == '|' && l.pos < len(l.input) && l.input[l.pos] == '|' {
		l.pos++
	}

	return Token{Type: Operator, Start: start, End: l.pos}
}

func isLetter(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch > 127
}

func isDigit(ch rune) bool {
	return ch >= '0' && ch <= '9'
}

func isOperator(ch rune) bool {
	switch ch {
	case '=', '<', '>', '!', '+', '-', '*', '/', '%', '|', '&', '^', '~':
		return true
	}
	return false
}

func isPunctuation(ch rune) bool {
	switch ch {
	case '(', ')', ',', ';', '.', '[', ']':
		return true
	}
	return false
}

// ColorFor returns a hex color string for a token type, or "" for the default
// (identifier, punctuation, whitespace).
func ColorFor(t TokenType) string {
	switch t {
	case Keyword:
		return "#1E90FF"
	case String:
		return "#FFA500"
	case Number:
		return "#32CD32"
	case Comment:
		return "#808080"
	case Function:
		return "#9370DB"
	case Operator:
		return "#FF8C00"
	case TypeDef:
		return "#008B8B"
	case Boolean:
		return "#FF4500"
	case Parameter:
		return "#FFD700"
	default:
		return ""
	}
}

var sqlKeywords = map[string]bool{
	"SELECT":      true,
	"FROM":        true,
	"WHERE":       true,
	"AND":         true,
	"OR":          true,
	"NOT":         true,
	"IN":          true,
	"IS":          true,
	"NULL":        true,
	"LIKE":        true,
	"BETWEEN":     true,
	"EXISTS":      true,
	"AS":          true,
	"ON":          true,
	"JOIN":        true,
	"INNER":       true,
	"LEFT":        true,
	"RIGHT":       true,
	"OUTER":       true,
	"CROSS":       true,
	"FULL":        true,
	"NATURAL":     true,
	"USING":       true,
	"INSERT":      true,
	"INTO":        true,
	"VALUES":      true,
	"UPDATE":      true,
	"SET":         true,
	"DELETE":      true,
	"CREATE":      true,
	"TABLE":       true,
	"INDEX":       true,
	"VIEW":        true,
	"PROCEDURE":   true,
	"FUNCTION":    true,
	"TRIGGER":     true,
	"ALTER":       true,
	"DROP":        true,
	"ADD":         true,
	"COLUMN":      true,
	"CONSTRAINT":  true,
	"PRIMARY":     true,
	"KEY":         true,
	"FOREIGN":     true,
	"UNIQUE":      true,
	"CHECK":       true,
	"DEFAULT":     true,
	"REFERENCES":  true,
	"CASCADE":     true,
	"ORDER":       true,
	"BY":          true,
	"ASC":         true,
	"DESC":        true,
	"GROUP":       true,
	"HAVING":      true,
	"LIMIT":       true,
	"OFFSET":      true,
	"UNION":       true,
	"ALL":         true,
	"INTERSECT":   true,
	"EXCEPT":      true,
	"DISTINCT":    true,
	"TOP":         true,
	"FETCH":       true,
	"NEXT":        true,
	"ROWS":        true,
	"ONLY":        true,
	"WITH":        true,
	"RECURSIVE":   true,
	"CASE":        true,
	"WHEN":        true,
	"THEN":        true,
	"ELSE":        true,
	"END":         true,
	"BEGIN":       true,
	"COMMIT":      true,
	"ROLLBACK":    true,
	"TRANSACTION": true,
	"SAVEPOINT":   true,
	"RELEASE":     true,
	"EXPLAIN":     true,
	"ANALYZE":     true,
	"DESCRIBE":    true,
	"SHOW":        true,
	"USE":         true,
	"GRANT":       true,
	"REVOKE":      true,
	"TRUNCATE":    true,
	"REPLACE":     true,
	"CALL":        true,
	"IF":          true,
	"ELSEIF":      true,
	"WHILE":       true,
	"LOOP":        true,
	"DECLARE":     true,
	"RETURN":      true,
	"DO":          true,
	"FOR":         true,
	"EACH":        true,
	"ROW":         true,
	"SCHEMA":      true,
	"DATABASE":    true,
	"TEMPORARY":   true,
	"TEMP":        true,
	"IFNULL":      true,
	"COALESCE":    true,
	"CAST":        true,
	"CONVERT":     true,
	"ANY":         true,
	"SOME":        true,
	"EXEC":        true,
	"EXECUTE":     true,
}

func isKeyword(upper string) bool {
	return sqlKeywords[upper]
}

var sqlTypes = map[string]bool{
	"INT":              true,
	"INTEGER":          true,
	"SMALLINT":         true,
	"BIGINT":           true,
	"TINYINT":          true,
	"MEDIUMINT":        true,
	"REAL":             true,
	"FLOAT":            true,
	"DOUBLE":           true,
	"DECIMAL":          true,
	"NUMERIC":          true,
	"CHAR":             true,
	"VARCHAR":          true,
	"TEXT":             true,
	"TINYTEXT":         true,
	"MEDIUMTEXT":       true,
	"LONGTEXT":         true,
	"BLOB":             true,
	"TINYBLOB":         true,
	"MEDIUMBLOB":       true,
	"LONGBLOB":         true,
	"BINARY":           true,
	"VARBINARY":        true,
	"BOOLEAN":          true,
	"BOOL":             true,
	"DATE":             true,
	"DATETIME":         true,
	"TIMESTAMP":        true,
	"TIME":             true,
	"YEAR":             true,
	"ENUM":             true,
	"SET":              true,
	"JSON":             true,
	"SERIAL":           true,
	"UUID":             true,
	"GEOMETRY":         true,
	"POINT":            true,
	"LINESTRING":       true,
	"POLYGON":          true,
	"INTERVAL":         true,
	"BYTEA":            true,
	"VARYING":          true,
	"CHARACTER":        true,
	"NVARCHAR":         true,
	"NCHAR":            true,
	"NTEXT":            true,
	"MONEY":            true,
	"SMALLMONEY":       true,
	"UNIQUEIDENTIFIER": true,
	"IMAGE":            true,
	"XML":              true,
	"CLOB":             true,
	"RAW":              true,
	"NUMBER":           true,
	"PLS_INTEGER":      true,
}

func isType(upper string) bool {
	return sqlTypes[upper]
}

var sqlBooleans = map[string]bool{
	"TRUE":    true,
	"FALSE":   true,
	"NULL":    true,
	"UNKNOWN": true,
}

func isBoolean(upper string) bool {
	return sqlBooleans[upper]
}

// visibleLen returns the display width of s, counting tabs as tabWidth spaces.
func visibleLen(s string, tabWidth int) int {
	w := 0
	for _, ch := range s {
		if ch == '\t' {
			w += tabWidth - (w % tabWidth)
		} else {
			w += runeWidth(ch)
		}
	}
	return w
}

func runeWidth(r rune) int {
	if r == '\t' || r == '\n' || r == '\r' {
		return 0
	}
	if r < 128 {
		return 1
	}
	return 1
}
