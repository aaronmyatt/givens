// Package tags reads Cucumber tag expressions, like "@error and not @slow", and matches them
// against a scenario's tags, the way test runners filter on the same tags.
// Ref: https://cucumber.io/docs/cucumber/api/#tag-expressions
//
// Parsing is the shunting-yard algorithm into postfix, then a stack evaluates it: no
// recursion, as CONSTRAINTS.md asks.
// Ref: https://en.wikipedia.org/wiki/Shunting_yard_algorithm
package tags

import (
	"errors"
	"fmt"
	"strings"

	"givens/internal/assert"
)

// TokensMax bounds one expression. Tiger Style: put a limit on everything.
const TokensMax = 256

// Operators, by precedence: not binds tightest, then and, then or.
const (
	operatorNot = "not"
	operatorAnd = "and"
	operatorOr  = "or"
)

// ErrSyntax is returned for an expression that doesn't parse.
var ErrSyntax = errors.New("not a tag expression")

// Expression is a parsed tag expression, in postfix order.
type Expression struct {
	postfix []string
}

// Parse reads a tag expression. "" matches every scenario.
func Parse(text string) (Expression, error) {
	assert.That(len(text) <= TokensMax*64, "an expression stays bounded")
	tokens := tokenize(text)
	if len(tokens) > TokensMax {
		return Expression{}, fmt.Errorf("%w: more than %d parts", ErrSyntax, TokensMax)
	}
	postfix := make([]string, 0, len(tokens))
	operators := make([]string, 0, len(tokens))
	expectOperand := true
	for _, token := range tokens {
		var err error
		postfix, operators, expectOperand, err = shunt(token, postfix, operators, expectOperand)
		if err != nil {
			return Expression{}, err
		}
	}
	if len(tokens) > 0 && expectOperand {
		return Expression{}, fmt.Errorf("%w: expected a tag after '%s'", ErrSyntax,
			tokens[len(tokens)-1])
	}
	for index := len(operators) - 1; index >= 0; index-- {
		if operators[index] == "(" {
			return Expression{}, fmt.Errorf("%w: expected ) to close (", ErrSyntax)
		}
		postfix = append(postfix, operators[index])
	}
	expression := Expression{postfix: postfix}
	assert.That(len(postfix) <= len(tokens), "brackets drop out of the postfix")
	if err := expression.check(); err != nil {
		return Expression{}, err
	}
	return expression, nil
}

// tokenize splits on spaces and parentheses.
func tokenize(text string) []string {
	assert.That(len(text) <= TokensMax*64, "an expression stays bounded")
	spaced := strings.NewReplacer("(", " ( ", ")", " ) ").Replace(text)
	tokens := strings.Fields(spaced)
	assert.That(len(tokens) <= len(spaced), "each token is part of the text")
	return tokens
}

// shunt moves one token through the shunting yard.
func shunt(token string, postfix []string, operators []string, expectOperand bool) ([]string,
	[]string, bool, error) {
	assert.That(token != "", "a token has text")
	assert.That(len(postfix) <= TokensMax, "the output stays bounded")
	switch {
	case token == "(" || token == operatorNot:
		if !expectOperand {
			return nil, nil, false, fmt.Errorf("%w: expected and or or before '%s'", ErrSyntax,
				token)
		}
		return postfix, append(operators, token), true, nil
	case token == ")":
		if expectOperand {
			return nil, nil, false, fmt.Errorf("%w: expected a tag before ')'", ErrSyntax)
		}
		top := len(operators) - 1
		// bound: top counts down to -1 at most.
		for ; top >= 0 && operators[top] != "("; top-- {
			postfix = append(postfix, operators[top])
		}
		if top < 0 {
			return nil, nil, false, fmt.Errorf("%w: ')' has no '('", ErrSyntax)
		}
		return postfix, operators[:top], false, nil
	case token == operatorAnd || token == operatorOr:
		if expectOperand {
			return nil, nil, false, fmt.Errorf("%w: expected a tag before '%s'", ErrSyntax, token)
		}
		top := len(operators) - 1
		// bound: top counts down to -1 at most.
		for ; top >= 0 && precedence(operators[top]) >= precedence(token); top-- {
			postfix = append(postfix, operators[top])
		}
		return postfix, append(operators[:top+1], token), true, nil
	case strings.HasPrefix(token, "@") && len(token) > 1:
		if !expectOperand {
			return nil, nil, false, fmt.Errorf("%w: expected and or or before '%s'", ErrSyntax,
				token)
		}
		return append(postfix, token), operators, false, nil
	}
	return nil, nil, false, fmt.Errorf("%w: '%s' isn't a tag; tags start with @", ErrSyntax,
		token)
}

// precedence ranks operators; "(" ranks lowest so nothing pops past it.
func precedence(operator string) int {
	assert.That(operator != "", "an operator has text")
	assert.That(!strings.HasPrefix(operator, "@"), "a tag isn't an operator")
	switch operator {
	case operatorNot:
		return 3
	case operatorAnd:
		return 2
	case operatorOr:
		return 1
	}
	return 0
}

// check evaluates the postfix once with no tags, so a malformed one fails here, not later.
func (e Expression) check() error {
	assert.That(len(e.postfix) <= TokensMax, "the postfix stays bounded")
	if _, ok := e.evaluate(nil); !ok {
		return ErrSyntax
	}
	assert.That(len(e.postfix) <= TokensMax, "a checked expression stays bounded")
	return nil
}

// Match is true when tags, a scenario's tags with its Feature's, satisfy the expression.
func (e Expression) Match(tags []string) bool {
	assert.That(len(e.postfix) <= TokensMax, "the postfix stays bounded")
	assert.That(len(tags) <= TokensMax*64, "tags stay bounded")
	matched, ok := e.evaluate(tags)
	assert.That(ok, "a parsed expression evaluates")
	return matched
}

// evaluate runs the postfix on a stack. ok is false when the stack doesn't end with one value.
func (e Expression) evaluate(tags []string) (bool, bool) {
	assert.That(len(e.postfix) <= TokensMax, "the postfix stays bounded")
	if len(e.postfix) == 0 {
		return true, true
	}
	stack := make([]bool, 0, len(e.postfix))
	for _, token := range e.postfix {
		top := len(stack) - 1
		switch token {
		case operatorNot:
			if top < 0 {
				return false, false
			}
			stack[top] = !stack[top]
		case operatorAnd, operatorOr:
			if top < 1 {
				return false, false
			}
			left, right := stack[top-1], stack[top]
			stack = stack[:top]
			stack[top-1] = (token == operatorAnd && left && right) ||
				(token == operatorOr && (left || right))
		default:
			stack = append(stack, contains(tags, token))
		}
	}
	return len(stack) == 1 && stack[0], len(stack) == 1
}

// contains is true when tags holds tag.
func contains(tags []string, tag string) bool {
	assert.That(strings.HasPrefix(tag, "@"), "a tag starts with @")
	assert.That(len(tags) <= TokensMax*64, "tags stay bounded")
	for _, candidate := range tags {
		if candidate == tag {
			return true
		}
	}
	return false
}
