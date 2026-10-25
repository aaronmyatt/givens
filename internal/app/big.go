// The big file: Givens's behaviour, told in the order of its stories.
//
// Each story in capabilities/ that's moved here has a section, in the order of the
// capabilities folders. A section starts with a "// Feature: <name>" comment, the Feature's
// name word for word, and runs to the next one. Inside it, each of the Feature's Rules has a
// "// Rule: <name>" comment, in the Feature's order, and a "// Scenario: <name>" comment marks
// the code a scenario starts in where that helps a reader; several can share one piece of code.
// Code that two or more stories use is in big_helpers.go, so this file reads as the stories
// alone.
//
// TestBigFile in acceptance/ checks those comments against the feature files, and
// scripts/entry.sh finds the section for a Feature, Rule or Scenario name. The workflow is
// tasks/one-big-file/essay.md; the pilot is tasks/one-big-file/notes.md. The rest of
// internal/app isn't here yet.
//
// Code outside this file that a story depends on, and why it's elsewhere:
//   - internal/kit/givens.sh: bash, run in the developer's shell by every setup script.

package app

import (
	"errors"
	"path/filepath"
	"slices"

	"givens/internal/assert"
	"givens/internal/helpers"
	"givens/internal/project"
)

// Feature: Copy a setup script
//
// capabilities/copy-a-setup-script. A scenario's setup goes on the clipboard as a command to
// paste anywhere in the project. "Run from a subfolder" is the kit's: it moves to the repo's
// root before running anything (the cd after the helpers load, in internal/kit/givens.sh).

// Scenario: Copy
// Scenario: Capabilities folder isn't at the root
// Scenario: Copy steps done by hand

// copySetup puts a scenario's setup on the clipboard: `bash <path>.sh`, the path from the git
// repo's root, or for a setup done only by hand, its markdown.
func (w *Window) copySetup(scenario project.Scenario) error {
	assert.That(w != nil, "copy runs on a window")
	assert.That(scenario.Capability != "", "the scenario belongs to a capability")
	clipboard := w.app.config.Clipboard
	if scenario.Setup == "" && scenario.SetupCards != "" {
		text, err := helpers.ReadScript(scenario.SetupCards)
		if err != nil {
			return err
		}
		return clipboard.WriteText(text)
	}
	if scenario.Setup == "" {
		return errors.New(scenario.Name + " has no setup to copy")
	}
	relative, err := filepath.Rel(repoRoot(w.path), scenario.Setup)
	if err != nil {
		return err
	}
	return clipboard.WriteText("bash " + filepath.ToSlash(relative))
}

// Scenario: Copy from search

// KeyCopySetup copies the selected search result's setup command.
const KeyCopySetup = "⇧⌘C"

// copyFromSearch copies the setup of the scenario first among the search's results.
func (w *Window) copyFromSearch() error {
	assert.That(w != nil, "copy from search runs on a window")
	assert.That(w.search != nil, "search is open")
	for _, group := range w.SearchResults().Groups {
		if group.Kind != SearchScenarios || len(group.Results) == 0 {
			continue
		}
		first := group.Results[0]
		found, ok := w.findScenario(first.Capability + "/" + first.Name)
		if !ok {
			return errors.New(first.Name + ": " + ErrNotFound.Error())
		}
		return w.copySetup(found)
	}
	return errors.New("no scenario is listed to copy")
}

// Feature: Trust a project
//
// capabilities/trust-a-project. Givens asks before it runs any of a project's scripts, and
// remembers the answer by the capabilities folder's path, in its own data folder: never in the
// project, so a cloned repo can't mark itself trusted. Reading, searching and copying need no
// trust. Drop in (startSession) asks here before its setup runs.

// Trust lives in the app's own data folder, one list of folders for each answer.
const (
	trustedFileName   = "trusted.txt"
	untrustedFileName = "untrusted.txt"
	// TrustedMax bounds the trusted folders Givens remembers.
	TrustedMax = 1024
)

// Trust prompt buttons and the message once scripts are off.
const (
	OfferTrust            = "Trust"
	OfferDontTrust        = "Don't trust"
	OfferTrustThisProject = "Trust this project"
	MessageScriptsOff     = "Scripts are off for this project"
)

// Scenario: Asked before the first run
// Scenario: Trust is remembered
// Scenario: Moved or re-cloned project

