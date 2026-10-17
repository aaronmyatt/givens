// Package writestoriesandscenarios runs capabilities/write-stories-and-scenarios. Steps shared
// with other capabilities are in acceptance/world.
package writestoriesandscenarios

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	messages "github.com/cucumber/messages/go/v34"

	"givens/acceptance/world"
	"givens/internal/app"
	"givens/internal/slug"
	"givens/internal/spec"
)

const capability = "write-stories-and-scenarios"

func TestScenarios(t *testing.T) { world.RunSuite(t, capability, steps) }

func TestStepsDefined(t *testing.T) { world.CheckStepsDefined(t, capability, steps) }

// state is what this capability's steps keep between steps of one scenario.
type state struct {
	*world.World
	subject string      // the scenario a step called "it"
	editor  *app.Editor // the editor a step opened
	story   app.StoryDraft

	featureBefore string            // a .feature file's text before the step acted on it
	before        map[string]string // every file in the capabilities folder, before a delete
	runsBefore    map[string]string // every file in runs/, before a rename

	changedFrom, changedTo string // a step line before and after an unsaved edit
	addedLine              string // a line a step added among the steps
	addedLineNumber        int    // its 1-based line number in the editor
	typed, offered         string // what was typed, and the first suggestion checked
	theirs                 string // checkout.feature as another tool saved it

	renamedFrom, renamedTo string
	actedOn                string   // the slug of the capability a rename or delete acts in
	scriptBefore           string   // the renamed scenario's setup script
	filesBefore            []string // a renamed capability's files, relative to its folder
	deleting               string   // what a step asked to delete
	deletingFolder         string   // its folder when it's a capability
	deletingScript         string   // its setup script when it's a scenario
	deletingFile           string   // the setup file open in the editor, for "Delete file"
}

func steps(w *world.World) []world.Step {
	s := &state{World: w}
	return slices.Concat(s.storySteps(), s.editorSteps(), s.renameSteps(), s.deleteSteps(),
		s.aboutSteps())
}

func (s *state) storySteps() []world.Step {
	return []world.Step{
		world.Def(`^they choose "New story" and fill in "([^"]*)", "([^"]*)", "([^"]*)" and `+
			`"([^"]*)"$`, s.newStory),
		world.Def(`^([a-z0-9-]+)/([a-z0-9-]+\.feature) is created, with "([^"]*)" and those `+
			`lines as its description$`, s.storyCreated),
		world.Def(`^"([^"]*)" appears in the sidebar$`, s.inSidebar),
		world.Def(`^they choose "New scenario" in ([A-Za-z ]+), name it "([^"]*)" and mark it `+
			`as an error state$`, s.newErrorScenario),
		world.Def(`^they choose "New scenario" in ([A-Za-z ]+) and name it "([^"]*)"$`,
			s.newHappyScenario),
		world.Def(`^a "Scenario: ([^"]*)" block tagged (@\S+) is added to checkout\.feature$`,
			s.blockAdded),
		world.Def(`^checkout/scripts/([a-z0-9-]+\.sh) is created, starting with the line that `+
			`sources givens\.sh$`, s.scriptCreated),
		world.Def(`^the new scenario opens in the editor$`, s.newScenarioInEditor),
		world.Def(`^"([^"]*)" has no setup yet$`, s.noSetupYet),
		world.Def(`^scripts/([a-z0-9-]+\.md) is created with an empty numbered list$`,
			s.markdownSetupCreated),
	}
}

func (s *state) window() (*app.Window, error) {
	if s.Window == nil && s.Project != "" {
		if err := s.OpenSampleShop(); err != nil {
			return nil, err
		}
	}
	return s.Front()
}

// featurePath is a capability's .feature file, relative to the sample shop.
func featurePath(capabilitySlug string) string {
	return filepath.Join("capabilities", capabilitySlug, capabilitySlug+".feature")
}

// scenarioNames lists a .feature file's scenarios with their steps, keyword included.
func (s *state) scenarioSteps(capabilitySlug string) (map[string][]string, []string, error) {
	scenarios, err := spec.ScenariosIn(filepath.Join(s.Project, featurePath(capabilitySlug)))
	if err != nil {
		return nil, nil, err
	}
	steps := make(map[string][]string, len(scenarios))
	names := make([]string, 0, len(scenarios))
	for _, scenario := range scenarios {
		names = append(names, scenario.Name)
		for _, step := range scenario.Steps {
			// The parser keeps the space after a keyword: "Given ". Ref:
			// https://pkg.go.dev/github.com/cucumber/messages/go/v34#Step
			steps[scenario.Name] = append(steps[scenario.Name], step.Keyword+step.Text)
		}
	}
	return steps, names, nil
}

