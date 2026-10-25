package world

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"givens/internal/app"
	"givens/internal/assert"
	"givens/internal/slug"
)

// KitSource is the line every setup script starts with.
const KitSource = `source "$(dirname "${BASH_SOURCE[0]}")/../../_shared/givens.sh"`

// PayInAppSetup is the markdown setup the spec uses for "Pay in app".
const PayInAppSetup = `You'll need the TestFlight build of the shop app.

1. Install it on your phone: [Shop beta](https://testflight.apple.com/join/abc)
2. Sign in as ` + "`sam@example.com`" + `
3. Add the test card ` + "`4242 4242 4242 4242`" + ` under Wallet

   ![Wallet screen](../images/wallet.png)
`

// Def makes a Step. Capability suites use it so their definitions read as a table.
func Def(pattern string, fn any) Step {
	assert.That(strings.HasPrefix(pattern, "^"), "a pattern is anchored at the start")
	assert.That(strings.HasSuffix(pattern, "$"), "a pattern is anchored at the end")
	return Step{Pattern: pattern, Func: fn}
}

// Common is every step used by more than one capability. A capability suite must not define
// a pattern that also matches one of these; CheckStepsDefined reports it if it does.
func Common(w *World) []Step {
	assert.That(w != nil, "steps bind to a world")
	steps := []Step{
		Def(`^a developer has the sample shop project$`, w.CopySampleShop),
		Def(`^a developer has the sample shop project open in Givens$`, w.OpenSampleShop),
		Def(`^a developer opens the sample shop in Givens$`, w.OpenSampleShop),
		Def(`^they see "([^"]*)"$`, w.see),
		Def(`^they're offered "([^"]*)"(?: and "([^"]*)")?$`, w.offered),
		Def(`^they can choose "([^"]*)" or "([^"]*)"$`, w.offered),
		Def(`^(?:they choose|a developer chooses) "([^"]*)"$`, w.choose),
		Def(`^(?:they select|a developer selects) "([^"]*)"$`, w.selectItem),
		Def(`^"([^"]*)" is selected$`, w.selectItem),
		Def(`^(?:they drop|a developer drops) in to "([^"]*)"$`, w.DropIn),
		Def(`^(?:they run|a developer runs) "([^"]*)"$`, w.Run),
		Def(`^it prints "([^"]*)"$`, w.prints),
		Def(`^"([^"]*)" shows "([^"]*)"$`, w.shows),
		Def(`^"([^"]*)" has a passing run recorded$`, w.RecordPassingRun),
		Def(`^the clipboard holds "([^"]*)"$`, w.clipboardHolds),
		Def(`^the file is saved anyway$`, w.savedAnyway),
		Def(`^it opens in the editor$`, w.opensInEditor),
		Def(`^the "([^"]*)" setup starts with "([^"]*)"$`, w.SetupStartsWith),
		Def(`^checkout\.feature has the line "([^"]*)" among the steps of "([^"]*)"$`,
			w.lineAmongSteps),
		Def(`^"([^"]*)" has only scripts/([a-z0-9-]+)\.md$`, w.onlyMarkdownSetup),
	}
	assert.That(len(steps) > 0, "there are common steps")
	return steps
}

func (w *World) see(text string) error {
	assert.That(w != nil, "see runs on a world")
	assert.That(text != "", "the step names text")
	window, err := w.Front()
	if err != nil {
		return err
	}
	return Expect(window.Visible(), text, "the window")
}

func (w *World) offered(first string, second string) error {
	assert.That(w != nil, "offered runs on a world")
	assert.That(first != "", "the step names an action")
	window, err := w.Front()
	if err != nil {
		return err
	}
	if err := Expect(window.Offers(), first, "the offered actions"); err != nil {
		return err
	}
	if second == "" {
		return nil
	}
	return Expect(window.Offers(), second, "the offered actions")
}

func (w *World) choose(action string) error {
	assert.That(w != nil, "choose runs on a world")
	assert.That(action != "", "the step names an action")
	window, err := w.Front()
	if err != nil {
		return err
	}
	return window.Choose(action)
}

