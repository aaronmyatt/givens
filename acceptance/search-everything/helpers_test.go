package searcheverything

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"givens/acceptance/world"
	"givens/internal/app"
)

// helperDoc is a helper as its `# @helper` comment documents it, read straight from the shop.
type helperDoc struct {
	signature   string // the text after "# @helper": "seed_shopper [email]"
	description string // the comment lines under it, joined with spaces
}

// helperListed expects the helper under Helpers, with its documented description beside it.
func (s *state) helperListed(name string) error {
	doc, err := s.readHelperDoc(name)
	if err != nil {
		return err
	}
	if err := s.listedUnder(name, app.SearchHelpers); err != nil {
		return err
	}
	if s.selected.Detail != doc.description {
		return fmt.Errorf("%s is listed with %q, not its description %q", name,
			s.selected.Detail, doc.description)
	}
	return nil
}

// helperDetail selects the helper found earlier and expects its signature and each script
// that calls it, by its path in the capabilities folder.
func (s *state) helperDetail(callers int) error {
	if s.selected.Name == "" {
		return fmt.Errorf("no earlier step found a helper to select")
	}
	doc, err := s.readHelperDoc(s.selected.Name)
	if err != nil {
		return err
	}
	scripts, err := s.scriptsCalling(s.selected.Name)
	if err != nil {
		return err
	}
	if len(scripts) != callers {
		return fmt.Errorf("the sample shop has %d scripts calling %s, not %d: %q",
			len(scripts), s.selected.Name, callers, scripts)
	}
	window, err := s.Front()
	if err != nil {
		return err
	}
	if err := window.Select(s.selected.Name); err != nil {
		return err
	}
	for _, want := range append([]string{doc.signature}, scripts...) {
		if err := world.Expect(window.Visible(), want, "the window"); err != nil {
			return err
		}
	}
	return nil
}

// readHelperDoc finds `# @helper <name>` in the shop's _shared folders and reads the comment
// lines under it, up to the function itself.
func (s *state) readHelperDoc(name string) (helperDoc, error) {
	files, err := s.sharedFiles()
	if err != nil {
		return helperDoc{}, err
	}
	for _, file := range files {
		text, err := os.ReadFile(file)
		if err != nil {
			return helperDoc{}, err
		}
		lines := strings.Split(string(text), "\n")
		for index, line := range lines {
			signature, found := strings.CutPrefix(line, "# @helper ")
			if found && strings.SplitN(signature, " ", 2)[0] == name {
				return helperDoc{signature, commentUnder(lines[index+1:])}, nil
			}
		}
	}
	return helperDoc{}, fmt.Errorf("the sample shop documents no helper %s", name)
}

// commentUnder joins the comment lines at the start of lines, without their "# ".
func commentUnder(lines []string) string {
	parts := make([]string, 0, 4)
	for _, line := range lines {
		text, found := strings.CutPrefix(line, "# ")
		if !found {
			break
		}
		parts = append(parts, text)
	}
	return strings.Join(parts, " ")
}

func (s *state) sharedFiles() ([]string, error) {
	shared, err := filepath.Glob(filepath.Join(s.Capabilities(), "_shared", "*.sh"))
	if err != nil {
		return nil, err
	}
	own, err := filepath.Glob(filepath.Join(s.Capabilities(), "*", "_shared", "*.sh"))
	if err != nil {
		return nil, err
	}
	return append(shared, own...), nil
}

// scriptsCalling lists the setup scripts whose text names the helper as a word, relative to
// the capabilities folder: "checkout/scripts/card-declined.sh".
func (s *state) scriptsCalling(name string) ([]string, error) {
	scripts, err := filepath.Glob(filepath.Join(s.Capabilities(), "*", "scripts", "*.sh"))
	if err != nil {
		return nil, err
	}
	call := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\b`)
	callers := make([]string, 0, len(scripts))
	for _, script := range scripts {
		text, err := os.ReadFile(script)
		if err != nil {
			return nil, err
		}
		if !call.Match(text) {
			continue
		}
		rel, err := filepath.Rel(s.Capabilities(), script)
		if err != nil {
			return nil, err
		}
		callers = append(callers, rel)
	}
	return callers, nil
}
