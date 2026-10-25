package app

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"givens/internal/assert"
	"givens/internal/helpers"
	"givens/internal/project"
	"givens/internal/runs"
	"givens/internal/spec"
)

// firstYouLine matches a `first_you '…'` line in a setup script: a Given done by hand, which
// the checklist shows first. Quiet mode hides the kit's own "First, …" line.
const firstYouPattern = `(?m)^\s*first_you\s+(.+?)\s*$`

var firstYouLine = regexp.MustCompile(firstYouPattern) //tiger:allow-global compiled once

// DropIn drops in to a scenario: it runs the setup and opens the Drop in page. Before anything
// runs it checks the commands the setup needs and the setups it builds on.
func (w *Window) DropIn(scenario string) (*Session, error) {
	return w.dropIn(scenario, false)
}

// dropIn drops in, leaving the teardown out when skipTeardown is set: "Run setup anyway".
func (w *Window) dropIn(scenario string, skipTeardown bool) (*Session, error) {
	assert.That(w != nil, "drop in runs on a window")
	assert.That(len(scenario) <= NameLengthMax, "the name fits NameLengthMax")
	found, ok := w.findScenario(scenario)
	if !ok {
		return nil, fmt.Errorf("%s: %w", scenario, ErrNotFound)
	}
	return w.startSession(found, outlineRow{}, skipTeardown)
}

