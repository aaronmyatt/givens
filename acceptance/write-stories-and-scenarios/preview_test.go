package writestoriesandscenarios

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"givens/internal/app"
)

// unsavedEdit is what "a developer is editing" adds to a scenario's last step without saving,
// so a step can tell the editor's text from the file's.
const unsavedEdit = " until the bank replies"

// editing opens a scenario in the editor and changes its last step, unsaved: a developer
// part-way through editing it.
func (s *state) editing(name string) error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	if err := s.open(name); err != nil {
		return err
	}
	lines := strings.Split(s.editor.Text(), "\n")
	last := lastStepIndex(lines, name)
	if last < 0 {
		return fmt.Errorf("the editor doesn't hold %q's steps: %q", name, lines)
	}
	s.changedFrom = strings.TrimSpace(lines[last])
	lines[last] += unsavedEdit
	s.changedTo = strings.TrimSpace(lines[last])
	return s.setText(strings.Join(lines, "\n"))
}

// lastStepIndex finds the last indented line of a scenario's block, or -1.
func lastStepIndex(lines []string, name string) int {
	start := slices.Index(trimmedLines(strings.Join(lines, "\n")), "Scenario: "+name)
	if start < 0 {
		return -1
	}
	last := -1
	for index := start + 1; index < len(lines) && strings.HasPrefix(lines[index], "    "); index++ {
		last = index
	}
	return last
}

// anotherToolSaves changes a different scenario's step in checkout.feature on disk.
func (s *state) anotherToolSaves() error {
	path := featurePath("checkout")
	text, err := s.ReadFile(path)
	if err != nil {
		return err
	}
	from, to := `Then they see "Order placed"`, `Then they see "Thanks for your order"`
	if !strings.Contains(text, from) {
		return fmt.Errorf("checkout.feature has no %q to change", from)
	}
	s.theirs = strings.Replace(text, from, to, 1)
	return s.WriteFile(path, s.theirs)
}

// canChooseThree expects the three choices, and the other tool's change still on disk.
func (s *state) canChooseThree(first string, second string, third string) error {
	window, err := s.window()
	if err != nil {
		return err
	}
	offers := window.Offers()
	for _, action := range []string{first, second, third} {
		if !slices.Contains(offers, action) {
			return fmt.Errorf("the window doesn't offer %q; it offers %q", action, offers)
		}
	}
	onDisk, err := s.ReadFile(featurePath("checkout"))
	if err != nil {
		return err
	}
	if onDisk != s.theirs {
		return errors.New("checkout.feature no longer holds the other tool's change")
	}
	return nil
}

// addLine adds line after the subject scenario's last step in the editor, unsaved.
func (s *state) addLine(line string) error {
	if s.editor == nil {
		return errors.New("nothing is open in the editor")
	}
	lines := strings.Split(s.editor.Text(), "\n")
	last := lastStepIndex(lines, s.subject)
	if last < 0 {
		return fmt.Errorf("the editor doesn't hold %q's steps: %q", s.subject, lines)
	}
	lines = slices.Insert(lines, last+1, "    "+line)
	s.addedLine, s.addedLineNumber = line, last+2
	return s.setText(strings.Join(lines, "\n"))
}

func (s *state) addNonStep() error { return s.addLine("Check the order is pending") }

func (s *state) lineMarked(message string) error {
	if s.editor == nil {
		return errors.New("nothing is open in the editor")
	}
	marks := s.editor.Marks()
	want := app.EditorMark{Line: s.addedLineNumber, Message: message}
	if !slices.Contains(marks, want) {
		return fmt.Errorf("line %d isn't marked %q; the marks are %v", s.addedLineNumber,
			message, marks)
	}
	return nil
}

// previewSteps is the subject's steps as the editor holds them: the file's, with the unsaved
// edit made.
func (s *state) previewSteps() ([]string, error) {
	capabilitySlug, err := s.ScenarioCapability(s.subject)
	if err != nil {
		return nil, err
	}
	steps, _, err := s.scenarioSteps(capabilitySlug)
	if err != nil {
		return nil, err
	}
	want := slices.Clone(steps[s.subject])
	if index := slices.Index(want, s.changedFrom); index >= 0 {
		want[index] = s.changedTo
	}
	return want, nil
}

