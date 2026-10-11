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
)

var (
	needLine = regexp.MustCompile(needPattern) //tiger:allow-global compiled once
	fromLine = regexp.MustCompile(fromPattern) //tiger:allow-global compiled once
)

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
