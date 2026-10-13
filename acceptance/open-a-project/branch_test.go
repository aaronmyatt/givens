package openaproject

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"givens/acceptance/world"
	"givens/internal/app"
)

// gitTimeout bounds each git call a step makes; checkoutFeature is the sample shop's Checkout.
const (
	gitTimeout      = 20 * time.Second
	checkoutFeature = "capabilities/checkout/checkout.feature"
)

// branchSteps cover reading another branch's capabilities without checking it out.
func branchSteps(s *state) []world.Step {
	return []world.Step{
		world.Def(`^the sample shop is a git repository with a branch "([^"]*)" that adds the `+
			`scenario "([^"]*)" to Checkout$`, s.branchAddsScenario),
		world.Def(`^they open the branch "([^"]*)"$`, s.openBranch),
		world.Def(`^a second window opens, its title ending "([^"]*)"$`, s.secondWindowTitled),
		world.Def(`^it lists "([^"]*)" under "([^"]*)"$`, s.listsUnder),
		world.Def(`^the sample shop's own checkout\.feature doesn't have "([^"]*)"$`,
			s.ownFeatureLacks),
		world.Def(`^"Drop in" isn't offered for "([^"]*)"$`, s.noDropIn),
	}
}

// git runs git in the sample shop, away from this Mac's own git settings.
// Ref: https://git-scm.com/docs/git#Documentation/git.txt-codeGITCONFIGGLOBALcode
func (s *state) git(args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, "git", args...)
	command.Dir = s.Project
	command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, output)
	}
	return nil
}

// branchAddsScenario commits the sample shop on main, then adds a scenario to Checkout on a
// branch, and goes back to main, so the working tree doesn't have it.
// Ref: https://git-scm.com/docs/git-init and https://git-scm.com/docs/git-checkout
func (s *state) branchAddsScenario(branch string, name string) error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	scenario := "\n  Scenario: " + name + "\n    Given a shopper with a gift card\n" +
		"    When they pay with it\n    Then the order is paid\n"
	commands := [][]string{
		{"init", "--quiet", "--initial-branch=main"},
		{"config", "user.name", "Sam"}, {"config", "user.email", "sam@example.com"},
		{"config", "commit.gpgsign", "false"},
		{"add", "--all"}, {"commit", "--quiet", "--message", "Sample shop"},
		{"checkout", "--quiet", "-b", branch},
	}
	for _, args := range commands {
		if err := s.git(args...); err != nil {
			return err
		}
	}
	text, err := s.ReadFile(checkoutFeature)
	if err != nil {
		return err
	}
	if err := s.WriteFile(checkoutFeature, text+scenario); err != nil {
		return err
	}
	for _, args := range [][]string{
		{"commit", "--quiet", "--all", "--message", name}, {"checkout", "--quiet", "main"},
	} {
		if err := s.git(args...); err != nil {
			return err
		}
	}
	return nil
}

func (s *state) openBranch(name string) error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	s.gotFront = window
	s.opened, err = window.OpenBranch(name)
	return err
}

func (s *state) secondWindowTitled(suffix string) error {
	if count := len(s.App.Windows()); count != 2 {
		return fmt.Errorf("%d windows are open, not 2", count)
	}
	front := s.App.Front()
	if front == s.gotFront || front != s.opened {
		return fmt.Errorf("the branch's window isn't in front")
	}
	if !strings.HasSuffix(front.Title(), suffix) {
		return fmt.Errorf("the front window's title is %q", front.Title())
	}
	return nil
}

func (s *state) listsUnder(scenario string, capability string) error {
	shown, err := s.App.Front().Capability(capability)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(shown.Scenarios))
	for _, listed := range shown.Scenarios {
		names = append(names, listed.Name)
	}
	return world.Expect(names, scenario, capability+"'s scenarios")
}

func (s *state) ownFeatureLacks(name string) error {
	text, err := os.ReadFile(filepath.Join(s.Project, checkoutFeature))
	if err != nil {
		return err
	}
	if strings.Contains(string(text), name) {
		return fmt.Errorf("the working tree's checkout.feature has %q", name)
	}
	return nil
}

// noDropIn selects the scenario in the branch's window: "Drop in" isn't offered, and asking
// for it anyway is refused.
func (s *state) noDropIn(name string) error {
	front := s.App.Front()
	if err := front.Select(name); err != nil {
		return err
	}
	if slices.Contains(front.Offers(), app.OfferDropIn) {
		return fmt.Errorf("the branch's window offers %q", front.Offers())
	}
	if _, err := front.DropIn(name); err == nil {
		return fmt.Errorf("the branch's window dropped in to %s", name)
	}
	return nil
}
