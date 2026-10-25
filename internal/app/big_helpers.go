// The big file's helpers: code that two or more of big.go's stories use, kept apart so that
// big.go reads as the stories alone. Each helper says what it does, not which story it serves;
// a function only one story uses belongs in that story's section of big.go. See
// tasks/one-big-file/plan.md.
//
// TestBigFile checks this file holds no Feature, Rule or Scenario headings. If this file grows
// past big.go, that's the tripwire: move a group that belongs together to a file of its own,
// and say why in big.go's header.

package app

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	messages "github.com/cucumber/messages/go/v34"

	"givens/internal/app/model"
	"givens/internal/assert"
	"givens/internal/slug"
	"givens/internal/spec"
)

// repoDepthMax bounds the walk up from a capabilities folder to its git repo's root.
const repoDepthMax = 64

// repoRoot is the root of the git repository holding capabilities, or, outside one, the
// folder holding capabilities. The kit moves there before running anything.
func repoRoot(capabilities string) string {
	assert.That(filepath.IsAbs(capabilities), "capabilities is absolute")
	assert.That(repoDepthMax > 0, "the walk up has a limit")
	dir := filepath.Dir(capabilities)
	for depth := 0; depth < repoDepthMax; depth++ {
		// .git is a folder in a clone and a file in a worktree.
		// Ref: https://git-scm.com/docs/gitrepository-layout
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return filepath.Dir(capabilities)
}

// remember adds folder to one of the app's folder lists: trusted, untrusted, recents.
func (a *App) remember(name string, folder string) error {
	assert.That(name != "", "a list has a file")
	assert.That(filepath.IsAbs(folder), "folders are absolute")
	folders, err := a.folders(name)
	if err != nil || slices.Contains(folders, folder) {
		return err
	}
	if len(folders) >= TrustedMax {
		return fmt.Errorf("Givens already keeps %d folders, the most it keeps", TrustedMax)
	}
	return a.writeFolders(name, append(folders, folder))
}

// forget takes folder out of one of the app's folder lists.
func (a *App) forget(name string, folder string) error {
	assert.That(name != "", "a list has a file")
	assert.That(filepath.IsAbs(folder), "folders are absolute")
	folders, err := a.folders(name)
	if err != nil || !slices.Contains(folders, folder) {
		return err
	}
	kept := slices.DeleteFunc(folders, func(other string) bool { return other == folder })
	return a.writeFolders(name, kept)
}

// writeFolders writes a folder list, one absolute path per line.
func (a *App) writeFolders(name string, folders []string) error {
	assert.That(name != "", "a list has a file")
	assert.That(len(folders) <= TrustedMax, "lists stay bounded")
	text := strings.Join(folders, "\n") + "\n"
	return os.WriteFile(filepath.Join(a.config.AppDataDir, name), []byte(text), 0o644)
}

// folders reads one of the app's folder lists, one absolute path per line.
func (a *App) folders(name string) ([]string, error) {
	assert.That(a != nil, "folders runs on an app")
	assert.That(a.config.AppDataDir != "", "lists live in the app's data folder")
	text, err := os.ReadFile(filepath.Join(a.config.AppDataDir, name))
	if errors.Is(err, os.ErrNotExist) {
		return make([]string, 0, 1), nil
	}
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(text)), "\n")
	if len(lines) > TrustedMax {
		return nil, fmt.Errorf("%s holds %d folders, more than %d", name, len(lines),
			TrustedMax)
	}
	folders := make([]string, 0, len(lines))
	for _, line := range lines {
		if filepath.IsAbs(line) {
			folders = append(folders, line)
		}
	}
	return folders, nil
}