// trustOf reads what the developer chose for this window's project: trusted, refused ("Don't
// trust"), or neither yet. It goes by path, so a moved or re-cloned project is neither.
func (w *Window) trustOf() (bool, bool, error) {
	assert.That(w != nil, "trust of runs on a window")
	assert.That(filepath.IsAbs(w.path), "a window's folder is absolute")
	trusted, err := w.app.Trusted(w.path)
	if err != nil {
		return false, false, err
	}
	refused, err := w.app.untrusted(w.path)
	return trusted, refused, err
}

// runTrusted begins a Drop in's setup if the project is trusted, asks "Run scripts from this
// project?" if it's neither trusted nor refused, and opens the page with scripts off if it's
// refused. A setup with no script needs no trust: nothing runs.
func (w *Window) runTrusted(session *Session, runsScripts bool, trusted bool,
	refused bool) error {
	assert.That(w != nil, "run trusted runs on a window")
	assert.That(session != nil && session.window == w, "the session is this window's")
	switch {
	case runsScripts && refused:
		session.scriptsOff()
	case runsScripts && !trusted:
		w.askEither("Run scripts from this project? "+filepath.Dir(w.path), OfferTrust,
			func() error {
				if err := w.app.MarkTrusted(w.path); err != nil {
					return err
				}
				return session.begin()
			}, OfferDontTrust, func() error {
				session.scriptsOff()
				return w.app.markUntrusted(w.path)
			})
	default:
		return session.begin()
	}
	return nil
}

// MarkTrusted records a project's folder as trusted, as choosing "Trust" on the "Run scripts
// from this project?" prompt does. path is a capabilities folder, or the project folder holding
// one, as Open takes. Drop in scenarios that aren't about trust use it to start trusted.
func (a *App) MarkTrusted(path string) error {
	assert.That(a != nil, "mark trusted runs on an app")
	assert.That(path != "", "a trusted folder has a path")
	assert.That(len(path) <= PathLengthMax, "the path fits PathLengthMax")
	folder, _, err := findCapabilities(path)
	if err != nil {
		return err
	}
	if err := a.forget(untrustedFileName, folder); err != nil {
		return err
	}
	return a.remember(trustedFileName, folder)
}

// Trusted is true when folder, a capabilities folder as a window holds it, was marked trusted.
func (a *App) Trusted(folder string) (bool, error) {
	assert.That(a != nil, "trusted runs on an app")
	assert.That(filepath.IsAbs(folder), "a window's folder is absolute")
	trusted, err := a.folders(trustedFileName)
	if err != nil {
		return false, err
	}
	return slices.Contains(trusted, folder), nil
}

// trustedFolders reads the trusted folders.
func (a *App) trustedFolders() ([]string, error) {
	assert.That(a != nil, "trusted folders runs on an app")
	assert.That(a.config.AppDataDir != "", "trust lives in the app's data folder")
	return a.folders(trustedFileName)
}

// Scenario: Choose not to trust

// markUntrusted records "Don't trust" for a capabilities folder: its scripts stay off until
// the developer chooses "Trust this project".
func (a *App) markUntrusted(folder string) error {
	assert.That(a != nil, "mark untrusted runs on an app")
	assert.That(filepath.IsAbs(folder), "a window's folder is absolute")
	if err := a.forget(trustedFileName, folder); err != nil {
		return err
	}
	return a.remember(untrustedFileName, folder)
}

// untrusted is true when the developer chose "Don't trust" for folder.
func (a *App) untrusted(folder string) (bool, error) {
	assert.That(a != nil, "untrusted runs on an app")
	assert.That(filepath.IsAbs(folder), "a window's folder is absolute")
	folders, err := a.folders(untrustedFileName)
	if err != nil {
		return false, err
	}
	return slices.Contains(folders, folder), nil
}

// scriptsOff opens the Drop in page with nothing run: the developer chose "Don't trust".
// Reading and copying still work.
func (s *Session) scriptsOff() {
	assert.That(s != nil, "scripts off runs on a session")
	assert.That(s.process == nil, "nothing has run")
	s.status, s.message, s.untrusted = SessionStopped, MessageScriptsOff, true
}

// trustThisProject trusts the project from a Drop in with scripts off, then drops in again.
func (s *Session) trustThisProject() error {
	assert.That(s != nil, "trust this project runs on a session")
	assert.That(s.untrusted, "scripts are off")
	if err := s.window.app.MarkTrusted(s.window.path); err != nil {
		return err
	}
	return s.restart(false)
}
