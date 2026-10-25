package helpers

import (
	"regexp"
	"strings"

	"givens/internal/assert"
)

// Patterns for the kit's need and from lines.
// Ref: capabilities/the-kit/about.md, in the Givens repository
const (
	needPattern = `(?m)^\s*need\s+([A-Za-z0-9_.+-]+)(?:\s+"([^"]*)"|\s+'([^']*)')?`
	fromPattern = `(?m)^\s*from\s+([^\s/]+/[^\s/]+)((?:[ \t]+[A-Za-z_]\w*=\S*)*)[ \t]*$`
	// browserPattern is a line calling one of the kit's browser functions, before any #.
	browserPattern = `(?m)^[^#\n]*\b(browser_open|in_browser|app_action)\b`
)

var (
	needLine    = regexp.MustCompile(needPattern)    //tiger:allow-global compiled once
	fromLine    = regexp.MustCompile(fromPattern)    //tiger:allow-global compiled once
	browserLine = regexp.MustCompile(browserPattern) //tiger:allow-global compiled once
)

// UsesBrowser is true when text calls browser_open, in_browser or app_action, which need
// Chrome. See capabilities/mix-shell-browser-and-hand-steps.
func UsesBrowser(text string) bool {
	assert.That(len(text) <= FileSizeBytesMax*HelpersMax, "text stays bounded")
	assert.That(browserLine != nil, "the pattern is compiled")
	return browserLine.MatchString(text)
}

// Need is one `need <command> [hint]` line.
type Need struct {
	Command string
	Hint    string
}

// Needs lists the need lines in text, in order, each command once.
func Needs(text string) []Need {
	assert.That(len(text) <= FileSizeBytesMax*HelpersMax, "text stays bounded")
	matches := needLine.FindAllStringSubmatch(text, -1)
	needs := make([]Need, 0, len(matches))
	seen := make(map[string]bool, len(matches))
	for _, match := range matches {
		if seen[match[1]] {
			continue
		}
		seen[match[1]] = true
		needs = append(needs, Need{Command: match[1], Hint: match[2] + match[3]})
	}
	assert.That(len(needs) <= len(matches), "each need comes from a line")
	return needs
}

// From is one `from <capability>/<slug> [column=value …]` line.
type From struct {
	Key    string   // "checkout/pay-with-saved-card"
	Values []string // "card=stolen", in order
	Line   string   // the line as written, trimmed
}

// Froms lists the from lines in a setup script, in order.
func Froms(text string) []From {
	assert.That(len(text) <= FileSizeBytesMax, "a script stays bounded")
	matches := fromLine.FindAllStringSubmatch(text, -1)
	froms := make([]From, 0, len(matches))
	for _, match := range matches {
		froms = append(froms, From{
			Key: match[1], Values: strings.Fields(match[2]),
			Line: strings.TrimSpace(match[0]),
		})
	}
	assert.That(len(froms) == len(matches), "each match is a from line")
	return froms
}
