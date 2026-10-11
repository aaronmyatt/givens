package dropintoascenario

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"givens/acceptance/world"
	"givens/internal/app"
)

// Lines the test setups print through the kit's ok, which prefixes "✓ ".
const (
	firstLine    = "✓ seeded shopper"
	secondLine   = "✓ card set to decline"
	stillRunning = "Still running"
)

// printsThenWaits gives Card declined a setup that prints one line, then waits for a step to
// release it before printing the next, so a step can see output arrive while setup runs.
func (s *state) printsThenWaits() error {
	err := s.writeSetup("Card declined", script(
		`ok "seeded shopper"`, waitForRelease, `ok "card set to decline"`))
	if err != nil {
		return err
	}
	return s.prepare()
}

// outputStreams expects the first line while setup is still running, then the rest.
func (s *state) outputStreams() error {
	session, err := s.session()
	if err != nil {
		return err
	}
	if !eventually(func() bool { return includes(session.Output(), firstLine) }) {
		return fmt.Errorf("the first line never appeared while setup ran; output: %q",
			session.Output())
	}
	if status := session.Status(); status != app.SessionRunning {
		return fmt.Errorf("setup is %q while it waits, not %q", status, app.SessionRunning)
	}
	if includes(session.Output(), secondLine) {
		return errors.New("the second line appeared before setup carried on")
	}
	if err := s.release(); err != nil {
		return err
	}
	if _, err := s.settleTo(app.SessionReady); err != nil {
		return err
	}
	output := session.Output()
	if indexOf(output, firstLine, 0) > indexOf(output, secondLine, 0) {
		return fmt.Errorf("the output is out of order: %q", output)
	}
	for _, line := range output {
		if strings.Contains(line, "\n") {
			return fmt.Errorf("an output entry holds more than one line: %q", line)
		}
	}
	return world.Expect(output, secondLine, "the setup's output")
}

// checklistShown expects the checklist to be any manual Givens, then the scenario's When
// steps, then its Then steps.
func (s *state) checklistShown() error {
	session, err := s.settleTo(app.SessionReady)
	if err != nil {
		return err
	}
	whens, thens, err := s.expectedChecks("Card declined")
	if err != nil {
		return err
	}
	items := session.Checklist()
	order := map[string]int{"given": 0, "when": 1, "then": 2}
	last := 0
	for _, item := range items {
		rank, known := order[item.Kind]
		if !known || rank < last {
			return fmt.Errorf("the checklist isn't Givens, then Whens, then Thens: %+v", items)
		}
		last = rank
	}
	return expectKinds(items, map[string][]string{"when": whens, "then": thens})
}

// expectKinds checks that the items of each kind are exactly the wanted steps, in order.
func expectKinds(items []app.CheckItem, wanted map[string][]string) error {
	for kind, steps := range wanted {
		got := make([]string, 0, len(items))
		for _, item := range items {
			if item.Kind == kind {
				got = append(got, item.Text)
			}
		}
		if len(got) != len(steps) {
			return fmt.Errorf("the checklist has %d %s items, not %d: %q", len(got), kind,
				len(steps), got)
		}
		for index, step := range steps {
			if !strings.Contains(got[index], step) {
				return fmt.Errorf("%s item %d is %q, not %q", kind, index+1, got[index], step)
			}
		}
	}
	return nil
}

// thenStepsMarkable marks each Then step "pass", then "fail", and expects a When step to
// refuse both.
func (s *state) thenStepsMarkable() error {
	session, err := s.session()
	if err != nil {
		return err
	}
	when, err := firstOfKind(session.Checklist(), "when")
	if err != nil {
		return err
	}
	if session.Mark(when.Text, "pass") == nil {
		return fmt.Errorf("the When step %q was marked \"pass\"", when.Text)
	}
	for _, item := range session.Checklist() {
		if item.Kind != "then" {
			continue
		}
		for _, mark := range []string{"pass", "fail"} {
			if err := session.Mark(item.Text, mark); err != nil {
				return fmt.Errorf("marking %q %q: %w", item.Text, mark, err)
			}
			if got, err := markOf(session, item.Text); err != nil || got != mark {
				return fmt.Errorf("%q shows mark %q after marking it %q (%v)", item.Text, got,
					mark, err)
			}
		}
	}
	return nil
}

// setupStopsPartWay gives Card declined a setup that exits 3 after two lines.
func (s *state) setupStopsPartWay() error {
	err := s.writeSetup("Card declined", script(
		`ok "seeded shopper"`, `ok "cart filled"`, "exit 3", `ok "card set to decline"`))
	if err != nil {
		return err
	}
	return s.prepare()
}

func (s *state) outputUpToStop() error {
	session, err := s.settleTo(app.SessionFailed)
	if err != nil {
		return err
	}
	output := session.Output()
	for _, line := range []string{firstLine, "✓ cart filled"} {
		if err := world.Expect(output, line, "the output"); err != nil {
			return err
		}
	}
	return world.ExpectNot(output, secondLine, "the output")
}

func (s *state) noChecklist() error {
	session, err := s.settleTo(app.SessionFailed)
	if err != nil {
		return err
	}
	if items := session.Checklist(); len(items) > 0 {
		return fmt.Errorf("a checklist is shown: %+v", items)
	}
	return nil
}

// needsShopctl gives Card declined a setup whose first line leaves a marker, then needs
// shopctl. Givens reads need lines before running anything, so the marker must never appear.
func (s *state) needsShopctl() error {
	// LookPath searches PATH like a shell. Ref: https://pkg.go.dev/os/exec#LookPath
	if path, err := exec.LookPath("shopctl"); err == nil {
		return fmt.Errorf("shopctl is installed at %s, so this scenario can't be set up", path)
	}
	s.markers = append(s.markers, "ran-card-declined")
	err := s.writeSetup("Card declined", script(touch("ran-card-declined"),
		`need shopctl "brew install shop/tap/shopctl"`, `ok "seeded shopper"`))
	if err != nil {
		return err
	}
	return s.prepare()
}

