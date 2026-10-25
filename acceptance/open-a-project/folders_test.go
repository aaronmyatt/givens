package openaproject

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"givens/internal/app"
	"givens/internal/slug"
	"givens/internal/starter"
)

// Polling for "within two seconds": a fixed number of looks, a fixed pause apart.
const (
	pollAttempts = 20
	pollInterval = 100 * time.Millisecond
)

// giftCardScenario is the scenario another tool adds to checkout.feature.
const giftCardScenario = "Pay with a gift card"

func (s *state) openRoot() error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	return s.open(s.Project)
}

func (s *state) foundCapabilities() error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	if window.Kind() != app.WindowProject {
		return fmt.Errorf("the window is a %q window, not a project window", window.Kind())
	}
	if !samePath(window.Path(), s.Capabilities()) {
		return fmt.Errorf("the window shows %q, not %q", window.Path(), s.Capabilities())
	}
	return nil
}

func (s *state) titleIsPath() error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	if !containsPath(window.Title(), s.Capabilities()) {
		return fmt.Errorf("the window's title is %q; it doesn't show %q",
			window.Title(), s.Capabilities())
	}
	top, _, _ := strings.Cut(screenOf(window), "\n")
	if !containsPath(top, s.Capabilities()) {
		return fmt.Errorf("the TUI's top line is %q; it doesn't show %q", top, s.Capabilities())
	}
	return nil
}

// bareProject makes a project folder holding code but no capabilities folder, and notes its
// files so a later step can tell whether anything was written.
func (s *state) bareProject() error {
	s.bare = filepath.Join(s.Dir, "plain-project")
	files := map[string]string{
		"README.md":   "# Plain project\n",
		"src/main.go": "package main\n\nfunc main() {}\n",
	}
	for rel, text := range files {
		path := filepath.Join(s.bare, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			return err
		}
	}
	before, err := filesUnder(s.bare)
	s.before = before
	return err
}

func (s *state) openBare() error {
	if s.bare == "" {
		return errors.New("no step made a project without a capabilities folder")
	}
	return s.open(s.bare)
}

func (s *state) bareProjectOpen() error {
	if err := s.bareProject(); err != nil {
		return err
	}
	return s.openBare()
}

func (s *state) nothingWritten() error {
	after, err := filesUnder(s.bare)
	if err != nil {
		return err
	}
	if !slices.Equal(after, s.before) {
		return fmt.Errorf("the project held %q and now holds %q", s.before, after)
	}
	return nil
}

// filesUnder lists every file and folder under root with its size, sorted, so two listings
// differ when anything is added, removed or rewritten to a new size.
func filesUnder(root string) ([]string, error) {
	listing := make([]string, 0, 16)
	// WalkDir visits the tree in lexical order. Ref: https://pkg.go.dev/path/filepath#WalkDir
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		listing = append(listing, fmt.Sprintf("%s %d", rel, info.Size()))
		return err
	})
	return listing, err
}

