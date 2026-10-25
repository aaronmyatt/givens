package app

import "testing"

func TestTagExpressionMatch(t *testing.T) {
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
		expression, err := ParseTagExpression(c.expression)
		if err != nil {
			t.Fatalf("ParseTagExpression(%q): %v", c.expression, err)
		}
		if got := expression.Match(c.tags); got != c.want {
			t.Errorf("%q on %q = %v, want %v", c.expression, c.tags, got, c.want)
		}
	}
}

func TestParseTagExpressionErrors(t *testing.T) {
	for _, text := range []string{"@a and", "and @a", "@a @b", "(@a", "@a)", "error", "not"} {
		if _, err := ParseTagExpression(text); err == nil {
			t.Errorf("ParseTagExpression(%q) parsed", text)
		}
	}
}

func TestParseTagExpressionErrorMessage(t *testing.T) {
	_, err := ParseTagExpression("@error and")
	if err == nil || err.Error() != "not a tag expression: expected a tag after 'and'" {
		t.Errorf("ParseTagExpression(\"@error and\") = %v", err)
	}
}
