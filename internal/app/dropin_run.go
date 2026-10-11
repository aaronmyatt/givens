package app

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"givens/internal/assert"
	"givens/internal/helpers"
	"givens/internal/project"
)

// firstYouLine matches a `first_you '…'` line in a setup script: a Given done by hand, which
// the checklist shows first. Quiet mode hides the kit's own "First, …" line.
const firstYouPattern = `(?m)^\s*first_you\s+(.+?)\s*$`

var firstYouLine = regexp.MustCompile(firstYouPattern) //tiger:allow-global compiled once

// DropIn drops in to a scenario: it runs the setup and opens the Drop in page. Before anything
// runs it checks the commands the setup needs and the setups it builds on.
func (w *Window) DropIn(scenario string) (*Session, error) {
	assert.That(w != nil, "drop in runs on a window")
	assert.That(len(scenario) <= NameLengthMax, "the name fits NameLengthMax")
	found, ok := w.findScenario(scenario)
	if !ok {
		return nil, fmt.Errorf("%s: %w", scenario, ErrNotFound)
	}
	trusted, err := w.app.Trusted(w.path)
	if err != nil {
		return nil, err
	}
	session := &Session{
		window: w, scenario: found, status: SessionRunning,
		started: w.app.config.Clock.Now(), env: setupEnv(found, nil),
	}
	w.session = session
	if !trusted {
		w.ask("Run scripts from this project? "+filepath.Dir(w.path), "Trust", func() error {
			if err := w.app.MarkTrusted(w.path); err != nil {
				return err
			}
			return session.begin()
		})
		return session, nil
	}
	return session, session.begin()
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
	for _, script := range []string{s.scenario.Teardown, s.scenario.Setup} {
		if script != "" {
			s.scripts = append(s.scripts, script)
		}
	}
	if len(s.scripts) == 0 {
		s.status, s.checklist = SessionReady, checklistOf(s.scenario)
		return nil
	}
	s.start()
	return nil
}

// setupEnv is the variables a Drop in runs setup with. GIVENS_QUIET hides the "your turn"
// lines, which the checklist shows instead; GIVENS_UI makes by_hand and ask cards.
// See capabilities/the-kit/about.md.
func setupEnv(scenario project.Scenario, row map[string]string) []string {
	assert.That(scenario.Capability != "", "the scenario belongs to a capability")
	assert.That(len(row) <= helpers.HelpersMax, "a row stays bounded")
	env := []string{"GIVENS_QUIET=1", "GIVENS_UI=1", "GIVENS_SCENARIO=" + scenario.Key()}
	for column, value := range row {
		env = append(env, "GIVENS_EX_"+column+"="+value)
	}
	return env
}

// precheck finds what would stop setup before it runs: a from line naming no scenario, or a
// command the setup or its helpers need that isn't installed. It returns the message to show,
// and any install hints.
func (w *Window) precheck(scenario project.Scenario) (string, []string, error) {
	assert.That(w.project != nil, "a Drop in runs in a loaded project")
	assert.That(scenario.Capability != "", "the scenario belongs to a capability")
	if scenario.Setup == "" {
		return "", nil, nil
	}
	text, err := helpers.ReadScript(scenario.Setup)
	if err != nil {
		return "", nil, err
	}
	for _, from := range helpers.Froms(text) {
		if _, err := w.project.Find(from.Key); err != nil {
			return "from " + from.Key + ": there's no scenario with that name", nil, nil
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
// script, then the When steps, then the Then steps. And and But take the kind before them.
func checklistOf(scenario project.Scenario) []CheckItem {
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
			items = append(items, CheckItem{Text: step.Keyword + " " + step.Text, Kind: kind})
		}
	}
	return items
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