// gotCapabilitiesFolder checks GIVENS.md and the kit, whose first line names its version
// (see capabilities/update-the-kit). When the shipped kit exists it must be copied unchanged.
func (s *state) gotCapabilitiesFolder() error {
	capabilities := filepath.Join(s.bare, "capabilities")
	guide, err := os.ReadFile(filepath.Join(capabilities, "GIVENS.md"))
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(guide)) == 0 {
		return errors.New("capabilities/GIVENS.md is empty")
	}
	kit, err := os.ReadFile(filepath.Join(capabilities, "_shared", "givens.sh"))
	if err != nil {
		return err
	}
	if !strings.HasPrefix(string(kit), app.KitVersionPrefix) {
		return fmt.Errorf("_shared/givens.sh doesn't start with %q", app.KitVersionPrefix)
	}
	shipped, err := os.ReadFile(filepath.Join(s.Root, "internal", "kit", "givens.sh"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !bytes.Equal(kit, shipped) {
		return errors.New("_shared/givens.sh differs from the kit in internal/kit/givens.sh")
	}
	return nil
}

// gotStarter checks every embedded starter file was written unchanged, and that each of the
// starter's scenarios has a setup script.
func (s *state) gotStarter(name string) error {
	capabilities := filepath.Join(s.bare, "capabilities")
	files := starter.Files()
	err := fs.WalkDir(files, "add-a-story", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		want, err := fs.ReadFile(files, path)
		if err != nil {
			return err
		}
		got, err := os.ReadFile(filepath.Join(capabilities, path))
		if err != nil {
			return err
		}
		if !bytes.Equal(got, want) {
			return fmt.Errorf("capabilities/%s differs from the starter's", path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if _, err := featureNamed(capabilities, name); err != nil {
		return err
	}
	return setupsFor(filepath.Join(capabilities, "add-a-story"))
}

// setupsFor expects scripts/<slug>.sh or scripts/<slug>.md for each scenario in a capability.
func setupsFor(dir string) error {
	scenarios, err := app.ScenariosIn(filepath.Join(dir, filepath.Base(dir)+".feature"))
	if err != nil {
		return err
	}
	for _, scenario := range scenarios {
		base := filepath.Join(dir, "scripts", slug.Of(scenario.Name))
		_, shell := os.Stat(base + ".sh")
		_, markdown := os.Stat(base + ".md")
		if shell != nil && markdown != nil {
			return fmt.Errorf("%q has no setup: neither %s.sh nor %s.md", scenario.Name, base, base)
		}
	}
	return nil
}

// gotAttributes expects a line like "runs/** merge=union" in capabilities/.gitattributes.
// Ref: https://git-scm.com/docs/gitattributes#_built_in_merge_drivers
func (s *state) gotAttributes() error {
	path := filepath.Join(s.bare, "capabilities", ".gitattributes")
	text, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(text), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 1 && strings.HasPrefix(fields[0], "runs/") &&
			slices.Contains(fields[1:], "merge=union") {
			return nil
		}
	}
	return fmt.Errorf(".gitattributes doesn't mark runs/ as merge=union; it holds %q", text)
}

// otherToolAdds appends a scenario to checkout.feature straight on disk, as an editor or an
// agent would, without telling Givens.
func (s *state) otherToolAdds() error {
	return s.EnsureScenario("checkout", giftCardScenario)
}

// appearsWithin polls the window for a bounded time; Givens has no copy of its own, so the
// new scenario must show up without the window being reopened.
func (s *state) appearsWithin(name string, capabilityName string) error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	seen := []string{}
	for attempt := 0; attempt < pollAttempts; attempt++ {
		shown, err := window.Capability(capabilityName)
		if err != nil {
			return err
		}
		seen = seen[:0]
		for _, scenario := range shown.Scenarios {
			seen = append(seen, scenario.Name)
		}
		if slices.Contains(seen, name) {
			return nil
		}
		time.Sleep(pollInterval)
	}
	return fmt.Errorf("after %s %s lists %q, without %q",
		pollAttempts*pollInterval, capabilityName, seen, name)
}

func (s *state) staysSelected(name string) error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	if got := window.Selected(); got != name {
		return fmt.Errorf("%q is selected, not %q", got, name)
	}
	return nil
}

// samePath compares two paths after resolving symbolic links, since macOS's temporary folder
// /var is a link to /private/var. Ref: https://pkg.go.dev/path/filepath#EvalSymlinks
func samePath(a string, b string) bool {
	return resolved(a) == resolved(b)
}

func resolved(path string) string {
	target, err := filepath.EvalSymlinks(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return target
}

// containsPath is true when text shows path as given or with its links resolved.
func containsPath(text string, path string) bool {
	return strings.Contains(text, path) || strings.Contains(text, resolved(path))
}

// expectWindows checks the app has exactly want project windows, showing those folders.
func expectWindows(core *app.App, want []string) error {
	windows := core.Windows()
	if len(windows) != len(want) {
		return fmt.Errorf("%d windows are open, not %d", len(windows), len(want))
	}
	for _, path := range want {
		found := false
		for _, window := range windows {
			found = found || (window.Kind() == app.WindowProject && samePath(window.Path(), path))
		}
		if !found {
			return fmt.Errorf("no project window shows %s", path)
		}
	}
	return nil
}
