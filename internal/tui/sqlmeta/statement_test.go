package sqlmeta

import (
	"reflect"
	"testing"
)

func TestSplitStatements(t *testing.T) {
	cases := []struct {
		name string
		text string
		want []string
	}{
		{"single", "select 1", []string{"select 1"}},
		{"multi with blanks trimmed", "select 1;\n\nselect 2;\n", []string{"select 1", "select 2"}},
		{"semicolon in string is not a split", "select ';' ; select 2", []string{"select ';'", "select 2"}},
		{"empty", "   \n ", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := SplitStatements(c.text); !reflect.DeepEqual(got, c.want) {
				t.Fatalf("SplitStatements(%q) = %#v, want %#v", c.text, got, c.want)
			}
		})
	}
}

func TestStatementBoundsAt(t *testing.T) {
	text := "select 1;\nselect 2;\nselect 3"
	// cursor in the middle statement ("select 2", starts at offset 10)
	start, end := StatementBoundsAt(text, 13)
	if got := text[start:end]; got != "\nselect 2" {
		t.Fatalf("middle bounds = %q, want '\\nselect 2'", got)
	}
	// cursor on a ';' selects the statement it closes
	start, end = StatementBoundsAt(text, 8)
	if got := text[start:end]; got != "select 1" {
		t.Fatalf("on-semicolon bounds = %q, want 'select 1'", got)
	}
}

func TestStatementAt(t *testing.T) {
	cases := []struct {
		name string
		text string
		off  int
		want string
	}{
		{"no separator returns whole text", "select * from users", 3, "select * from users"},
		{"cursor in first statement", "select 1;\nselect 2", 2, "select 1"},
		{"cursor in second statement", "select 1;\nselect 2", 12, "select 2"},
		{"semicolon inside string is not a boundary", "select ';' as x, 2", 15, "select ';' as x, 2"},
		{"semicolon inside line comment is not a boundary", "select 1 -- a; b\nfrom t", 20, "select 1 -- a; b\nfrom t"},
		{"cursor on the semicolon runs the closing statement", "select 1;select 2", 8, "select 1"},
		{"cursor in trailing space after final semicolon walks back", "select 1;  ", 11, "select 1"},
		{"empty buffer", "", 0, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := StatementAt(c.text, c.off); got != c.want {
				t.Fatalf("StatementAt(%q, %d) = %q, want %q", c.text, c.off, got, c.want)
			}
		})
	}
}
