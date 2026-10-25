package writestoriesandscenarios

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	messages "github.com/cucumber/messages/go/v34"

	"givens/acceptance/world"
	"givens/internal/app"
	"givens/internal/app/model"
	"givens/internal/spec"
)

// notAStep is Givens's wording of the Gherkin parser's error for a line in a scenario that
// isn't a step, tag, comment or new scenario.
const notAStep = "Expected a step, a tag, a comment or a new scenario"

// markdownImage matches a markdown image and captures its path.
// Ref: https://spec.commonmark.org/0.31.2/#images
const markdownImage = `!\[[^\]]*\]\(([^)\s]+)\)`

// defaultAbout and flowDiagram stand in for Checkout's about.md and its diagram when the
// sample shop has none.
const (
	defaultAbout = "# How checkout works\n\nThe shopper pays with the card saved on " +
		"their account.\n\n![Checkout flow](images/flow.svg)\n"
	flowDiagram = `<svg xmlns="http://www.w3.org/2000/svg" width="120" height="40">` +
		`<text x="4" y="24">cart → pay → placed</text></svg>` + "\n"
)

func (s *state) editorSteps() []world.Step {
	return []world.Step{
		world.Def(`^a developer is writing a new scenario in Checkout$`, s.writingNewScenario),
		world.Def(`^they type "([^"]*)"$`, s.typeText),
		world.Def(`^(?:the steps suggested include )?"([^"]*)" with "([^"]*)"$`, s.offeredWith),
		world.Def(`^choosing one puts it in word for word$`, s.choosingInserts),
		world.Def(`^a developer has the sample shop project in git and open in Givens$`,
			s.openInGit),
		world.Def(`^they change "([^"]*)" to "([^"]*)" in "([^"]*)"$`, s.changeIn),
		world.Def(`^they save$`, s.save),
		world.Def(`^git diff shows that one line changed and nothing else$`, s.diffOneLine),
		world.Def(`^a developer has unsaved changes to "([^"]*)"$`, s.editing),
		world.Def(`^a developer is editing "([^"]*)"$`, s.editing),
		world.Def(`^another tool saves a different change to checkout\.feature$`,
			s.anotherToolSaves),
		world.Def(`^they can choose "([^"]*)", "([^"]*)" or "([^"]*)"$`, s.canChooseThree),
		world.Def(`^they add the line "([^"]*)" among its steps$`, s.addLine),
		world.Def(`^one line among its steps doesn't start with Given, When, Then, And, But `+
			`or \*$`, s.addNonStep),
		world.Def(`^that line is marked "([^"]*)"$`, s.lineMarked),
		world.Def(`^(?:the preview shows the scenario with its keywords lined up|the steps `+
			`around it are shown as normal)$`, s.previewLinedUp),
		world.Def(`^their unsaved changes are included$`, s.previewHasChanges),
		world.Def(`^a developer is editing Checkout's about\.md$`, s.editingAbout),
		world.Def(`^the checkout flow diagram is shown as Checkout's story shows it$`,
			s.previewAsStory),
		world.Def(`^that line is shown marked with the parser's message$`, s.previewMarked),
	}
}

// writingNewScenario makes sure "a signed-in shopper with one item in the cart" is used in
// 4 scenarios, then adds a new scenario to Checkout, which leaves it open to edit.
func (s *state) writingNewScenario() error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	if err := s.ensureStepUses("a signed-in shopper with one item in the cart", 4); err != nil {
		return err
	}
	window, err := s.window()
	if err != nil {
		return err
	}
	if err := window.Select("Checkout"); err != nil {
		return err
	}
	return window.NewScenario(model.ScenarioDraft{Name: "Pay with store credit"})
}

// ensureStepUses adds Checkout scenarios using step until want scenarios use it.
func (s *state) ensureStepUses(step string, want int) error {
	features, err := filepath.Glob(filepath.Join(s.Capabilities(), "*", "*.feature"))
	if err != nil {
		return err
	}
	uses := 0
	for _, feature := range features {
		scenarios, err := spec.ScenariosIn(feature)
		if err != nil {
			return err
		}
		for _, scenario := range scenarios {
			if slices.ContainsFunc(scenario.Steps, func(each *messages.Step) bool {
				return each.Text == step
			}) {
				uses++
			}
		}
	}
	if uses > want {
		return fmt.Errorf("the sample shop uses %q in %d scenarios, more than %d", step, uses, want)
	}
	text, err := s.ReadFile(featurePath("checkout"))
	if err != nil {
		return err
	}
	for added := 1; uses < want; added++ { // bound: uses grows by one each time, to want
		text = strings.TrimRight(text, "\n") + fmt.Sprintf("\n\n  Scenario: Reorder %d\n"+
			"    Given %s\n    When they place the order\n    Then they see \"Order placed\"\n",
			added, step)
		uses++
	}
	return s.WriteFile(featurePath("checkout"), text)
}

func (s *state) typeText(text string) error {
	window, err := s.window()
	if err != nil {
		return err
	}
	s.typed = text
	return window.Type(text)
}