// startSession opens the Drop in page for a scenario, or one row of an outline, and runs its
// setup once the project is trusted. A setup with no script needs no trust: nothing runs.
func (w *Window) startSession(found project.Scenario, row outlineRow, skipTeardown bool) (
	*Session, error) {
	assert.That(w != nil, "start session runs on a window")
	assert.That(found.Capability != "", "the scenario belongs to a capability")
	if w.branch != "" {
		return nil, fmt.Errorf("%s isn't checked out, so its setup scripts don't run", w.branch)
	}
	trusted, refused, err := w.trustOf()
	if err != nil {
		return nil, err
	}
	session := &Session{
		window: w, scenario: found, status: SessionRunning, skipTeardown: skipTeardown,
		started: w.app.config.Clock.Now(), env: setupEnv(found, row.values),
		markdownFrom: -1, row: row.values, examples: row.examples,
	}
	if w.session != nil {
		w.session.removeArguments()
	}
	w.session = session
	// A markdown setup's run blocks are code too, so they're asked about as a script is.
	runsScripts := found.Setup != "" || found.Teardown != "" ||
		len(runBlocksIn(found.SetupCards)) > 0
	return session, w.runTrusted(session, runsScripts, trusted, refused)
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

// begin checks what setup needs, then starts its scripts: teardown first, then setup.
func (s *Session) begin() error {
	assert.That(s.status == SessionRunning, "a session begins running")
	assert.That(s.process == nil, "a session begins once")
	s.started = s.window.app.config.Clock.Now()
	if problem, hints, err := s.window.precheck(s.scenario); err != nil || problem != "" {
		s.hints = hints
		if err == nil {
			s.fail(problem)
		}
		return err
	}
	arguments, err := s.writeArguments()
	if err != nil {
		return err
	}
	bin, err := s.givensBin()
	if err != nil {
		return err
	}
	s.env = append(append(s.env, arguments...), bin)
	for _, script := range []string{s.scenario.Teardown, s.scenario.Setup} {
		if script != "" && !(s.skipTeardown && script == s.scenario.Teardown) {
			s.scripts = append(s.scripts, script)
		}
	}
	if len(s.scripts) == 0 {
		s.finishScripts()
		return nil
	}
	s.start()
	return nil
}

// setupEnv is the variables a Drop in runs setup with. GIVENS_QUIET hides the "your turn"
// lines, which the checklist shows instead; GIVENS_UI makes by_hand and ask cards. begin adds
// GIVENS_BIN and the arguments' files.
// See capabilities/the-kit/about.md.
func setupEnv(scenario project.Scenario, row []runs.Pair) []string {
	assert.That(scenario.Capability != "", "the scenario belongs to a capability")
	assert.That(len(row) <= helpers.HelpersMax, "a row stays bounded")
	env := []string{"GIVENS_QUIET=1", "GIVENS_UI=1", "GIVENS_SCENARIO=" + scenario.Key()}
	for _, value := range row {
		env = append(env, "GIVENS_EX_"+value.Name+"="+value.Value)
	}
	return env
}

// precheck finds what would stop setup before it runs: a from line naming no scenario, or a
// command the setup or its helpers need that isn't installed. It returns the message to show,
// and any install hints.
func (w *Window) precheck(scenario project.Scenario) (string, []string, error) {
	assert.That(w.project != nil, "a Drop in runs in a loaded project")
	assert.That(scenario.Capability != "", "the scenario belongs to a capability")
	if scenario.Setup == "" && len(runBlocksIn(scenario.SetupCards)) == 0 {
		return "", nil, nil
	}
	if scenario.Setup != "" {
		if problem, err := w.fromProblem(scenario); err != nil || problem != "" {
			return problem, nil, err
		}
	}
	required, err := w.RequiredCommands(scenario.Capability + "/" + scenario.Name)
	if err != nil {
		return "", nil, err
	}
	for _, command := range required {
		if !command.Installed {
			return "This scenario needs " + command.Command, []string{command.Hint}, nil
		}
	}
	return "", nil, nil
}

// checklistOf is what the developer does once setup is done: first_you lines from the setup
// script, then the When and Then steps in the order they're written. And and But take the
// kind before them.
func checklistOf(scenario project.Scenario, row []runs.Pair) []CheckItem {
	assert.That(scenario.Name != "", "the scenario has a name")
	assert.That(len(scenario.Steps) <= helpers.HelpersMax, "steps stay bounded")
	items := make([]CheckItem, 0, len(scenario.Steps)+2)
	if scenario.Setup != "" {
		if text, err := helpers.ReadScript(scenario.Setup); err == nil {
			for _, match := range firstYouLine.FindAllStringSubmatch(text, -1) {
				items = append(items, CheckItem{Text: "First, " + unquote(match[1]),
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
			items = append(items, CheckItem{Text: text, Kind: kind})
		}
	}
	return items
}

// givensOf are a scenario's Given steps, Background first, each And and But after a Given
// with them, placeholders filled in: what setup sets up, which the checklist leaves out.
func givensOf(scenario project.Scenario, row []runs.Pair) []string {
	assert.That(scenario.Name != "", "the scenario has a name")
	assert.That(len(scenario.Steps) <= helpers.HelpersMax, "steps stay bounded")
	givens := make([]string, 0, len(scenario.Background)+len(scenario.Steps))
	for _, steps := range [][]project.Step{scenario.Background, scenario.Steps} {
		kind := "given"
		for _, step := range steps {
			switch step.Keyword {
			case "Given", "When", "Then":
				kind = strings.ToLower(step.Keyword)
			}
			if kind == "given" {
				givens = append(givens, step.Keyword+" "+fillPlaceholders(step.Text, row))
			}
		}
	}
	return givens
}

// unquote takes the quotes off one bash word, as the kit would see it: 'x' or "x".
func unquote(word string) string {
	assert.That(!strings.Contains(word, "\n"), "a word is on one line")
	if len(word) >= 2 && word[0] == '\'' && word[len(word)-1] == '\'' {
		return word[1 : len(word)-1]
	}
	if len(word) >= 2 && word[0] == '"' && word[len(word)-1] == '"' {
		return strings.ReplaceAll(word[1:len(word)-1], `\"`, `"`)
	}
	return word
}

// fromProblem finds what's wrong with the setups a scenario builds on: a from line naming no
// scenario, an outline named without a row, or setups that build on each other in a loop.
func (w *Window) fromProblem(scenario project.Scenario) (string, error) {
	assert.That(w.project != nil, "from lines are read in a loaded project")
	assert.That(scenario.Setup != "", "the scenario has a setup script")
	type visit struct {
		scenario project.Scenario
		path     []string // slugs from the scenario dropped in to
	}
	stack := []visit{{scenario: scenario, path: []string{scenario.Slug}}}
	visits := 0
	// A loop is reported the first time a slug repeats on a path.
	// bound: visits stop at the most scenarios a project can hold.
	for len(stack) > 0 && visits <= spec.CapabilitiesMax*spec.ScenariosPerFeatureMax {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		visits++
		if current.scenario.Setup == "" {
			continue
		}
		text, err := helpers.ReadScript(current.scenario.Setup)
		if err != nil {
			return "", err
		}
		for _, from := range helpers.Froms(text) {
			built, err := w.project.Find(from.Key)
			if err != nil {
				return "from " + from.Key + ": there's no scenario with that name", nil
			}
			if problem := outlineWithoutRow(built, from.Values); problem != "" {
				return problem, nil
			}
			path := append(append([]string{}, current.path...), built.Slug)
			if slices.Contains(current.path, built.Slug) {
				return "These setups build on each other in a loop: " +
					strings.Join(path, " → "), nil
			}
			stack = append(stack, visit{scenario: built, path: path})
		}
	}
	return "", nil
}

// outlineWithoutRow says an outline was named without a row to build on, or "".
func outlineWithoutRow(built project.Scenario, values []string) string {
	assert.That(built.Slug != "", "the scenario has a slug")
	assert.That(len(values) <= helpers.HelpersMax, "values stay bounded")
	if !built.Outline || len(values) > 0 {
		return ""
	}
	example := ""
	for _, block := range built.Examples {
		if len(block.Header) > 0 && len(block.Rows) > 0 && example == "" {
			example = ", like " + block.Header[0] + "=" + block.Rows[0][0]
		}
	}
	return built.Slug + " is an outline: pick a row" + example
}
