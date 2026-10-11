package writestoriesandscenarios

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"givens/acceptance/world"
	"givens/internal/slug"
	"givens/internal/spec"
)

// fromPayWithSavedCard is the line that makes 3-D Secure challenge's setup build on Pay with
// saved card's, which the rename and delete scenarios rely on.
const fromPayWithSavedCard = "from checkout/pay-with-saved-card"

func (s *state) renameSteps() []world.Step {
	return []world.Step{
		world.Def(`^they rename "([^"]*)" to "([^"]*)"$`, s.rename),
		world.Def(`^a developer renames "([^"]*)" to "([^"]*)" and confirms$`,
			s.renameAndConfirm),
		world.Def(`^a developer renames it to "([^"]*)"( and confirms)?$`, s.renameIt),
		world.Def(`^its Scenario: line is updated, and its setup script is renamed to `+
			`([a-z0-9-]+\.sh)$`, s.scenarioLineUpdated),
		world.Def(`^the from line in 3-D Secure challenge's setup is updated(?: to match)?$`,
			s.fromLineUpdated),
		world.Def(`^it shows "([^"]*)", since its runs were recorded under the old name$`,
			s.renamedShows),
		world.Def(`^the sidebar lists "([^"]*)" in place of "([^"]*)"$`, s.sidebarInPlaceOf),
		world.Def(`^its folder and \.feature file are renamed to ([a-z0-9-]+) and `+
			`([a-z0-9-]+\.feature), with its scripts and helpers inside$`, s.folderRenamed),
		world.Def(`^its scenarios show "([^"]*)"$`, s.scenariosShow),
		world.Def(`^that line becomes "([^"]*)"$`, s.lineBecomes),
		world.Def(`^"([^"]*)" and "([^"]*)" show "([^"]*)"$`, s.bothShow),
		world.Def(`^"([^"]*)" has (\d+) runs recorded$`, s.runsRecorded),
		world.Def(`^"([^"]*)" has a passing run recorded today$`, s.passingRunToday),
		world.Def(`^it's listed in the Unverified view$`, s.listedAsUnverified),
		world.Def(`^nothing in runs/ changes$`, s.runsUnchanged),
	}
}

// arrangeSetups gives every sample shop scenario without one a setup script, and makes
// 3-D Secure challenge's setup build on Pay with saved card's, as the spec assumes.
func (s *state) arrangeSetups() error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	features, err := filepath.Glob(filepath.Join(s.Capabilities(), "*", "*.feature"))
	if err != nil {
		return err
	}
	for _, feature := range features {
		scenarios, err := spec.ScenariosIn(feature)
		if err != nil {
			return err
		}
		for _, scenario := range scenarios {
			if err := s.ensureScript(scenario.Name); err != nil {
				return err
			}
		}
	}
	script, err := s.ScriptPath("3-D Secure challenge")
	if err != nil {
		return err
	}
	text, err := s.ReadFile(script)
	if err != nil || strings.Contains(text, fromPayWithSavedCard+"\n") {
		return err
	}
	return s.SetupStartsWith("3-D Secure challenge", fromPayWithSavedCard)
}

