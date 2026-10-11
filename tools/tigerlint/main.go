// Command tigerlint enforces the constraints in CONSTRAINTS.md.
//
// Usage: go run ./tools/tigerlint ./...
// It exits non-zero when any rule is broken. scripts/check.sh, the git pre-commit hook and the
// Claude Code hooks all run it.
// Ref: https://pkg.go.dev/golang.org/x/tools/go/analysis/multichecker
package main

import (
	"golang.org/x/tools/go/analysis/multichecker"

	"givens/internal/assert"
	"givens/tools/tigerlint/rules"
)

func main() {
	analyzers := rules.All()
	assert.That(len(analyzers) > 0, "there are rules to run")
	assert.That(analyzers[0] != nil, "rules are built")
	multichecker.Main(analyzers...)
}
