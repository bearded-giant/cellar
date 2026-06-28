package sqlmeta

import (
	"sort"
	"strings"
	"unicode"

	"github.com/lithammer/fuzzysearch/fuzzy"
)

// CompletionItem represents a single autocomplete suggestion.
type CompletionItem struct {
	Text        string // what gets inserted
	Description string // displayed help text
}

// Autocompleter manages SQL keywords and schema-aware completions.
type Autocompleter struct {
	keywords []CompletionItem
	tables   []CompletionItem
	columns  map[string][]CompletionItem // table name -> columns
}

// NewAutocompleter creates an autocompleter with built-in SQL keywords.
func NewAutocompleter() *Autocompleter {
	return &Autocompleter{
		keywords: builtinKeywords(),
		tables:   nil,
		columns:  make(map[string][]CompletionItem),
	}
}

// SetTables updates the list of known table names.
func (a *Autocompleter) SetTables(tables []string) {
	a.tables = make([]CompletionItem, len(tables))
	for i, t := range tables {
		a.tables[i] = CompletionItem{Text: t, Description: "table"}
	}
}

// SetColumns sets the columns for a given table.
func (a *Autocompleter) SetColumns(table string, columns []string) {
	items := make([]CompletionItem, len(columns))
	for i, c := range columns {
		items[i] = CompletionItem{Text: c, Description: "column"}
	}
	a.columns[strings.ToLower(table)] = items
}

// GetCompletions returns completion items matching prefix using fuzzy search
// (exact > prefix > substring > fuzzy). If tableHint is non-empty it prioritises
// columns from that table. When prefix is empty but tableHint is set (user typed
// "table."), all columns for that table are returned.
func (a *Autocompleter) GetCompletions(prefix string, tableHint string) []CompletionItem {
	if prefix == "" && tableHint != "" {
		if cols, ok := a.columns[strings.ToLower(tableHint)]; ok {
			return cols
		}
		return nil
	}
	if prefix == "" {
		return nil
	}

	type scoredCandidate struct {
		item  CompletionItem
		score int
		order int
	}

	var candidates []scoredCandidate
	seen := make(map[string]bool)
	lowerPrefix := strings.ToLower(prefix)

	tryAdd := func(items []CompletionItem, order int) {
		for _, item := range items {
			key := strings.ToLower(item.Text)
			if seen[key] {
				continue
			}
			rank := fuzzy.RankMatch(lowerPrefix, key)
			if rank < 0 {
				continue
			}
			score := prioritizeResult(lowerPrefix, key, rank)
			candidates = append(candidates, scoredCandidate{item, score, order})
			seen[key] = true
		}
	}

	if tableHint != "" {
		if cols, ok := a.columns[strings.ToLower(tableHint)]; ok {
			tryAdd(cols, 0)
		}
	}

	tryAdd(a.tables, 1)
	tryAdd(a.keywords, 2)

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score < candidates[j].score
		}
		return candidates[i].order < candidates[j].order
	})

	maxResults := 20
	if len(candidates) < maxResults {
		maxResults = len(candidates)
	}

	results := make([]CompletionItem, maxResults)
	for i := 0; i < maxResults; i++ {
		results[i] = candidates[i].item
	}

	return results
}

// GetAllCompletions returns all known keywords, tables, and column names
// matching the prefix, in order of relevance.
func (a *Autocompleter) GetAllCompletions(prefix string) []CompletionItem {
	return a.GetCompletions(prefix, "")
}

// Complete runs the full completion pipeline for text with the cursor at
// cursorPos (a RUNE index): it extracts the prefix/table context, resolves
// aliases, falls back to a table hint, then returns matching completions.
func (a *Autocompleter) Complete(text string, cursorPos int) []CompletionItem {
	prefix, tableName := extractCompletionContext(text, cursorPos)

	if tableName != "" {
		aliases := resolveAliases(text, cursorPos)
		if resolved, ok := aliases[strings.ToLower(tableName)]; ok {
			tableName = resolved
		}
	} else {
		tableName = extractTableHint(text, cursorPos)
	}

	if tableName != "" || len(prefix) >= 1 {
		return a.GetCompletions(prefix, tableName)
	}
	return nil
}

// extractPrefix extracts the word being typed at cursorPos (a rune offset).
func extractPrefix(text string, cursorPos int) string {
	prefix, _ := extractCompletionContext(text, cursorPos)
	return prefix
}