// tagViews lists the tags that are views, without their @, in the order first seen. @error
// and @timeout have meanings of their own, so they aren't views.
func (w *Window) tagViews() []string {
	assert.That(w != nil, "tag views run on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	views := make([]string, 0, 8)
	if w.project == nil {
		return views
	}
	for _, capability := range w.project.Capabilities {
		for _, scenario := range capability.Scenarios {
			for _, tag := range scenario.Tags {
				name := strings.TrimPrefix(tag, "@")
				if isViewTag(tag) && !slices.Contains(views, name) {
					views = append(views, name)
				}
			}
		}
	}
	return views
}

// isViewTag is true for a tag that's a view: any but @error and @timeout:….
func isViewTag(tag string) bool {
	assert.That(strings.HasPrefix(tag, "@"), "a tag starts with @")
	assert.That(len(tag) <= NameLengthMax, "a tag fits NameLengthMax")
	return tag != TagError && !strings.HasPrefix(tag, "@timeout")
}

// From internal/tags, which reads Cucumber tag expressions, like "@error and not @slow", and
// matches them against a scenario's tags, the way test runners filter on the same tags. Tags'
// parseTags and `givens list --tags` (cmd/givens/list.go) parse them, and Search's
// findScenarios matches them.
// Ref: https://cucumber.io/docs/cucumber/api/#tag-expressions
//
// Parsing is the shunting-yard algorithm into postfix, then a stack evaluates it: no
// recursion, as CONSTRAINTS.md asks.
// Ref: https://en.wikipedia.org/wiki/Shunting_yard_algorithm

// TagExpressionTokensMax bounds one expression. Tiger Style: put a limit on everything.
const TagExpressionTokensMax = 256

// Operators, by precedence: not binds tightest, then and, then or.
const (
	tagOperatorNot = "not"
	tagOperatorAnd = "and"
	tagOperatorOr  = "or"
)

// ErrTagSyntax is returned for an expression that doesn't parse.
var ErrTagSyntax = errors.New("not a tag expression")

// TagExpression is a parsed tag expression, in postfix order.
type TagExpression struct {
	postfix []string
}

// ParseTagExpression reads a tag expression. "" matches every scenario.
func ParseTagExpression(text string) (TagExpression, error) {
	assert.That(len(text) <= TagExpressionTokensMax*64, "an expression stays bounded")
	tokens := tokenizeTagExpression(text)
	if len(tokens) > TagExpressionTokensMax {
		return TagExpression{}, fmt.Errorf("%w: more than %d parts", ErrTagSyntax,
			TagExpressionTokensMax)
	}
	postfix := make([]string, 0, len(tokens))
	operators := make([]string, 0, len(tokens))
	expectOperand := true
	for _, token := range tokens {
		var err error
		postfix, operators, expectOperand, err = shunt(token, postfix, operators, expectOperand)
		if err != nil {
			return TagExpression{}, err
		}
	}
	if len(tokens) > 0 && expectOperand {
		return TagExpression{}, fmt.Errorf("%w: expected a tag after '%s'", ErrTagSyntax,
			tokens[len(tokens)-1])
	}
	for index := len(operators) - 1; index >= 0; index-- {
		if operators[index] == "(" {
			return TagExpression{}, fmt.Errorf("%w: expected ) to close (", ErrTagSyntax)
		}
		postfix = append(postfix, operators[index])
	}
	expression := TagExpression{postfix: postfix}
	assert.That(len(postfix) <= len(tokens), "brackets drop out of the postfix")
	if err := expression.check(); err != nil {
		return TagExpression{}, err
	}
	return expression, nil
}

// tokenizeTagExpression splits on spaces and parentheses.
func tokenizeTagExpression(text string) []string {
	assert.That(len(text) <= TagExpressionTokensMax*64, "an expression stays bounded")
	spaced := strings.NewReplacer("(", " ( ", ")", " ) ").Replace(text)
	tokens := strings.Fields(spaced)
	assert.That(len(tokens) <= len(spaced), "each token is part of the text")
	return tokens
}

// shunt moves one token through the shunting yard.
func shunt(token string, postfix []string, operators []string, expectOperand bool) ([]string,
	[]string, bool, error) {
	assert.That(token != "", "a token has text")
	assert.That(len(postfix) <= TagExpressionTokensMax, "the output stays bounded")
	switch {
	case token == "(" || token == tagOperatorNot:
		if !expectOperand {
			return nil, nil, false, fmt.Errorf("%w: expected and or or before '%s'", ErrTagSyntax,
				token)
		}
		return postfix, append(operators, token), true, nil
	case token == ")":
		if expectOperand {
			return nil, nil, false, fmt.Errorf("%w: expected a tag before ')'", ErrTagSyntax)
		}
		top := len(operators) - 1
		// bound: top counts down to -1 at most.
		for ; top >= 0 && operators[top] != "("; top-- {
			postfix = append(postfix, operators[top])
		}
		if top < 0 {
			return nil, nil, false, fmt.Errorf("%w: ')' has no '('", ErrTagSyntax)
		}
		return postfix, operators[:top], false, nil
	case token == tagOperatorAnd || token == tagOperatorOr:
		if expectOperand {
			return nil, nil, false, fmt.Errorf("%w: expected a tag before '%s'", ErrTagSyntax,
				token)
		}
		top := len(operators) - 1
		// bound: top counts down to -1 at most.
		for ; top >= 0 && tagPrecedence(operators[top]) >= tagPrecedence(token); top-- {
			postfix = append(postfix, operators[top])
		}
		return postfix, append(operators[:top+1], token), true, nil
	case strings.HasPrefix(token, "@") && len(token) > 1:
		if !expectOperand {
			return nil, nil, false, fmt.Errorf("%w: expected and or or before '%s'", ErrTagSyntax,
				token)
		}
		return append(postfix, token), operators, false, nil
	}
	return nil, nil, false, fmt.Errorf("%w: '%s' isn't a tag; tags start with @", ErrTagSyntax,
		token)
}

// tagPrecedence ranks operators; "(" ranks lowest so nothing pops past it.
func tagPrecedence(operator string) int {
	assert.That(operator != "", "an operator has text")
	assert.That(!strings.HasPrefix(operator, "@"), "a tag isn't an operator")
	switch operator {
	case tagOperatorNot:
		return 3
	case tagOperatorAnd:
		return 2
	case tagOperatorOr:
		return 1
	}
	return 0
}

// check evaluates the postfix once with no tags, so a malformed one fails here, not later.
func (e TagExpression) check() error {
	assert.That(len(e.postfix) <= TagExpressionTokensMax, "the postfix stays bounded")
	if _, ok := e.evaluate(nil); !ok {
		return ErrTagSyntax
	}
	assert.That(len(e.postfix) <= TagExpressionTokensMax, "a checked expression stays bounded")
	return nil
}

// Match is true when tags, a scenario's tags with its Feature's, satisfy the expression.
func (e TagExpression) Match(tags []string) bool {
	assert.That(len(e.postfix) <= TagExpressionTokensMax, "the postfix stays bounded")
	assert.That(len(tags) <= TagExpressionTokensMax*64, "tags stay bounded")
	matched, ok := e.evaluate(tags)
	assert.That(ok, "a parsed expression evaluates")
	return matched
}

// evaluate runs the postfix on a stack. ok is false when the stack doesn't end with one value.
func (e TagExpression) evaluate(tags []string) (bool, bool) {
	assert.That(len(e.postfix) <= TagExpressionTokensMax, "the postfix stays bounded")
	if len(e.postfix) == 0 {
		return true, true
	}
	stack := make([]bool, 0, len(e.postfix))
	for _, token := range e.postfix {
		top := len(stack) - 1
		switch token {
		case tagOperatorNot:
			if top < 0 {
				return false, false
			}
			stack[top] = !stack[top]
		case tagOperatorAnd, tagOperatorOr:
			if top < 1 {
				return false, false
			}
			left, right := stack[top-1], stack[top]
			stack = stack[:top]
			stack[top-1] = (token == tagOperatorAnd && left && right) ||
				(token == tagOperatorOr && (left || right))
		default:
			stack = append(stack, hasTag(tags, token))
		}
	}
	return len(stack) == 1 && stack[0], len(stack) == 1
}

// hasTag is true when tags holds tag.
func hasTag(tags []string, tag string) bool {
	assert.That(strings.HasPrefix(tag, "@"), "a tag starts with @")
	assert.That(len(tags) <= TagExpressionTokensMax*64, "tags stay bounded")
	for _, candidate := range tags {
		if candidate == tag {
			return true
		}
	}
	return false
}

// SearchResults is the ⌘K panel's query, filter and grouped results, or the zero value while
// search is closed. A query that matches nothing, or a tag expression that doesn't parse,
// has no groups. Needed by every scenario in capabilities/search-everything and the search
// scenarios in capabilities/tags.
func (w *Window) SearchResults() SearchResults {
	assert.That(w != nil, "search results run on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	if w.search == nil {
		return SearchResults{Groups: []SearchGroup{}}
	}
	if memo := w.searchMemo; memo != nil && memo.query == w.search.query &&
		memo.filter == w.search.filter {
		return memo.results
	}
	results := SearchResults{
		Query: w.search.query, Filter: w.search.filter, Groups: make([]SearchGroup, 0, 5),
	}
	defer func() {
		w.searchMemo = &searchMemo{query: results.Query, filter: results.Filter, results: results}
	}()
	read := readQuery(w.search.query)
	if w.project == nil || read.problem != "" || (read.words == "" && !read.tagged) {
		return results
	}
	for _, group := range w.searchGroups(read) {
		if len(group.Results) > 0 && (results.Filter == "" || results.Filter == group.Kind) {
			results.Groups = append(results.Groups, group)
		}
	}
	return results
}

// uniqueSteps is a scenario's steps, each text once, so a step it repeats counts as one use.
func uniqueSteps(scenario LoadedScenario) []GherkinStep {
	assert.That(scenario.Name != "", "the scenario has a name")
	assert.That(len(scenario.Steps) <= HelpersMax, "steps stay bounded")
	steps := make([]GherkinStep, 0, len(scenario.Steps))
	seen := make(map[string]bool, len(scenario.Steps))
	for _, step := range scenario.Steps {
		if !seen[step.Text] {
			seen[step.Text] = true
			steps = append(steps, step)
		}
	}
	return steps
}

// Helpers is the "Shared" section of the sidebar: every documented helper in the project's
// _shared folders, _shared's first, in file order. A helper defined twice in one folder is
// flagged; a capability's helper that shadows a shared one says so.
func (w *Window) Helpers() []model.Helper {
	assert.That(w != nil, "helpers runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	w.refresh()
	if w.project == nil {
		return []model.Helper{}
	}
	// Kept until a file changes: finding callers reads every script once per helper.
	if w.helpers != nil {
		return append([]model.Helper{}, w.helpers...)
	}
	shared, err := HelpersInFolder(w.path, "", "")
	if err != nil {
		return []model.Helper{}
	}
	listed := w.listHelpers(shared, "", nil)
	for _, capability := range w.project.Capabilities {
		own, err := HelpersInFolder(w.path, capability.Slug, capability.Slug)
		if err != nil {
			continue
		}
		listed = append(listed, w.listHelpers(own, capability.Name, shared)...)
	}
	w.helpers = listed
	return append([]model.Helper{}, listed...)
}

// Limits on checking a helper file.
const (
	// syntaxCheckTimeout bounds one `bash -n`, which only reads the file.
	syntaxCheckTimeout = 5 * time.Second
	// helperProblemsMax bounds the lines marked in one file.
	helperProblemsMax = 64
)

// bashError matches one of bash's syntax reports: "x.sh: line 9: syntax error: …".
const bashErrorPattern = `line (\d+): (.+)`

var bashError = regexp.MustCompile(bashErrorPattern) //tiger:allow-global compiled once

// RequiredCommands lists the commands a scenario's setup needs, whether each is installed, and
// its install hint: the `need` lines in its script, the setups it builds on, and every helper
// those call. Needed by "See what a scenario needs installed" and by Drop in's check before
// anything runs.
func (w *Window) RequiredCommands(scenario string) ([]model.RequiredCommand, error) {
	assert.That(w != nil, "required commands runs on a window")
	assert.That(len(scenario) <= NameLengthMax, "the name fits NameLengthMax")
	found, ok := w.findScenario(scenario)
	if !ok {
		return nil, fmt.Errorf("%s: %w", scenario, ErrNotFound)
	}
	texts, err := w.setupTexts(found)
	if err != nil {
		return nil, err
	}
	all := ""
	for _, text := range texts {
		all += text + "\n"
	}
	needs := NeedLines(all)
	required := make([]model.RequiredCommand, 0, len(needs)+1)
	if chrome, uses := chromeRequired(all); uses {
		required = append(required, chrome)
	}
	for _, need := range needs {
		// LookPath searches PATH as a shell would. Ref: https://pkg.go.dev/os/exec#LookPath
		_, missing := exec.LookPath(need.Command)
		required = append(required, model.RequiredCommand{
			Command: need.Command, Hint: need.Hint, Installed: missing == nil,
		})
	}
	return required, nil
}

// From internal/helpers, which reads the shell functions in a capabilities folder's _shared
// folders: where each is defined and its `# @helper` documentation, and the `from` lines a setup
// builds on. Copy, Drop in, Search, Share setup and Write stories read scripts with it. Which
// helpers a script calls, and its `need` lines, are Share setup's, at the end of its section in
// big.go; the browser lines are Mix shell's.
//
// It reads bash by pattern, not by parsing it: a helper is a line `name() {` at the start of a
// line, ending at the next line that is just `}`. That's the shape GIVENS.md asks for, and what
// `bash -n` can't tell Givens without running anything.
// Ref: https://www.gnu.org/software/bash/manual/html_node/Shell-Functions.html

// Limits. Tiger Style: put a limit on everything.
// Ref: https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md#safety
const (
	// ScriptSizeBytesMax bounds one helper file or setup script.
	ScriptSizeBytesMax = 1 << 20
	// HelpersMax bounds the functions one capability can see.
	HelpersMax = 4096
	// KitFileName is the kit's file name; the kit isn't a helper file.
	KitFileName = "givens.sh"
)

// Patterns, compiled once.
//
//tiger:allow-global compiled once
var (
	helperDefinition = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)\s*\(\)\s*\{`)
	helperDoc        = regexp.MustCompile(`^#\s*@helper\s+(\S+.*)$`)
)

// HelperDefinition is one shell function in a _shared folder.
type HelperDefinition struct {
	Name        string
	Signature   string // the `# @helper` line's text, like "seed_shopper [email]"; "" if none
	Description string // the comment lines under `# @helper`, joined with spaces
	Capability  string // the capability slug whose _shared holds it, or "" for _shared
	File        string // relative to the capabilities folder: "_shared/users.sh"
	Line        int    // the `name() {` line, counting from 1
	Body        string // the lines between `name() {` and its closing `}`
}

// Documented is true for a helper with a `# @helper` comment, which the Shared list shows.
func (h HelperDefinition) Documented() bool {
	assert.That(h.Name != "", "a helper has a name")
	assert.That(h.File != "", "a helper has a file")
	return h.Signature != ""
}

// HelpersInFolder reads every function in root/<folder>/_shared/*.sh but the kit, in file name
// order then line order. folder is "" for root/_shared. capability names what
// HelperDefinition.Capability says.
func HelpersInFolder(root string, folder string, capability string) ([]HelperDefinition, error) {
	assert.That(filepath.IsAbs(root), "root is absolute")
	assert.That(!strings.Contains(folder, "/"), "folder is a capability's folder or \"\"")
	// Glob returns names in lexical order. Ref: https://pkg.go.dev/path/filepath#Glob
	files, err := filepath.Glob(filepath.Join(root, folder, "_shared", "*.sh"))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	found := make([]HelperDefinition, 0, 16)
	for _, path := range files {
		if filepath.Base(path) == KitFileName {
			continue
		}
		read, err := helpersInFile(root, path, capability)
		if err != nil {
			return nil, err
		}
		found = append(found, read...)
		if len(found) > HelpersMax {
			return nil, fmt.Errorf("more than %d helpers in %s", HelpersMax, folder)
		}
	}
	return found, nil
}

// helpersInFile reads the functions in one file.
func helpersInFile(root string, path string, capability string) ([]HelperDefinition, error) {
	assert.That(filepath.IsAbs(path), "path is absolute")
	assert.That(strings.HasPrefix(path, root), "the file is in the capabilities folder")
	text, err := ReadScript(path)
	if err != nil {
		return nil, err
	}
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(text, "\n")
	found := make([]HelperDefinition, 0, 8)
	for index, line := range lines {
		match := helperDefinition.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		signature, description := docAbove(lines, index)
		found = append(found, HelperDefinition{
			Name: match[1], Signature: signature, Description: description,
			Capability: capability, File: filepath.ToSlash(relative), Line: index + 1,
			Body: bodyBelow(lines, index),
		})
	}
	return found, nil
}

// docAbove reads the comment block right above a definition: its `# @helper` line, and the
// comment lines after it.
func docAbove(lines []string, definitionIndex int) (string, string) {
	assert.That(definitionIndex >= 0, "the definition is a line")
	assert.That(definitionIndex < len(lines), "the definition is in the file")
	start := definitionIndex
	for start > 0 && strings.HasPrefix(strings.TrimSpace(lines[start-1]), "#") { // bound: start
		start--
	}
	signature, description := "", make([]string, 0, 4)
	for _, line := range lines[start:definitionIndex] {
		trimmed := strings.TrimSpace(line)
		if match := helperDoc.FindStringSubmatch(trimmed); match != nil {
			signature = strings.TrimSpace(match[1])
			continue
		}
		if signature != "" {
			description = append(description, strings.TrimSpace(strings.TrimPrefix(trimmed, "#")))
		}
	}
	return signature, strings.Join(description, " ")
}

// bodyBelow is the text between a definition line and the next line that's just `}`.
func bodyBelow(lines []string, definitionIndex int) string {
	assert.That(definitionIndex >= 0, "the definition is a line")
	assert.That(definitionIndex < len(lines), "the definition is in the file")
	body := make([]string, 0, 8)
	for _, line := range lines[definitionIndex+1:] {
		if strings.TrimRight(line, " \t") == "}" {
			break
		}
		body = append(body, line)
	}
	return strings.Join(body, "\n")
}

// ReadScript reads a script or helper file, within ScriptSizeBytesMax.
func ReadScript(path string) (string, error) {
	assert.That(filepath.IsAbs(path), "path is absolute")
	assert.That(ScriptSizeBytesMax > 0, "scripts have a size limit")
	text, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if len(text) > ScriptSizeBytesMax {
		return "", fmt.Errorf("%s is larger than %d bytes", path, ScriptSizeBytesMax)
	}
	return string(text), nil
}

// The pattern for the kit's from lines.
// Ref: capabilities/the-kit/about.md, in the Givens repository
const fromPattern = `(?m)^\s*from\s+([^\s/]+/[^\s/]+)((?:[ \t]+[A-Za-z_]\w*=\S*)*)[ \t]*$`

var fromLineRegexp = regexp.MustCompile(fromPattern) //tiger:allow-global compiled once

// FromLine is one `from <capability>/<slug> [column=value …]` line.
type FromLine struct {
	Key    string   // "checkout/pay-with-saved-card"
	Values []string // "card=stolen", in order
	Line   string   // the line as written, trimmed
}

// FromLines lists the from lines in a setup script, in order.
func FromLines(text string) []FromLine {
	assert.That(len(text) <= ScriptSizeBytesMax, "a script stays bounded")
	matches := fromLineRegexp.FindAllStringSubmatch(text, -1)
	froms := make([]FromLine, 0, len(matches))
	for _, match := range matches {
		froms = append(froms, FromLine{
			Key: match[1], Values: strings.Fields(match[2]),
			Line: strings.TrimSpace(match[0]),
		})
	}
	assert.That(len(froms) == len(matches), "each match is a from line")
	return froms
}

// From internal/runs, which keeps the run log: one line per recorded Drop in, in
// <capabilities>/runs/YYYY-MM-DD.log, and each run's attached files and note. Drop in, Keep runs
// off main, Read a capability, Share setup and Write stories read it. Writing a run, its files
// and its note, merging a day's log and trimming the ref are Keep runs off main's, at the end of
// its section in big.go; RunAuthor and AttachedFiles are Drop in's, ReadRunNote is Read a
// capability's and LinkRunsRefCopy is Open a project's.
//
// Lines are only ever appended, never rewritten, so capabilities/.gitattributes can mark the
// folder `runs/** merge=union` and git keeps both branches' lines in a merge.
// Ref: https://git-scm.com/docs/gitattributes#_built_in_merge_drivers
//
// A line is tab-separated: time (RFC 3339), run key, result, author, then one field per row
// value (`card=expired`) and per fingerprint part (`fp:steps=1a2b3c4d`), and the note last.

// Limits. Tiger Style: put a limit on everything.
// Ref: https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md#safety
const (
	// RunLogFilesMax is about 27 years of daily files.
	RunLogFilesMax = 10000
	// RunLineLengthBytesMax bounds one run line, note included.
	RunLineLengthBytesMax = 64 << 10
	// RunFieldsMax bounds the row values and fingerprint parts on one line.
	RunFieldsMax = 256
)

// Results.
const (
	RunResultPass = "pass"
	RunResultFail = "fail"
)

// fingerprintPrefix starts a fingerprint part's field, so it can't be read as a row value.
const fingerprintPrefix = "fp:"

// ErrBadRunLine is returned for a run line that can't be read.
var ErrBadRunLine = errors.New("not a run line")

// RunPair is a name and a value: a row value (card, expired) or a fingerprint part (steps, 1a2b).
type RunPair struct {
	Name  string
	Value string
}

// Run is one recorded Drop in.
type Run struct {
	Time        time.Time
	Key         string    // see RunKey
	Result      string    // RunResultPass or RunResultFail
	Author      string    // git's user.name, or ""
	Values      []RunPair // the outline row's values, in column order
	Fingerprint []RunPair // what the setup was made of when it ran
	Note        string
}

// RunKey is the name a run is recorded against: <capability>/<scenario slug>, plus
// /<examples slug> for a row of a named Examples block. examplesSlug is "" otherwise.
func RunKey(scenarioKey string, examplesSlug string) string {
	assert.That(strings.Count(scenarioKey, "/") == 1, "a scenario key is capability/slug")
	assert.That(!strings.Contains(examplesSlug, "/"), "an examples slug is one slug")
	if examplesSlug == "" {
		return scenarioKey
	}
	return scenarioKey + "/" + examplesSlug
}

// RunsDir is the run log's folder inside a capabilities folder.
func RunsDir(capabilities string) string {
	assert.That(capabilities != "", "capabilities is a path")
	assert.That(filepath.IsAbs(capabilities), "capabilities is absolute")
	return filepath.Join(capabilities, "runs")
}

// formatRun writes one run as a line. Tabs and line breaks inside fields become spaces.
func formatRun(run Run) string {
	assert.That(run.Key != "", "a run has a key")
	assert.That(len(run.Values)+len(run.Fingerprint) <= RunFieldsMax, "fields stay bounded")
	fields := make([]string, 0, 5+len(run.Values)+len(run.Fingerprint))
	fields = append(fields, run.Time.Format(time.RFC3339), run.Key, run.Result, run.Author)
	for _, value := range run.Values {
		fields = append(fields, value.Name+"="+value.Value)
	}
	for _, part := range run.Fingerprint {
		fields = append(fields, fingerprintPrefix+part.Name+"="+part.Value)
	}
	fields = append(fields, run.Note)
	for index, field := range fields {
		fields[index] = strings.NewReplacer("\t", " ", "\r", " ", "\n", " ").Replace(field)
	}
	return strings.Join(fields, "\t")
}

// ParseRun reads one run line.
func ParseRun(line string) (Run, error) {
	assert.That(len(line) <= RunLineLengthBytesMax, "a line fits RunLineLengthBytesMax")
	assert.That(!strings.Contains(line, "\n"), "a line is one line")
	fields := strings.Split(line, "\t")
	if len(fields) < 5 || len(fields) > 5+RunFieldsMax {
		return Run{}, fmt.Errorf("%w: %d fields", ErrBadRunLine, len(fields))
	}
	at, err := time.Parse(time.RFC3339, fields[0])
	if err != nil {
		return Run{}, fmt.Errorf("%w: %v", ErrBadRunLine, err)
	}
	run := Run{
		Time: at, Key: fields[1], Result: fields[2], Author: fields[3],
		Note: fields[len(fields)-1],
	}
	for _, field := range fields[4 : len(fields)-1] {
		name, value, ok := strings.Cut(field, "=")
		if !ok {
			return Run{}, fmt.Errorf("%w: %q is neither a row value nor a fingerprint part",
				ErrBadRunLine, field)
		}
		if part, isPart := strings.CutPrefix(name, fingerprintPrefix); isPart {
			run.Fingerprint = append(run.Fingerprint, RunPair{Name: part, Value: value})
		} else {
			run.Values = append(run.Values, RunPair{Name: name, Value: value})
		}
	}
	return run, nil
}

// ReadRuns reads every run in runs/ and, when the project keeps its runs off main, in the copy of
// refs/givens/runs (see RunsRefCopy). A run in both is read once.
func ReadRuns(capabilities string) ([]Run, error) {
	assert.That(filepath.IsAbs(capabilities), "capabilities is absolute")
	all, err := ReadRunsFolder(capabilities)
	if err != nil {
		return nil, err
	}
	copied := RunsRefCopy(capabilities)
	if copied == "" {
		return all, nil
	}
	kept, err := ReadRunsFolder(copied)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(all)+len(kept))
	for _, run := range all {
		seen[formatRun(run)] = true
	}
	for _, run := range kept {
		if !seen[formatRun(run)] {
			all = append(all, run)
		}
	}
	assert.That(len(all) <= len(seen)+len(kept), "a run is read at most once more")
	return all, nil
}

// ReadRunsFolder reads every run in root/runs/, oldest file first and in file order within a day.
// No runs/ folder means no runs. A line that can't be read is skipped: one bad merge shouldn't
// hide every run.
func ReadRunsFolder(capabilities string) ([]Run, error) {
	assert.That(filepath.IsAbs(capabilities), "capabilities is absolute")
	// Glob returns names in lexical order, which for YYYY-MM-DD is date order.
	// Ref: https://pkg.go.dev/path/filepath#Glob
	paths, err := filepath.Glob(filepath.Join(RunsDir(capabilities), "*.log"))
	if err != nil {
		return nil, err
	}
	if len(paths) > RunLogFilesMax {
		return nil, fmt.Errorf("runs/ holds %d files, more than %d", len(paths), RunLogFilesMax)
	}
	all := make([]Run, 0, len(paths)*4)
	for _, path := range paths {
		found, err := readRunFile(path)
		if err != nil {
			return nil, err
		}
		all = append(all, found...)
	}
	assert.That(len(paths) <= RunLogFilesMax, "files read stay bounded")
	return all, nil
}

// readRunFile reads one day's runs.
func readRunFile(path string) ([]Run, error) {
	assert.That(strings.HasSuffix(path, ".log"), "path is a run file")
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	// Scanner splits on lines; Buffer raises its limit to a whole run line.
	// Ref: https://pkg.go.dev/bufio#Scanner.Buffer
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 4096), RunLineLengthBytesMax+1)
	found := make([]Run, 0, 16)
	// bound: Scan stops at the end of the file, which ReadRuns's callers keep to one day of runs.
	for scanner.Scan() {
		if run, err := ParseRun(scanner.Text()); err == nil {
			found = append(found, run)
		}
	}
	if err := scanner.Err(); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	assert.That(scanner.Err() == nil, "the file was read to its end")
	return found, file.Close()
}

