package sqlmeta

import (
	"strings"
)

// sqlContext is the structured model built from the lexer token stream: known
// aliases, table names, CTEs, and per-depth table references so completion can
// answer "what table is visible near the cursor?".
type sqlContext struct {
	Aliases map[string]string // alias -> resolved table name (lowercase keys)
	Tables  []string          // all unique table names in order of appearance
	CTEs    map[string]bool   // CTE names (also appear in Aliases)

	tableRefs []tableRef
}

type tableRef struct {
	name  string
	alias string // empty when the table has no alias
	depth int    // subquery nesting depth (0 = main query)
	pos   int    // rune offset in the original SQL text
}

// slice returns the substring of runes for token tok. Offsets are rune indices.
func slice(runes []rune, tok Token) string {
	return string(runes[tok.Start:tok.End])
}

func tokenWord(runes []rune, tokens []Token, i int) string {
	return slice(runes, tokens[i])
}

// scanSQLContext tokenises sql and walks the token stream once to build a
// structured context model handling subquery nesting, CTE declarations,
// comma-separated table lists, subqueries in FROM/JOIN, and schema-qualified
// names.
func scanSQLContext(text string) *sqlContext {
	runes := []rune(text)
	tokens := Tokenize(text)
	ctx := &sqlContext{
		Aliases: make(map[string]string),
		CTEs:    make(map[string]bool),
	}

	depth := 0
	i := 0
	for i < len(tokens) {
		tok := tokens[i]
		word := slice(runes, tok)

		switch word {
		case "(":
			depth++
			i++
			continue
		case ")":
			depth--
			i++
			continue
		}

		if tok.Type == Keyword && depth >= 0 {
			upper := strings.ToUpper(word)
			switch upper {
			case "FROM", "JOIN", "INTO", "UPDATE":
				refs := readTableRefs(runes, tokens, &i, depth)
				ctx.tableRefs = append(ctx.tableRefs, refs...)
				for _, ref := range refs {
					if ref.alias != "" {
						ctx.Aliases[strings.ToLower(ref.alias)] = ref.name
					}
				}
				continue
			case "WITH":
				if depth == 0 {
					readCTEs(runes, tokens, &i, ctx)
					continue
				}
			}
		}

		i++
	}

	seen := make(map[string]bool)
	for _, ref := range ctx.tableRefs {
		lower := strings.ToLower(ref.name)
		if !seen[lower] {
			ctx.Tables = append(ctx.Tables, ref.name)
			seen[lower] = true
		}
	}

	return ctx
}

// cursorDepth returns the subquery nesting depth at cursorPos (a rune offset)
// by walking tokens and counting ( / ) up to that offset.
func cursorDepth(text string, cursorPos int) int {
	runes := []rune(text)
	tokens := Tokenize(text)
	depth := 0
	for _, t := range tokens {
		if t.Start >= cursorPos {
			break
		}
		w := slice(runes, t)
		if w == "(" {
			depth++
		} else if w == ")" {
			depth--
		}
	}
	if depth < 0 {
		depth = 0
	}
	return depth
}

var clauseBoundaries = map[string]bool{
	"WHERE": true, "GROUP": true, "ORDER": true, "HAVING": true,
	"LIMIT": true, "OFFSET": true,
	"UNION": true, "INTERSECT": true, "EXCEPT": true,
	"RETURNING": true, "VALUES": true, "SET": true,
}

var joinModifiers = map[string]bool{
	"INNER": true, "LEFT": true, "RIGHT": true, "CROSS": true,
	"FULL": true, "OUTER": true, "NATURAL": true, "LATERAL": true,
}

var aliasFollowers = map[string]bool{
	",": true, "ON": true, "USING": true,
	"WHERE": true, "ORDER": true, "GROUP": true, "HAVING": true,
	"LIMIT": true, "OFFSET": true, "RETURNING": true,
	"JOIN": true, "INNER": true, "LEFT": true, "RIGHT": true,
	"CROSS": true, "FULL": true, "OUTER": true, "NATURAL": true,
	"UNION": true, "INTERSECT": true, "EXCEPT": true,
	")": true, ";": true, "END": true,
	"(": true, "SET": true,
}

