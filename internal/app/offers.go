package app

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"givens/internal/assert"
	"givens/internal/project"
	"givens/internal/runref"
	"givens/internal/slug"
)

// Offers is every action the window offers right now, by its label: "Drop in", "Copy",
// "Trust", "Create script". It's what "they're offered" checks.
func (w *Window) Offers() []string {
	assert.That(w != nil, "offers runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	return w.readOnly(w.allOffers())
}

// allOffers is every offer before a branch's window cuts them to the read-only ones.
func (w *Window) allOffers() []string {
	assert.That(w != nil, "offers runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	if w.prompt != nil {
		return []string{w.prompt.confirm, w.prompt.cancel}
	}
	if w.kind == WindowWelcome {
		return []string{OfferOpenFolder}
	}
	if w.project == nil {
		return []string{OfferCreateFolder}
	}
	if w.session != nil {
		// Runs waiting to be shared can be shared from a Drop in too, right after recording.
		if slices.Contains(w.runsOffers(), OfferShareRuns) {
			return append(w.session.offers(), OfferShareRuns)
		}
		return w.session.offers()
	}
	offers := make([]string, 0, 8)
	if w.editor != nil {
		offers = append(offers, w.editor.editorOffers()...)
	}
	if w.project != nil {
		if messages := w.kitMessages(); len(messages) > 0 &&
			strings.HasPrefix(messages[0], "Kit update available") {
			offers = append(offers, OfferUpdateKit)
		}
		offers = append(append(offers, OfferNewStory), w.runsOffers()...)
		if w.selectedCapability != "" {
			offers = append(offers, OfferNewScenario)
		}
	}
	if w.search != nil && len(w.searchMessages()) > 0 &&
		strings.HasPrefix(w.searchMessages()[0], "Nothing matches") {
		return []string{OfferNewScenario}
	}
	if w.selectedHelper != "" {
		return []string{OfferEdit}
	}
	if w.emptyView() != "" {
		return []string{OfferNewScenario}
	}
	if scenario := w.shownScenario(); scenario != nil {
		loaded, _ := w.findScenario(w.selectedScenario)
		offers = append(offers, scenarioPageOffers(*scenario, loaded)...)
	} else if w.project != nil {
		// A capability's warning in the sidebar carries its fix, so fixes are offered with
		// nothing selected; the selected capability's come first.
		for _, capability := range w.problemCapabilities() {
			for _, offer := range capabilityOffers(capability) {
				if !slices.Contains(offers, offer) {
					offers = append(offers, offer)
				}
			}
		}
	}
	return append(offers, w.aboutOffers()...)
}

// problemCapabilities lists capabilities with something to fix, the selected one first.
func (w *Window) problemCapabilities() []project.Capability {
	assert.That(w != nil, "problems runs on a window")
	assert.That(w.project != nil, "problems are in a loaded project")
	found := make([]project.Capability, 0, 4)
	for _, capability := range w.project.Capabilities {
		if capability.ParseError == "" && len(capability.Ignored) == 0 {
			continue
		}
		if capability.Slug == w.selectedCapability {
			found = append([]project.Capability{capability}, found...)
		} else {
			found = append(found, capability)
		}
	}
	return found
}

// capabilityOffers is what a capability's page offers: a way to fix what's wrong with it.
func capabilityOffers(capability project.Capability) []string {
	assert.That(capability.Slug != "", "a capability has a slug")
	assert.That(capability.FeaturePath != "", "a capability has a .feature file")
	if capability.ParseError != "" {
		return []string{editPrefix + filepath.Base(capability.FeaturePath)}
	}
	if len(capability.Ignored) > 0 {
		return []string{OfferMoveToOwnCapability}
	}
	return []string{}
}

// editPrefix starts an offer to open a file in the editor: "Edit checkout.feature".
const editPrefix = "Edit "

// Choose takes an offered action by its label. An action that isn't offered is an error,
// as a disabled button would be.
func (w *Window) Choose(action string) error {
	assert.That(w != nil, "choose runs on a window")
	assert.That(action != "", "an action has a label")
	if !slices.Contains(w.Offers(), action) {
		return fmt.Errorf("%q isn't offered; the window offers %q", action, w.Offers())
	}
	if answered, err := w.answer(action); answered {
		return err
	}
	switch action {
	case OfferUpdateKit:
		return w.updateKit()
	case OfferOpenFolder:
		return w.openFolder()
	case OfferCreateFolder:
		return w.createCapabilities()
	}
	if file, ok := strings.CutPrefix(action, editPrefix); ok {
		for _, capability := range w.problemCapabilities() {
			if filepath.Base(capability.FeaturePath) == file {
				_, err := w.Edit(filepath.Join(capability.Slug, file))
				return err
			}
		}
	}
	if action == OfferMoveToOwnCapability {
		return w.moveToOwnCapability()
	}
	if action == OfferEditAbout || action == OfferAddAbout {
		return w.editAbout()
	}
	switch action {
	case OfferMoveRunsOffMain:
		return w.askMoveRunsOffMain()
	case OfferShareRuns:
		return w.syncRuns(runref.NetworkTimeout)
	}
	if w.session != nil {
		return w.session.choose(action)
	}
	if w.editor != nil && slices.Contains(w.editor.editorOffers(), action) {
		return w.editor.chooseInEditor(action)
	}
	if action == OfferNewStory || action == OfferNewScenario {
		return nil // the dialog opens; NewStory and NewScenario take what's filled in
	}
	if action == OfferEdit && w.selectedHelper != "" {
		return w.editHelper()
	}
	if w.selectedScenario != "" {
		return w.chooseOnScenarioPage(action)
	}
	return ErrNotImplemented
}

// Scenario and capability page actions.
const (
	OfferNewScenario         = "New scenario"
	OfferMoveToOwnCapability = "Move to its own capability"
	OfferCopy                = "Copy"
	OfferDropIn              = "Drop in"
	OfferRecordRun           = "Record run"
)

// scenarioOffers is what a scenario's page offers. A scenario whose slug clashes can't run or
// be recorded: both would land on the other's files.
func scenarioOffers(scenario Scenario) []string {
	assert.That(scenario.Name != "", "a shown scenario has a name")
	assert.That(scenario.Slug != "" || scenario.Warning != "", "a nameless slug is warned")
	if strings.HasPrefix(scenario.Warning, "Both names make the slug") {
		return []string{OfferCopy}
	}
	if scenario.Warning == MessageNameExamples {
		return []string{OfferCopy, OfferDropIn}
	}
	return []string{OfferCopy, OfferDropIn}
}

// moveToOwnCapability moves an extra .feature file, the selected capability's first, into a
// folder of its own, named after its slug, so it's read as a capability.
func (w *Window) moveToOwnCapability() error {
	assert.That(w != nil, "move runs on a window")
	assert.That(w.project != nil, "a move happens in a loaded project")
	source := ""
	for _, capability := range w.problemCapabilities() {
		if source == "" && len(capability.Ignored) > 0 {
			source = capability.Ignored[0]
		}
	}
	if source == "" {
		return fmt.Errorf("no capability has an extra .feature file: %w", ErrNotFound)
	}
	name := slug.Of(strings.TrimSuffix(filepath.Base(source), ".feature"))
	if name == "" {
		return fmt.Errorf("%s makes no folder name", filepath.Base(source))
	}
	dir := filepath.Join(w.path, name)
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("there's already a %s folder", name)
	}
	if err := os.Mkdir(dir, 0o755); err != nil {
		return err
	}
	if err := os.Rename(source, filepath.Join(dir, name+".feature")); err != nil {
		return err
	}
	return w.reload()
}