// extractCompletionContext splits the current word segment at the cursor into a
// column prefix and an optional table name. For "table.col|" it returns
// ("col", "table"); for "prefix|" alone it returns ("prefix", ""). cursorPos is
// a RUNE offset into text.
func extractCompletionContext(text string, cursorPos int) (prefix, tableName string) {
	runes := []rune(text)
	if cursorPos <= 0 || cursorPos > len(runes) {
		return "", ""
	}

	start := cursorPos - 1
	for start >= 0 {
		ch := runes[start]
		if unicode.IsSpace(ch) || ch == ';' || ch == ',' || ch == '(' || ch == ')' {
			break
		}
		start--
	}
	start++

	if start >= cursorPos {
		return "", ""
	}

	segment := string(runes[start:cursorPos])

	if dotIdx := strings.LastIndex(segment, "."); dotIdx >= 0 {
		tableName = segment[:dotIdx]
		prefix = segment[dotIdx+1:]
		return prefix, tableName
	}

	return segment, ""
}

// resolveAliases scans the query for table alias definitions using the
// lexer-based context scanner. Returns aliases visible at cursorPos (a rune
// offset): outer-scope aliases are visible, deeper-subquery aliases are not.
func resolveAliases(sql string, cursorPos int) map[string]string {
	ctx := scanSQLContext(sql)
	depth := cursorDepth(sql, cursorPos)
	result := make(map[string]string)

	for _, ref := range ctx.tableRefs {
		if ref.alias != "" && ref.depth <= depth {
			result[strings.ToLower(ref.alias)] = ref.name
		}
	}
	for cteName := range ctx.CTEs {
		if 0 <= depth {
			result[cteName] = cteName
		}
	}

	return result
}

// extractTableHint finds the most recently mentioned table at the cursor's
// subquery depth. cursorPos is a RUNE offset.
func extractTableHint(text string, cursorPos int) string {
	runes := []rune(text)
	if cursorPos > len(runes) {
		cursorPos = len(runes)
	}

	ctx := scanSQLContext(text)
	depth := cursorDepth(text, cursorPos)

	var best tableRef
	bestPos := -1
	for _, ref := range ctx.tableRefs {
		if ref.depth == depth && ref.pos < cursorPos && ref.pos > bestPos {
			best = ref
			bestPos = ref.pos
		}
	}

	if best.name != "" {
		trimmed := strings.TrimRight(string(runes[:cursorPos]), " \t\n\r")
		if len(trimmed) > 0 && trimmed[len(trimmed)-1] == ',' {
			return ""
		}
		return best.name
	}
	return ""
}

// prioritizeResult plays match golf — lowest score wins (0=exact, 1-99=prefix,
// 100+=substring, 10000+=fuzzy).
func prioritizeResult(pattern, target string, fuzzyRank int) int {
	if pattern == target {
		return 0
	}

	if strings.HasPrefix(target, pattern) {
		lengthDiff := len(target) - len(pattern)
		if lengthDiff > 98 {
			lengthDiff = 98
		}
		return 1 + lengthDiff
	}

	if strings.Contains(target, pattern) {
		index := strings.Index(target, pattern)
		lengthPenalty := len(target) - len(pattern)
		score := 100 + index + lengthPenalty
		if score > 9999 {
			score = 9999
		}
		return score
	}

	return 10000 + fuzzyRank
}

