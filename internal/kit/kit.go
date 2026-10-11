// Package kit holds the kit, givens.sh, built into the givens binary. New projects and kit
// updates get this copy; acceptance/world copies the same file into each sample shop.
//
// The kit's first line names its version, like "# givens kit 2". Update the kit compares
// versions; see capabilities/update-the-kit.
package kit

import (
	_ "embed"
	"strconv"
	"strings"

	"givens/internal/assert"
)

// versionPrefix starts the kit's first line.
const versionPrefix = "# givens kit "

// versionMax bounds a version number. Tiger Style: put a limit on everything.
const versionMax = 1 << 20

// Script is givens.sh as Givens ships it.
// Ref: https://pkg.go.dev/embed
//
//go:embed givens.sh
var Script string //tiger:allow-global go:embed target

// Version reads the version from a kit's first line. It returns false when the first line
// isn't a version line, as in a kit edited past recognition.
func Version(text string) (int, bool) {
	assert.That(len(text) < 1<<24, "a kit is a small file")
	first, _, _ := strings.Cut(text, "\n")
	number, ok := strings.CutPrefix(strings.TrimSpace(first), versionPrefix)
	if !ok {
		return 0, false
	}
	// Atoi rejects signs only when asked to; a version is a plain positive number.
	// Ref: https://pkg.go.dev/strconv#Atoi
	version, err := strconv.Atoi(number)
	if err != nil || version < 1 || version > versionMax || strings.HasPrefix(number, "+") {
		return 0, false
	}
	assert.That(version >= 1, "a read version is positive")
	return version, true
}

// ShippedVersion is the version of the kit built into this Givens.
func ShippedVersion() int {
	version, ok := Version(Script)
	assert.That(ok, "the shipped kit starts with a version line")
	assert.That(version >= 1, "the shipped kit has a positive version")
	return version
}
