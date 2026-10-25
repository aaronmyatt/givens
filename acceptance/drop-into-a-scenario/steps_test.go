// Package dropintoascenario runs capabilities/drop-into-a-scenario. Steps shared with other
// capabilities are in acceptance/world.
//
// Each Given writes the setup scripts it needs into the scenario's copy of the sample shop, so
// a scenario never depends on what the fixture's own scripts print. Scripts leave marker files
// in the shop's shop-data/ folder; Then steps read them to see what ran, and with what.
//
// Timers run on the world's fake clock: a step moves it forward instead of waiting two minutes.
package dropintoascenario

import (
	"testing"
	"time"

	"givens/acceptance/world"
)

const capability = "drop-into-a-scenario"

func TestScenarios(t *testing.T) { world.RunSuite(t, capability, steps) }

func TestStepsDefined(t *testing.T) { world.CheckStepsDefined(t, capability, steps) }

// state is what this capability's steps keep between steps of one scenario.
type state struct {
	*world.World
	scenario      string              // the scenario a Given arranged, for steps that don't name it
	markers       []string            // shop-data files a script writes only if it runs
	slowFor       time.Duration       // how long the blocked setup "takes", on the fake clock
	sawSlow       bool                // "Still running" appeared while a step watched for it
	note          string              // the note added before "Record run"
	attached      string              // the path of the file attached before "Record run"
	runKey        string              // the run key the scenario recorded against
	snippet       string              // the by_hand card's snippet
	link          string              // the by_hand card's --url link
	fromSetupLine string              // what the setup a from line names prints
	cardIndex     int                 // the card a step picked, by its index in Cards
	opener        *urlOpener          // records links "Open" sends to the default browser
	outputs       map[string][]string // setup output after each way in, for teardown checks
	mergeOutput   string              // git merge's output
	mergeExit     int                 // git merge's exit code
}

func steps(w *world.World) []world.Step {
	s := &state{World: w, opener: &urlOpener{}, outputs: make(map[string][]string, 3)}
	all := make([]world.Step, 0, 128)
	all = append(all, runSteps(s)...)
	all = append(all, recordSteps(s)...)
	all = append(all, sequenceSteps(s)...)
	all = append(all, noteSteps(s)...)
	all = append(all, teardownSteps(s)...)
	all = append(all, fromSteps(s)...)
	all = append(all, byHandSteps(s)...)
	all = append(all, rowSteps(s)...)
	return append(all, markdownSteps(s)...)
}

// runSteps cover running setup: output, failures, missing commands and timeouts.
func runSteps(s *state) []world.Step {
	return []world.Step{
		world.Def(`^the "Card declined" setup prints a line, then waits before printing the next$`,
			s.printsThenWaits),
		world.Def(`^the setup script's output appears line by line as it runs$`, s.outputStreams),
		world.Def(`^when it finishes, any steps done by hand, then the When and Then steps, are `+
			`shown as a checklist$`, s.checklistShown),
		world.Def(`^each Then step can be marked "Pass" or "Fail"$`, s.thenStepsMarkable),
		world.Def(`^the "Card declined" setup script stops with an error part way through$`,
			s.setupStopsPartWay),
		world.Def(`^the output is shown up to the point it stopped$`, s.outputUpToStop),
		world.Def(`^no checklist is shown$`, s.noChecklist),
		world.Def(`^the "Card declined" setup script needs a command called shopctl, which `+
			`isn't installed$`, s.needsShopctl),
		world.Def(`^nothing in the script runs$`, s.nothingRuns),
		world.Def(`^the install hint "([^"]*)" is shown, and "Copy" copies it$`, s.installHint),
		world.Def(`^the "Card declined" setup script takes more than two minutes$`,
			s.setupTakesTooLong),
		world.Def(`^"([^"]*)" isn't tagged @timeout$`, s.untagTimeout),
		world.Def(`^after two minutes they see "([^"]*)"$`, s.afterTwoMinutes),
		world.Def(`^choosing "Stop" ends the script and keeps its output so far$`,
			s.stopKeepsOutput),
		world.Def(`^"([^"]*)" is tagged (@timeout:\S+)$`, s.tagTimeout),
		world.Def(`^its setup takes (\d+) minutes$`, s.setupTakesMinutes),
		world.Def(`^no "Still running" message is shown$`, s.noStillRunning),
		world.Def(`^setup finishes as normal$`, s.finishesAsNormal),
		world.Def(`^setup runs with the usual 2-minute timeout$`, s.usualTimeout),
		world.Def(`^"([^"]*)" is marked "([^"]*)"$`, s.markedWith),
	}
}