// builtinKeywords returns the list of SQL keywords for autocomplete.
func builtinKeywords() []CompletionItem {
	return []CompletionItem{
		{Text: "SELECT", Description: "Retrieve rows from a table"},
		{Text: "FROM", Description: "Specify the source table"},
		{Text: "WHERE", Description: "Filter results"},
		{Text: "AND", Description: "Logical AND"},
		{Text: "OR", Description: "Logical OR"},
		{Text: "NOT", Description: "Logical NOT"},
		{Text: "IN", Description: "Check membership"},
		{Text: "IS", Description: "Null comparison"},
		{Text: "NULL", Description: "Null value"},
		{Text: "LIKE", Description: "Pattern matching"},
		{Text: "BETWEEN", Description: "Range check"},
		{Text: "EXISTS", Description: "Check existence"},
		{Text: "AS", Description: "Alias"},
		{Text: "ON", Description: "Join condition"},
		{Text: "JOIN", Description: "Join tables"},
		{Text: "INNER", Description: "Inner join"},
		{Text: "LEFT", Description: "Left join"},
		{Text: "RIGHT", Description: "Right join"},
		{Text: "OUTER", Description: "Outer join"},
		{Text: "CROSS", Description: "Cross join"},
		{Text: "FULL", Description: "Full join"},
		{Text: "NATURAL", Description: "Natural join"},
		{Text: "USING", Description: "Join using columns"},
		{Text: "INSERT", Description: "Insert rows"},
		{Text: "INTO", Description: "Specify target table"},
		{Text: "VALUES", Description: "Row values"},
		{Text: "UPDATE", Description: "Update rows"},
		{Text: "SET", Description: "Set column values"},
		{Text: "DELETE", Description: "Delete rows"},
		{Text: "CREATE", Description: "Create object"},
		{Text: "TABLE", Description: "Create table"},
		{Text: "INDEX", Description: "Create index"},
		{Text: "VIEW", Description: "Create view"},
		{Text: "ALTER", Description: "Modify object"},
		{Text: "DROP", Description: "Delete object"},
		{Text: "ADD", Description: "Add column"},
		{Text: "COLUMN", Description: "Column keyword"},
		{Text: "CONSTRAINT", Description: "Add constraint"},
		{Text: "PRIMARY KEY", Description: "Primary key"},
		{Text: "FOREIGN KEY", Description: "Foreign key"},
		{Text: "UNIQUE", Description: "Unique constraint"},
		{Text: "CHECK", Description: "Check constraint"},
		{Text: "DEFAULT", Description: "Default value"},
		{Text: "REFERENCES", Description: "Foreign key reference"},
		{Text: "ORDER BY", Description: "Order results"},
		{Text: "ASC", Description: "Ascending order"},
		{Text: "DESC", Description: "Descending order"},
		{Text: "GROUP BY", Description: "Group results"},
		{Text: "HAVING", Description: "Filter groups"},
		{Text: "LIMIT", Description: "Limit rows"},
		{Text: "OFFSET", Description: "Offset rows"},
		{Text: "UNION", Description: "Combine queries"},
		{Text: "ALL", Description: "All results"},
		{Text: "DISTINCT", Description: "Distinct rows"},
		{Text: "CASE", Description: "Case expression"},
		{Text: "WHEN", Description: "Case when"},
		{Text: "THEN", Description: "Case then"},
		{Text: "ELSE", Description: "Case else"},
		{Text: "END", Description: "End expression"},
		{Text: "BEGIN", Description: "Start transaction"},
		{Text: "COMMIT", Description: "Commit transaction"},
		{Text: "ROLLBACK", Description: "Rollback transaction"},
		{Text: "EXPLAIN", Description: "Explain query plan"},
		{Text: "DESCRIBE", Description: "Describe table"},
		{Text: "SHOW", Description: "Show objects"},
		{Text: "USE", Description: "Use database"},
		{Text: "TRUNCATE", Description: "Truncate table"},
		{Text: "REPLACE", Description: "Replace rows"},
		{Text: "CALL", Description: "Call procedure"},
		{Text: "WITH", Description: "Common table expression"},
		{Text: "RECURSIVE", Description: "Recursive CTE"},
		{Text: "FETCH", Description: "Fetch rows"},
		{Text: "NEXT", Description: "Fetch next"},
		{Text: "ROWS", Description: "Row count"},
		{Text: "ONLY", Description: "Fetch only"},
		{Text: "TOP", Description: "Select top"},
		{Text: "COUNT", Description: "Count rows"},
		{Text: "SUM", Description: "Sum values"},
		{Text: "AVG", Description: "Average values"},
		{Text: "MIN", Description: "Minimum value"},
		{Text: "MAX", Description: "Maximum value"},
		{Text: "COALESCE", Description: "First non-null"},
		{Text: "IFNULL", Description: "Null fallback"},
		{Text: "CAST", Description: "Type cast"},
		{Text: "CONVERT", Description: "Type convert"},
		{Text: "CROSS JOIN", Description: "Cross join"},
		{Text: "NATURAL JOIN", Description: "Natural join"},
		{Text: "INNER JOIN", Description: "Inner join"},
		{Text: "LEFT JOIN", Description: "Left join"},
		{Text: "RIGHT JOIN", Description: "Right join"},
		{Text: "FULL JOIN", Description: "Full join"},
		{Text: "OUTER JOIN", Description: "Outer join"},
		{Text: "ASC", Description: "Ascending"},
		{Text: "DESC", Description: "Descending"},
		{Text: "COUNT(*)", Description: "Count all rows"},
		{Text: "DISTINCT", Description: "Unique values only"},
	}
}