// previewLinedUp expects the scenario's name, then its steps lined up and unmarked, in order.
func (s *state) previewLinedUp() error {
	if s.editor == nil {
		return errors.New("nothing is open in the editor")
	}
	preview := s.editor.Preview()
	if !slices.ContainsFunc(preview, func(line app.PreviewLine) bool {
		return strings.Contains(line.Text, s.subject)
	}) {
		return fmt.Errorf("the preview doesn't name %q: %v", s.subject, preview)
	}
	steps, err := s.previewSteps()
	if err != nil {
		return err
	}
	from := 0
	for _, want := range linedUp(steps) {
		found := slices.Index(preview[from:], app.PreviewLine{Text: want})
		if found < 0 {
			return fmt.Errorf("the preview doesn't show %q unmarked after line %d: %v",
				want, from, preview)
		}
		from += found + 1
	}
	return nil
}

func (s *state) previewHasChanges() error {
	if s.editor == nil {
		return errors.New("nothing is open in the editor")
	}
	shown := make([]string, 0, 16)
	for _, line := range s.editor.Preview() {
		shown = append(shown, strings.TrimSpace(line.Text))
	}
	if !slices.Contains(shown, s.changedTo) {
		return fmt.Errorf("the preview doesn't show the unsaved %q: %q", s.changedTo, shown)
	}
	if slices.Contains(shown, s.changedFrom) {
		return fmt.Errorf("the preview shows the saved %q", s.changedFrom)
	}
	return nil
}

func (s *state) previewMarked() error {
	if s.editor == nil {
		return errors.New("nothing is open in the editor")
	}
	preview := s.editor.Preview()
	for _, line := range preview {
		if strings.Contains(line.Text, s.addedLine) && line.Mark == notAStep {
			return nil
		}
	}
	return fmt.Errorf("the preview doesn't mark %q with %q: %v", s.addedLine, notAStep, preview)
}

// editingAbout opens Checkout's about.md, giving the sample shop one with its flow diagram
// when it has none.
func (s *state) editingAbout() error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	if _, err := s.aboutImages(); err != nil {
		return err
	}
	window, err := s.window()
	if err != nil {
		return err
	}
	s.editor, err = window.Edit("checkout/about.md")
	return err
}

// aboutImages lists the images about.md shows, writing about.md and any missing image first.
func (s *state) aboutImages() ([]string, error) {
	about := filepath.Join("capabilities", "checkout", "about.md")
	text, err := s.ReadFile(about)
	if errors.Is(err, os.ErrNotExist) {
		text, err = defaultAbout, s.WriteFile(about, defaultAbout)
	}
	if err != nil {
		return nil, err
	}
	images := make([]string, 0, 2)
	for _, match := range regexp.MustCompile(markdownImage).FindAllStringSubmatch(text, -1) {
		images = append(images, match[1])
		image := filepath.Join("capabilities", "checkout", filepath.FromSlash(match[1]))
		_, err := os.Stat(filepath.Join(s.Project, image))
		if errors.Is(err, os.ErrNotExist) {
			err = s.WriteFile(image, flowDiagram)
		}
		if err != nil {
			return nil, err
		}
	}
	if len(images) == 0 {
		return nil, errors.New("checkout/about.md shows no image, so no flow diagram")
	}
	return images, nil
}

// previewAsLibrary expects the preview to be exactly what the library shows as Checkout's
// about.md, with its images.
func (s *state) previewAsLibrary() error {
	if s.editor == nil {
		return errors.New("nothing is open in the editor")
	}
	images, err := s.aboutImages()
	if err != nil {
		return err
	}
	window, err := s.window()
	if err != nil {
		return err
	}
	library, err := window.Capability("Checkout")
	if err != nil {
		return err
	}
	shown := make([]string, 0, 16)
	for _, line := range s.editor.Preview() {
		shown = append(shown, line.Text)
	}
	preview := strings.Join(shown, "\n")
	if library.About == "" || preview != library.About {
		return fmt.Errorf("the preview is %q, not the library's about.md: %q", preview,
			library.About)
	}
	for _, image := range images {
		if !strings.Contains(preview, image) {
			return fmt.Errorf("the preview doesn't show %s: %q", image, preview)
		}
	}
	return nil
}
