// Package updatethekit runs capabilities/update-the-kit. Steps shared with other
// capabilities are in acceptance/world.
//
// Each scenario decides which kit versions Givens ships through App.ShipKits, and writes the
// project's own givens.sh, so the versions in the steps are the versions under test.
package updatethekit

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"givens/acceptance/world"
	"givens/internal/app"
)

const capability = "update-the-kit"

// kitPath is the kit inside the sample shop.
const kitPath = "capabilities/_shared/givens.sh"

// localEdit is what a developer added to their kit by hand.
const localEdit = "\n# Added by hand for this project.\nshop_helper() { ok \"shop helper\"; }\n"

func TestScenarios(t *testing.T) { world.RunSuite(t, capability, steps) }

func TestStepsDefined(t *testing.T) { world.CheckStepsDefined(t, capability, steps) }

// state is what this capability's steps keep between steps of one scenario.
type state struct {
	*world.World
	releases []app.KitRelease // what Givens ships, oldest first
}

func steps(w *world.World) []world.Step {
	s := &state{World: w}
	return []world.Step{
		world.Def(`^the sample shop's givens\.sh is version (\d+)$`, s.kitIsVersion),
		world.Def(`^Givens ships version (\d+)$`, s.ships),
		world.Def(`^a developer opens the sample shop$`, s.openShop),
		world.Def(`^the sample shop's givens\.sh is version (\d+) and hasn't been edited$`,
			s.kitUnedited),
		world.Def(`^givens\.sh is replaced with version (\d+)$`, s.replacedWith),
		world.Def(`^git diff shows only givens\.sh changed$`, s.onlyKitChanged),
		world.Def(`^the sample shop's givens\.sh has local edits$`, s.kitEdited),
		world.Def(`^reading, searching and copying still work$`, s.readSearchCopyWork),
		world.Def(`^the sample shop has no givens\.sh$`, s.noKit),
		world.Def(`^givens\.sh is written at version (\d+)$`, s.replacedWith),
		world.Def(`^deleting givens\.sh while the sample shop is open puts version (\d+) back$`,
			s.deletedKitComesBack),
	}
}

// kitText is the kit as Givens shipped it at version: a version line, then a body that
// differs between versions.
func kitText(version int) string {
	number := strconv.Itoa(version)
	return app.KitVersionPrefix + number + "\n" +
		"# The kit, release " + number + ", as this scenario ships it.\n" +
		"set -euo pipefail\n" +
		"_givens_kit_version=" + number + "\n" +
		"ok() { printf '✓ %s\\n' \"$*\"; }\n"
}

// releasesTo lists kit releases 1 to last, oldest first.
func releasesTo(last int) []app.KitRelease {
	releases := make([]app.KitRelease, 0, last)
	for version := 1; version <= last; version++ {
		releases = append(releases, app.KitRelease{Version: version, Text: kitText(version)})
	}
	return releases
}

// writeKit copies the shop, gives Card declined a setup so "Copy" has something to copy, and
// writes the project's kit.
func (s *state) writeKit(text string) error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	if err := s.SetupStartsWith("Card declined", `ok "card declined is ready"`); err != nil {
		return err
	}
	return s.WriteFile(kitPath, text)
}

func (s *state) kitIsVersion(version int) error {
	return s.writeKit(kitText(version))
}

func (s *state) ships(version int) error {
	s.releases = releasesTo(version)
	return nil
}

// openShop starts Givens shipping the scenario's kits and opens the sample shop.
func (s *state) openShop() error {
	if len(s.releases) == 0 {
		return errors.New("no step said which kit version Givens ships")
	}
	if err := s.StartApp(); err != nil {
		return err
	}
	if err := s.App.ShipKits(s.releases); err != nil {
		return err
	}
	window, err := s.App.Open(s.Capabilities())
	if err != nil {
		return fmt.Errorf("opening the sample shop: %w", err)
	}
	s.Window = window
	return nil
}