// KitSourceLine is the first line of every setup script: it sources the kit.
const KitSourceLine = `source "$(dirname "${BASH_SOURCE[0]}")/../../_shared/givens.sh"`

// addTeardown creates the selected scenario's scripts/<slug>.teardown.sh, sourcing the kit,
// and opens it in the editor. Givens runs it before every setup run.
func (w *Window) addTeardown() error {
	assert.That(w != nil, "add teardown runs on a window")
	assert.That(w.selectedScenario != "", "a scenario is selected")
	scenario, ok := w.findScenario(w.selectedScenario)
	if !ok || scenario.Setup == "" {
		return fmt.Errorf("%s has no setup to tear down: %w", w.selectedScenario, ErrNotFound)
	}
	path := strings.TrimSuffix(scenario.Setup, ".sh") + ".teardown.sh"
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists", filepath.Base(path))
	}
	text := KitSourceLine + "\n# Clears what " + scenario.Name +
		"'s setup leaves behind. It runs before every setup run,\n" +
		"# so it must cope when there's nothing to clear.\n"
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return err
	}
	if err := w.reload(); err != nil {
		return err
	}
	return w.editFile(path)
}

// Offers on a scenario with no setup yet, and what the page says in its place.
const (
	OfferCreateScript    = "Create script"
	OfferWriteSetupSteps = "Write setup steps"
	MessageNoSetup       = "No setup yet"
)

