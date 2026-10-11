package dropintoascenario

import (
	"fmt"
	"path/filepath"
	"strings"

	messages "github.com/cucumber/messages/go/v34"

	"givens/internal/spec"
)

const (
	checkoutFeature = "capabilities/checkout/checkout.feature"
	outlineName     = "Card rejected by the bank"
	outlineRunKey   = "checkout/card-rejected-by-the-bank/expired-and-stolen-cards"
)

// outlineSteps are the steps of "Card rejected by the bank", with placeholders for each row.
const outlineSteps = `  Scenario Outline: Card rejected by the bank
    Given a signed-in shopper whose card is <card>
    When they place the order
    Then they see "<message>"
`

// outlineExamples is one named Examples block holding both rows.
const outlineExamples = `
    Examples: Expired and stolen cards
      | card    | message                       |
      | expired | Your card has expired         |
      | stolen  | Your card was reported stolen |
`

// unnamedExamples is two Examples blocks with no names, one row each.
const unnamedExamples = `
    Examples:
      | card    | message               |
      | expired | Your card has expired |

    Examples:
      | card   | message                       |
      | stolen | Your card was reported stolen |
`

// rowMessages is the message column of each row, by its card value.
func rowMessages() map[string]string {
	return map[string]string{
		"expired": "Your card has expired",
		"stolen":  "Your card was reported stolen",
	}
}

// writeOutline puts "Card rejected by the bank" in checkout.feature, replacing any outline of
// that name the fixture has, so the rows are the ones these steps check. Its setup records the
// row it ran for in shop-data/ex-card.
func (s *state) writeOutline(named bool) error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	text, err := s.ReadFile(checkoutFeature)
	if err != nil {
		return err
	}
	examples := outlineExamples
	if !named {
		examples = unnamedExamples
	}
	text = removeScenario(text, outlineName)
	text = strings.TrimRight(text, "\n") + "\n\n" + outlineSteps + examples
	if err := s.WriteFile(checkoutFeature, text); err != nil {
		return err
	}
	s.markers = append(s.markers, "ex-card")
	return s.writeSetup(outlineName, script(
		`printf '%s' "$GIVENS_EX_card" > `+shopData+`/ex-card"`,
		`ok "card set to $GIVENS_EX_card"`,
	))
}

// removeScenario removes the named scenario or outline, with its tags, from a feature's text.
func removeScenario(text string, name string) string {
	lines := strings.Split(text, "\n")
	start := -1
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "Scenario: "+name || trimmed == "Scenario Outline: "+name {
			start = index
			break
		}
	}
	if start < 0 {
		return text
	}
	end := start + 1
	for end < len(lines) && !startsBlock(lines[end]) { // bound: len(lines)
		end++
	}
	for start > 0 && strings.HasPrefix(strings.TrimSpace(lines[start-1]), "@") { // bound: start
		start--
	}
	kept := append(append([]string{}, lines[:start]...), lines[end:]...)
	return strings.Join(kept, "\n")
}

// startsBlock reports whether a feature line starts the next scenario, its tags, or a Rule.
func startsBlock(line string) bool {
	trimmed := strings.TrimSpace(line)
	for _, prefix := range []string{"Scenario", "@", "Rule:", "Background:"} {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	return false
}

// setTimeoutTag replaces the named checkout scenario's @timeout tag with tag, or removes it
// when tag is "".
func (s *state) setTimeoutTag(name string, tag string) error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	text, err := s.ReadFile(checkoutFeature)
	if err != nil {
		return err
	}
	lines := strings.Split(text, "\n")
	at := -1
	for index, line := range lines {
		if strings.TrimSpace(line) == "Scenario: "+name {
			at = index
		}
	}
	if at < 0 {
		return fmt.Errorf("checkout.feature has no scenario %q", name)
	}
	tags := make([]string, 0, 4)
	if at > 0 && strings.HasPrefix(strings.TrimSpace(lines[at-1]), "@") {
		tags = strings.Fields(lines[at-1])
		lines = append(lines[:at-1], lines[at:]...)
		at--
	}
	kept := make([]string, 0, len(tags)+1)
	for _, existing := range tags {
		if !strings.HasPrefix(existing, "@timeout") {
			kept = append(kept, existing)
		}
	}
	if tag != "" {
		kept = append(kept, tag)
	}
	if len(kept) > 0 {
		tagLine := "  " + strings.Join(kept, " ")
		lines = append(lines[:at], append([]string{tagLine}, lines[at:]...)...)
	}
	return s.WriteFile(checkoutFeature, strings.Join(lines, "\n"))
}

// expectedChecks reads the named scenario's When and Then steps from the sample shop's copy,
// "And" and "But" steps counted with the step before them.
func (s *state) expectedChecks(name string) ([]string, []string, error) {
	capability, err := s.ScenarioCapability(name)
	if err != nil {
		return nil, nil, err
	}
	path := filepath.Join(s.Capabilities(), capability, capability+".feature")
	scenarios, err := spec.ScenariosIn(path)
	if err != nil {
		return nil, nil, err
	}
	for _, scenario := range scenarios {
		if scenario.Name == name {
			whens, thens := splitSteps(scenario.Steps)
			return whens, thens, nil
		}
	}
	return nil, nil, fmt.Errorf("%s has no scenario %q", path, name)
}

// splitSteps sorts steps into Whens and Thens by keyword type.
// Ref: https://pkg.go.dev/github.com/cucumber/messages/go/v34#StepKeywordType
func splitSteps(steps []*messages.Step) ([]string, []string) {
	whens := make([]string, 0, len(steps))
	thens := make([]string, 0, len(steps))
	kind := messages.StepKeywordType_CONTEXT
	for _, step := range steps {
		if step.KeywordType != messages.StepKeywordType_CONJUNCTION {
			kind = step.KeywordType
		}
		switch kind {
		case messages.StepKeywordType_ACTION:
			whens = append(whens, step.Text)
		case messages.StepKeywordType_OUTCOME:
			thens = append(thens, step.Text)
		}
	}
	return whens, thens
}