// LatestRun returns the newest run for key whose row values are values, and false when there's
// none. Runs are compared by time, so a merged file's order doesn't matter.
func LatestRun(all []Run, key string, values []RunPair) (Run, bool) {
	assert.That(key != "", "a key names a scenario")
	assert.That(len(values) <= RunFieldsMax, "values stay bounded")
	var latest Run
	found := false
	for _, run := range all {
		if run.Key != key || !samePairs(run.Values, values) {
			continue
		}
		if !found || !run.Time.Before(latest.Time) {
			latest, found = run, true
		}
	}
	return latest, found
}

// samePairs is true when a and b hold the same pairs in the same order.
func samePairs(a []RunPair, b []RunPair) bool {
	assert.That(len(a) <= RunFieldsMax, "a stays bounded")
	assert.That(len(b) <= RunFieldsMax, "b stays bounded")
	if len(a) != len(b) {
		return false
	}
	for index := range a {
		if a[index] != b[index] {
			return false
		}
	}
	return true
}

// Attached files: proof of what a run saw, like a screenshot. Each run keeps its own folder,
// runs/files/<run key>/<time>/, found from the run line's key and time, so the line format
// doesn't change. Names under one run's folder can't collide with another branch's, so a merge
// only ever adds folders.

// Limits on attached files.
const (
	// FilesPerRunMax bounds the files one run keeps.
	FilesPerRunMax = 32
	// AttachedFileBytesMax bounds one attached file: room for a screen recording, not a disk image.
	AttachedFileBytesMax = 100 << 20
)

// filesFolderTimeLayout names a run's folder by its time in UTC, so folders sort by time and
// the name has no colon, which Finder shows as a slash.
// Ref: https://pkg.go.dev/time#pkg-constants
const filesFolderTimeLayout = "2006-01-02T150405Z"

// AttachedFilesDir is the folder holding the files attached to the run recorded against key at at.
func AttachedFilesDir(capabilities string, key string, at time.Time) string {
	assert.That(filepath.IsAbs(capabilities), "capabilities is absolute")
	assert.That(key != "", "a run has a key")
	assert.That(!at.IsZero(), "a run has a time")
	return filepath.Join(RunsDir(capabilities), "files", filepath.FromSlash(key),
		at.UTC().Format(filesFolderTimeLayout))
}

// CheckAttachedFile returns an error unless path is a regular file of at most AttachedFileBytesMax
// bytes.
func CheckAttachedFile(path string) error {
	assert.That(path != "", "a file has a path")
	assert.That(filepath.IsAbs(path), "an attached file's path is absolute")
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s isn't a file", filepath.Base(path))
	}
	if info.Size() > AttachedFileBytesMax {
		return fmt.Errorf("%s is %d MB, more than %d MB", filepath.Base(path), info.Size()>>20,
			AttachedFileBytesMax>>20)
	}
	return nil
}

// The run's note, when it runs to more than one line: a run line is one line, so the whole
// note is kept beside the run's files and the line keeps its first line.
const (
	// RunNoteFileName is the note's file in a run's folder; no attached file may take the name.
	RunNoteFileName = "note.md"
	// RunNoteBytesMax bounds a note: pages of Markdown, not a book.
	RunNoteBytesMax = 64 << 10
)

// A project can keep its runs off main: in the git ref refs/givens/runs, which RunStore (Keep runs
// off main's section) commits, fetches and pushes. On each device the ref is checked out in <git
// common dir>/givens/tree, laid out like a capabilities folder's runs/, so the run log and its
// files are read from there just as from runs/. See capabilities/keep-runs-off-main.

// Where the ref's copy lives inside the git common dir. git ignores folders it doesn't know
// in there, as git-lfs's .git/lfs shows. Ref: https://git-scm.com/docs/gitrepository-layout
const (
	RunsRefDirName  = "givens"
	runsRefTreeName = "tree"
	// gitDepthMax bounds the walk up from a capabilities folder to its repository's root.
	gitDepthMax = 64
	// gitFileBytesMax bounds a .git file or commondir file: one path.
	gitFileBytesMax = 4096
)

// RunsRefCopyLinkName is a file in a branch window's copied runs/ that names the checked-out
// project's ref copy. The branch's copy isn't in a repository, so RunsRefCopy can't find the ref
// by walking up from it.
const RunsRefCopyLinkName = ".givens-ref-copy"

// RunsRefCopy is where this project's refs/givens/runs is checked out:
// <git common dir>/givens/tree, or "" when capabilities isn't in a git repository. It's found
// without running git, so reading runs stays a few file reads.
func RunsRefCopy(capabilities string) string {
	assert.That(filepath.IsAbs(capabilities), "capabilities is absolute")
	linked := readSmall(filepath.Join(RunsDir(capabilities), RunsRefCopyLinkName))
	if filepath.IsAbs(linked) && filepath.Base(linked) == runsRefTreeName {
		return filepath.Clean(linked)
	}
	common := GitCommonDir(capabilities)
	if common == "" {
		return ""
	}
	assert.That(filepath.IsAbs(common), "the git dir is absolute")
	return filepath.Join(common, RunsRefDirName, runsRefTreeName)
}

