package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

// The cursor's line is a reverse-video bar the list's full width; Text draws it plain.
func TestCursorLineHighlightsTheWholeLine(t *testing.T) {
	model := &Model{width: 80, height: 24}
	line := model.cursorLine("Card declined", true, 40)
	// SGR 7 is reverse video. Ref: https://en.wikipedia.org/wiki/ANSI_escape_code#SGR
	if !strings.Contains(line, "\x1b[7m") {
		t.Fatalf("the cursor's line isn't in reverse video: %q", line)
	}
	if width := lipgloss.Width(line); width != 40 {
		t.Fatalf("the bar is %d columns wide, want 40", width)
	}
	model.plain = true
	if line := model.cursorLine("Card declined", true, 40); line != "▸ Card declined" {
		t.Fatalf("plain, the cursor's line is %q", line)
	}
	if line := model.cursorLine("Card declined", false, 40); line != "  Card declined" {
		t.Fatalf("off the cursor, the line is %q", line)
	}
}

// The list scrolls as little as keeps the cursor on show.
func TestScrolledTop(t *testing.T) {
	cases := []struct{ cursor, top, height, count, want int }{
		{cursor: 0, top: 0, height: 5, count: 12, want: 0},
		{cursor: 4, top: 0, height: 5, count: 12, want: 0},
		{cursor: 5, top: 0, height: 5, count: 12, want: 1},
		{cursor: 11, top: 0, height: 5, count: 12, want: 7},
		{cursor: 3, top: 7, height: 5, count: 12, want: 3},
		{cursor: 2, top: 9, height: 20, count: 12, want: 0},
	}
	for _, c := range cases {
		if got := scrolledTop(c.cursor, c.top, c.height, c.count); got != c.want {
			t.Errorf("scrolledTop(%d, %d, %d, %d) = %d, want %d", c.cursor, c.top, c.height,
				c.count, got, c.want)
		}
	}
}

// A long line is cut with "…"; a short one is padded.
func TestFit(t *testing.T) {
	if got := fit("Pay with a saved card", 10); got != "Pay with …" {
		t.Errorf("fit cut to %q", got)
	}
	if got := fit("Pay", 6); got != "Pay   " {
		t.Errorf("fit padded to %q", got)
	}
}

// j and k wrap round at the ends; g and G jump to them.
func TestCursorAfter(t *testing.T) {
	cases := []struct {
		key           string
		cursor, count int
		want          int
	}{
		{key: "j", cursor: 0, count: 3, want: 1},
		{key: "j", cursor: 2, count: 3, want: 0},
		{key: "k", cursor: 0, count: 3, want: 2},
		{key: "up", cursor: 1, count: 3, want: 0},
		{key: "g", cursor: 2, count: 3, want: 0},
		{key: "G", cursor: 0, count: 3, want: 2},
		{key: "j", cursor: 9, count: 3, want: 0},
	}
	for _, c := range cases {
		if got := cursorAfter(c.key, c.cursor, c.count); got != c.want {
			t.Errorf("cursorAfter(%q, %d, %d) = %d, want %d", c.key, c.cursor, c.count, got,
				c.want)
		}
	}
}