// linedUp is how steps read in the library: keywords right-aligned to the widest one, so
// every step's text starts in the same column.
func linedUp(steps []string) []string {
	width := 0
	for _, step := range steps {
		width = max(width, len(strings.Fields(step)[0]))
	}
	lines := make([]string, 0, len(steps))
	for _, step := range steps {
		keyword, text, _ := strings.Cut(step, " ")
		lines = append(lines, strings.Repeat(" ", width-len(keyword))+keyword+" "+text)
	}
	return lines
}

// snapshot reads every file under root, relative to root, keeping those keep accepts.
// Ref: https://pkg.go.dev/path/filepath#WalkDir
func snapshot(root string, keep func(rel string) bool) (map[string]string, error) {
	files := make(map[string]string, 64)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil || !keep(rel) {
			return err
		}
		content, err := os.ReadFile(path)
		files[rel] = string(content)
		return err
	})
	return files, err
}

func everyFile(string) bool { return true }

// sameFiles describes how two snapshots differ, or returns nil when they don't.
func sameFiles(before map[string]string, after map[string]string) error {
	for rel, content := range before {
		now, found := after[rel]
		if !found {
			return fmt.Errorf("%s was removed", rel)
		}
		if now != content {
			return fmt.Errorf("%s changed", rel)
		}
	}
	for rel := range after {
		if _, found := before[rel]; !found {
			return fmt.Errorf("%s was added", rel)
		}
	}
	return nil
}

func (s *state) newStory(name string, asA string, iWant string, soThat string) error {
	window, err := s.window()
	if err != nil {
		return err
	}
	s.story = app.StoryDraft{Name: name, AsA: asA, IWant: iWant, SoThat: soThat}
	if err := window.Choose("New story"); err != nil {
		return err
	}
	return window.NewStory(s.story)
}

