package sharesetupbetweenscenarios

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"givens/internal/app"
	"givens/internal/app/model"
)

// sharedFiles lists the sample shop's helper files, relative to its capabilities folder: the
// project's _shared/*.sh, then each capability's, leaving out the kit.
func (s *state) sharedFiles() ([]string, error) {
	files := make([]string, 0, 16)
	for _, pattern := range []string{"_shared/*.sh", "*/_shared/*.sh"} {
		matches, err := filepath.Glob(filepath.Join(s.Capabilities(), pattern))
		if err != nil {
			return nil, err
		}
		for _, match := range matches {
			if filepath.Base(match) != "givens.sh" {
				files = append(files, strings.TrimPrefix(match, s.Capabilities()+"/"))
			}
		}
	}
	return files, nil
}

// definitionPattern matches the line `name() {` that starts a helper.
func definitionPattern(helper string) *regexp.Regexp {
	return regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(helper) + `\(\)\s*\{\s*$`)
}

// definersOf lists the helper files that define helper.
func (s *state) definersOf(helper string) ([]string, error) {
	files, err := s.sharedFiles()
	if err != nil {
		return nil, err
	}
	definers := make([]string, 0, 2)
	for _, file := range files {
		text, err := s.ReadFile(filepath.Join("capabilities", file))
		if err != nil {
			return nil, err
		}
		if definitionPattern(helper).MatchString(text) {
			definers = append(definers, file)
		}
	}
	return definers, nil
}

// helperBody returns the lines between `helper() {` and its closing `}`, and the file defining
// it relative to the sample shop. Exactly one file must define it.
func (s *state) helperBody(helper string) (string, string, error) {
	definers, err := s.definersOf(helper)
	if err != nil {
		return "", "", err
	}
	if len(definers) != 1 {
		return "", "", fmt.Errorf("expected one file to define %s, got %q", helper, definers)
	}
	path := filepath.Join("capabilities", definers[0])
	text, err := s.ReadFile(path)
	if err != nil {
		return "", "", err
	}
	lines := strings.Split(text, "\n")
	start := slices.IndexFunc(lines, definitionPattern(helper).MatchString)
	end := slices.Index(lines[start:], "}")
	if end < 0 {
		return "", "", fmt.Errorf("%s in %s has no closing brace", helper, path)
	}
	return strings.Join(lines[start+1:start+end], "\n"), path, nil
}

// definitionLine is the line, counting from 1, of `helper() {` in file, relative to the
// capabilities folder.
func (s *state) definitionLine(file string, helper string) (int, error) {
	text, err := s.ReadFile(filepath.Join("capabilities", file))
	if err != nil {
		return 0, err
	}
	at := slices.IndexFunc(strings.Split(text, "\n"), definitionPattern(helper).MatchString)
	if at < 0 {
		return 0, fmt.Errorf("expected %s to define %s, got:\n%s", file, helper, text)
	}
	return at + 1, nil
}

// callersOf lists, sorted, the setup scripts that call helper, relative to the capabilities
// folder.
func (s *state) callersOf(helper string) ([]string, error) {
	scripts, err := filepath.Glob(filepath.Join(s.Capabilities(), "*", "scripts", "*.sh"))
	if err != nil {
		return nil, err
	}
	callers := make([]string, 0, len(scripts))
	for _, script := range scripts {
		rel := strings.TrimPrefix(script, s.Capabilities()+"/")
		text, err := s.ReadFile(filepath.Join("capabilities", rel))
		if err != nil {
			return nil, err
		}
		if callsWord(text, helper) {
			callers = append(callers, rel)
		}
	}
	slices.Sort(callers)
	return callers, nil
}

// callsWord reports whether a line of shell, outside a comment, uses word as a whole word.
// Ref: https://pkg.go.dev/regexp/syntax (\b is an ASCII word boundary; _ is a word character)
func callsWord(text string, word string) bool {
	return regexp.MustCompile(`(?m)^[^#\n]*\b` + regexp.QuoteMeta(word) + `\b`).MatchString(text)
}

// withoutClosingBrace removes the `}` that closes helper's definition.
func withoutClosingBrace(text string, helper string) (string, error) {
	lines := strings.Split(text, "\n")
	start := slices.IndexFunc(lines, definitionPattern(helper).MatchString)
	if start < 0 {
		return "", fmt.Errorf("the file open in the editor doesn't define %s", helper)
	}
	end := slices.Index(lines[start:], "}")
	if end < 0 {
		return "", fmt.Errorf("%s has no closing brace to remove", helper)
	}
	return strings.Join(slices.Delete(lines, start+end, start+end+1), "\n"), nil
}

func (s *state) scenario(name string) (app.Scenario, error) {
	window, err := s.Front()
	if err != nil {
		return app.Scenario{}, err
	}
	return window.Scenario(name)
}

// helpersNamed lists the Shared list's entries for a helper; it's an error when there's none.
func (s *state) helpersNamed(name string) ([]model.Helper, error) {
	window, err := s.Front()
	if err != nil {
		return nil, err
	}
	all := window.Helpers()
	named := make([]model.Helper, 0, 2)
	for _, helper := range all {
		if helper.Name == name {
			named = append(named, helper)
		}
	}
	if len(named) == 0 {
		return nil, fmt.Errorf("expected %s in the Shared list, got %+v", name, all)
	}
	return named, nil
}

func firstLine(text string) string {
	first, _, _ := strings.Cut(text, "\n")
	return first
}
