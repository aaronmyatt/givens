package mixshellbrowserandhandsteps

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"givens/acceptance/world"
	"givens/internal/app"
	"givens/internal/browser"
)

// neverSettlesWait is how long "JavaScript that never answers" waits: in_browser's 30 s,
// plus room for Chrome to start.
const neverSettlesWait = browser.EvalSecondsMax*time.Second + 20*time.Second

// browserSteps are the When and Then steps about the Givens browser and setup's output.
func browserSteps(s *state) []world.Step {
	return []world.Step{
		world.Def(`^the output shows "([^"]*)"$`, s.outputShows),
		world.Def(`^the Givens browser shows the shop's cart page$`,
			func() error { return s.pageIs("/cart") }),
		world.Def(`^the output shows the seed line, then the app action's line, then the `+
			`by_hand card$`, s.seedActionCard),
		world.Def(`^choosing "Done" on the card shows the checklist$`, s.doneShowsChecklist),
		world.Def(`^"([^"]*)" is printed only after the promise settles$`, s.afterSettled),
		world.Def(`^the rest of setup sees ORDER as "([^"]*)"$`, func(order string) error {
			return s.outputShows("✓ ORDER is " + order)
		}),
		world.Def(`^no checklist is shown$`, s.noChecklist),
		world.Def(`^after 30 seconds the output shows "([^"]*)"$`, func(text string) error {
			session, err := s.session()
			if err != nil {
				return err
			}
			session.Wait(neverSettlesWait)
			return s.outputShows(text)
		}),
		world.Def(`^nothing in the script runs$`, s.nothingRan),
		world.Def(`^the install hint "([^"]*)" is shown, and Copy copies it$`, s.installHint),
		world.Def(`^a developer drops in to "([^"]*)" and setup finishes$`, s.dropInReady),
		world.Def(`^the Givens browser stays open on the shop, signed in as "([^"]*)"$`,
			s.signedIn),
		world.Def(`^the developer's own Chrome profile is left as it was$`, s.ownProfile),
		world.Def(`^a developer has dropped in to "([^"]*)" and the Givens browser is open$`,
			s.droppedInWithBrowser),
		world.Def(`^setup runs again in the same Givens browser window$`, s.samePage),
		world.Def(`^setup uses that same Givens browser window$`, s.samePage),
		world.Def(`^no second browser window opens$`, s.onePage),
		world.Def(`^a developer drops in and chooses "Open" on the card$`, s.openOnCard),
		world.Def(`^the admin page opens in the Givens browser, not the default browser$`,
			s.adminInGivensBrowser),
		world.Def(`^a developer runs "([^"]*)" in a terminal$`, s.Run),
		world.Def(`^the Givens browser opens and the app action runs, as in Drop in$`,
			s.terminalRan),
		world.Def(`^the browser stays open after the script exits$`, s.stillOpen),
		world.Def(`^the givens command isn't on the PATH$`, func() error { return nil }),
		world.Def(`^a developer runs the script in a terminal$`, s.runWithoutGivens),
		// The common "it prints" takes no quotes inside; this line has some.
		world.Def(`^nothing after that line runs$`, s.nothingAfter),
		world.Def(`^a developer ran the "([^"]*)" setup in a terminal and the Givens browser `+
			`is open$`, s.ranInTerminal),
	}
}

// eval runs JavaScript in the sample shop's Givens browser.
func (s *state) eval(js string) (string, error) {
	profile, err := s.Browser()
	if err != nil {
		return "", err
	}
	return browser.Eval(profile, js)
}

func (s *state) pageIs(path string) error {
	at, err := s.eval(`location.pathname`)
	if err != nil {
		return err
	}
	if at != path {
		return fmt.Errorf("the Givens browser is at %q, not %q", at, path)
	}
	return nil
}