func (s *state) ensureScript(name string) error {
	script, err := s.ScriptPath(name)
	if err != nil {
		return err
	}
	for _, path := range []string{script, strings.TrimSuffix(script, ".sh") + ".md"} {
		_, err := os.Stat(filepath.Join(s.Project, path))
		if err == nil {
			return nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return s.WriteFile(script, world.KitSource+"\nok \"set up "+name+"\"\n")
}

// ensureHelper gives a capability a helper file of its own when it has none.
func (s *state) ensureHelper(capabilitySlug string) error {
	shared := filepath.Join("capabilities", capabilitySlug, "_shared")
	found, err := filepath.Glob(filepath.Join(s.Project, shared, "*.sh"))
	if err != nil || len(found) > 0 {
		return err
	}
	function := "seed_" + strings.ReplaceAll(capabilitySlug, "-", "_")
	helper := "# shellcheck shell=bash\n\n# @helper " + function + "\n" +
		"# Seeds what " + capabilitySlug + "'s scenarios start from.\n" +
		function + "() {\n  ok \"seeded " + capabilitySlug + "\"\n}\n"
	return s.WriteFile(filepath.Join(shared, capabilitySlug+".sh"), helper)
}

// capabilityNamed returns the slug of the capability called name, or "" for a scenario.
func (s *state) capabilityNamed(name string) (string, error) {
	capabilitySlug := slug.Of(name)
	_, err := os.Stat(filepath.Join(s.Project, featurePath(capabilitySlug)))
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	return capabilitySlug, err
}

func isRunsFile(rel string) bool {
	return slices.Contains(strings.Split(filepath.ToSlash(rel), "/"), "runs")
}

// rename renames a scenario or capability, remembering what it had before.
func (s *state) rename(name string, newName string) error {
	if err := s.arrangeSetups(); err != nil {
		return err
	}
	window, err := s.window()
	if err != nil {
		return err
	}
	s.renamedFrom, s.renamedTo, s.subject = name, newName, newName
	if s.runsBefore, err = snapshot(s.Project, isRunsFile); err != nil {
		return err
	}
	capabilitySlug, err := s.capabilityNamed(name)
	if err != nil {
		return err
	}
	if capabilitySlug != "" {
		err = s.rememberCapability(capabilitySlug)
	} else {
		err = s.rememberScenario(name)
	}
	if err != nil {
		return err
	}
	return window.Rename(name, newName)
}

func (s *state) rememberCapability(capabilitySlug string) error {
	if err := s.ensureHelper(capabilitySlug); err != nil {
		return err
	}
	files, err := snapshot(filepath.Join(s.Capabilities(), capabilitySlug), everyFile)
	if err != nil {
		return err
	}
	s.actedOn, s.filesBefore = capabilitySlug, make([]string, 0, len(files))
	for rel := range files {
		s.filesBefore = append(s.filesBefore, filepath.ToSlash(rel))
	}
	return nil
}

func (s *state) rememberScenario(name string) error {
	capabilitySlug, err := s.ScenarioCapability(name)
	if err != nil {
		return err
	}
	script, err := s.ScriptPath(name)
	if err != nil {
		return err
	}
	s.actedOn = capabilitySlug
	s.scriptBefore, err = s.ReadFile(script)
	return err
}

// confirmRename chooses "Rename" when Givens asks first; a rename that loses no runs doesn't.
func (s *state) confirmRename() error {
	window, err := s.window()
	if err != nil {
		return err
	}
	if !slices.Contains(window.Offers(), "Rename") {
		return nil
	}
	return window.Choose("Rename")
}

func (s *state) renameAndConfirm(name string, newName string) error {
	if err := s.rename(name, newName); err != nil {
		return err
	}
	return s.confirmRename()
}

func (s *state) renameIt(newName string, confirms string) error {
	if s.subject == "" {
		return errors.New("no step named the scenario to rename")
	}
	if err := s.rename(s.subject, newName); err != nil {
		return err
	}
	if confirms == "" {
		return nil
	}
	return s.confirmRename()
}

func (s *state) scenarioLineUpdated(file string) error {
	if file != slug.Of(s.renamedTo)+".sh" {
		return fmt.Errorf("%s isn't named after %q", file, s.renamedTo)
	}
	text, err := s.ReadFile(featurePath(s.actedOn))
	if err != nil {
		return err
	}
	lines := trimmedLines(text)
	if !slices.Contains(lines, "Scenario: "+s.renamedTo) {
		return fmt.Errorf("%s has no line %q", s.actedOn, "Scenario: "+s.renamedTo)
	}
	if slices.Contains(lines, "Scenario: "+s.renamedFrom) {
		return fmt.Errorf("%s still has %q", s.actedOn, "Scenario: "+s.renamedFrom)
	}
	scripts := filepath.Join("capabilities", s.actedOn, "scripts")
	renamed, err := s.ReadFile(filepath.Join(scripts, file))
	if err != nil {
		return err
	}
	if renamed != s.scriptBefore {
		return fmt.Errorf("%s holds %q, not the old script %q", file, renamed, s.scriptBefore)
	}
	_, err = os.Stat(filepath.Join(s.Project, scripts, slug.Of(s.renamedFrom)+".sh"))
	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%s.sh is still there: %v", slug.Of(s.renamedFrom), err)
	}
	return nil
}

// fromLines lists a setup script's from lines by their first two words.
func (s *state) fromLines(scenario string) ([]string, error) {
	script, err := s.ScriptPath(scenario)
	if err != nil {
		return nil, err
	}
	text, err := s.ReadFile(script)
	if err != nil {
		return nil, err
	}
	lines := make([]string, 0, 2)
	for _, line := range strings.Split(text, "\n") {
		if fields := strings.Fields(line); len(fields) >= 2 && fields[0] == "from" {
			lines = append(lines, fields[0]+" "+fields[1])
		}
	}
	return lines, nil
}

func (s *state) fromLineUpdated() error {
	lines, err := s.fromLines("3-D Secure challenge")
	if err != nil {
		return err
	}
	want := "from checkout/" + slug.Of(s.renamedTo)
	if !slices.Contains(lines, want) {
		return fmt.Errorf("3-D Secure challenge's setup has the from lines %q, not %q", lines, want)
	}
	if old := "from checkout/" + slug.Of(s.renamedFrom); slices.Contains(lines, old) {
		return fmt.Errorf("3-D Secure challenge's setup still has %q", old)
	}
	return nil
}

func (s *state) statusOf(name string, want string) error {
	window, err := s.window()
	if err != nil {
		return err
	}
	scenario, err := window.Scenario(name)
	if err != nil {
		return err
	}
	if scenario.Status != want {
		return fmt.Errorf("%q shows %q, not %q", name, scenario.Status, want)
	}
	return nil
}

func (s *state) renamedShows(status string) error { return s.statusOf(s.renamedTo, status) }

func (s *state) sidebarInPlaceOf(newName string, oldName string) error {
	window, err := s.window()
	if err != nil {
		return err
	}
	items := window.Sidebar()
	if err := sidebarHas(items, newName); err != nil {
		return err
	}
	if sidebarHas(items, oldName) == nil {
		return fmt.Errorf("the sidebar still lists %q", oldName)
	}
	return nil
}

// folderRenamed expects every file the capability had, under its new folder and name.
func (s *state) folderRenamed(folder string, file string) error {
	if folder != slug.Of(s.renamedTo) || file != folder+".feature" {
		return fmt.Errorf("%s/%s isn't named after %q", folder, file, s.renamedTo)
	}
	_, err := os.Stat(filepath.Join(s.Capabilities(), s.actedOn))
	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%s is still there: %v", s.actedOn, err)
	}
	text, err := s.ReadFile(featurePath(folder))
	if err != nil {
		return err
	}
	if first, _, _ := strings.Cut(text, "\n"); first != "Feature: "+s.renamedTo {
		return fmt.Errorf("%s starts %q, not %q", file, first, "Feature: "+s.renamedTo)
	}
	kinds := map[string]int{}
	for _, rel := range s.filesBefore {
		moved := rel
		if rel == s.actedOn+".feature" {
			moved = file
		}
		if _, err := os.Stat(filepath.Join(s.Capabilities(), folder, moved)); err != nil {
			return fmt.Errorf("%s/%s: %w", folder, moved, err)
		}
		kind, _, _ := strings.Cut(rel, "/")
		kinds[kind]++
	}
	if kinds["scripts"] == 0 || kinds["_shared"] == 0 {
		return fmt.Errorf("%s had no scripts or no helpers to move: %q", s.renamedFrom,
			s.filesBefore)
	}
	return nil
}

