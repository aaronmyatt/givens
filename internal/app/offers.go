package app

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"givens/internal/assert"
	"givens/internal/project"
	"givens/internal/slug"
)

// Offers is every action the window offers right now, by its label: "Drop in", "Copy",
// "Trust", "Create script". It's what "they're offered" checks.
func (w *Window) Offers() []string {
	assert.That(w != nil, "offers runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	if w.prompt != nil {
		return []string{w.prompt.confirm, OfferCancel}
	}
	if w.session != nil {
		return w.session.offers()
	}
	offers := make([]string, 0, 8)
	if w.search != nil && len(w.searchMessages()) > 0 &&
		strings.HasPrefix(w.searchMessages()[0], "Nothing matches") {
		return []string{OfferNewScenario}
	}
	if w.selectedHelper != "" {
		return []string{OfferEdit}
	}
	if scenario := w.shownScenario(); scenario != nil {
		offers = append(offers, scenarioOffers(*scenario)...)
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
	return offers
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
	if w.session != nil {
		return w.session.choose(action)
	}
	if action == OfferEdit && w.selectedHelper != "" {
		return w.editHelper()
	}
	if action == OfferDropIn && w.selectedScenario != "" {
		_, err := w.DropIn(w.selectedScenario)
		return err
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
	return []string{OfferCopy, OfferDropIn, OfferRecordRun}
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