// noSetup is true for a scenario with no setup files at all.
func noSetup(scenario project.Scenario) bool {
	assert.That(scenario.Capability != "" || scenario.Name == "", "a scenario has a home")
	assert.That(scenario.Setup == "" || filepath.IsAbs(scenario.Setup), "paths are absolute")
	return scenario.Name != "" && scenario.Setup == "" && scenario.SetupCards == "" &&
		scenario.Teardown == ""
}

// createSetup starts the selected scenario's setup: a script sourcing the kit, or setup steps
// done by hand as an empty numbered list. It opens the new file in the editor.
func (w *Window) createSetup(script bool) error {
	assert.That(w != nil, "create setup runs on a window")
	assert.That(w.selectedScenario != "", "a scenario is selected")
	scenario, ok := w.findScenario(w.selectedScenario)
	if !ok || scenario.Slug == "" {
		return fmt.Errorf("%s: %w", w.selectedScenario, ErrNotFound)
	}
	capability, _ := w.findCapability(scenario.Capability)
	path := filepath.Join(capability.Dir, "scripts", scenario.Slug+".md")
	text := "1.\n"
	if script {
		path = filepath.Join(capability.Dir, "scripts", scenario.Slug+".sh")
		text = KitSourceLine + "\n"
	}
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists", filepath.Base(path))
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return err
	}
	if err := w.reload(); err != nil {
		return err
	}
	return w.editFile(path)
}

// scenarioFileOf says which of a scenario's setup files an edit offer opens, or nil for any
// other offer.
func scenarioFileOf(action string) func(project.Scenario) string {
	assert.That(action != "", "an action has a label")
	assert.That(OfferEditScript != OfferEditSetup, "each file has its own offer")
	switch action {
	case OfferEditScript:
		return func(scenario project.Scenario) string { return scenario.Setup }
	case OfferEditSetup:
		return func(scenario project.Scenario) string { return scenario.SetupCards }
	case OfferEditTeardown:
		return func(scenario project.Scenario) string { return scenario.Teardown }
	}
	return nil
}

// scenarioPageOffers is what a scenario's page offers: its own actions, then ways to write
// a missing setup or to open each setup file there is.
func scenarioPageOffers(shown Scenario, loaded project.Scenario) []string {
	assert.That(shown.Name != "", "a shown scenario has a name")
	assert.That(loaded.Name == "" || loaded.Name == shown.Name, "both are the same scenario")
	offers := append(scenarioOffers(shown), OfferCopySteps)
	if noSetup(loaded) {
		// With no setup there's nothing to copy; the setup can be written instead.
		offers = slices.DeleteFunc(offers, func(offer string) bool { return offer == OfferCopy })
		offers = append(offers, OfferCreateScript, OfferWriteSetupSteps)
	}
	// Each setup file can be read and changed from the scenario's page.
	for _, file := range []struct{ path, offer string }{
		{loaded.Setup, OfferEditScript}, {loaded.SetupCards, OfferEditSetup},
		{loaded.Teardown, OfferEditTeardown},
	} {
		if file.path != "" {
			offers = append(offers, file.offer)
		}
	}
	if loaded.Setup != "" && loaded.Teardown == "" {
		offers = append(offers, OfferAddTeardown)
	}
	if loaded.Name != "" {
		offers = append(offers, todoOffers(loaded)...)
	}
	return offers
}

// chooseOnScenarioPage takes an action offered on the selected scenario's page.
func (w *Window) chooseOnScenarioPage(action string) error {
	assert.That(w.selectedScenario != "", "a scenario is selected")
	assert.That(w.session == nil, "the scenario's page has no Drop in open")
	scenario, ok := w.findScenario(w.selectedScenario)
	if !ok {
		return fmt.Errorf("%s: %w", w.selectedScenario, ErrNotFound)
	}
	if editing := scenarioFileOf(action); editing != nil {
		return w.editFile(editing(scenario))
	}
	switch action {
	case OfferCopy:
		return w.copySetup(scenario)
	case OfferCopySteps:
		shown := w.shownScenario()
		if shown == nil {
			return fmt.Errorf("%s: %w", w.selectedScenario, ErrNotFound)
		}
		return w.app.config.Clipboard.WriteText(gherkinOf(*shown))
	case OfferAddTeardown:
		return w.addTeardown()
	case OfferCreateScript, OfferWriteSetupSteps:
		return w.createSetup(action == OfferCreateScript)
	case OfferFlagTodo, OfferRemoveTodo:
		return w.flagTodo(scenario, action == OfferFlagTodo)
	case OfferDropIn:
		_, err := w.DropIn(w.selectedScenario)
		return err
	}
	return ErrNotImplemented
}
