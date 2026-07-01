package sqlmeta

import "strings"

// SplitStatements returns the ';'-delimited statements in source order, trimmed
// and with blanks dropped. Semicolons inside strings/comments are ignored.
func SplitStatements(text string) []string {
	runes := []rune(text)
	var out []string
	prev := 0
	push := func(a, b int) {
		if s := strings.TrimSpace(string(runes[a:b])); s != "" {
			out = append(out, s)
		}
	}
	for _, t := range Tokenize(text) {
		if t.Type == Punctuation && t.Start < len(runes) && runes[t.Start] == ';' {
			push(prev, t.Start)
			prev = t.Start + 1
		}
	}
	push(prev, len(runes))
	return out
}

// StatementBoundsAt returns the rune-offset span [start,end) of the statement
// containing `off` — bounded by top-level ';' separators (the ';' itself is
// excluded; semicolons inside strings/comments never surface as Punctuation).
// Cursor sitting on a ';' selects the statement it closes.
func StatementBoundsAt(text string, off int) (start, end int) {
	runes := []rune(text)
	if off < 0 {
		off = 0
	}
	if off > len(runes) {
		off = len(runes)
	}
	start = 0
	for _, t := range Tokenize(text) {
		if t.Type != Punctuation || t.Start >= len(runes) || runes[t.Start] != ';' {
			continue
		}
		if t.Start < off {
			start = t.Start + 1
		} else {
			return start, t.Start
		}
	}
	return start, len(runes)
}

// StatementAt returns the SQL statement that the rune offset `off` falls in.
// With no separators it returns the whole text. When the cursor sits in trailing
// space after the final ';', it walks back to the nearest non-empty statement.
func StatementAt(text string, off int) string {
	start, end := StatementBoundsAt(text, off)
	if s := strings.TrimSpace(string([]rune(text)[start:end])); s != "" {
		return s
	}
	stmts := SplitStatements(text)
	if len(stmts) == 0 {
		return ""
	}
	return stmts[len(stmts)-1]
}