// storyCreated checks the new .feature file line by line, and that it follows GIVENS.md.
func (s *state) storyCreated(folder string, file string, featureLine string) error {
	if folder != slug.Of(s.story.Name) || file != folder+".feature" {
		return fmt.Errorf("%s/%s isn't named after %q", folder, file, s.story.Name)
	}
	path := filepath.Join("capabilities", folder, file)
	text, err := s.ReadFile(path)
	if err != nil {
		return err
	}
	lines := make([]string, 0, 4)
	for _, line := range strings.Split(text, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	want := []string{
		featureLine, "As a " + s.story.AsA, "I want " + s.story.IWant, "So that " + s.story.SoThat,
	}
	if !slices.Equal(lines, want) {
		return fmt.Errorf("%s holds %q, not %q", path, lines, want)
	}
	problems, err := spec.CheckFeature(filepath.Join(s.Project, path))
	if err != nil {
		return err
	}
	if len(problems) > 0 {
		return fmt.Errorf("%s breaks GIVENS.md: %v", path, problems)
	}
	return nil
}

func (s *state) inSidebar(name string) error {
	window, err := s.window()
	if err != nil {
		return err
	}
	return sidebarHas(window.Sidebar(), name)
}

func sidebarHas(items []app.SidebarItem, name string) error {
	for _, item := range items {
		if item.Name == name && item.Kind == "capability" {
			return nil
		}
	}
	return fmt.Errorf("the sidebar has no capability %q: %v", name, items)
}

func (s *state) newErrorScenario(capabilityName string, name string) error {
	return s.newScenario(capabilityName, app.ScenarioDraft{Name: name, Error: true})
}

func (s *state) newHappyScenario(capabilityName string, name string) error {
	return s.newScenario(capabilityName, app.ScenarioDraft{Name: name})
}

// newScenario fills in the New scenario dialog in a capability, keeping its .feature file's
// text from before.
func (s *state) newScenario(capabilityName string, draft app.ScenarioDraft) error {
	window, err := s.window()
	if err != nil {
		return err
	}
	s.subject = draft.Name
	s.featureBefore, err = s.ReadFile(featurePath(slug.Of(capabilityName)))
	if err != nil {
		return err
	}
	if err := window.Select(capabilityName); err != nil {
		return err
	}
	if err := window.Choose("New scenario"); err != nil {
		return err
	}
	return window.NewScenario(draft)
}

// blockAdded checks the new block, and that every other scenario is as it was.
func (s *state) blockAdded(name string, tag string) error {
	scenarios, err := spec.ScenariosIn(filepath.Join(s.Project, featurePath("checkout")))
	if err != nil {
		return err
	}
	index := slices.IndexFunc(scenarios, func(each *messages.Scenario) bool {
		return each.Name == name
	})
	if index < 0 {
		return fmt.Errorf("checkout.feature has no scenario %q", name)
	}
	tagged := slices.ContainsFunc(scenarios[index].Tags, func(each *messages.Tag) bool {
		return each.Name == tag
	})
	if !tagged {
		return fmt.Errorf("%q isn't tagged %s", name, tag)
	}
	text, err := s.ReadFile(featurePath("checkout"))
	if err != nil {
		return err
	}
	for _, line := range strings.Split(s.featureBefore, "\n") {
		if !strings.Contains(text, line+"\n") {
			return fmt.Errorf("checkout.feature lost the line %q", line)
		}
	}
	if len(scenarios) != strings.Count(s.featureBefore, "Scenario:")+1 {
		return fmt.Errorf("checkout.feature has %d scenarios; want one more than before",
			len(scenarios))
	}
	return nil
}

func (s *state) scriptCreated(file string) error {
	if file != slug.Of(s.subject)+".sh" {
		return fmt.Errorf("%s isn't named after %q", file, s.subject)
	}
	text, err := s.ReadFile(filepath.Join("capabilities", "checkout", "scripts", file))
	if err != nil {
		return err
	}
	first, _, _ := strings.Cut(text, "\n")
	if first != world.KitSource {
		return fmt.Errorf("%s starts with %q, not %q", file, first, world.KitSource)
	}
	return nil
}

// newScenarioInEditor checks that checkout.feature is open at the new scenario.
func (s *state) newScenarioInEditor() error {
	window, err := s.window()
	if err != nil {
		return err
	}
	editor := window.Editor()
	if editor == nil {
		return errors.New("nothing is open in the editor")
	}
	feature := filepath.Join(s.Project, featurePath("checkout"))
	if err := samePath(window.Editing(), feature); err != nil {
		return err
	}
	if !strings.Contains(editor.Text(), "Scenario: "+s.subject) {
		return fmt.Errorf("the editor doesn't hold %q: %q", s.subject, editor.Text())
	}
	return nil
}

// samePath compares two paths once symlinks are resolved, since macOS's temporary folder is
// reached through /var, a link to /private/var. Ref: https://pkg.go.dev/path/filepath#EvalSymlinks
func samePath(got string, want string) error {
	if got == "" {
		return fmt.Errorf("the editor isn't open; want %s", want)
	}
	gotReal, err := filepath.EvalSymlinks(got)
	if err != nil {
		return err
	}
	wantReal, err := filepath.EvalSymlinks(want)
	if err != nil {
		return err
	}
	if gotReal != wantReal {
		return fmt.Errorf("the editor holds %s, not %s", got, want)
	}
	return nil
}

// noSetupYet adds the scenario to Checkout if needed, removes its setup files, opens the shop
// and selects it, so "Write setup steps" has a scenario to act on.
func (s *state) noSetupYet(name string) error {
	s.subject = name
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	if err := s.EnsureScenario("checkout", name); err != nil {
		return err
	}
	scripts := filepath.Join(s.Capabilities(), "checkout", "scripts")
	for _, extension := range []string{".sh", ".md", ".teardown.sh"} {
		err := os.Remove(filepath.Join(scripts, slug.Of(name)+extension))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	window, err := s.window()
	if err != nil {
		return err
	}
	return window.Select(name)
}

func (s *state) markdownSetupCreated(file string) error {
	if file != slug.Of(s.subject)+".md" {
		return fmt.Errorf("%s isn't named after %q", file, s.subject)
	}
	text, err := s.ReadFile(filepath.Join("capabilities", "checkout", "scripts", file))
	if err != nil {
		return err
	}
	lines := make([]string, 0, 2)
	for _, line := range strings.Split(text, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	// An empty ordered list item is "1." alone. Ref: https://spec.commonmark.org/0.31.2/#lists
	if !slices.Equal(lines, []string{"1."}) {
		return fmt.Errorf("%s holds %q, not an empty numbered list", file, text)
	}
	return nil
}