func (s *state) scenariosShow(status string) error {
	_, names, err := s.scenarioSteps(slug.Of(s.renamedTo))
	if err != nil {
		return err
	}
	if len(names) == 0 {
		return fmt.Errorf("%q has no scenarios", s.renamedTo)
	}
	for _, name := range names {
		if err := s.statusOf(name, status); err != nil {
			return err
		}
	}
	return nil
}

// lineBecomes expects Accept invite's setup to start with line, and no from line naming the
// capability's old slug.
func (s *state) lineBecomes(line string) error {
	script, err := s.ScriptPath("Accept invite")
	if err != nil {
		return err
	}
	text, err := s.ReadFile(script)
	if err != nil {
		return err
	}
	lines := strings.Split(text, "\n")
	if len(lines) < 2 || lines[1] != line {
		return fmt.Errorf("Accept invite's setup doesn't start with %q: %q", line, text)
	}
	if old := "from " + slug.Of(s.renamedFrom) + "/"; strings.Contains(text, old) {
		return fmt.Errorf("Accept invite's setup still has %q: %q", old, text)
	}
	return nil
}

func (s *state) bothShow(first string, second string, status string) error {
	if err := s.statusOf(first, status); err != nil {
		return err
	}
	return s.statusOf(second, status)
}

// runsRecorded records passing runs a minute apart, as a developer dropping in again would.
func (s *state) runsRecorded(name string, count string) error {
	runs, err := strconv.Atoi(count)
	if err != nil {
		return err
	}
	s.subject = name
	if err := s.arrangeSetups(); err != nil {
		return err
	}
	for run := 0; run < runs; run++ {
		if err := s.RecordPassingRun(name); err != nil {
			return err
		}
		s.Clock.Advance(time.Minute)
	}
	return nil
}

func (s *state) passingRunToday(name string) error { return s.runsRecorded(name, "1") }

func (s *state) listedAsUnverified() error {
	window, err := s.window()
	if err != nil {
		return err
	}
	if err := window.Select("Unverified"); err != nil {
		return err
	}
	listed := window.ListedScenarios()
	for _, scenario := range listed {
		if scenario.Name == s.subject {
			return nil
		}
	}
	return fmt.Errorf("the Unverified view doesn't list %q: %v", s.subject, listed)
}

func (s *state) runsUnchanged() error {
	if len(s.runsBefore) == 0 {
		return errors.New("no runs were recorded before the rename")
	}
	after, err := snapshot(s.Project, isRunsFile)
	if err != nil {
		return err
	}
	return sameFiles(s.runsBefore, after)
}