// kitUnedited writes an unchanged older kit, commits the shop to git so a later step can
// see what changed, and opens it with Givens shipping the next version.
func (s *state) kitUnedited(version int) error {
	if err := s.writeKit(kitText(version)); err != nil {
		return err
	}
	if err := s.commitShop(); err != nil {
		return err
	}
	s.releases = releasesTo(version + 1)
	return s.openShop()
}

// kitEdited writes kit version 1 with a local edit and opens it with Givens shipping 2.
func (s *state) kitEdited() error {
	if err := s.writeKit(kitText(1) + localEdit); err != nil {
		return err
	}
	s.releases = releasesTo(2)
	return s.openShop()
}

// git runs one git command in the sample shop, with an identity and no hooks or signing from
// this Mac's settings. Ref: https://git-scm.com/docs/git#Documentation/git.txt--cltnamegtltvaluegt
func (s *state) git(args ...string) (string, error) {
	settings := []string{
		"-c", "user.name=Givens", "-c", "user.email=givens@example.com",
		"-c", "commit.gpgsign=false", "-c", "core.hooksPath=/dev/null",
	}
	command := exec.Command("git", append(settings, args...)...)
	command.Dir = s.Project
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return string(output), nil
}

// commitShop makes the shop a git repository with everything committed.
// Ref: https://git-scm.com/docs/git-init and https://git-scm.com/docs/git-commit
func (s *state) commitShop() error {
	for _, args := range [][]string{
		{"init", "--quiet"}, {"add", "--all"}, {"commit", "--quiet", "-m", "Sample shop"},
	} {
		if _, err := s.git(args...); err != nil {
			return err
		}
	}
	return nil
}

func (s *state) replacedWith(version int) error {
	got, err := s.ReadFile(kitPath)
	if err != nil {
		return err
	}
	if got != kitText(version) {
		return fmt.Errorf("givens.sh isn't kit version %d; it holds:\n%s", version, got)
	}
	return nil
}

// onlyKitChanged lists every changed and new file, so a file written beside the kit shows.
// Ref: https://git-scm.com/docs/git-status#_porcelain_format_version_1
func (s *state) onlyKitChanged() error {
	output, err := s.git("status", "--porcelain=v1", "--untracked-files=all")
	if err != nil {
		return err
	}
	changed := strings.Split(strings.TrimRight(output, "\n"), "\n")
	if len(changed) != 1 || filepath.ToSlash(strings.TrimSpace(changed[0])) != "M "+kitPath {
		return fmt.Errorf("git shows %q changed, not only %s", changed, kitPath)
	}
	return nil
}

// readSearchCopyWork reads Checkout, copies Card declined's setup, then searches for it.
func (s *state) readSearchCopyWork() error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	checkout, err := window.Capability("Checkout")
	if err != nil {
		return err
	}
	if len(checkout.Scenarios) == 0 {
		return errors.New("reading Checkout shows no scenarios")
	}
	if err := window.Select("Card declined"); err != nil {
		return err
	}
	if err := window.Choose("Copy"); err != nil {
		return err
	}
	copied, err := s.Clipboard.ReadText()
	if err != nil {
		return err
	}
	if !strings.Contains(copied, "card-declined") {
		return fmt.Errorf("\"Copy\" put %q on the clipboard, not Card declined's setup", copied)
	}
	if err := s.Search("declined"); err != nil {
		return err
	}
	return world.Expect(window.Visible(), "Card declined", "the search results")
}

// noKit copies the shop and takes its kit away.
func (s *state) noKit() error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	return os.Remove(filepath.Join(s.Project, filepath.FromSlash(kitPath)))
}

// deletedKitComesBack deletes the kit from the open project, then reads the window, as the
// TUI does each time it draws, and checks the kit is back.
func (s *state) deletedKitComesBack(version int) error {
	if err := os.Remove(filepath.Join(s.Project, filepath.FromSlash(kitPath))); err != nil {
		return err
	}
	window, err := s.Front()
	if err != nil {
		return err
	}
	if _, err := window.Capability("Checkout"); err != nil {
		return err
	}
	return s.replacedWith(version)
}
