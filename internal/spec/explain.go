package spec

import (
	"regexp"
	"strconv"
	"strings"

	"givens/internal/assert"
)

// parseErrorLine matches one error in the Gherkin parser's message:
// "(17:5): expected: #EOF, #StepLine, …, got 'Check the order is pending'".
// Ref: https://github.com/cucumber/gherkin/blob/main/go/errors.go
const parseErrorPattern = `\((\d+):\d+\): (?:expected: ([^\n]*?), got '[^\n]*'|([^\n]*))`

var parseErrorLine = regexp.MustCompile(parseErrorPattern) //tiger:allow-global compiled once

// expectedWords says, in plain words and in this order, the tokens worth naming. Tokens a
// person wouldn't type at that point, like #EOF or #TableRow, are left out.
func expectedWords() [][2]string {
	words := [][2]string{
		{"#StepLine", "a step"},
		{"#TagLine", "a tag"},
		{"#Comment", "a comment"},
		{"#ScenarioLine", "a new scenario"},
		{"#FeatureLine", "Feature:"},
	}
	assert.That(len(words) <= 8, "joinOr takes a short list")
	assert.That(words[0][0] == "#StepLine", "a step is named first")
	return words
}

// Explain turns the Gherkin parser's error into the first line it names and a message a
// person can act on: "expected a step, a tag, a comment or a new scenario". A message it can't
// read is passed on as it is, at line 0.
func Explain(err error) (int64, string) {
	assert.That(err != nil, "only an error is explained")
	assert.That(err.Error() != "", "an error has a message")
	match := parseErrorLine.FindStringSubmatch(err.Error())
	if match == nil {
		return 0, err.Error()
	}
	line, convertErr := strconv.ParseInt(match[1], 10, 64)
	if convertErr != nil {
		return 0, err.Error()
	}
	if match[2] == "" {
		return line, match[3]
	}
	tokens := strings.Split(match[2], ", ")
	words := make([]string, 0, len(tokens))
	for _, pair := range expectedWords() {
		for _, token := range tokens {
			if token == pair[0] {
				words = append(words, pair[1])
			}
		}
	}
	if len(words) == 0 {
		return line, "this line isn't valid Gherkin"
	}
	return line, "expected " + joinOr(words)
}

// joinOr joins words as "a, b or c".
func joinOr(words []string) string {
	assert.That(len(words) > 0, "there are words to join")
	assert.That(len(words) <= 8, "a short list")
	if len(words) == 1 {
		return words[0]
	}
	return strings.Join(words[:len(words)-1], ", ") + " or " + words[len(words)-1]
}
