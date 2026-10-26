package app

import "testing"

// Table-driven test: each row is a CLAUDE.md before givens skill and what it must become.
// Ref: https://go.dev/wiki/TableDrivenTests
func TestWithGivensParagraph(t *testing.T) {
	own := "# Shop\n\nRun npm test.\n"
	cases := []struct{ name, claude, want string }{
		{"no CLAUDE.md", "", givensParagraph},
		{"after the project's own", own, own + "\n" + givensParagraph},
		{"replaces an old one, keeps what follows",
			own + "\n## Givens\n\nOld words.\n\n## Style\n\nTabs.\n",
			own + "\n" + givensParagraph + "\n## Style\n\nTabs.\n"},
		{"a deeper heading is part of it",
			"## Givens\n\nOld.\n\n### More\n\nOld too.\n", givensParagraph},
	}
	for _, c := range cases {
		got := withGivensParagraph(c.claude)
		if got != c.want {
			t.Errorf("%s: got\n%q\nwant\n%q", c.name, got, c.want)
		}
		if again := withGivensParagraph(got); again != got {
			t.Errorf("%s: a second run changed it to\n%q", c.name, again)
		}
	}
}
