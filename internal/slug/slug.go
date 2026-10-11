// Package slug turns names into the slugs that name files and run keys.
//
// The rule (capabilities/GIVENS.md, "Names and slugs"): lower case; letters and digits in any
// language are kept; every run of other characters becomes one hyphen; hyphens are trimmed from
// the ends. "Card declined" gives "card-declined"; "Carte refusée" gives "carte-refusée".
package slug

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"givens/internal/assert"
)

// NameLengthMax bounds a name in bytes. Tiger Style: put a limit on everything.
// Ref: https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md#safety
const NameLengthMax = 1024

// Of returns the slug of name. A name with no letters or digits gives "".
func Of(name string) string {
	assert.That(utf8.ValidString(name), "a name is valid UTF-8")
	assert.That(len(name) <= NameLengthMax, "a name fits in NameLengthMax bytes")

	var out strings.Builder
	out.Grow(len(name))
	hyphen := false
	// A range loop is bounded by len(name). unicode.IsLetter/IsDigit cover every script.
	// Ref: https://pkg.go.dev/unicode#IsLetter
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if hyphen && out.Len() > 0 {
				out.WriteByte('-')
			}
			out.WriteRune(unicode.ToLower(r))
			hyphen = false
		} else {
			hyphen = true
		}
	}

	result := out.String()
	assert.That(!strings.HasPrefix(result, "-"), "a slug never starts with a hyphen")
	assert.That(!strings.HasSuffix(result, "-"), "a slug never ends with a hyphen")
	return result
}