func (w *World) selectItem(name string) error {
	assert.That(w != nil, "select runs on a world")
	assert.That(name != "", "the step names an item")
	window, err := w.Front()
	if err != nil {
		return err
	}
	return window.Select(name)
}

// DropIn drops in to a scenario in the front window and waits for setup to finish or pause.
func (w *World) DropIn(scenario string) error {
	assert.That(w != nil, "drop in runs on a world")
	assert.That(scenario != "", "the step names a scenario")
	window, err := w.Front()
	if err != nil {
		return err
	}
	session, err := window.DropIn(scenario)
	if err != nil {
		return err
	}
	w.Session = session
	session.Wait(SetupWait)
	return nil
}

// RecordPassingRun drops in to a scenario, marks every check, and records a passing run.
func (w *World) RecordPassingRun(scenario string) error {
	assert.That(w != nil, "record runs on a world")
	assert.That(scenario != "", "the step names a scenario")
	if w.Window == nil {
		if err := w.OpenSampleShop(); err != nil {
			return err
		}
	}
	if err := w.DropIn(scenario); err != nil {
		return err
	}
	if status := w.Session.Status(); status != app.SessionReady {
		return fmt.Errorf("drop in to %q ended %q, not ready", scenario, status)
	}
	for _, item := range w.Session.Checklist() {
		mark := "done"
		if item.Kind == "then" {
			mark = "pass"
		}
		if err := w.Session.Mark(item.Text, mark); err != nil {
			return err
		}
	}
	return w.Session.RecordRun("")
}

// Search opens search with ⌘K and types query.
func (w *World) Search(query string) error {
	assert.That(w != nil, "search runs on a world")
	assert.That(query != "", "the step names a query")
	window, err := w.Front()
	if err != nil {
		return err
	}
	if err := window.Press("⌘K"); err != nil {
		return err
	}
	return window.Type(query)
}

func (w *World) prints(text string) error {
	assert.That(w != nil, "prints runs on a world")
	assert.That(text != "", "the step names text")
	return Expect(strings.Split(w.Output, "\n"), text, "the command's output")
}

func (w *World) shows(name string, text string) error {
	assert.That(w != nil, "shows runs on a world")
	assert.That(name != "", "the step names a scenario")
	window, err := w.Front()
	if err != nil {
		return err
	}
	scenario, err := window.Scenario(name)
	if err != nil {
		return err
	}
	return Expect([]string{scenario.Status, scenario.Warning}, text, name+"'s status")
}

func (w *World) clipboardHolds(text string) error {
	assert.That(w != nil, "clipboard runs on a world")
	assert.That(w.Clipboard != nil, "the scenario has a clipboard")
	got, err := w.Clipboard.ReadText()
	if err != nil {
		return err
	}
	if got != text {
		return fmt.Errorf("the clipboard holds %q, not %q", got, text)
	}
	return nil
}

func (w *World) savedAnyway() error {
	assert.That(w != nil, "saved runs on a world")
	assert.That(w.EditedPath == "" || filepath.IsAbs(w.EditedPath), "an edited path is absolute")
	if w.EditedPath == "" {
		return errors.New("no step edited a file in this scenario")
	}
	saved, err := os.ReadFile(w.EditedPath)
	if err != nil {
		return err
	}
	if string(saved) != w.EditedText {
		return fmt.Errorf("%s wasn't saved with the edit", w.EditedPath)
	}
	return nil
}

func (w *World) opensInEditor() error {
	assert.That(w != nil, "editor runs on a world")
	assert.That(w.Dir != "", "the scenario has a folder")
	window, err := w.Front()
	if err != nil {
		return err
	}
	if window.Editing() == "" {
		return errors.New("nothing is open in the editor")
	}
	return nil
}

// ScenarioCapability finds which sample shop capability holds the named scenario.
func (w *World) ScenarioCapability(name string) (string, error) {
	assert.That(w != nil, "lookup runs on a world")
	assert.That(name != "", "a scenario has a name")
	features, err := filepath.Glob(filepath.Join(w.Capabilities(), "*", "*.feature"))
	if err != nil {
		return "", err
	}
	for _, feature := range features {
		scenarios, err := app.ScenariosIn(feature)
		if err != nil {
			return "", err
		}
		for _, scenario := range scenarios {
			if scenario.Name == name {
				return filepath.Base(filepath.Dir(feature)), nil
			}
		}
	}
	return "", fmt.Errorf("the sample shop has no scenario %q", name)
}