func (s *state) currentEditor() (*app.Editor, error) {
	if s.editor != nil {
		return s.editor, nil
	}
	window, err := s.window()
	if err != nil {
		return nil, err
	}
	if editor := window.Editor(); editor != nil {
		return editor, nil
	}
	return nil, errors.New("nothing is open to edit")
}

func (s *state) offeredWith(text string, label string) error {
	editor, err := s.currentEditor()
	if err != nil {
		return err
	}
	suggestions := editor.Suggestions()
	want := model.StepSuggestion{Text: text, Label: label}
	if !slices.Contains(suggestions, want) {
		return fmt.Errorf("Givens doesn't suggest %q with %q; it suggests %v", text, label,
			suggestions)
	}
	if s.offered == "" {
		s.offered = text
	}
	return nil
}

// choosingInserts chooses the first suggestion checked and expects the typed line replaced
// by the whole step, keyword kept.
func (s *state) choosingInserts() error {
	editor, err := s.currentEditor()
	if err != nil {
		return err
	}
	if s.offered == "" {
		return errors.New("no step offered a suggestion to choose")
	}
	if err := editor.ChooseSuggestion(s.offered); err != nil {
		return err
	}
	keyword, _, _ := strings.Cut(s.typed, " ")
	lines := trimmedLines(editor.Text())
	if !slices.Contains(lines, keyword+" "+s.offered) {
		return fmt.Errorf("the editor has no line %q: %q", keyword+" "+s.offered, lines)
	}
	if slices.Contains(lines, s.typed) {
		return fmt.Errorf("the editor still has the typed line %q", s.typed)
	}
	return nil
}

func trimmedLines(text string) []string {
	lines := strings.Split(text, "\n")
	for index, line := range lines {
		lines[index] = strings.TrimSpace(line)
	}
	return lines
}

// git runs a git command in the sample shop. Ref: https://git-scm.com/docs/git
func (s *state) git(args ...string) (string, error) {
	command := exec.Command("git", args...)
	command.Dir = s.Project
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return string(output), nil
}

// openInGit commits the sample shop to a new repository, then opens it.
func (s *state) openInGit() error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	commands := [][]string{
		{"init", "--quiet"},
		{"add", "--all"},
		{"-c", "user.name=Givens", "-c", "user.email=givens@example.com",
			"-c", "commit.gpgsign=false", "commit", "--quiet", "-m", "Sample shop"},
	}
	for _, args := range commands {
		if _, err := s.git(args...); err != nil {
			return err
		}
	}
	_, err := s.window()
	return err
}

// changeIn opens a scenario in the editor and changes one piece of its text, unsaved.
func (s *state) changeIn(from string, to string, name string) error {
	if err := s.open(name); err != nil {
		return err
	}
	text := s.editor.Text()
	if strings.Count(text, from) != 1 {
		return fmt.Errorf("the editor holds %q %d times, not once: %q", from,
			strings.Count(text, from), text)
	}
	s.changedFrom, s.changedTo = from, to
	return s.setText(strings.Replace(text, from, to, 1))
}

// open opens a scenario in the editor, remembering checkout.feature's text on disk.
func (s *state) open(name string) error {
	s.subject = name
	window, err := s.window()
	if err != nil {
		return err
	}
	capabilitySlug, err := s.ScenarioCapability(name)
	if err != nil {
		return err
	}
	if s.featureBefore, err = s.ReadFile(featurePath(capabilitySlug)); err != nil {
		return err
	}
	s.editor, err = window.Edit(name)
	return err
}

func (s *state) setText(text string) error {
	s.EditedPath, s.EditedText = s.editor.Path(), text
	return s.editor.SetText(text)
}

func (s *state) save() error {
	if s.editor == nil {
		return errors.New("nothing is open to edit")
	}
	return s.editor.Save()
}

// diffOneLine expects git to see one line of checkout.feature replaced, and no other change.
func (s *state) diffOneLine() error {
	feature := filepath.ToSlash(featurePath("checkout"))
	status, err := s.git("status", "--porcelain")
	if err != nil {
		return err
	}
	if strings.TrimRight(status, "\n") != " M "+feature {
		return fmt.Errorf("git status shows %q, not only %s modified", status, feature)
	}
	diff, err := s.git("diff", "--unified=0", "--no-color")
	if err != nil {
		return err
	}
	changed := make([]string, 0, 2)
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++") {
			continue
		}
		if strings.HasPrefix(line, "-") || strings.HasPrefix(line, "+") {
			changed = append(changed, line)
		}
	}
	oldLine := lineContaining(s.featureBefore, s.changedFrom)
	want := []string{"-" + oldLine, "+" + strings.Replace(oldLine, s.changedFrom, s.changedTo, 1)}
	if !slices.Equal(changed, want) {
		return fmt.Errorf("git diff changes %q, not %q:\n%s", changed, want, diff)
	}
	return nil
}

func lineContaining(text string, part string) string {
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, part) {
			return line
		}
	}
	return ""
}
