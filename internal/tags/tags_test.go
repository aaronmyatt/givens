package tags

import "testing"

func TestMatch(t *testing.T) {
	cases := []struct {
		expression string
		tags       []string
		want       bool
	}{
		{"", nil, true},
		{"@error", []string{"@error"}, true},
		{"@error", []string{"@slow"}, false},
		{"not @error", []string{"@slow"}, true},
		{"@error and @payments", []string{"@error", "@payments"}, true},
		{"@error and not @payments", []string{"@error", "@payments"}, false},
		{"@a or @b and @c", []string{"@a"}, true},
		{"(@a or @b) and @c", []string{"@a"}, false},
		{"not (@a or @b)", []string{"@c"}, true},
	}
	for _, c := range cases {
		expression, err := Parse(c.expression)
		if err != nil {
			t.Fatalf("Parse(%q): %v", c.expression, err)
		}
		if got := expression.Match(c.tags); got != c.want {
			t.Errorf("%q on %q = %v, want %v", c.expression, c.tags, got, c.want)
		}
	}
}

func TestParseErrors(t *testing.T) {
	for _, text := range []string{"@a and", "and @a", "@a @b", "(@a", "@a)", "error", "not"} {
		if _, err := Parse(text); err == nil {
			t.Errorf("Parse(%q) parsed", text)
		}
	}
}