// GitCommonDir is the .git folder of the repository holding start, or "" when there's none.
// A worktree or submodule has a .git file naming its git dir, whose commondir file names the
// shared one. Ref: https://git-scm.com/docs/gitrepository-layout#_description
func GitCommonDir(start string) string {
	assert.That(filepath.IsAbs(start), "the walk starts at an absolute path")
	dir := filepath.Clean(start)
	for range gitDepthMax {
		dotGit := filepath.Join(dir, ".git")
		info, err := os.Stat(dotGit)
		if err == nil && info.IsDir() {
			return dotGit
		}
		if err == nil && info.Mode().IsRegular() {
			return linkedCommonDir(dotGit)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
	return ""
}

// linkedCommonDir follows a .git file ("gitdir: <path>") to its git dir, then that dir's
// commondir file, if it has one, to the shared git dir.
func linkedCommonDir(dotGit string) string {
	assert.That(filepath.Base(dotGit) == ".git", "a .git file is read")
	gitdir, ok := strings.CutPrefix(readSmall(dotGit), "gitdir: ")
	if !ok || gitdir == "" {
		return ""
	}
	if !filepath.IsAbs(gitdir) {
		gitdir = filepath.Join(filepath.Dir(dotGit), gitdir)
	}
	common := readSmall(filepath.Join(gitdir, "commondir"))
	if common == "" {
		return filepath.Clean(gitdir)
	}
	if !filepath.IsAbs(common) {
		common = filepath.Join(gitdir, common)
	}
	assert.That(filepath.IsAbs(common), "the common dir is absolute")
	return filepath.Clean(common)
}

// readSmall reads a one-line file, trimmed, or "" when it's missing or too big.
func readSmall(path string) string {
	assert.That(filepath.IsAbs(path), "the path is absolute")
	info, err := os.Stat(path)
	if err != nil || info.Size() > gitFileBytesMax {
		return ""
	}
	text, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(text))
}

// AttachedFilesFolder is the folder holding run's files: in the ref's copy when it's there, or else
// under capabilities/runs/files.
func AttachedFilesFolder(capabilities string, run Run) string {
	assert.That(run.Key != "", "a run has a key")
	assert.That(!run.Time.IsZero(), "a run has a time")
	if copied := RunsRefCopy(capabilities); copied != "" {
		folder := AttachedFilesDir(copied, run.Key, run.Time)
		if _, err := os.Stat(folder); err == nil {
			return folder
		}
	}
	return AttachedFilesDir(capabilities, run.Key, run.Time)
}

// Scenario statuses, as the library shows them.
const (
	StatusNeverRun      = "Never run"
	StatusLastRunFailed = "Last run failed"
	statusVerified      = "Verified"
	statusUnverified    = "Unverified: "
)

// statusOf says where a scenario stands, by the app's clock. Answers are kept until a file in
// the capabilities folder changes (runs/ included) or the day turns over, since the sidebar,
// the capability page and the views all ask, and each answer reads every run file and
// fingerprints the setup.
func (w *Window) statusOf(scenario LoadedScenario) string {
	assert.That(w.project != nil, "status is read in a loaded project")
	assert.That(scenario.Capability != "", "the scenario belongs to a capability")
	now := w.app.config.Clock.Now()
	if day := now.Format(time.DateOnly); day != w.statusDay || w.statuses == nil {
		w.statuses, w.statusDay = make(map[string]string, 16), day
	}
	key := scenario.Capability + "/" + scenario.Name
	if status, kept := w.statuses[key]; kept {
		return status
	}
	status := StatusOf(w.project, scenario, now)
	w.statuses[key] = status
	return status
}

// StatusOf says where a scenario stands: verified while its latest run passed and nothing its
// setup depends on has changed since. An outline sums up its rows. The givens command uses it
// too. See capabilities/views and capabilities/share-setup-between-scenarios.
func StatusOf(loaded *LoadedProject, scenario LoadedScenario, now time.Time) string {
	assert.That(loaded != nil, "status is read in a loaded project")
	assert.That(!now.IsZero(), "now is a time")
	if scenario.Slug == "" {
		return StatusNeverRun
	}
	if scenario.Outline {
		return outlineStatus(loaded, scenario, now)
	}
	return statusFor(loaded, scenario, scenario.Key(), nil, now)
}

// statusFor says where one run key, with its row's values, stands.
func statusFor(loaded *LoadedProject, scenario LoadedScenario, key string,
	values []RunPair, now time.Time) string {
	assert.That(loaded != nil, "status is read in a loaded project")
	assert.That(key != "", "a run key is given")
	recorded, err := ReadRuns(loaded.Root)
	if err != nil {
		return StatusNeverRun
	}
	latest, ok := LatestRun(recorded, key, values)
	if !ok {
		return StatusNeverRun
	}
	current, err := FingerprintOf(loaded, scenario, values)
	if err != nil {
		return StatusNeverRun
	}
	if changed := FingerprintChanged(latest.Fingerprint, current); changed != "" {
		return statusUnverified + changed + " changed since the last run"
	}
	if latest.Result != RunResultPass {
		return StatusLastRunFailed
	}
	return verified(latest, now)
}

// verified says when, and by whom, a passing run was recorded: "Verified today by Sam".
func verified(run Run, now time.Time) string {
	assert.That(run.Result == RunResultPass, "only a passing run verifies")
	assert.That(!now.IsZero(), "now is a time")
	day := "on " + run.Time.Format(time.DateOnly)
	switch run.Time.In(now.Location()).Format(time.DateOnly) {
	case now.Format(time.DateOnly):
		day = "today"
	case now.AddDate(0, 0, -1).Format(time.DateOnly):
		day = "yesterday"
	}
	status := statusVerified + " " + day
	if run.Author != "" {
		status += " by " + run.Author
	}
	return status
}

// From internal/fingerprint: FingerprintOf, the way in to a scenario's setup fingerprint,
// which statusFor and Drop in's RecordRun call, with the part names and hash. Working out each
// part, and saying which changed, is Share setup's, in its section of big.go.

// Part names that aren't helper names.
const (
	FingerprintPartSteps = "steps"
	FingerprintPartSetup = "setup"
	fromPrefix           = "from:"
)

// FingerprintOf works out the fingerprint of scenario, one of loaded's, for the row whose values
// are row (nil for a scenario with no Examples).
func FingerprintOf(loaded *LoadedProject, scenario LoadedScenario,
	row []RunPair) ([]RunPair, error) {
	assert.That(loaded != nil, "the project is loaded")
	assert.That(scenario.Capability != "", "the scenario belongs to a capability")
	parts := []RunPair{{Name: FingerprintPartSteps, Value: hash(stepsText(scenario, row))}}
	own, err := setupText(scenario)
	if err != nil {
		return nil, err
	}
	parts = append(parts, RunPair{Name: FingerprintPartSetup, Value: hash(own)})
	chain, err := fromChain(loaded, scenario)
	if err != nil {
		return nil, err
	}
	for _, built := range chain {
		text, err := setupText(built)
		if err != nil {
			return nil, err
		}
		parts = append(parts, RunPair{Name: fromPrefix + built.Key(), Value: hash(text)})
	}
	called, err := calledHelpers(loaded, append([]LoadedScenario{scenario}, chain...))
	if err != nil {
		return nil, err
	}
	return append(parts, called...), nil
}

// hash is a short code for text: the first 8 hex digits of its SHA-256.
// Ref: https://pkg.go.dev/crypto/sha256
func hash(text string) string {
	assert.That(len(text) < 1<<30, "hashed text stays bounded")
	sum := sha256.Sum256([]byte(text))
	code := hex.EncodeToString(sum[:4])
	assert.That(len(code) == 8, "a code is 8 hex digits")
	return code
}

// aboutImage matches a markdown image: ![alt](path).
const aboutImagePattern = `!\[([^\]]*)\]\(([^)\s]+)\)`

var aboutImage = regexp.MustCompile(aboutImagePattern) //tiger:allow-global compiled once

// withImagePlaceholders replaces each image about.md shows that isn't on disk with a line
// naming it, so the rest of the story still reads: "Image not found: images/flow.svg".
func withImagePlaceholders(about string, dir string) string {
	assert.That(filepath.IsAbs(dir), "the capability's folder is absolute")
	assert.That(len(about) <= model.EditorTextBytesMax, "about.md stays bounded")
	return aboutImage.ReplaceAllStringFunc(about, func(found string) string {
		path := aboutImage.FindStringSubmatch(found)[2]
		if strings.Contains(path, "://") {
			return found
		}
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(path))); err == nil {
			return found
		}
		return "Image not found: " + path
	})
}

// MessageTimeoutTag is the warning on a scenario whose @timeout tag can't be read.
const MessageTimeoutTag = "@timeout must look like @timeout:90s or @timeout:5m"

// scenarioWarning says what's wrong with a scenario as written, or "".
func scenarioWarning(capability LoadedCapability, scenario LoadedScenario) string {
	assert.That(scenario.Capability == capability.Slug, "the scenario is in the capability")
	assert.That(scenario.StillRunning >= 0, "a timeout is never negative")
	if clash := slugClash(capability, scenario); clash != "" {
		return clash
	}
	if scenario.StillRunning == 0 {
		return MessageTimeoutTag
	}
	return examplesWarning(scenario)
}

// slugClash is the warning on a scenario whose slug another scenario in its capability
// makes too, or "". Runs and setup files are named by slug, so neither scenario can use them.
func slugClash(capability LoadedCapability, scenario LoadedScenario) string {
	assert.That(scenario.Capability == capability.Slug, "the scenario is in the capability")
	assert.That(len(capability.Scenarios) <= spec.ScenariosPerFeatureMax, "scenarios bounded")
	for _, other := range capability.Scenarios {
		if other.Slug == scenario.Slug && other.Line != scenario.Line {
			return "Both names make the slug " + scenario.Slug
		}
	}
	return ""
}

// From internal/project, which loads a capabilities folder into the capabilities, scenarios,
// tags and setup files Givens shows and runs. It reads; it never writes. Every story reads the
// project it loads: reload and findCapabilities load it, as do `givens list` and `givens run`
// (cmd/givens). Window's Capability and Scenario are what a window shows of a LoadedCapability
// and a LoadedScenario (capabilityOf and scenarioOf, project.go).
//
// The layout it reads is written down in capabilities/GIVENS.md. Parsing is internal/spec's, so
// what Givens loads is what spec.Check judges and what test runners accept.

// Limits. Tiger Style: put a limit on everything.
// Ref: https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md#safety
const (
	// HelperFilesMax bounds the .sh files in one _shared folder.
	HelperFilesMax = 1000
	// StillRunningDefault is when Givens shows "Still running" without a @timeout tag.
	StillRunningDefault = 2 * time.Minute
)

// Tags with a meaning of their own. Every other tag is a view.
const (
	TagError         = "@error"
	TagTodo          = "@todo" // flags a scenario that needs work; see capabilities/tags
	TagTimeoutPrefix = "@timeout:"
)

// LoadedProject is one loaded capabilities folder.
type LoadedProject struct {
	Root         string             // the capabilities folder, absolute
	Capabilities []LoadedCapability // sorted by folder name
	Helpers      []string           // _shared/*.sh at the root, sorted; the kit included
	Problems     []spec.Problem
}

// LoadedCapability is one folder holding <slug>/<slug>.feature.
type LoadedCapability struct {
	Name        string // the Feature's name, or the folder's when the file can't be read
	Slug        string // the folder's name
	Dir         string
	FeaturePath string
	Story       []string // the As a, I want and So that lines, in file order
	Description string   // the Feature's whole description, trimmed
	Tags        []string // tags on the Feature
	About       string   // about.md's path, or "" when there is none
	Helpers     []string // <slug>/_shared/*.sh, sorted
	Scenarios   []LoadedScenario
	ParseError  string   // why the .feature file couldn't be read, or ""
	Ignored     []string // other .feature files in the folder, which aren't read; sorted
}