func isClauseBoundary(word string) bool {
	return clauseBoundaries[strings.ToUpper(word)]
}

func isJoinModifier(word string) bool {
	return joinModifiers[strings.ToUpper(word)]
}

func isAliasFollower(word string) bool {
	return aliasFollowers[strings.ToUpper(word)]
}

func skipWhitespace(tokens []Token, i *int) {
	for *i < len(tokens) && tokens[*i].Type == Whitespace {
		*i++
	}
}

// readQualifiedName reads a possibly schema/db-qualified name starting at *i,
// advancing *i past the last consumed token. Each part is unquoted so
// `"public"."Order"` matches the schema tree's bare names.
func readQualifiedName(runes []rune, tokens []Token, i *int) string {
	name := unquoteIdent(tokenWord(runes, tokens, *i))
	*i++

	for *i < len(tokens) && tokenWord(runes, tokens, *i) == "." {
		*i++ // skip dot
		if *i < len(tokens) && (tokens[*i].Type == Identifier || tokens[*i].Type == Keyword) {
			name += "." + unquoteIdent(tokenWord(runes, tokens, *i))
			*i++
		} else {
			break
		}
	}
	return name
}

// skipParenBlock skips from the current '(' to its matching ')'. *i must point
// to the opening '(' token; on return *i points just after the matching ')'.
func skipParenBlock(runes []rune, tokens []Token, i *int) {
	if *i >= len(tokens) || tokenWord(runes, tokens, *i) != "(" {
		return
	}
	depth := 1
	*i++
	for *i < len(tokens) && depth > 0 {
		w := tokenWord(runes, tokens, *i)
		if w == "(" {
			depth++
		} else if w == ")" {
			depth--
		}
		*i++
	}
}

func isFromOrJoin(word string) bool {
	switch strings.ToUpper(word) {
	case "FROM", "JOIN", "INTO", "UPDATE":
		return true
	}
	return false
}

// scanParenForTableRefs scans tokens inside a parenthesised block (starting at
// *i which points to the opening '(') for nested FROM/JOIN keywords and records
// every table reference found at baseDepth+1 and deeper. On return *i points
// just after the matching ')'.
func scanParenForTableRefs(runes []rune, tokens []Token, i *int, baseDepth int, refs *[]tableRef) {
	if *i >= len(tokens) || tokenWord(runes, tokens, *i) != "(" {
		return
	}
	innerDepth := 1
	*i++
	for *i < len(tokens) && innerDepth > 0 {
		w := tokenWord(runes, tokens, *i)

		if w == "(" {
			innerDepth++
		} else if w == ")" {
			innerDepth--
			if innerDepth == 0 {
				*i++ // skip past ')'
				break
			}
		}

		if tokens[*i].Type == Keyword && innerDepth >= 0 {
			if isFromOrJoin(w) {
				innerRefs := readTableRefs(runes, tokens, i, baseDepth+1)
				*refs = append(*refs, innerRefs...)
				continue
			}
		}

		*i++
	}
}

