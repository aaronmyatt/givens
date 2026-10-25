package app

import (
	"path/filepath"
	"slices"
	"testing"
)

// shop is the sample shop's capabilities folder.
func shop(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../../testdata/sample-shop/capabilities")
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func TestHelpersInFolderAndCalled(t *testing.T) {
	root := shop(t)
	shared, err := HelpersInFolder(root, "", "")
	if err != nil {
		t.Fatal(err)
	}
	own, err := HelpersInFolder(root, "checkout", "checkout")
	if err != nil {
		t.Fatal(err)
	}
	visible := VisibleHelpers(shared, own)
	shopper := visible["seed_shopper"]
	if shopper.File != "_shared/users.sh" || shopper.Signature != "seed_shopper [email]" ||
		!shopper.Documented() || shopper.Line == 0 {
		t.Errorf("seed_shopper = %+v", shopper)
	}
	if visible["mock_payments"].Capability != "checkout" {
		t.Errorf("mock_payments = %+v", visible["mock_payments"])
	}
	script, err := ReadScript(filepath.Join(root, "checkout/scripts/card-declined.sh"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"mock_payments", "seed_address", "seed_cart", "seed_shopper"}
	if got := HelpersCalled(script, visible); !slices.Equal(got, want) {
		t.Errorf("HelpersCalled = %q, want %q", got, want)
	}
}

func TestNeedsAndFroms(t *testing.T) {
	text := "need bb \"brew install babashka\"\nneed jq\nneed bb\n" +
		"from checkout/card-rejected card=stolen\n  from sign-in/magic-link\n# from x/y\n"
	needs := NeedLines(text)
	if len(needs) != 2 || needs[0] != (NeedLine{"bb", "brew install babashka"}) ||
		needs[1] != (NeedLine{"jq", ""}) {
		t.Errorf("NeedLines = %+v", needs)
	}
	froms := FromLines(text)
	if len(froms) != 2 || froms[0].Key != "checkout/card-rejected" ||
		!slices.Equal(froms[0].Values, []string{"card=stolen"}) ||
		froms[1].Key != "sign-in/magic-link" {
		t.Errorf("FromLines = %+v", froms)
	}
}