// seedActionCard expects the seed line, then signIn's result, in the output, and the by_hand
// card waiting after them.
func (s *state) seedActionCard() error {
	session, err := s.session()
	if err != nil {
		return err
	}
	output := session.Output()
	seed := slices.Index(output, "✓ seeded sam")
	action := slices.Index(output, "sam@example.com")
	if seed < 0 || action < seed {
		return fmt.Errorf("the output isn't the seed line, then the action's: %q", output)
	}
	cards := session.Cards()
	if len(cards) != 1 || cards[0].Where != "Phone" || cards[0].Text != "Approve the sign-in" {
		return fmt.Errorf("the cards are %+v", cards)
	}
	if session.Status() != app.SessionWaiting {
		return fmt.Errorf("setup is %s, not waiting on the card", session.Status())
	}
	return nil
}

func (s *state) doneShowsChecklist() error {
	session, err := s.session()
	if err != nil {
		return err
	}
	if err := session.ChooseOnCard(0, app.OfferDone); err != nil {
		return err
	}
	session.Wait(world.SetupWait)
	if len(session.Checklist()) == 0 {
		return fmt.Errorf("no checklist; setup is %s: %q", session.Status(), session.Output())
	}
	return nil
}

// afterSettled expects the action's settled value right before text, and the page to have
// told the shop it settled.
func (s *state) afterSettled(text string) error {
	session, err := s.session()
	if err != nil {
		return err
	}
	output := session.Output()
	at := slices.Index(output, text)
	if at < 1 || output[at-1] != `{"cart":["tee"]}` {
		return fmt.Errorf("%q doesn't follow the settled cart: %q", text, output)
	}
	if !s.ShopFor().Settled() {
		return errors.New("the shop never heard the promise settle")
	}
	return nil
}

// nothingRan expects no output from setup at all: the check stopped it first.
func (s *state) nothingRan() error {
	session, err := s.session()
	if err != nil {
		return err
	}
	if output := session.Output(); len(output) > 0 {
		return fmt.Errorf("setup printed %q", output)
	}
	return nil
}

func (s *state) installHint(hint string) error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	if err := world.Expect(window.Visible(), hint, "the window"); err != nil {
		return err
	}
	if err := window.Choose(app.OfferCopy); err != nil {
		return err
	}
	copied, err := s.Clipboard.ReadText()
	if err != nil {
		return err
	}
	if strings.TrimSpace(copied) != hint {
		return fmt.Errorf("Copy put %q on the clipboard, not %q", copied, hint)
	}
	return nil
}

func (s *state) dropInReady(name string) error {
	if err := s.DropIn(name); err != nil {
		return err
	}
	if status := s.Session.Status(); status != app.SessionReady {
		return fmt.Errorf("setup ended %s: %q", status, s.Session.Output())
	}
	return nil
}

func (s *state) signedIn(email string) error {
	if err := s.stillOpen(); err != nil {
		return err
	}
	who, err := s.eval(`window.app.whoami()`)
	if err != nil {
		return err
	}
	if who != email {
		return fmt.Errorf("the Givens browser is signed in as %q, not %q", who, email)
	}
	return nil
}

// ownProfile expects Chrome to be on the project's profile, in the scenario's folder, not on
// the developer's own.
func (s *state) ownProfile() error {
	profile, err := s.Browser()
	if err != nil {
		return err
	}
	if !strings.HasPrefix(profile, filepath.Join(s.Dir, "browsers")) {
		return fmt.Errorf("the profile %s isn't Givens's own", profile)
	}
	if _, err := os.Stat(filepath.Join(profile, "DevToolsActivePort")); err != nil {
		return fmt.Errorf("Chrome didn't start on Givens's profile: %w", err)
	}
	return nil
}

func (s *state) droppedInWithBrowser(name string) error {
	if err := s.withPage(name); err != nil {
		return err
	}
	if err := s.dropInReady(name); err != nil {
		return err
	}
	return s.rememberPage()
}

// rememberPage keeps the project's page, so a later step can check it's the same one.
func (s *state) rememberPage() error {
	profile, err := s.Browser()
	if err != nil {
		return err
	}
	if s.page = browser.Page(profile); s.page == "" {
		return errors.New("the Givens browser has no page")
	}
	return nil
}

