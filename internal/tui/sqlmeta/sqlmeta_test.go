package sqlmeta

import (
	"testing"
)

func TestTokenize(t *testing.T) {
	input := "SELECT id FROM users WHERE x = 1"
	runes := []rune(input)
	tokens := Tokenize(input)

	want := map[string]TokenType{
		"SELECT": Keyword,
		"FROM":   Keyword,
		"WHERE":  Keyword,
		"id":     Identifier,
		"users":  Identifier,
		"x":      Identifier,
		"1":      Number,
	}

	got := make(map[string]TokenType)
	for _, tok := range tokens {
		if tok.Type == Whitespace {
			continue
		}
		word := string(runes[tok.Start:tok.End])
		got[word] = tok.Type
	}

	for word, typ := range want {
		if got[word] != typ {
			t.Errorf("token %q: got type %d, want %d", word, got[word], typ)
		}
	}
}

func TestColorFor(t *testing.T) {
	if c := ColorFor(Keyword); c != "#1E90FF" {
		t.Errorf("ColorFor(Keyword) = %q, want #1E90FF", c)
	}
	if c := ColorFor(Identifier); c != "" {
		t.Errorf("ColorFor(Identifier) = %q, want empty", c)
	}
}

func TestComplete_Keywords(t *testing.T) {
	a := NewAutocompleter()
	items := a.Complete("SEL", 3)
	if len(items) == 0 {
		t.Fatal("Complete(\"SEL\", 3) returned no items")
	}
	found := false
	for _, it := range items {
		if it.Text == "SELECT" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Complete(\"SEL\", 3) did not surface SELECT; got %+v", items)
	}
}

func TestComplete_Columns(t *testing.T) {
	a := NewAutocompleter()
	a.SetTables([]string{"users"})
	a.SetColumns("users", []string{"id", "name"})

	// Direct GetCompletions with explicit table hint.
	items := a.GetCompletions("na", "users")
	if !containsText(items, "name") {
		t.Errorf("GetCompletions(\"na\", \"users\") did not surface name; got %+v", items)
	}

	// Complete with table-hint extraction: cursor after "u." resolves alias
	// "u" -> "users", prefix "na".
	text := "SELECT u.na FROM users u"
	cursor := len([]rune("SELECT u.na"))
	colItems := a.Complete(text, cursor)
	if !containsText(colItems, "name") {
		t.Errorf("Complete(%q, %d) did not surface column name; got %+v", text, cursor, colItems)
	}
}

func containsText(items []CompletionItem, text string) bool {
	for _, it := range items {
		if it.Text == text {
			return true
		}
	}
	return false
}