// recordSteps cover recording runs in runs/, and merging them in git.
func recordSteps(s *state) []world.Step {
	return []world.Step{
		world.Def(`^a developer has dropped in to "([^"]*)" and marked every Then step "Pass"$`,
			s.droppedInAllPassed),
		world.Def(`^they add the note "([^"]*)"$`, s.addNote),
		world.Def(`^a line for (\S+) is added to today's file in runs/, with the result and `+
			`their note$`, s.runLineWithNote),
		world.Def(`^a developer has dropped in to "([^"]*)" and marked only the first Then step$`,
			s.droppedInFirstMarked),
		world.Def(`^nothing is added to runs/$`, s.nothingInRuns),
		world.Def(`^they attach the file "([^"]*)"$`, s.attachFile),
		world.Def(`^"([^"]*)" is copied into the run's folder in runs/$`, s.copiedIntoRunFolder),
		world.Def(`^the run, in the scenario's run history, lists "([^"]*)"$`,
			s.historyListsFile),
		world.Def(`^a developer whose git user\.name is "([^"]*)" has dropped in to "([^"]*)" `+
			`and marked every Then step "Pass"$`, s.namedDeveloperAllPassed),
		world.Def(`^the run line includes "([^"]*)"$`, s.runLineIncludes),
		world.Def(`^Sam and Ali each recorded a run for "([^"]*)" today, on different branches$`,
			s.runsOnTwoBranches),
		world.Def(`^the branches are merged$`, s.mergeBranches),
		world.Def(`^git merges runs/ with no conflict$`, s.mergedCleanly),
		world.Def(`^both run lines are kept$`, s.bothLinesKept),
	}
}

// teardownSteps cover Reset state, idempotent setup and teardown scripts.
func teardownSteps(s *state) []world.Step {
	return []world.Step{
		world.Def(`^a developer has dropped in to "([^"]*)" and marked "([^"]*)" done$`,
			s.droppedInMarkedDone),
		world.Def(`^the setup script runs again from the start$`, s.setupRanAgain),
		world.Def(`^the checklist is cleared$`, s.checklistCleared),
		world.Def(`^"([^"]*)" has no teardown script$`, s.noTeardown),
		world.Def(`^a developer has dropped in to "([^"]*)" once$`, s.droppedInOnce),
		world.Def(`^setup finishes without errors$`, s.finishesWithoutErrors),
		world.Def(`^the shop has one shopper "([^"]*)", not two$`, s.oneShopper),
		world.Def(`^"([^"]*)" has a script called ([a-z0-9-]+\.teardown\.sh)$`, s.hasTeardown),
		world.Def(`^a developer drops in, or chooses "Reset state" or "Try again"$`,
			s.everyWayIn),
		world.Def(`^the output shows "→ teardown" followed by the teardown's output$`,
			s.teardownFirst),
		world.Def(`^then setup runs from the start$`, s.setupAfterTeardown),
		world.Def(`^checkout/scripts/card-declined\.teardown\.sh is created, starting with the `+
			`line that sources givens\.sh$`, s.teardownCreated),
		world.Def(`^the "Card declined" teardown stops with an error$`, s.teardownFails),
		world.Def(`^setup doesn't run$`, s.setupDidNotRun),
	}
}

// fromSteps cover setups that build on other scenarios with `from`.
func fromSteps(s *state) []world.Step {
	return []world.Step{
		world.Def(`^"([^"]*)" has a setup that prints "([^"]*)"$`, s.hasSetupPrinting),
		world.Def(`^the output shows "([^"]*)" followed by that scenario's setup$`,
			s.fromFollowedBySetup),
		world.Def(`^the checklist has only the steps of "([^"]*)"$`, s.checklistOnlyOf),
		world.Def(`^both scenarios have a teardown script$`, s.bothHaveTeardowns),
		world.Def(`^the output shows 3-D Secure challenge's teardown first$`, s.ownTeardownFirst),
		world.Def(`^then "([^"]*)", followed by Pay with saved card's teardown and setup$`,
			s.fromThenTeardownAndSetup),
		world.Def(`^then the rest of 3-D Secure challenge's setup$`, s.restOfSetup),
		world.Def(`^"([^"]*)" starts with "([^"]*)"$`, s.startsWith),
		world.Def(`^the same loop as from-loop$`, s.sameLoop),
		world.Def(`^a developer runs "([^"]*)" from the project's root folder$`, s.runFromRoot),
		world.Def(`^the kit prints "([^"]*)"$`, s.kitPrints),
		world.Def(`^the script stops with a non-zero exit code$`, s.nonZeroExit),
		world.Def(`^the output shows "([^"]*)"$`, s.outputShows),
	}
}
