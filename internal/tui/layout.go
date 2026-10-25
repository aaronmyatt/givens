package tui

import (
	"strings"

	"charm.land/lipgloss/v2"

	"givens/internal/assert"
)

// Layout: the cursor's line is drawn in reverse video across the list's width, long lists
// scroll to keep the cursor on show, and the project page sets the scenario beside the list
// when the terminal is wide enough. Text, which the acceptance steps read, draws the same page
// with no styling.

// Limits on the layout.
const (
	// sideBySideColumnsMin is the narrowest terminal that sets details beside the list;
	// narrower, they go under it.
	sideBySideColumnsMin = 100
	// listColumnsMax is the widest the list gets beside the details.
	listColumnsMax = 64
	// nameColumns is a scenario name's room in the list; a longer one ends in "…".
	nameColumns = 34
	// chromeLines are the lines around a page's body: its heading line and the blank under
	// it, the blank and the keys line at the bottom, and a note and its blank.
	chromeLines = 6
	// bodyLinesMin is the fewest body lines drawn, however short the terminal.
	bodyLinesMin = 3
)

// highlighted is the cursor's line: the whole line in reverse video, padded to width, so it
// reads as a bar across the list. Plain, it's the line as it is.
// Ref: https://pkg.go.dev/charm.land/lipgloss/v2#Style.Reverse
func (m *Model) highlighted(line string, width int) string {
	assert.That(width > 0, "a line has room")
	assert.That(!strings.Contains(line, "\n"), "one line is highlighted")
	if m.plain {
		return line
	}
	return lipgloss.NewStyle().Reverse(true).Render(fit(line, width))
}

// cursorLine is a list's line with the cursor's mark: "▸ " and highlighted on the cursor,
// two spaces off it.
func (m *Model) cursorLine(text string, on bool, width int) string {
	assert.That(width > 0, "a line has room")
	assert.That(!strings.Contains(text, "\n"), "a list line is one line")
	if !on {
		return "  " + text
	}
	return m.highlighted("▸ "+text, width)
}

// fit is a line cut to width, ending in "…" when cut, padded with spaces to width.
// Ref: https://pkg.go.dev/charm.land/lipgloss/v2#Width
func fit(line string, width int) string {
	assert.That(width > 0, "a line has room")
	assert.That(!strings.Contains(line, "\n"), "one line is fit")
	if lipgloss.Width(line) > width {
		runes := []rune(line)
		// bound: each pass drops a rune, and line has a bounded number of them.
		for len(runes) > 0 && lipgloss.Width(string(runes))+1 > width {
			runes = runes[:len(runes)-1]
		}
		line = string(runes) + "…"
	}
	return line + strings.Repeat(" ", max(width-lipgloss.Width(line), 0))
}

// wrapped is text broken into lines no wider than width, at spaces where it can.
// Ref: https://pkg.go.dev/charm.land/lipgloss/v2#Wrap
func wrapped(text string, width int) []string {
	assert.That(width > 0, "text has room")
	assert.That(len(text) <= 1<<20, "text stays bounded")
	text = strings.TrimRight(text, "\n")
	if text == "" {
		return []string{}
	}
	return strings.Split(lipgloss.Wrap(text, width, ""), "\n")
}

// scrolledTop is the first line on show of a list of count lines, shown height at a time,
// moved from top as little as keeps cursor on show.
func scrolledTop(cursor int, top int, height int, count int) int {
	assert.That(height > 0, "a list shows a line")
	assert.That(cursor >= 0, "the cursor is on the list")
	top = min(top, cursor)
	top = max(top, cursor-height+1)
	top = min(top, max(count-height, 0))
	return max(top, 0)
}

// bodyLines is how many lines a page's body gets once its chrome and extra lines are drawn.
func (m *Model) bodyLines(extra int) int {
	assert.That(extra >= 0, "extra lines are counted")
	assert.That(m.height > 0, "the terminal has a height")
	return max(m.height-chromeLines-extra, bodyLinesMin)
}

// besideOrUnder sets details beside the list, a column each, or under it on a narrow
// terminal. Both are cut to height lines.
// Ref: https://pkg.go.dev/charm.land/lipgloss/v2#JoinHorizontal
func (m *Model) besideOrUnder(list []string, details []string, height int) string {
	assert.That(height > 0, "the body has a line")
	assert.That(len(list) <= height, "the list was scrolled to fit")
	if m.width < sideBySideColumnsMin {
		room := max(height-len(list)-1, 0)
		return lines(list, []string{""}, details[:min(len(details), room)])
	}
	page := make([]string, 0, height)
	for index := range height {
		left := ""
		if index < len(list) {
			left = list[index] // the cursor's line is already listWidth wide: fit leaves it
		}
		left = fit(left, m.listWidth())
		right := ""
		if index < len(details) {
			right = details[index]
		}
		page = append(page, strings.TrimRight(left+" │ "+right, " "))
	}
	return lines(page)
}

// detailsWidth is the details' room: beside the list, or the whole width under it.
func (m *Model) detailsWidth() int {
	assert.That(m.width >= 40, "the view has room")
	assert.That(listColumnsMax < sideBySideColumnsMin, "the list leaves room beside it")
	if m.width < sideBySideColumnsMin {
		return m.width
	}
	return m.width - m.listWidth() - 3
}

// listWidth is the list's room: beside the details, or the whole width over them.
func (m *Model) listWidth() int {
	assert.That(m.width >= 40, "the view has room")
	assert.That(listColumnsMax > nameColumns, "a name fits the list")
	if m.width < sideBySideColumnsMin {
		return m.width
	}
	return min(listColumnsMax, m.width*55/100)
}

// cursorAfter is where a moving key puts a cursor on a list of count rows: j and ↓ down, k
// and ↑ up, each wrapping round at the ends; g and Home to the top, G and End to the bottom.
func cursorAfter(key string, cursor int, count int) int {
	assert.That(count > 0, "the list has a row")
	assert.That(cursor >= 0, "the cursor is on the list")
	cursor = min(cursor, count-1)
	switch key {
	case "j", "down":
		return (cursor + 1) % count
	case "k", "up":
		return (cursor - 1 + count) % count
	case "g", "home":
		return 0
	case "G", "end":
		return count - 1
	}
	return cursor
}
