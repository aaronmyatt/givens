package slug

import "testing"

// Table-driven test: each row is one name and the slug it must give.
// Ref: https://go.dev/wiki/TableDrivenTests
func TestOf(t *testing.T) {
	cases := []struct{ name, want string }{
		{"Card declined", "card-declined"},
		{"Pay with saved card", "pay-with-saved-card"},
		{"3-D Secure challenge", "3-d-secure-challenge"},
		{"Card declined!", "card-declined"},
		{"Carte refusée", "carte-refusée"},
		{"A line that isn't valid Gherkin", "a-line-that-isn-t-valid-gherkin"},
		{"  --  ", ""},
	}
	for _, c := range cases {
		if got := Of(c.name); got != c.want {
			t.Errorf("Of(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}