// ScriptPath is the setup script of the named scenario, relative to the sample shop.
func (w *World) ScriptPath(name string) (string, error) {
	assert.That(w != nil, "script path runs on a world")
	assert.That(name != "", "a scenario has a name")
	capability, err := w.ScenarioCapability(name)
	if err != nil {
		return "", err
	}
	return filepath.Join("capabilities", capability, "scripts", slug.Of(name)+".sh"), nil
}

// SetupStartsWith makes line the first line after the kit's source line in a setup script.
func (w *World) SetupStartsWith(name string, line string) error {
	assert.That(w != nil, "setup runs on a world")
	assert.That(!strings.Contains(line, "\n"), "the step gives one line")
	if err := w.CopySampleShop(); err != nil {
		return err
	}
	path, err := w.ScriptPath(name)
	if err != nil {
		return err
	}
	existing, err := w.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	body := strings.TrimPrefix(existing, KitSource+"\n")
	// A fixture script may already start with the line; don't run it twice.
	body = strings.TrimPrefix(body, line+"\n")
	return w.WriteFile(path, KitSource+"\n"+line+"\n"+body)
}

func (w *World) lineAmongSteps(line string, name string) error {
	assert.That(w != nil, "line runs on a world")
	assert.That(name != "", "the step names a scenario")
	if err := w.CopySampleShop(); err != nil {
		return err
	}
	path := "capabilities/checkout/checkout.feature"
	text, err := w.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(text, "\n")
	start := indexOf(lines, "  Scenario: "+name)
	if start < 0 {
		return fmt.Errorf("checkout.feature has no scenario %q", name)
	}
	end := start + 1
	for end < len(lines) && strings.HasPrefix(lines[end], "    ") { // bound: len(lines)
		end++
	}
	edited := append(append(append([]string{}, lines[:end]...), "    "+line), lines[end:]...)
	return w.WriteFile(path, strings.Join(edited, "\n"))
}

func (w *World) onlyMarkdownSetup(name string, scriptSlug string) error {
	assert.That(w != nil, "markdown setup runs on a world")
	assert.That(slug.Of(name) == scriptSlug, "the script is named after the scenario")
	if err := w.CopySampleShop(); err != nil {
		return err
	}
	if err := w.EnsureScenario("checkout", name); err != nil {
		return err
	}
	scripts := filepath.Join("capabilities", "checkout", "scripts")
	removeErr := os.Remove(filepath.Join(w.Project, scripts, scriptSlug+".sh"))
	if removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
		return removeErr
	}
	return w.WriteFile(filepath.Join(scripts, scriptSlug+".md"), PayInAppSetup)
}

// EnsureScenario adds a three-step scenario named name to a sample shop capability, unless it
// already has one.
func (w *World) EnsureScenario(capability string, name string) error {
	assert.That(w != nil, "ensure runs on a world")
	assert.That(capability != "" && !strings.Contains(capability, "/"), "capability is a slug")
	path := filepath.Join("capabilities", capability, capability+".feature")
	text, err := w.ReadFile(path)
	if err != nil {
		return err
	}
	if strings.Contains(text, "  Scenario: "+name+"\n") {
		return nil
	}
	scenario := "\n  Scenario: " + name + "\n" +
		"    Given a shopper has a saved card on their phone\n" +
		"    When they tap \"Pay\" in the app\n" +
		"    Then they see \"Payment sent\"\n"
	return w.WriteFile(path, strings.TrimRight(text, "\n")+"\n"+scenario)
}

// indexOf returns the index of the first line equal to want, or -1.
func indexOf(lines []string, want string) int {
	assert.That(lines != nil, "there are lines")
	assert.That(want != "", "a line is wanted")
	for index, line := range lines {
		if line == want {
			return index
		}
	}
	return -1
}
