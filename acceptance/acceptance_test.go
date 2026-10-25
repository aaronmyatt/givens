package acceptance

import (
	"path/filepath"
	"strings"
	"testing"

	"givens/internal/app"
)

// capabilitiesDir is Givens's own spec, relative to this package. Each capability's scenarios
// run in their own suite, in acceptance/<capability>/.
const capabilitiesDir = "../capabilities"

// TestSpecConventions checks Givens's own capabilities, the embedded starter capability and the
// sample shop fixture against the conventions in capabilities/GIVENS.md.
func TestSpecConventions(t *testing.T) {
	roots := []string{
		capabilitiesDir, "../internal/starter", "../testdata/sample-shop/capabilities",
	}
	for _, root := range roots {
		problems, err := app.CheckCapabilities(root)
		if err != nil {
			t.Fatalf("reading %s: %v", root, err)
		}
		for _, problem := range problems {
			t.Errorf("%s", problem)
		}
	}
}

// TestWorkInProgress reports how many scenarios still carry @wip. It never fails: it's the
// progress meter for the backlog in todo.txt. See it with `go test ./acceptance -v -run WorkIn`.
func TestWorkInProgress(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join(capabilitiesDir, "*", "*.feature"))
	if err != nil {
		t.Fatal(err)
	}
	wip, total := 0, 0
	for _, path := range paths {
		scenarios, err := app.ScenariosIn(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, scenario := range scenarios {
			total++
			for _, tag := range scenario.Tags {
				if strings.EqualFold(tag.Name, "@wip") {
					wip++
				}
			}
		}
	}
	t.Logf("%d of %d scenarios still tagged @wip", wip, total)
}
