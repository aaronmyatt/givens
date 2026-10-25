package app

import (
	"errors"
	"testing"
)

func TestExplainParseError(t *testing.T) {
	cases := []struct {
		message string
		line    int64
		want    string
	}{
		{"Parser errors:\n(17:5): expected: #EOF, #TableRow, #DocStringSeparator, #StepLine, " +
			"#TagLine, #ExamplesLine, #ScenarioLine, #RuleLine, #Comment, #Empty, got 'Check'",
			17, "expected a step, a tag, a comment or a new scenario"},
		{"Parser errors:\n(1:1): expected: #EOF, #Language, #TagLine, #FeatureLine, #Comment, " +
			"#Empty, got 'Scenario: Orphan'", 1, "expected a tag, a comment or Feature:"},
		{"(3:7): inconsistent cell count within the table", 3,
			"inconsistent cell count within the table"},
		{"no Feature", 0, "no Feature"},
	}
	for _, c := range cases {
		line, got := ExplainParseError(errors.New(c.message))
		if line != c.line || got != c.want {
			t.Errorf("Explain(%q) = %d, %q; want %d, %q", c.message, line, got, c.line, c.want)
		}
	}
}