// readTableRefs reads table references starting after FROM/JOIN/INTO/UPDATE,
// handling aliases, comma-separated lists, subqueries as table sources,
// schema-qualified names, stopping at clause boundaries. On return *i points to
// the first token after the table list.
func readTableRefs(runes []rune, tokens []Token, i *int, baseDepth int) []tableRef {
	*i++ // skip the introductory keyword
	var refs []tableRef

loop:
	for *i < len(tokens) {
		skipWhitespace(tokens, i)
		if *i >= len(tokens) {
			break
		}

		tok := tokens[*i]
		word := slice(runes, tok)

		if isClauseBoundary(word) {
			break
		}

		if isJoinModifier(word) {
			*i++
			continue
		}

		if word == "," {
			*i++
			continue
		}

		if word == "(" {
			scanParenForTableRefs(runes, tokens, i, baseDepth, &refs)

			skipWhitespace(tokens, i)
			if *i < len(tokens) && strings.EqualFold(tokenWord(runes, tokens, *i), "AS") {
				*i++
				skipWhitespace(tokens, i)
			}
			if *i < len(tokens) && (tokens[*i].Type == Identifier || tokens[*i].Type == Keyword) {
				alias := unquoteIdent(tokenWord(runes, tokens, *i))
				refs = append(refs, tableRef{name: alias, alias: alias, depth: baseDepth, pos: tokens[*i].Start})
				*i++
			}

			skipWhitespace(tokens, i)
			if *i < len(tokens) && tokenWord(runes, tokens, *i) == "," {
				*i++
				continue
			}
			break
		}

		if tok.Type == Identifier || tok.Type == Keyword {
			name := readQualifiedName(runes, tokens, i)
			alias := ""
			skipWhitespace(tokens, i)

			if *i < len(tokens) {
				nextTok := tokens[*i]
				nextWord := slice(runes, nextTok)

				if strings.EqualFold(nextWord, "AS") {
					*i++
					skipWhitespace(tokens, i)
					if *i < len(tokens) && (tokens[*i].Type == Identifier || tokens[*i].Type == Keyword) {
						alias = unquoteIdent(tokenWord(runes, tokens, *i))
						*i++
					}
				} else if (nextTok.Type == Identifier || nextTok.Type == Keyword) &&
					!isClauseBoundary(nextWord) && !isJoinModifier(nextWord) && nextWord != "," && nextWord != "(" {
					save := *i
					*i++
					skipWhitespace(tokens, i)
					if *i >= len(tokens) || isAliasFollower(tokenWord(runes, tokens, *i)) {
						alias = unquoteIdent(nextWord)
					} else {
						*i = save
					}
				}
			}

			refs = append(refs, tableRef{
				name:  name,
				alias: alias,
				depth: baseDepth,
				pos:   tok.Start,
			})

			skipWhitespace(tokens, i)
			if *i < len(tokens) && tokenWord(runes, tokens, *i) == "," {
				*i++
				continue
			}

			break loop
		}

		break
	}

	return refs
}

// readCTEs processes WITH ... AS ( ... ) definitions at depth 0, registering CTE
// names in ctx.CTEs and ctx.Aliases, then skipping past the CTE bodies.
func readCTEs(runes []rune, tokens []Token, i *int, ctx *sqlContext) {
	*i++ // skip WITH

	skipWhitespace(tokens, i)
	if *i < len(tokens) && strings.EqualFold(tokenWord(runes, tokens, *i), "RECURSIVE") {
		*i++
	}

	for *i < len(tokens) {
		skipWhitespace(tokens, i)
		if *i >= len(tokens) {
			break
		}

		tok := tokens[*i]
		if tok.Type != Identifier && tok.Type != Keyword && tok.Type != Function {
			break
		}
		name := unquoteIdent(slice(runes, tok))
		*i++

		skipWhitespace(tokens, i)
		if *i < len(tokens) && tokenWord(runes, tokens, *i) == "(" {
			skipParenBlock(runes, tokens, i)
		}

		skipWhitespace(tokens, i)
		if *i < len(tokens) && strings.EqualFold(tokenWord(runes, tokens, *i), "AS") {
			*i++
		} else {
			break
		}

		skipWhitespace(tokens, i)
		if *i < len(tokens) && tokenWord(runes, tokens, *i) == "(" {
			skipParenBlock(runes, tokens, i)
		}

		lower := strings.ToLower(name)
		ctx.CTEs[lower] = true
		ctx.Aliases[lower] = name

		skipWhitespace(tokens, i)
		if *i < len(tokens) && tokenWord(runes, tokens, *i) == "," {
			*i++
			continue
		}
		break
	}
}