func (s *state) samePage() error {
	session, err := s.session()
	if err != nil {
		return err
	}
	session.Wait(world.SetupWait)
	if err := s.outputShows("✓ opened " + world.ShopURL + "/"); err != nil {
		return err
	}
	profile, err := s.Browser()
	if err != nil {
		return err
	}
	if now := browser.Page(profile); now != s.page {
		return fmt.Errorf("setup used page %s, not %s", now, s.page)
	}
	return nil
}

func (s *state) onePage() error {
	profile, err := s.Browser()
	if err != nil {
		return err
	}
	pages, err := browser.Pages(profile)
	if err != nil {
		return err
	}
	if len(pages) != 1 {
		return fmt.Errorf("the Givens browser has %d pages", len(pages))
	}
	return nil
}

func (s *state) openOnCard() error {
	if err := s.DropIn("Card declined"); err != nil {
		return err
	}
	opener := &urlOpener{}
	if err := s.App.UseURLOpener(opener); err != nil {
		return err
	}
	s.opener = opener
	return s.Session.ChooseOnCard(0, app.OfferOpen)
}

func (s *state) adminInGivensBrowser() error {
	if err := s.pageIs("/admin"); err != nil {
		return err
	}
	if s.opener != nil && len(s.opener.Opened) > 0 {
		return fmt.Errorf("the default browser opened %q too", s.opener.Opened)
	}
	return nil
}

func (s *state) terminalRan() error {
	if s.ExitCode != 0 {
		return fmt.Errorf("the script exited %d:\n%s", s.ExitCode, s.Output)
	}
	opened := s.ShopFor().URL("✓ opened " + world.ShopURL + "/")
	if !strings.Contains(s.Output, opened) || !strings.Contains(s.Output, "sam@example.com") {
		return fmt.Errorf("the script printed:\n%s", s.Output)
	}
	return s.signedIn("sam@example.com")
}

func (s *state) stillOpen() error {
	profile, err := s.Browser()
	if err != nil {
		return err
	}
	if !browser.Running(profile) {
		return errors.New("the Givens browser isn't open")
	}
	return nil
}

// runWithoutGivens runs Card declined's script with only the system's folders on the PATH.
func (s *state) runWithoutGivens() error {
	script, err := s.ScriptPath("Card declined")
	if err != nil {
		return err
	}
	if _, err := exec.LookPath("/usr/bin/givens"); err == nil {
		return errors.New("/usr/bin has a givens, so it can't be missing")
	}
	run := exec.Command("/bin/bash", script)
	run.Dir = s.Project
	run.Env = append(os.Environ(), "PATH=/usr/bin:/bin")
	output, err := run.CombinedOutput()
	s.Output, s.ExitCode = string(output), run.ProcessState.ExitCode()
	if _, exited := err.(*exec.ExitError); exited {
		return nil
	}
	return err
}

func (s *state) nothingAfter() error {
	if s.ExitCode == 0 || strings.Contains(s.Output, "✓") {
		return fmt.Errorf("the script carried on (exit %d):\n%s", s.ExitCode, s.Output)
	}
	return nil
}

func (s *state) ranInTerminal(name string) error {
	if err := s.withPage(name); err != nil {
		return err
	}
	script, err := s.ScriptPath(name)
	if err != nil {
		return err
	}
	if err := s.Run("bash " + script); err != nil {
		return err
	}
	if s.ExitCode != 0 {
		return fmt.Errorf("the script exited %d:\n%s", s.ExitCode, s.Output)
	}
	return s.rememberPage()
}

// urlOpener stands in for the default browser, keeping the links it was asked to open.
type urlOpener struct {
	Opened []string
}

// OpenURL keeps url.
func (o *urlOpener) OpenURL(url string) error {
	o.Opened = append(o.Opened, url)
	return nil
}
