package dropintoascenario

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"givens/acceptance/world"
	"givens/internal/app"
	"givens/internal/app/model"
	"givens/internal/slug"
)

// Limits. Polling stays short so a scenario fails in seconds, not minutes.
const (
	pollInterval   = 50 * time.Millisecond
	pollsMax       = 100 // 5 seconds: long enough for a setup that's already finished
	quietPollsMax  = 20  // 1 second of watching for something that mustn't appear
	shopDataFolder = "shop-data"
	// slowSetup is one tick past the default timeout, on the fake clock.
	slowSetup = 2*time.Minute + time.Second
)

// Lines test scripts use. Each script makes shop-data/ first, so a marker can always be written.
const (
	makeShopData = `mkdir -p "$GIVENS_ROOT/../shop-data"`
	shopData     = `"$GIVENS_ROOT/../shop-data`
	// waitForRelease blocks setup until a step writes shop-data/release, for at most a minute,
	// so a setup that's never released can't outlive the test run by long.
	waitForRelease = `for _ in $(seq 1 600); do ` +
		`[ -f "$GIVENS_ROOT/../shop-data/release" ] && break; sleep 0.1; done`
	writePID = `echo "$$" > "$GIVENS_ROOT/../shop-data/setup.pid"`
)

// urlOpener is a fake default browser: it records the links it was asked to open.
type urlOpener struct {
	urls []string
}

func (o *urlOpener) OpenURL(url string) error {
	o.urls = append(o.urls, url)
	return nil
}

// script is a setup script that sources the kit, makes shop-data/, then runs lines.
func script(lines ...string) string {
	body := append([]string{world.KitSource, makeShopData}, lines...)
	return strings.Join(body, "\n") + "\n"
}

// scriptFrom is a setup script whose first line after the kit's is fromLine, a `from` line.
func scriptFrom(fromLine string, lines ...string) string {
	body := append([]string{world.KitSource, fromLine, makeShopData}, lines...)
	return strings.Join(body, "\n") + "\n"
}

// quote wraps text in single quotes for bash, so a step's text reaches a kit function intact.
// Ref: https://www.gnu.org/software/bash/manual/bash.html#Single-Quotes
func quote(text string) string {
	return "'" + strings.ReplaceAll(text, "'", `'\''`) + "'"
}

// touch is a script line that writes a marker file in shop-data/.
func touch(name string) string {
	return "touch " + shopData + "/" + name + `"`
}

// eventually polls check until it's true, for at most pollsMax polls.
func eventually(check func() bool) bool {
	for poll := 0; poll < pollsMax; poll++ {
		if check() {
			return true
		}
		time.Sleep(pollInterval)
	}
	return check()
}

// staysFalse watches check for about a second and reports whether it never became true.
func staysFalse(check func() bool) bool {
	for poll := 0; poll < quietPollsMax; poll++ {
		if check() {
			return false
		}
		time.Sleep(pollInterval)
	}
	return !check()
}

// includes reports whether one of lines contains text.
func includes(lines []string, text string) bool {
	return world.Expect(lines, text, "lines") == nil
}

// indexOf is the index of the first line containing text at or after from, or -1.
func indexOf(lines []string, text string, from int) int {
	for index := from; index < len(lines); index++ {
		if strings.Contains(lines[index], text) {
			return index
		}
	}
	return -1
}

// prepare opens the sample shop, unless it's open, and trusts it, so a Drop in runs setup
// without the trust prompt. Scenarios about trust live in capabilities/trust-a-project.
func (s *state) prepare() error {
	if s.Window == nil {
		if err := s.OpenSampleShop(); err != nil {
			return err
		}
	}
	return s.App.MarkTrusted(s.Capabilities())
}

// window is the front window, opening the sample shop if a Given only copied it.
func (s *state) window() (*app.Window, error) {
	return s.Front()
}

// session is the window's current Drop in: the one a Choose may have replaced, or the one
// the last drop in step started.
func (s *state) session() (*app.Session, error) {
	window, err := s.window()
	if err != nil {
		return nil, err
	}
	if current := window.Session(); current != nil {
		return current, nil
	}
	if s.Session != nil {
		return s.Session, nil
	}
	return nil, errors.New("no Drop in is open")
}

// settle waits for the current Drop in to finish or pause, and returns it with its status.
func (s *state) settle() (*app.Session, string, error) {
	session, err := s.session()
	if err != nil {
		return nil, "", err
	}
	return session, session.Wait(world.SetupWait), nil
}

// settleTo waits for the Drop in to finish or pause, and fails unless it ends in want.
func (s *state) settleTo(want string) (*app.Session, error) {
	session, status, err := s.settle()
	if err != nil {
		return nil, err
	}
	if status != want {
		return nil, fmt.Errorf("setup ended %q, not %q; output: %q", status, want,
			session.Output())
	}
	return session, nil
}

// dropInTo drops in to name and waits for setup to finish or pause in want.
func (s *state) dropInTo(name string, want string) (*app.Session, error) {
	if err := s.DropIn(name); err != nil {
		return nil, err
	}
	return s.settleTo(want)
}

// writeSetup replaces the named scenario's setup script.
func (s *state) writeSetup(name string, content string) error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	path, err := s.ScriptPath(name)
	if err != nil {
		return err
	}
	return s.WriteFile(path, content)
}

// teardownPath is the named scenario's teardown script, relative to the sample shop.
func (s *state) teardownPath(name string) (string, error) {
	path, err := s.ScriptPath(name)
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(path, ".sh") + ".teardown.sh", nil
}

// writeTeardown replaces the named scenario's teardown script.
func (s *state) writeTeardown(name string, content string) error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	path, err := s.teardownPath(name)
	if err != nil {
		return err
	}
	return s.WriteFile(path, content)
}

// shopFile reads a marker file a script wrote in shop-data/; ok is false when there's none.
func (s *state) shopFile(name string) (string, bool, error) {
	content, err := s.ReadFile(filepath.Join(shopDataFolder, name))
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	return content, err == nil, err
}

// release lets a setup blocked on waitForRelease carry on.
func (s *state) release() error {
	return s.WriteFile(filepath.Join(shopDataFolder, "release"), "")
}

// runKeyOf is the run key of a scenario with no Examples: capability/scenario-slug.
func (s *state) runKeyOf(name string) (string, error) {
	capability, err := s.ScenarioCapability(name)
	if err != nil {
		return "", err
	}
	return capability + "/" + slug.Of(name), nil
}

// markAll marks every checklist item: Givens and Whens "done", Thens "pass".
func markAll(session *app.Session) error {
	items := session.Checklist()
	if len(items) == 0 {
		return errors.New("the checklist is empty, so there's nothing to mark")
	}
	for _, item := range items {
		mark := "done"
		if item.Kind == "then" {
			mark = "pass"
		}
		if err := session.Mark(item.Text, mark); err != nil {
			return err
		}
	}
	return nil
}

// firstOfKind is the first checklist item of kind, like "then".
func firstOfKind(items []model.CheckItem, kind string) (model.CheckItem, error) {
	for _, item := range items {
		if item.Kind == kind {
			return item, nil
		}
	}
	return model.CheckItem{}, fmt.Errorf("the checklist has no %q item: %+v", kind, items)
}

// markOf is the mark on the checklist item whose text is text.
func markOf(session *app.Session, text string) (string, error) {
	for _, item := range session.Checklist() {
		if item.Text == text {
			return item.Mark, nil
		}
	}
	return "", fmt.Errorf("the checklist has no item %q", text)
}