// LoadProject reads root, a capabilities folder. An error means root couldn't be read; a broken
// capability is loaded with its ParseError set and its problems in Problems.
func LoadProject(root string) (*LoadedProject, error) {
	assert.That(root != "", "root is a path")
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(absolute)
	if err != nil {
		return nil, err
	}
	if len(entries) > spec.CapabilitiesMax {
		return nil, fmt.Errorf("%s holds %d entries, more than %d",
			absolute, len(entries), spec.CapabilitiesMax)
	}
	project := &LoadedProject{Root: absolute}
	project.Capabilities = make([]LoadedCapability, 0, len(entries))
	// ReadDir returns entries sorted by name. Ref: https://pkg.go.dev/os#ReadDir
	for _, entry := range entries {
		if !isCapabilityFolder(absolute, entry) {
			continue
		}
		project.Capabilities = append(project.Capabilities,
			loadCapability(filepath.Join(absolute, entry.Name())))
	}
	if project.Helpers, err = helperFiles(absolute); err != nil {
		return nil, err
	}
	if project.Problems, err = spec.Check(absolute); err != nil {
		return nil, err
	}
	assert.That(len(project.Capabilities) <= len(entries), "each capability is an entry")
	return project, nil
}

// isCapabilityFolder is true for a visible folder holding <name>/<name>.feature.
func isCapabilityFolder(root string, entry os.DirEntry) bool {
	assert.That(root != "", "root is a path")
	assert.That(entry != nil, "the entry was read")
	name := entry.Name()
	// _shared, runs and hidden folders hold helpers and history, not capabilities.
	if !entry.IsDir() || strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".") {
		return false
	}
	_, err := os.Stat(filepath.Join(root, name, name+".feature"))
	return err == nil
}

// loadCapability reads one capability folder. It never fails: what can't be read is reported
// in ParseError, so the folder still shows.
func loadCapability(dir string) LoadedCapability {
	assert.That(filepath.IsAbs(dir), "dir is absolute")
	name := filepath.Base(dir)
	capability := LoadedCapability{
		Name: name, Slug: name, Dir: dir,
		FeaturePath: filepath.Join(dir, name+".feature"),
	}
	if _, err := os.Stat(filepath.Join(dir, "about.md")); err == nil {
		capability.About = filepath.Join(dir, "about.md")
	}
	helpers, err := helperFiles(dir)
	if err != nil {
		capability.ParseError = err.Error()
		return capability
	}
	capability.Helpers = helpers
	capability.Ignored = otherFeatures(dir, capability.FeaturePath)
	feature, err := spec.ParseFeature(capability.FeaturePath)
	if err != nil {
		capability.ParseError = parseErrorOf(capability.FeaturePath, err)
		capability.Name = featureNameIn(capability.FeaturePath, name)
		return capability
	}
	capability.Name = feature.Name
	capability.Description = strings.TrimSpace(feature.Description)
	capability.Story = storyLines(feature.Description)
	capability.Tags = tagNames(feature.Tags)
	capability.Scenarios = loadScenarios(capability, feature)
	assert.That(capability.Slug == name, "a capability's slug is its folder's name")
	return capability
}

// otherFeatures lists the .feature files in dir besides the capability's own.
func otherFeatures(dir string, own string) []string {
	assert.That(filepath.IsAbs(dir), "dir is absolute")
	assert.That(filepath.Dir(own) == dir, "the capability's file is in its folder")
	// Glob only fails on a bad pattern, and this one is fixed.
	// Ref: https://pkg.go.dev/path/filepath#Glob
	paths, _ := filepath.Glob(filepath.Join(dir, "*.feature"))
	others := make([]string, 0, len(paths))
	for _, path := range paths {
		if path != own {
			others = append(others, path)
		}
	}
	return others
}

// parseErrorOf says why a .feature file can't be read, naming the file and its line:
// "checkout.feature line 17: expected a step, a tag, a comment or a new scenario".
func parseErrorOf(path string, err error) string {
	assert.That(err != nil, "only an error is described")
	assert.That(strings.HasSuffix(path, ".feature"), "path is a .feature file")
	line, message := spec.Explain(err)
	if line > 0 {
		return fmt.Sprintf("%s line %d: %s", filepath.Base(path), line, message)
	}
	return filepath.Base(path) + ": " + message
}

// featureNameIn reads the name on a file's "Feature:" line without parsing the rest, so a
// capability whose file doesn't parse keeps its name. fallback is used when there's none.
func featureNameIn(path string, fallback string) string {
	assert.That(strings.HasSuffix(path, ".feature"), "path is a .feature file")
	assert.That(fallback != "", "there's a name to fall back on")
	text, err := os.ReadFile(path)
	if err != nil || len(text) > spec.FeatureFileSizeBytesMax {
		return fallback
	}
	for _, line := range strings.Split(string(text), "\n") {
		if name, ok := strings.CutPrefix(strings.TrimSpace(line), "Feature:"); ok {
			if name = strings.TrimSpace(name); name != "" {
				return name
			}
		}
	}
	return fallback
}

// storyLines picks the As a, I want and So that lines out of a Feature's description.
func storyLines(description string) []string {
	assert.That(len(description) <= spec.FeatureFileSizeBytesMax, "a description fits its file")
	lines := strings.Split(description, "\n")
	story := make([]string, 0, 3)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		for _, start := range []string{"As a", "I want", "So that"} {
			if strings.HasPrefix(line, start) {
				story = append(story, line)
			}
		}
	}
	assert.That(len(story) <= len(lines), "each story line is a description line")
	return story
}

// helperFiles lists dir/_shared/*.sh, sorted. No _shared folder means no helpers.
func helperFiles(dir string) ([]string, error) {
	assert.That(filepath.IsAbs(dir), "dir is absolute")
	// Glob returns matches in lexical order and only fails on a bad pattern.
	// Ref: https://pkg.go.dev/path/filepath#Glob
	paths, err := filepath.Glob(filepath.Join(dir, "_shared", "*.sh"))
	if err != nil {
		return nil, err
	}
	if len(paths) > HelperFilesMax {
		return nil, fmt.Errorf("%s/_shared holds %d helper files, more than %d",
			dir, len(paths), HelperFilesMax)
	}
	sort.Strings(paths)
	assert.That(len(paths) <= HelperFilesMax, "helper files stay bounded")
	return paths, nil
}

// ErrNoScenario is returned by Find when no scenario has the key.
var ErrNoScenario = errors.New("there's no scenario with that name")

// Find returns the scenario whose key is <capability slug>/<scenario slug>.
func (p *LoadedProject) Find(key string) (LoadedScenario, error) {
	assert.That(p != nil, "find runs on a project")
	assert.That(key != "", "a key names a scenario")
	capabilitySlug, scenarioSlug, ok := strings.Cut(key, "/")
	if !ok {
		return LoadedScenario{}, fmt.Errorf("%s: %w", key, ErrNoScenario)
	}
	for _, capability := range p.Capabilities {
		if capability.Slug != capabilitySlug {
			continue
		}
		for _, scenario := range capability.Scenarios {
			if scenario.Slug == scenarioSlug {
				return scenario, nil
			}
		}
	}
	assert.That(len(key) <= slug.NameLengthMax*2, "a key is two slugs")
	return LoadedScenario{}, fmt.Errorf("%s: %w", key, ErrNoScenario)
}

// LoadedScenario is one scenario with everything Givens needs to show it and set it up.
type LoadedScenario struct {
	Name       string
	Slug       string
	Capability string // the capability's slug
	Rule       string // the name of the Rule: block it's under, or ""
	Line       int64
	Outline    bool
	Tags       []string // the scenario's tags, then its Rule's and Feature's, without repeats
	OwnTags    []string // the tags on the lines above its Scenario: line only
	Error      bool     // tagged @error
	Todo       bool     // tagged @todo, on itself, its Rule or its Feature
	// StillRunning is when to show "Still running": StillRunningDefault, or the @timeout tag's.
	// It is zero when a @timeout tag can't be read.
	StillRunning time.Duration
	Background   []GherkinStep // the Feature's Background, then its Rule's
	Steps        []GherkinStep
	Examples     []ExamplesBlock
	Setup        string // scripts/<slug>.sh, or "" when there is none
	SetupCards   string // scripts/<slug>.md, or "" when there is none
	Teardown     string // scripts/<slug>.teardown.sh, or "" when there is none
}

// GherkinStep is one Gherkin step.
type GherkinStep struct {
	Keyword   string // "Given", "When", "Then", "And", "But" or "*", as written
	Text      string
	Line      int64
	DocString string     // the step's doc string, or ""
	Table     [][]string // the step's data table, header row first, or nil
}

// ExamplesBlock is one Examples block of a Scenario Outline.
type ExamplesBlock struct {
	Name   string
	Slug   string // "" for an unnamed block
	Header []string
	Rows   [][]string
}

// Key is <capability slug>/<scenario slug>, the name runs are recorded against.
func (s LoadedScenario) Key() string {
	assert.That(s.Capability != "", "a scenario belongs to a capability")
	assert.That(s.Slug != "", "a scenario has a slug")
	return s.Capability + "/" + s.Slug
}

// loadScenarios lists a Feature's scenarios in file order, Rules included, each with the
// Background steps and tags it inherits.
func loadScenarios(capability LoadedCapability, feature *messages.Feature) []LoadedScenario {
	assert.That(capability.Dir != "", "the capability has a folder")
	assert.That(feature != nil, "the feature is parsed")
	scenarios := make([]LoadedScenario, 0, len(feature.Children))
	featureSteps := make([]GherkinStep, 0, 4)
	for _, child := range feature.Children {
		if child.Background != nil {
			featureSteps = stepsOf(child.Background.Steps)
		}
		if child.Scenario != nil {
			scenarios = append(scenarios,
				loadScenario(capability, child.Scenario, featureSteps, feature.Tags, ""))
		}
		if child.Rule == nil {
			continue
		}
		ruleSteps := featureSteps
		inherited := append(append([]*messages.Tag{}, child.Rule.Tags...), feature.Tags...)
		for _, ruleChild := range child.Rule.Children {
			if ruleChild.Background != nil {
				ruleSteps = append(append([]GherkinStep{}, featureSteps...),
					stepsOf(ruleChild.Background.Steps)...)
			}
			if ruleChild.Scenario != nil {
				scenarios = append(scenarios, loadScenario(capability, ruleChild.Scenario,
					ruleSteps, inherited, child.Rule.Name))
			}
		}
	}
	assert.That(len(scenarios) <= spec.ScenariosPerFeatureMax, "scenarios stay bounded")
	return scenarios
}

// loadScenario builds one LoadedScenario and finds its setup files.
func loadScenario(
	capability LoadedCapability, scenario *messages.Scenario, background []GherkinStep,
	inherited []*messages.Tag, rule string,
) LoadedScenario {
	assert.That(scenario != nil, "the scenario is parsed")
	assert.That(scenario.Location != nil, "a parsed scenario has a location")
	key := slug.Of(scenario.Name)
	tags := tagNames(append(append([]*messages.Tag{}, scenario.Tags...), inherited...))
	built := LoadedScenario{
		Name: scenario.Name, Slug: key, Capability: capability.Slug, Rule: rule,
		Line:       scenario.Location.Line,
		Outline:    len(scenario.Examples) > 0,
		Tags:       tags,
		OwnTags:    tagNames(scenario.Tags),
		Todo:       slices.Contains(tags, TagTodo),
		Background: background,
		Steps:      stepsOf(scenario.Steps),
		Examples:   examplesOf(scenario.Examples),
	}
	built.Error, built.StillRunning = readScenarioTags(tags)
	if key != "" {
		scripts := filepath.Join(capability.Dir, "scripts")
		built.Setup = existingFile(filepath.Join(scripts, key+".sh"))
		built.SetupCards = existingFile(filepath.Join(scripts, key+".md"))
		built.Teardown = existingFile(filepath.Join(scripts, key+".teardown.sh"))
	}
	return built
}

