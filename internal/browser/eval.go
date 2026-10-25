package browser

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/chromedp"

	"givens/internal/assert"
)

// Eval runs JavaScript in the project's page, waits for a promise it returns to settle, and
// says what it returned: a string as it is, anything else as JSON, undefined as "". A thrown
// error comes back as its message, without the "TypeError: " before it. Needed by in_browser.
// Ref: https://chromedevtools.github.io/devtools-protocol/tot/Runtime/#method-evaluate
func Eval(profile string, js string) (string, error) {
	assert.That(filepath.IsAbs(profile), "the profile is absolute")
	assert.That(js != "", "there's JavaScript to run")
	return evaluate(profile, js, EvalSecondsMax*time.Second)
}

// evaluate is Eval within limit; the unit tests use a short one.
func evaluate(profile string, js string, limit time.Duration) (string, error) {
	assert.That(filepath.IsAbs(profile), "the profile is absolute")
	assert.That(limit > 0, "an evaluation has a limit")
	raw, err := onPage(profile, chromedp.Evaluate[[]byte](js, chromedp.EvalAwaitPromise), limit)
	var thrown *chromedp.ExceptionError
	switch {
	case errors.As(err, &thrown):
		return "", errors.New(thrownMessage(thrown))
	case errors.Is(err, context.DeadlineExceeded):
		return "", ErrNoAnswer
	case err != nil:
		return "", err
	}
	return printed(raw)
}

// printed is a result as in_browser prints it, so X="$(in_browser …)" holds a string as is.
func printed(raw []byte) (string, error) {
	assert.That(ResultBytesMax > 0, "results have a limit")
	assert.That(raw == nil || len(raw) > 0, "a result is JSON or nothing")
	if len(raw) > ResultBytesMax {
		return "", fmt.Errorf("the result is %d bytes, more than %d", len(raw), ResultBytesMax)
	}
	if raw == nil {
		return "", nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text, nil
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err != nil {
		return "", err
	}
	return compact.String(), nil
}

// thrownMessage is the first line of what was thrown, without an error class before it:
// "TypeError: window.app.nope is not a function" says "window.app.nope is not a function".
func thrownMessage(thrown *chromedp.ExceptionError) string {
	assert.That(thrown != nil, "something was thrown")
	assert.That(thrown.ExceptionDetails != nil, "a throw has details")
	message := thrown.Text
	if thrown.Exception != nil && thrown.Exception.Description != "" {
		message = thrown.Exception.Description
	}
	message, _, _ = strings.Cut(message, "\n")
	if class, rest, ok := strings.Cut(message, ": "); ok && strings.HasSuffix(class, "Error") &&
		!strings.ContainsAny(class, " .") {
		message = rest
	}
	return strings.TrimPrefix(message, "Uncaught ")
}

// Action calls window.app.<name> in the project's page, with argument as JSON ("" for none),
// and says what it returned, as Eval does. A page without that function is ErrNoAction.
// Needed by app_action.
// Ref: https://www.cypress.io/blog/stop-using-page-objects-and-start-using-app-actions
func Action(profile string, name string, argument string) (string, error) {
	assert.That(filepath.IsAbs(profile), "the profile is absolute")
	assert.That(len(name) < 256, "an action's name is short")
	if !isIdentifier(name) {
		return "", fmt.Errorf("%q isn't a JavaScript name", name)
	}
	if argument != "" && !json.Valid([]byte(argument)) {
		return "", fmt.Errorf("the argument to %s isn't JSON: %s", name, argument)
	}
	check := `typeof window.app?.` + name + ` === "function"`
	found, err := onPage(profile, chromedp.Evaluate[bool](check), EvalSecondsMax*time.Second)
	if err != nil {
		return "", err
	}
	if !found {
		return "", missingAction{name: name}
	}
	return Eval(profile, "window.app."+name+"("+argument+")")
}

// isIdentifier is true for a plain JavaScript name: letters, digits, _ and $, not starting
// with a digit. Ref: https://tc39.es/ecma262/#prod-IdentifierName
func isIdentifier(name string) bool {
	assert.That(len(name) < 256, "a name is short")
	if name == "" || (name[0] >= '0' && name[0] <= '9') {
		return false
	}
	for _, letter := range name {
		plain := letter == '_' || letter == '$' || (letter >= 'a' && letter <= 'z') ||
			(letter >= 'A' && letter <= 'Z') || (letter >= '0' && letter <= '9')
		if !plain {
			return false
		}
	}
	assert.That(!strings.ContainsAny(name, " .()"), "a plain name can't reach past itself")
	return true
}

// missingAction is ErrNoAction for one name, said as "the page has no window.app.<name>".
type missingAction struct {
	name string
}

// Error says which action the page lacks.
func (m missingAction) Error() string {
	assert.That(m.name != "", "an action has a name")
	assert.That(isIdentifier(m.name), "only a checked name is missing")
	return "the page has no window.app." + m.name
}

// Is makes errors.Is(err, ErrNoAction) true. Ref: https://pkg.go.dev/errors#Is
func (m missingAction) Is(target error) bool {
	assert.That(m.name != "", "an action has a name")
	assert.That(ErrNoAction != nil, "the sentinel is set")
	return target == ErrNoAction
}
