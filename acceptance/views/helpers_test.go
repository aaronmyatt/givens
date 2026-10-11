package views

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"givens/internal/app"
)

// helperMarker starts a helper's doc comment: "# @helper seed_shopper [email]", then its
// description on the comment lines below, then the function.
const helperMarker = "# @helper "

// helperLinesMax bounds the lines read from one helper file, so a runaway file fails loudly.
const helperLinesMax = 10000

// diskHelper is a documented helper as its file defines it, read without Givens.
type diskHelper struct {
	app.ViewHelper
}

// projectHelpers reads every documented helper in capabilities/_shared and each capability's
// _shared folder, and counts the setup scripts that call each one.
func (s *state) projectHelpers() ([]diskHelper, error) {
	root := s.Capabilities()
	files, err := filepath.Glob(filepath.Join(root, "_shared", "*.sh"))
	if err != nil {
		return nil, err
	}
	own, err := filepath.Glob(filepath.Join(root, "*", "_shared", "*.sh"))
	if err != nil {
		return nil, err
	}
	helpers := make([]diskHelper, 0, 16)
	for _, file := range append(files, own...) {
		found, err := helpersIn(root, file)
		if err != nil {
			return nil, err
		}
		helpers = append(helpers, found...)
	}
	if len(helpers) == 0 {
		return nil, errors.New("the sample shop has no documented helpers")
	}
	for index := range helpers {
		callers, err := s.callers(helpers[index].Name)
		if err != nil {
			return nil, err
		}
		helpers[index].Callers = callers
	}
	return helpers, nil
}

// helpersIn parses one helper file's "# @helper" blocks.
func helpersIn(root string, file string) ([]diskHelper, error) {
	source, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(root, file)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(source), "\n")
	if len(lines) > helperLinesMax {
		return nil, fmt.Errorf("%s has more than %d lines", rel, helperLinesMax)
	}
	helpers := make([]diskHelper, 0, 4)
	for index, line := range lines {
		if !strings.HasPrefix(line, helperMarker) {
			continue
		}
		signature := strings.TrimSpace(strings.TrimPrefix(line, helperMarker))
		description := make([]string, 0, 4)
		for _, below := range lines[index+1:] {
			if !strings.HasPrefix(below, "#") {
				break
			}
			description = append(description, strings.TrimSpace(strings.TrimPrefix(below, "#")))
		}
		helpers = append(helpers, diskHelper{app.ViewHelper{
			Name: strings.Fields(signature)[0], Signature: signature,
			Description: strings.Join(description, " "), File: filepath.ToSlash(rel),
		}})
	}
	return helpers, nil
}

// callers counts the setup scripts with a line, other than a comment, calling name.
func (s *state) callers(name string) (int, error) {
	scripts, err := filepath.Glob(filepath.Join(s.Capabilities(), "*", "scripts", "*.sh"))
	if err != nil {
		return 0, err
	}
	// \b keeps seed_shopper from matching seed_shopper_admin. Ref: https://pkg.go.dev/regexp/syntax
	call := regexp.MustCompile(`(?m)^[^#\n]*\b` + regexp.QuoteMeta(name) + `\b`)
	count := 0
	for _, script := range scripts {
		source, err := os.ReadFile(script)
		if err != nil {
			return 0, err
		}
		if call.Match(source) {
			count++
		}
	}
	return count, nil
}

// shownHelpers pairs each helper on disk with the one the Shared view lists for it.
func (s *state) shownHelpers() ([][2]app.ViewHelper, error) {
	onDisk, err := s.projectHelpers()
	if err != nil {
		return nil, err
	}
	window, err := s.Front()
	if err != nil {
		return nil, err
	}
	shown := window.ViewHelpers()
	if len(shown) != len(onDisk) {
		return nil, fmt.Errorf("Shared lists %d helpers, not the %d documented: %v",
			len(shown), len(onDisk), shown)
	}
	pairs := make([][2]app.ViewHelper, 0, len(onDisk))
	for _, want := range onDisk {
		got, err := findHelper(shown, want.Name, want.File)
		if err != nil {
			return nil, err
		}
		pairs = append(pairs, [2]app.ViewHelper{want.ViewHelper, got})
	}
	return pairs, nil
}

func findHelper(shown []app.ViewHelper, name string, file string) (app.ViewHelper, error) {
	for _, helper := range shown {
		if helper.Name == name && helper.File == file {
			return helper, nil
		}
	}
	return app.ViewHelper{}, fmt.Errorf("Shared doesn't list %s from %s; it lists %v",
		name, file, shown)
}

func (s *state) helpersWithSignatures() error {
	pairs, err := s.shownHelpers()
	if err != nil {
		return err
	}
	for _, pair := range pairs {
		want, got := pair[0], pair[1]
		if got.Signature != want.Signature {
			return fmt.Errorf("%s shows the signature %q, not %q", want.Name, got.Signature,
				want.Signature)
		}
		if got.Description != want.Description {
			return fmt.Errorf("%s shows the description %q, not %q", want.Name,
				got.Description, want.Description)
		}
	}
	return nil
}

func (s *state) helpersWithFilesAndCallers() error {
	pairs, err := s.shownHelpers()
	if err != nil {
		return err
	}
	for _, pair := range pairs {
		want, got := pair[0], pair[1]
		if got.Callers != want.Callers {
			return fmt.Errorf("%s (%s) shows %d calling scripts, not %d", want.Name, want.File,
				got.Callers, want.Callers)
		}
	}
	return nil
}

// openAllHappyPaths copies the sample shop, takes @error off every scenario and Feature, and
// opens it, so no capability has an error state.
func (s *state) openAllHappyPaths() error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	features, err := filepath.Glob(filepath.Join(s.Capabilities(), "*", "*.feature"))
	if err != nil {
		return err
	}
	for _, feature := range features {
		if err := removeErrorTags(feature); err != nil {
			return err
		}
	}
	onDisk, err := s.projectScenarios()
	if err != nil {
		return err
	}
	for _, scenario := range onDisk {
		if scenario.Error {
			return fmt.Errorf("%q is still an error state", scenario.Name)
		}
	}
	return s.OpenSampleShop()
}

// removeErrorTags drops @error from each tag line, and a tag line left with no tags.
func removeErrorTags(feature string) error {
	source, err := os.ReadFile(feature)
	if err != nil {
		return err
	}
	lines := strings.Split(string(source), "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "@") {
			kept = append(kept, line)
			continue
		}
		tags := make([]string, 0, 4)
		for _, tag := range strings.Fields(trimmed) {
			if tag != "@error" {
				tags = append(tags, tag)
			}
		}
		if len(tags) > 0 {
			indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			kept = append(kept, indent+strings.Join(tags, " "))
		}
	}
	return os.WriteFile(feature, []byte(strings.Join(kept, "\n")), 0o644)
}