// readScenarioTags reads @error and @timeout out of a scenario's tags.
func readScenarioTags(tags []string) (bool, time.Duration) {
	assert.That(len(tags) <= spec.ScenariosPerFeatureMax, "tags stay bounded")
	isError := false
	stillRunning := StillRunningDefault
	for _, tag := range tags {
		if tag == TagError {
			isError = true
		}
		if strings.HasPrefix(tag, "@timeout") {
			stillRunning = timeoutOf(tag)
		}
	}
	assert.That(stillRunning >= 0, "a timeout is never negative")
	return isError, stillRunning
}

// timeoutOf reads @timeout:90s or @timeout:5m, and gives zero for anything else.
func timeoutOf(tag string) time.Duration {
	assert.That(strings.HasPrefix(tag, "@timeout"), "tag is a timeout tag")
	value, ok := strings.CutPrefix(tag, TagTimeoutPrefix)
	if !ok || (!strings.HasSuffix(value, "s") && !strings.HasSuffix(value, "m")) {
		return 0
	}
	// ParseDuration also reads "1.5s" and "1m30s"; GIVENS.md allows whole seconds or minutes,
	// so those count as unreadable. Ref: https://pkg.go.dev/time#ParseDuration
	duration, err := time.ParseDuration(value)
	whole := strings.Trim(value[:len(value)-1], "0123456789") == ""
	if err != nil || duration <= 0 || !whole {
		return 0
	}
	assert.That(duration > 0, "a readable timeout is positive")
	return duration
}

// stepsOf copies parsed steps into GherkinSteps.
func stepsOf(parsed []*messages.Step) []GherkinStep {
	assert.That(len(parsed) <= spec.ScenariosPerFeatureMax, "steps stay bounded")
	steps := make([]GherkinStep, 0, len(parsed))
	for _, step := range parsed {
		built := GherkinStep{
			Keyword: strings.TrimSpace(step.Keyword), Text: step.Text,
			Line: step.Location.Line,
		}
		if step.DocString != nil {
			built.DocString = step.DocString.Content
		}
		if step.DataTable != nil {
			built.Table = tableRowsOf(step.DataTable.Rows)
		}
		steps = append(steps, built)
	}
	assert.That(len(steps) == len(parsed), "every step is copied")
	return steps
}

// examplesOf copies a Scenario Outline's Examples blocks.
func examplesOf(parsed []*messages.Examples) []ExamplesBlock {
	assert.That(len(parsed) <= spec.ScenariosPerFeatureMax, "examples stay bounded")
	blocks := make([]ExamplesBlock, 0, len(parsed))
	for _, block := range parsed {
		built := ExamplesBlock{Name: block.Name, Slug: slug.Of(block.Name)}
		if block.TableHeader != nil {
			built.Header = tableRowsOf([]*messages.TableRow{block.TableHeader})[0]
		}
		built.Rows = tableRowsOf(block.TableBody)
		blocks = append(blocks, built)
	}
	assert.That(len(blocks) == len(parsed), "every block is copied")
	return blocks
}

// tableRowsOf copies table rows into cell values.
func tableRowsOf(parsed []*messages.TableRow) [][]string {
	assert.That(len(parsed) <= spec.ScenariosPerFeatureMax, "rows stay bounded")
	rows := make([][]string, 0, len(parsed))
	for _, row := range parsed {
		cells := make([]string, 0, len(row.Cells))
		for _, cell := range row.Cells {
			cells = append(cells, cell.Value)
		}
		rows = append(rows, cells)
	}
	assert.That(len(rows) == len(parsed), "every row is copied")
	return rows
}

// tagNames lists tag names in order, without repeats.
func tagNames(tags []*messages.Tag) []string {
	assert.That(len(tags) <= spec.ScenariosPerFeatureMax, "tags stay bounded")
	names := make([]string, 0, len(tags))
	seen := make(map[string]bool, len(tags))
	for _, tag := range tags {
		if !seen[tag.Name] {
			names = append(names, tag.Name)
		}
		seen[tag.Name] = true
	}
	assert.That(len(names) <= len(tags), "no tag is added")
	return names
}

// existingFile returns path when a file is there, and "" when not.
func existingFile(path string) string {
	assert.That(filepath.IsAbs(path), "path is absolute")
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return ""
	}
	assert.That(!info.IsDir(), "only a file is returned")
	return path
}

// capabilitiesFolderName is the folder Open looks for inside a project folder.
const capabilitiesFolderName = "capabilities"

// findCapabilities resolves what Open was given to a capabilities folder: the folder itself
// when it holds capabilities or is named "capabilities", else its "capabilities" subfolder.
// found is false when neither is there; path is then the absolute folder given.
func findCapabilities(path string) (string, bool, error) {
	assert.That(path != "", "a path is given")
	assert.That(len(path) <= PathLengthMax, "the path fits PathLengthMax")
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", false, err
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", false, err
	}
	if !info.IsDir() {
		return "", false, errors.New(absolute + " isn't a folder")
	}
	inside := filepath.Join(absolute, capabilitiesFolderName)
	if info, err := os.Stat(inside); err == nil && info.IsDir() {
		return inside, true, nil
	}
	if filepath.Base(absolute) == capabilitiesFolderName {
		return absolute, true, nil
	}
	loaded, err := LoadProject(absolute)
	if err != nil {
		return "", false, err
	}
	return absolute, len(loaded.Capabilities) > 0, nil
}

// reload reads the window's capabilities folder again and starts watching it. A window with
// no capabilities folder has no project.
func (w *Window) reload() error {
	assert.That(w != nil, "reload runs on a window")
	assert.That(w.kind == WindowProject, "only a project window has a project")
	if !w.found {
		w.project, w.watcher = nil, nil
		return nil
	}
	w.restoreKit()
	// The watcher's first look comes before the load, so a write in between is seen next time.
	watcher, err := NewWatcher(w.path, time.Now())
	if err != nil {
		return err
	}
	loaded, err := LoadProject(w.path)
	if err != nil {
		return err
	}
	w.app.withoutTrashed(loaded)
	w.project, w.watcher, w.statuses, w.helpers = loaded, watcher, nil, nil
	w.histories, w.index, w.searchMemo = nil, nil, nil
	return nil
}

// refresh reloads the project when another tool changed its files. Givens keeps no copy of
// its own, so every read of a window starts here, and looks at the files each time: a step
// that writes a file and reads the window at once must see the write. While a snapshot is
// being built it looks once, not once per part. A folder that can't be read keeps showing what
// was last read.
func (w *Window) refresh() {
	assert.That(w != nil, "refresh runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	w.pollRunsSync()
	if w.watcher == nil || w.holdRefresh {
		return
	}
	changed, err := w.watcher.Look(time.Now())
	if err != nil || !changed {
		return
	}
	w.statuses, w.helpers, w.histories, w.index, w.searchMemo = nil, nil, nil, nil, nil
	w.restoreKit()
	if loaded, err := LoadProject(w.path); err == nil {
		w.app.withoutTrashed(loaded)
		w.project = loaded
	}
}

// findCapability finds a capability by its name or slug.
func (w *Window) findCapability(name string) (LoadedCapability, bool) {
	assert.That(w != nil, "find runs on a window")
	assert.That(name != "", "a capability has a name")
	w.refresh()
	if w.project == nil {
		return LoadedCapability{}, false
	}
	for _, capability := range w.project.Capabilities {
		if capability.Name == name || capability.Slug == name {
			return capability, true
		}
	}
	return LoadedCapability{}, false
}

// findScenario finds a scenario by its name, "Capability/Scenario", or its run key. An exact
// name wins over a slug, so two names with the same slug can still each be found.
func (w *Window) findScenario(name string) (LoadedScenario, bool) {
	assert.That(w != nil, "find runs on a window")
	assert.That(name != "", "a scenario has a name")
	w.refresh()
	if w.project == nil {
		return LoadedScenario{}, false
	}
	if capabilityName, scenarioName, ok := strings.Cut(name, "/"); ok && scenarioName != "" {
		if found, ok := w.findIn(capabilityName, scenarioName); ok {
			return found, true
		}
	}
	return w.findIn("", name)
}

// findIn finds a scenario by exact name, then by slug, in one capability or, for "", in any.
func (w *Window) findIn(capabilityName string, name string) (LoadedScenario, bool) {
	assert.That(w.project != nil, "find in runs on a loaded project")
	assert.That(name != "", "a scenario has a name")
	want := slug.Of(name)
	for _, bySlug := range []bool{false, true} {
		for _, capability := range w.project.Capabilities {
			if capabilityName != "" && capability.Name != capabilityName &&
				capability.Slug != capabilityName {
				continue
			}
			for _, scenario := range capability.Scenarios {
				if (!bySlug && scenario.Name == name) || (bySlug && scenario.Slug == want) {
					return scenario, true
				}
			}
		}
	}
	return LoadedScenario{}, false
}

// writable refuses a change in a branch's window. The UI doesn't offer one there; this stops
// a Needs call or a CLI one that asks anyway.
func (w *Window) writable() error {
	assert.That(w != nil, "writable runs on a window")
	assert.That(w.branch == "" || w.branchOrigin != "", "a branch's window knows its origin")
	if w.branch != "" {
		return fmt.Errorf("%s @ %s: %w", filepath.Base(w.branchOrigin), w.branch, ErrReadOnly)
	}
	return nil
}

// Edit opens a scenario, by name, at its block in its .feature file, or a file, by its path
// relative to the capabilities folder ("checkout/about.md"), in the editor. The editor holds
// the file's whole text, comments and all, so saving writes back only what was typed.
// Needed by the editing, saving and preview scenarios in capabilities/write-stories-and-scenarios.
func (w *Window) Edit(target string) (*Editor, error) {
	assert.That(w != nil, "edit runs on a window")
	assert.That(target != "", "edit names a scenario or a file")
	assert.That(!filepath.IsAbs(target), "a file is named relative to the capabilities folder")
	if err := w.writable(); err != nil {
		return nil, err
	}
	if w.project == nil {
		return nil, fmt.Errorf("%s: %w", target, ErrNotFound)
	}
	path := filepath.Join(w.path, target)
	if scenario, ok := w.findScenario(target); ok {
		capability, _ := w.findCapability(scenario.Capability)
		path = capability.FeaturePath
	}
	if !strings.HasPrefix(path, w.path+string(filepath.Separator)) {
		return nil, fmt.Errorf("%s is outside the capabilities folder: %w", target, ErrNotFound)
	}
	text, err := readEditable(path)
	if err != nil {
		return nil, err
	}
	w.editor = &Editor{window: w, path: path, text: text, opened: text, problems: []string{},
		typing: -1, marks: []model.EditorMark{}}
	if scenario, ok := w.findScenario(target); ok {
		w.editor.scenario = scenario.Name
		w.editor.line = int(scenario.Line)
		w.editor.cursor = blockEnd(strings.Split(text, "\n"), int(scenario.Line)-1)
	}
	return w.editor, nil
}

// readEditable reads a file the editor can hold: text, within EditorTextBytesMax.
func readEditable(path string) (string, error) {
	assert.That(filepath.IsAbs(path), "the path is absolute")
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("%s: %w", filepath.Base(path), ErrNotFound)
	}
	if err != nil {
		return "", err
	}
	if len(content) > model.EditorTextBytesMax || !utf8.Valid(content) {
		return "", fmt.Errorf("%s isn't a text file Givens can edit", filepath.Base(path))
	}
	assert.That(len(content) <= model.EditorTextBytesMax, "editable text stays bounded")
	return string(content), nil
}

// writeNew writes a new file, making its folder, unless one is already there.
func writeNew(path string, text string) error {
	assert.That(filepath.IsAbs(path), "the path is absolute")
	assert.That(utf8.ValidString(text), "the file is text")
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(text), 0o644)
}

// trash moves a path to the Trash. Under the Mac app the move happens a moment later, so
// until it's gone the path is pending and loads leave it out.
func (a *App) trash(path string) error {
	assert.That(a != nil, "trash runs on an app")
	assert.That(filepath.IsAbs(path), "a trashed path is absolute")
	if err := a.config.Trash.Move(path); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		if a.pendingTrash == nil {
			a.pendingTrash = make(map[string]bool, 4)
		}
		a.pendingTrash[path] = true
	}
	return nil
}

