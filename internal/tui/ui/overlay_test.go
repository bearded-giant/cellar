package ui

import (
	"regexp"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

var ansiRe = regexp.MustCompile("\x1b\\[[0-9;]*m")

func stripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

// testBox is a 5x3 bordered box: ┌───┐ / │BOX│ / └───┘.
func testBox() string {
	return lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Render("BOX")
}

func repeatLines(line string, n int) string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = line
	}
	return strings.Join(lines, "\n")
}

func TestOverlay_Geometry(t *testing.T) {
	box := testBox()
	tests := []struct {
		name         string
		base         string
		termW, termH int
		x, y         int
		center       bool
		want         []string // stripped, right-trimmed lines
	}{
		{
			name:  "plain base centered",
			base:  repeatLines("abcdefghij", 5),
			termW: 10, termH: 5, center: true,
			want: []string{
				"abcdefghij",
				"ab┌───┐hij",
				"ab│BOX│hij",
				"ab└───┘hij",
				"abcdefghij",
			},
		},
		{
			name:  "unicode base splice at rune columns",
			base:  repeatLines("héllöwörld", 5),
			termW: 10, termH: 5, center: true,
			want: []string{
				"héllöwörld",
				"hé┌───┐rld",
				"hé│BOX│rld",
				"hé└───┘rld",
				"héllöwörld",
			},
		},
		{
			name:  "box wider than base clamps to left edge and clips",
			base:  repeatLines("ab", 3),
			termW: 4, termH: 3, center: true,
			want: []string{
				"┌───",
				"│BOX",
				"└───",
			},
		},
		{
			name:  "box taller than base clamps to top and clips",
			base:  "abcdefghij",
			termW: 10, termH: 2, center: true,
			want: []string{
				"ab┌───┐hij",
				"  │BOX│",
			},
		},
		{
			name:  "negative position clamps to origin",
			base:  repeatLines("abcdefghij", 4),
			termW: 10, termH: 4, x: -3, y: -2,
			want: []string{
				"┌───┐fghij",
				"│BOX│fghij",
				"└───┘fghij",
				"abcdefghij",
			},
		},
		{
			name:  "base narrower than box start pads the gap",
			base:  repeatLines("ab", 4),
			termW: 12, termH: 4, x: 5, y: 0,
			want: []string{
				"ab   ┌───┐",
				"ab   │BOX│",
				"ab   └───┘",
				"ab",
			},
		},
		{
			name:  "base shorter than canvas keeps full height",
			base:  "abcdefghij",
			termW: 10, termH: 6, x: 2, y: 3,
			want: []string{
				"abcdefghij",
				"",
				"",
				"  ┌───┐",
				"  │BOX│",
				"  └───┘",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out string
			if tt.center {
				out = overlayCenter(tt.base, box, tt.termW, tt.termH)
			} else {
				out = overlayAt(tt.base, box, tt.termW, tt.termH, tt.x, tt.y)
			}
			lines := strings.Split(out, "\n")
			if len(lines) != tt.termH {
				t.Fatalf("composed height = %d, want %d:\n%s", len(lines), tt.termH, out)
			}
			for i, l := range lines {
				got := strings.TrimRight(stripANSI(l), " ")
				if got != tt.want[i] {
					t.Errorf("line %d = %q, want %q", i, got, tt.want[i])
				}
			}
		})
	}
}

func TestOverlay_StyledBaseSurvivesSplice(t *testing.T) {
	base := repeatLines(accentStyle.Render("abcde")+errorStyle.Render("fghij"), 5)
	out := overlayCenter(base, testBox(), 10, 5)
	lines := strings.Split(out, "\n")
	if len(lines) != 5 {
		t.Fatalf("composed height = %d, want 5", len(lines))
	}
	if !strings.Contains(out, "\x1b[") {
		t.Error("styled base should keep ANSI sequences in the composition")
	}
	if got := strings.TrimRight(stripANSI(lines[2]), " "); got != "ab│BOX│hij" {
		t.Errorf("styled cut zone = %q, want ab│BOX│hij", got)
	}
	for _, l := range lines {
		if strings.Contains(stripANSI(l), "\x1b") {
			t.Errorf("unstripped escape residue in %q", l)
		}
	}
}

func TestOverlay_StyledBoxKeepsStyle(t *testing.T) {
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("39")).
		Render("hi")
	out := overlayCenter(repeatLines(strings.Repeat("x", 12), 6), box, 12, 6)
	if !strings.Contains(stripANSI(out), "│hi│") {
		t.Errorf("box content missing from composition:\n%s", stripANSI(out))
	}
	if !strings.Contains(out, "\x1b[") {
		t.Error("box border styling should survive compositing")
	}
}

func TestOverlay_TinyCanvasReturnsBase(t *testing.T) {
	if got := overlayAt("base", "box", 0, 5, 0, 0); got != "base" {
		t.Errorf("zero-width canvas should return base, got %q", got)
	}
	if got := overlayCenter("base", "box", 10, 0); got != "base" {
		t.Errorf("zero-height canvas should return base, got %q", got)
	}
}
