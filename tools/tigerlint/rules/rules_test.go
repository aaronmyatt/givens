package rules

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

// TestRules runs every rule over its own package in testdata/src/givens/<rule>. Each `// want`
// comment there is a diagnostic the rule must report on that line; any other diagnostic fails
// the test. The packages sit under givens/ so they may import the stand-in
// givens/internal/assert: Go only lets a module import its own internal packages.
// Ref: https://pkg.go.dev/golang.org/x/tools/go/analysis/analysistest#Run
func TestRules(t *testing.T) {
	dir := analysistest.TestData()
	for _, analyzer := range All() {
		t.Run(analyzer.Name, func(t *testing.T) {
			analysistest.Run(t, dir, analyzer, "givens/"+analyzer.Name)
		})
	}
}