// withoutTrashed leaves out of a loaded project what's waiting to go to the Trash, and
// forgets paths the Trash now has.
func (a *App) withoutTrashed(loaded *LoadedProject) {
	assert.That(a != nil, "without trashed runs on an app")
	assert.That(loaded != nil, "a project was loaded")
	for path := range a.pendingTrash {
		if _, err := os.Stat(path); err != nil {
			delete(a.pendingTrash, path)
		}
	}
	if len(a.pendingTrash) == 0 {
		return
	}
	kept := loaded.Capabilities[:0]
	for _, capability := range loaded.Capabilities {
		if a.pendingTrash[capability.Dir] {
			continue
		}
		for index := range capability.Scenarios {
			scenario := &capability.Scenarios[index]
			for _, path := range []*string{&scenario.Setup, &scenario.SetupCards,
				&scenario.Teardown} {
				if a.pendingTrash[*path] {
					*path = ""
				}
			}
		}
		kept = append(kept, capability)
	}
	loaded.Capabilities = kept
}

// From internal/markdown, which reads a setup done by hand, scripts/<slug>.md: the text before
// the first numbered item, and each top-level numbered item as one step, with its fenced code
// blocks, inline code, links and images. It reads only what setups use, not all of CommonMark.
// Drop in's cards and runBlocksIn read it, and `givens run` (cmd/givens/run.go) prints it.
// Drop in renders a step's text with RenderedMarkdown, in its section of big.go, and `givens
// run` with PlainMarkdown, in Use Givens from the terminal's.
// Ref: https://spec.commonmark.org/0.31.2/#ordered-list
// Ref: https://spec.commonmark.org/0.31.2/#fenced-code-blocks

// Limits. Tiger Style: put a limit on everything.
const (
	// MarkdownSetupBytesMax bounds one setup file.
	MarkdownSetupBytesMax = 1 << 20
	// MarkdownStepsMax bounds the numbered items in one file.
	MarkdownStepsMax = 256
)

// Patterns, compiled once.
const (
	markdownItemPattern   = `^(\d+)\.\s+(.*)$`
	markdownLinkPattern   = `(!?)\[([^\]]*)\]\(([^)\s]*)\)`
	markdownInlinePattern = "`([^`]+)`"
	// A fence's info string: its language, then any other words, like "run".
	// Ref: https://spec.commonmark.org/0.31.2/#info-string
	markdownFencePattern = "^\\s*(```+|~~~+)\\s*([A-Za-z0-9_+-]*)((?:[ \\t]+[^\\s`]+)*)\\s*$"
)

var (
	markdownItem   = regexp.MustCompile(markdownItemPattern)   //tiger:allow-global compiled once
	markdownLink   = regexp.MustCompile(markdownLinkPattern)   //tiger:allow-global compiled once
	markdownInline = regexp.MustCompile(markdownInlinePattern) //tiger:allow-global compiled once
	markdownFence  = regexp.MustCompile(markdownFencePattern)  //tiger:allow-global compiled once
)

// FencedBlock is one fenced code block.
type FencedBlock struct {
	Language string // the info string's first word: "bash", "js", or ""
	Run      bool   // the info string's next word is "run": Givens runs the block itself
	Code     string // exactly the code: no fences, no list indent
	Closed   bool   // false when the fence never closes
	Line     int    // the opening fence's line, counting from 1
}

// MarkdownStep is one top-level numbered item.
type MarkdownStep struct {
	Number int
	Text   string        // the item's markdown, list number taken off, code blocks left out
	Blocks []FencedBlock // its fenced code blocks, in order
	Links  []string      // link targets, in order
	Images []string      // image paths, in order
	Inline []string      // inline code spans, in order
}

// MarkdownSetup is a whole markdown setup.
type MarkdownSetup struct {
	Intro string // the text before the first numbered item, trimmed
	Steps []MarkdownStep
}

// ParseMarkdownSetup reads a markdown setup.
func ParseMarkdownSetup(text string) (MarkdownSetup, error) {
	assert.That(len(text) <= MarkdownSetupBytesMax*2, "the text stays bounded")
	if len(text) > MarkdownSetupBytesMax {
		return MarkdownSetup{}, fmt.Errorf("a markdown setup is at most %d bytes",
			MarkdownSetupBytesMax)
	}
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	setup := MarkdownSetup{Steps: make([]MarkdownStep, 0, 8)}
	intro := make([]string, 0, 4)
	var current *MarkdownStep
	var open *FencedBlock
	indent := 0
	for index, line := range lines {
		if open != nil {
			open, indent = closeOrAdd(current, open, line, indent)
			continue
		}
		if match := markdownItem.FindStringSubmatch(line); match != nil {
			if len(setup.Steps) >= MarkdownStepsMax {
				return MarkdownSetup{}, fmt.Errorf("a markdown setup has at most %d steps",
					MarkdownStepsMax)
			}
			setup.Steps = append(setup.Steps, MarkdownStep{Number: len(setup.Steps) + 1,
				Text: match[2]})
			current = &setup.Steps[len(setup.Steps)-1]
			continue
		}
		if current == nil {
			intro = append(intro, line)
			continue
		}
		if match := markdownFence.FindStringSubmatch(line); match != nil {
			words := strings.Fields(match[3])
			current.Blocks = append(current.Blocks, FencedBlock{Language: match[2],
				Run: len(words) > 0 && words[0] == "run", Line: index + 1})
			open, indent = &current.Blocks[len(current.Blocks)-1], leadingSpaces(line)
			continue
		}
		current.Text += "\n" + strings.TrimSpace(line)
	}
	setup.Intro = strings.TrimSpace(strings.Join(intro, "\n"))
	for index := range setup.Steps {
		finishMarkdownStep(&setup.Steps[index])
	}
	assert.That(len(setup.Steps) <= MarkdownStepsMax, "steps stay bounded")
	return setup, nil
}

// closeOrAdd ends an open code block at its closing fence, or adds a line to it with the
// fence's indent taken off. It returns the block still open, or nil.
func closeOrAdd(step *MarkdownStep, open *FencedBlock, line string,
	indent int) (*FencedBlock, int) {
	assert.That(step != nil, "a code block is inside a step")
	assert.That(open != nil, "a block is open")
	// A closing fence is three or more backticks or tildes, and nothing else.
	if bare := strings.TrimSpace(line); len(bare) >= 3 && strings.Trim(bare, "`~") == "" {
		open.Closed = true
		return nil, 0
	}
	trimmed := strings.TrimLeft(line, " ")
	if leadingSpaces(line) >= indent {
		trimmed = line[indent:]
	}
	if open.Code == "" {
		open.Code = trimmed
	} else {
		open.Code += "\n" + trimmed
	}
	return open, indent
}

// leadingSpaces counts the spaces a line starts with.
func leadingSpaces(line string) int {
	assert.That(len(line) <= MarkdownSetupBytesMax, "a line stays bounded")
	count := len(line) - len(strings.TrimLeft(line, " "))
	assert.That(count >= 0 && count <= len(line), "spaces are part of the line")
	return count
}

// finishMarkdownStep trims a step's text and reads its links, images and inline code.
func finishMarkdownStep(step *MarkdownStep) {
	assert.That(step != nil, "there's a step to finish")
	assert.That(step.Number > 0, "steps count from 1")
	step.Text = strings.TrimSpace(step.Text)
	for index := range step.Blocks {
		step.Blocks[index].Code = strings.TrimRight(step.Blocks[index].Code, "\n")
	}
	for _, match := range markdownLink.FindAllStringSubmatch(step.Text, -1) {
		if match[1] == "!" {
			step.Images = append(step.Images, match[3])
		} else {
			step.Links = append(step.Links, match[3])
		}
	}
	for _, match := range markdownInline.FindAllStringSubmatch(step.Text, -1) {
		step.Inline = append(step.Inline, match[1])
	}
}

// runBlocksIn reads a markdown setup's run blocks' code, a js block as the in_browser line it
// runs as, so Drop in's checks read them as a script's lines. Empty when nothing runs.
func runBlocksIn(path string) []string {
	assert.That(path == "" || filepath.IsAbs(path), "a setup file is absolute")
	assert.That(MarkdownStepsMax > 0, "steps have a limit")
	texts := make([]string, 0, 2)
	if path == "" {
		return texts
	}
	text, err := ReadScript(path)
	if err != nil {
		return texts
	}
	parsed, err := ParseMarkdownSetup(text)
	if err != nil {
		return texts
	}
	for _, step := range parsed.Steps {
		for _, block := range step.Blocks {
			switch runLanguage(block) {
			case "bash":
				texts = append(texts, block.Code)
			case "js":
				texts = append(texts, "in_browser js")
			}
		}
	}
	return texts
}

// scratchDir is the session's folder of its own, made on first use: argument files and run
// blocks' scripts. removeArguments removes it.
func (s *Session) scratchDir() (string, error) {
	assert.That(s != nil, "scratch dir runs on a session")
	assert.That(s.window != nil, "a session belongs to a window")
	if s.argumentsDir == "" {
		// Ref: https://pkg.go.dev/os#MkdirTemp
		dir, err := os.MkdirTemp("", "givens-arguments-")
		if err != nil {
			return "", err
		}
		s.argumentsDir = dir
	}
	return s.argumentsDir, nil
}

// checklistOf is what the developer does once setup is done: first_you lines from the setup
// script, then the When and Then steps in the order they're written. And and But take the
// kind before them.
func checklistOf(scenario LoadedScenario, row []RunPair) []model.CheckItem {
	assert.That(scenario.Name != "", "the scenario has a name")
	assert.That(len(scenario.Steps) <= HelpersMax, "steps stay bounded")
	items := make([]model.CheckItem, 0, len(scenario.Steps)+2)
	if scenario.Setup != "" {
		if text, err := ReadScript(scenario.Setup); err == nil {
			for _, match := range firstYouLine.FindAllStringSubmatch(text, -1) {
				items = append(items, model.CheckItem{Text: "First, " + unquote(match[1]),
					Kind: "given"})
			}
		}
	}
	kind := "given"
	for _, step := range scenario.Steps {
		switch step.Keyword {
		case "Given", "When", "Then":
			kind = strings.ToLower(step.Keyword)
		}
		if kind != "given" {
			text := step.Keyword + " " + fillPlaceholders(step.Text, row)
			items = append(items, model.CheckItem{Text: text, Kind: kind})
		}
	}
	return items
}

// fail ends the session with a message and no checklist.
func (s *Session) fail(message string) {
	assert.That(message != "", "a failure says what failed")
	assert.That(s.status != SessionReady, "a finished setup doesn't fail afterwards")
	s.status, s.message, s.checklist = SessionFailed, message, nil
}

// restart drops in again to the same scenario and row: "Try again" and "Reset state".
func (s *Session) restart(skipTeardown bool) error {
	assert.That(s != nil, "restart runs on a session")
	assert.That(s.window != nil, "a session belongs to a window")
	scenario, ok := s.window.findScenario(s.scenario.Capability + "/" + s.scenario.Name)
	if !ok {
		return fmt.Errorf("%s: %w", s.scenario.Name, ErrNotFound)
	}
	_, err := s.window.startSession(scenario, outlineRow{values: s.row, examples: s.examples},
		skipTeardown)
	return err
}

// KitSourceLine is the first line of every setup script: it sources the kit.
const KitSourceLine = `source "$(dirname "${BASH_SOURCE[0]}")/../../_shared/givens.sh"`

// resume ends a wait at a card, adding the time waited so it doesn't count as slow.
func (s *Session) resume() {
	assert.That(s != nil, "resume runs on a session")
	assert.That(!s.started.IsZero(), "the session started")
	if s.status == SessionWaiting {
		s.waited += s.window.app.config.Clock.Now().Sub(s.waitingSince)
		s.status = SessionRunning
	}
}