// nothingRuns expects setup to have stopped before running a line: no kit output, and no
// marker a script line would have written.
func (s *state) nothingRuns() error {
	session, err := s.settleTo(app.SessionFailed)
	if err != nil {
		return err
	}
	for _, line := range session.Output() {
		if strings.HasPrefix(line, "✓") || strings.HasPrefix(line, "✗") {
			return fmt.Errorf("the script ran: its output has %q", line)
		}
	}
	for _, marker := range s.markers {
		_, found, err := s.shopFile(marker)
		if err != nil {
			return err
		}
		if found {
			return fmt.Errorf("the script ran: it wrote shop-data/%s", marker)
		}
	}
	return nil
}

func (s *state) installHint(hint string) error {
	window, err := s.window()
	if err != nil {
		return err
	}
	if err := world.Expect(window.Visible(), hint, "the window"); err != nil {
		return err
	}
	if err := window.Choose("Copy"); err != nil {
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

// setupTakesTooLong gives Card declined a setup that blocks until released, writing its
// process id so a step can check "Stop" ended it.
func (s *state) setupTakesTooLong() error {
	err := s.writeSetup("Card declined", script(
		writePID, `ok "started"`, waitForRelease, `ok "finished"`))
	if err != nil {
		return err
	}
	return s.prepare()
}

func (s *state) untagTimeout(name string) error {
	return s.setTimeoutTag(name, "")
}

func (s *state) afterTwoMinutes(text string) error {
	window, err := s.window()
	if err != nil {
		return err
	}
	if includes(window.Visible(), stillRunning) {
		return errors.New("\"Still running\" shows before two minutes have passed")
	}
	s.Clock.Advance(slowSetup)
	if !eventually(func() bool { return includes(window.Visible(), text) }) {
		return fmt.Errorf("two minutes on, the window doesn't show %q; it has %q", text,
			window.Visible())
	}
	return nil
}

// stopKeepsOutput chooses "Stop", then expects the script's process gone and its output kept.
func (s *state) stopKeepsOutput() error {
	window, err := s.window()
	if err != nil {
		return err
	}
	if err := window.Choose("Stop"); err != nil {
		return err
	}
	session, err := s.settleTo(app.SessionStopped)
	if err != nil {
		return err
	}
	pidText, found, err := s.shopFile("setup.pid")
	if err != nil || !found {
		return fmt.Errorf("the setup never wrote its process id (%v)", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(pidText))
	if err != nil {
		return err
	}
	// Signal 0 checks a process exists without signalling it. Ref: https://pkg.go.dev/syscall#Kill
	if !eventually(func() bool { return syscall.Kill(pid, 0) != nil }) {
		return fmt.Errorf("the setup script (process %d) is still running after Stop", pid)
	}
	if err := world.Expect(session.Output(), "✓ started", "the output after Stop"); err != nil {
		return err
	}
	return world.ExpectNot(session.Output(), "✓ finished", "the output after Stop")
}

// tagTimeout sets the scenario's @timeout tag and gives it a setup that blocks until released.
func (s *state) tagTimeout(name string, tag string) error {
	if err := s.setTimeoutTag(name, tag); err != nil {
		return err
	}
	s.scenario = name
	return s.setupTakesTooLong()
}

func (s *state) setupTakesMinutes(minutes string) error {
	count, err := strconv.Atoi(minutes)
	if err != nil {
		return err
	}
	s.slowFor = time.Duration(count) * time.Minute
	return nil
}

// noStillRunning moves the clock on by however long setup takes, watches for "Still running",
// then lets a blocked setup finish.
func (s *state) noStillRunning() error {
	window, err := s.window()
	if err != nil {
		return err
	}
	if s.slowFor > 0 {
		s.Clock.Advance(s.slowFor)
	}
	if !staysFalse(func() bool { return includes(window.Visible(), stillRunning) }) {
		s.sawSlow = true
	}
	if s.sawSlow {
		return fmt.Errorf("the window shows %q: %q", stillRunning, window.Visible())
	}
	return s.release()
}

func (s *state) finishesAsNormal() error {
	if err := s.release(); err != nil {
		return err
	}
	session, err := s.settleTo(app.SessionReady)
	if err != nil {
		return err
	}
	if len(session.Checklist()) == 0 {
		return errors.New("setup finished but no checklist is shown")
	}
	return world.Expect(session.Output(), "✓ finished", "the output")
}

// usualTimeout expects no "Still running" one minute in, then the message just past two.
func (s *state) usualTimeout() error {
	window, err := s.window()
	if err != nil {
		return err
	}
	s.Clock.Advance(time.Minute)
	if !staysFalse(func() bool { return includes(window.Visible(), stillRunning) }) {
		return errors.New("\"Still running\" showed after one minute")
	}
	s.Clock.Advance(slowSetup - time.Minute)
	want := "Still running after 2 minutes"
	if !eventually(func() bool { return includes(window.Visible(), want) }) {
		return fmt.Errorf("two minutes on, the window doesn't show %q: %q", want,
			window.Visible())
	}
	if err := s.release(); err != nil {
		return err
	}
	_, err = s.settleTo(app.SessionReady)
	return err
}

func (s *state) markedWith(name string, text string) error {
	window, err := s.window()
	if err != nil {
		return err
	}
	scenario, err := window.Scenario(name)
	if err != nil {
		return err
	}
	return world.Expect([]string{scenario.Warning}, text, name+"'s warning")
}