// editFile opens a file inside the capabilities folder in the editor.
func (w *Window) editFile(path string) error {
	assert.That(w != nil, "edit file runs on a window")
	assert.That(path == "" || filepath.IsAbs(path), "the path is absolute")
	if path == "" {
		return ErrNotFound
	}
	relative, err := filepath.Rel(w.path, path)
	if err != nil {
		return err
	}
	_, err = w.Edit(relative)
	return err
}

// outlineRow is one Examples row: its block's slug and its values in column order. The zero
// value is "not an outline row".
type outlineRow struct {
	examples string
	values   []RunPair
}

// rowsOf lists an outline's rows, block by block, in file order.
func rowsOf(scenario LoadedScenario) []outlineRow {
	assert.That(scenario.Name != "", "the scenario has a name")
	assert.That(len(scenario.Examples) <= HelperFilesMax, "blocks stay bounded")
	rows := make([]outlineRow, 0, 4)
	for _, block := range scenario.Examples {
		for _, cells := range block.Rows {
			values := make([]RunPair, 0, len(block.Header))
			for column, name := range block.Header {
				if column < len(cells) {
					values = append(values, RunPair{Name: name, Value: cells[column]})
				}
			}
			rows = append(rows, outlineRow{examples: block.Slug, values: values})
		}
	}
	return rows
}

// rowStatus says where one outline row stands, as StatusOf does for a scenario.
func rowStatus(loaded *LoadedProject, scenario LoadedScenario, row outlineRow,
	now time.Time) string {
	assert.That(loaded != nil, "status is read in a loaded project")
	assert.That(scenario.Outline, "only an outline has rows")
	return statusFor(loaded, scenario, RunKey(scenario.Key(), row.examples), row.values, now)
}

// rowName says a row as a person would: its first value, like "expired".
func rowName(row []RunPair) string {
	assert.That(len(row) <= RunFieldsMax, "a row stays bounded")
	if len(row) == 0 {
		return ""
	}
	assert.That(row[0].Name != "", "a value has a column")
	return row[0].Value
}

// removeArguments removes the session's argument files, if it wrote any.
func (s *Session) removeArguments() {
	assert.That(s != nil, "arguments are removed for a session")
	assert.That(s.argumentsDir == "" || strings.Contains(s.argumentsDir, "givens-arguments-"),
		"only an arguments folder is removed")
	if s.argumentsDir != "" {
		_ = os.RemoveAll(s.argumentsDir)
	}
}

// From internal/setup, which runs one setup script: bash, in its own process group, with
// GIVENS_* variables, its output read line by line, answers written to its stdin, and a stop
// that ends the script and everything it started. Drop in runs setups with it and Mix shell
// its run blocks.
//
// It decides nothing about what a scenario means; its callers choose the script, the
// variables and when to show "Still running".

// Limits. Tiger Style: put a limit on everything.
// Ref: https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md#safety
const (
	// SetupDurationMax ends a setup nobody stopped. "Keep waiting" waits up to here.
	SetupDurationMax = time.Hour
	// SetupOutputLinesMax bounds the lines kept; later lines are counted, not kept.
	SetupOutputLinesMax = 10000
	// SetupLineLengthBytesMax bounds one output line; a longer line is cut.
	SetupLineLengthBytesMax = 64 << 10
	// SetupEnvMax bounds the variables a request adds.
	SetupEnvMax = 256
	// stopGrace is how long Stop waits after SIGTERM before SIGKILL.
	stopGrace = 2 * time.Second
	// drainWait is how long output may stay open after the script exits, held by something it
	// left running in the background.
	drainWait = time.Second
)

// ErrSetupStopped is the SetupResult.Err of a setup ended by Stop or by SetupDurationMax.
var ErrSetupStopped = errors.New("setup was stopped")

// SetupRequest is one script to run.
type SetupRequest struct {
	Script string   // absolute path to a .sh file
	Dir    string   // where it runs: the project's folder
	Env    []string // NAME=value pairs added to Givens's own environment
}

// SetupResult is how a finished setup ended.
type SetupResult struct {
	ExitCode int   // the script's exit code; -1 when it was stopped or killed
	Err      error // nil, ErrSetupStopped, or why it couldn't run to its end
}

// SetupProcess is one running setup.
type SetupProcess struct {
	command *exec.Cmd
	stdin   *os.File
	done    chan struct{}

	mutex   sync.Mutex
	lines   []string
	dropped int
	stopped bool
	result  SetupResult
}

// StartSetup runs request's script and returns at once. Read its progress with Output and Done.
func StartSetup(request SetupRequest) (*SetupProcess, error) {
	assert.That(filepath.IsAbs(request.Script), "the script path is absolute")
	assert.That(filepath.IsAbs(request.Dir), "the folder is absolute")
	if len(request.Env) > SetupEnvMax {
		return nil, fmt.Errorf("setup has %d variables, more than %d", len(request.Env),
			SetupEnvMax)
	}
	command := exec.Command("bash", request.Script)
	command.Dir = request.Dir
	command.Env = append(os.Environ(), request.Env...)
	// Setpgid puts the script in a new process group, so Stop can signal everything it
	// started. Ref: https://pkg.go.dev/syscall#SysProcAttr
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	pipes, err := newPipes()
	if err != nil {
		return nil, err
	}
	command.Stdin, command.Stdout, command.Stderr = pipes.stdinRead, pipes.outWrite,
		pipes.outWrite
	if err := command.Start(); err != nil {
		pipes.closeAll()
		return nil, err
	}
	// The child holds its own copies now; closing ours lets the reader see the end of output.
	_ = pipes.stdinRead.Close()
	_ = pipes.outWrite.Close()
	process := &SetupProcess{
		command: command, stdin: pipes.stdinWrite, done: make(chan struct{}),
		lines: make([]string, 0, 64),
	}
	go process.run(pipes.outRead)
	assert.That(command.Process != nil, "a started command has a process")
	return process, nil
}

// pipes are the script's stdin and its combined stdout and stderr.
type pipes struct {
	stdinRead, stdinWrite, outRead, outWrite *os.File
}

// newPipes opens both pipes. Ref: https://pkg.go.dev/os#Pipe
func newPipes() (pipes, error) {
	stdinRead, stdinWrite, err := os.Pipe()
	if err != nil {
		return pipes{}, err
	}
	outRead, outWrite, err := os.Pipe()
	if err != nil {
		_ = stdinRead.Close()
		_ = stdinWrite.Close()
		return pipes{}, err
	}
	assert.That(stdinRead != nil && stdinWrite != nil, "the stdin pipe is open")
	assert.That(outRead != nil && outWrite != nil, "the output pipe is open")
	return pipes{stdinRead, stdinWrite, outRead, outWrite}, nil
}

// closeAll closes every end, for a script that never started.
func (p pipes) closeAll() {
	assert.That(p.stdinRead != nil, "the pipes were opened")
	assert.That(p.outRead != nil, "the pipes were opened")
	for _, file := range []*os.File{p.stdinRead, p.stdinWrite, p.outRead, p.outWrite} {
		_ = file.Close()
	}
}

// run reads output to its end, waits for the script, and closes done.
func (p *SetupProcess) run(output *os.File) {
	assert.That(p != nil, "run runs on a process")
	assert.That(output != nil, "the output pipe is open")
	limit := time.AfterFunc(SetupDurationMax, func() { _ = p.Stop() })
	defer limit.Stop()
	drained := make(chan struct{})
	go p.read(output, drained)
	err := p.command.Wait()
	select {
	case <-drained:
	case <-time.After(drainWait):
		// Something the script left running still holds the output open: end it.
		_ = syscall.Kill(-p.command.Process.Pid, syscall.SIGKILL)
		_ = output.Close()
		<-drained
	}
	_ = p.stdin.Close()
	p.finish(err)
}

// read appends each output line until the pipe closes.
func (p *SetupProcess) read(output *os.File, drained chan struct{}) {
	assert.That(p != nil, "read runs on a process")
	assert.That(drained != nil, "read signals when it's done")
	defer close(drained)
	defer func() { _ = output.Close() }()
	reader := bufio.NewReaderSize(output, 4096)
	// bound: ReadString returns an error at the end of output, which ends the loop.
	for {
		line, err := reader.ReadString('\n')
		if line != "" {
			p.add(strings.TrimRight(line, "\r\n"))
		}
		if err != nil {
			return
		}
	}
}

// add keeps one line, cut to SetupLineLengthBytesMax, while fewer than SetupOutputLinesMax are
// kept.
func (p *SetupProcess) add(line string) {
	assert.That(p != nil, "add runs on a process")
	if len(line) > SetupLineLengthBytesMax {
		line = line[:SetupLineLengthBytesMax]
	}
	p.mutex.Lock()
	defer p.mutex.Unlock()
	if len(p.lines) >= SetupOutputLinesMax {
		p.dropped++
		return
	}
	p.lines = append(p.lines, line)
	assert.That(len(p.lines) <= SetupOutputLinesMax, "kept lines stay bounded")
}

// finish records how the script ended and closes done.
func (p *SetupProcess) finish(err error) {
	assert.That(p != nil, "finish runs on a process")
	p.mutex.Lock()
	result := SetupResult{ExitCode: p.command.ProcessState.ExitCode()}
	var exit *exec.ExitError
	switch {
	case p.stopped:
		result = SetupResult{ExitCode: -1, Err: ErrSetupStopped}
	case err != nil && !errors.As(err, &exit):
		result.Err = err
	}
	p.result = result
	p.mutex.Unlock()
	close(p.done)
	assert.That(result.Err != nil || result.ExitCode >= 0, "a script that ran has an exit code")
}

// Output is a copy of the lines read so far.
func (p *SetupProcess) Output() []string {
	assert.That(p != nil, "output runs on a process")
	p.mutex.Lock()
	defer p.mutex.Unlock()
	lines := make([]string, len(p.lines))
	copy(lines, p.lines)
	assert.That(len(lines) <= SetupOutputLinesMax, "output stays bounded")
	return lines
}

// Dropped counts the lines read after SetupOutputLinesMax were kept.
func (p *SetupProcess) Dropped() int {
	assert.That(p != nil, "dropped runs on a process")
	p.mutex.Lock()
	defer p.mutex.Unlock()
	assert.That(p.dropped >= 0, "a count is never negative")
	return p.dropped
}

// Done is closed once the script has ended and its output is read.
func (p *SetupProcess) Done() <-chan struct{} {
	assert.That(p != nil, "done runs on a process")
	assert.That(p.done != nil, "the process was started")
	return p.done
}

// Wait waits up to limit for the script to end. It returns the SetupResult and true when it has,
// or false when it's still running.
func (p *SetupProcess) Wait(limit time.Duration) (SetupResult, bool) {
	assert.That(p != nil, "wait runs on a process")
	assert.That(limit > 0, "a wait has a limit")
	select {
	case <-p.done:
		p.mutex.Lock()
		defer p.mutex.Unlock()
		return p.result, true
	case <-time.After(limit):
		return SetupResult{}, false
	}
}

// Answer writes one line to the script's stdin, for a card that asks a question.
func (p *SetupProcess) Answer(text string) error {
	assert.That(p != nil, "answer runs on a process")
	assert.That(!strings.Contains(text, "\n"), "an answer is one line")
	_, err := p.stdin.WriteString(text + "\n")
	return err
}

// Stop ends the script and everything it started: SIGTERM to its process group, then SIGKILL
// after a grace period. Output read so far is kept. Stopping a finished script does nothing.
// Ref: https://man7.org/linux/man-pages/man2/kill.2.html (a negative pid names a group)
func (p *SetupProcess) Stop() error {
	assert.That(p != nil, "stop runs on a process")
	assert.That(p.command.Process != nil, "the process was started")
	select {
	case <-p.done:
		return nil
	default:
	}
	p.mutex.Lock()
	p.stopped = true
	p.mutex.Unlock()
	group := -p.command.Process.Pid
	if err := syscall.Kill(group, syscall.SIGTERM); err != nil && err != syscall.ESRCH {
		return err
	}
	select {
	case <-p.done:
		return nil
	case <-time.After(stopGrace):
	}
	if err := syscall.Kill(group, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
		return err
	}
	<-p.done
	return nil
}
