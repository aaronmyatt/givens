package readacapability

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"givens/acceptance/world"
	"givens/internal/app"
	"givens/internal/slug"
)

// missingImageAbout is the about.md "A story image that isn't on disk" writes: a paragraph on
// each side of an image whose file is gone, so the placeholder's place can be checked.
const (
	textBeforeImage   = "Checkout takes the shopper from their cart to a placed order."
	textAfterImage    = "A declined card leaves the order pending."
	missingImageAbout = "# How checkout works\n\n" + textBeforeImage + "\n\n" +
		"![Checkout flow](images/flow.svg)\n\n" + textAfterImage + "\n"
)

// markdownImage matches a markdown image and captures its path.
// Ref: https://spec.commonmark.org/0.31.2/#images
const markdownImage = `!\[[^\]]*\]\(([^)\s]+)\)`

// stepsLinedUp checks the scenario's steps against its file, and that the window shows them
// with their keywords lined up, in order.
func (s *state) stepsLinedUp() error {
	window, shown, onDisk, err := s.shownScenario()
	if err != nil {
		return err
	}
	if !slices.Equal(shown.Steps, onDisk.Steps) {
		return fmt.Errorf("%q shows the steps %q, not %q", onDisk.Name, shown.Steps, onDisk.Steps)
	}
	lines := visibleLines(window)
	from := 0
	for _, want := range linedUp(onDisk.Steps) {
		found := slices.Index(lines[from:], want)
		if found < 0 {
			return fmt.Errorf("the window doesn't show %q after line %d; it shows %q",
				want, from, lines)
		}
		from += found + 1
	}
	return nil
}

func (s *state) tagsAboveSteps(first string, second string) error {
	window, shown, onDisk, err := s.shownScenario()
	if err != nil {
		return err
	}
	lines := visibleLines(window)
	firstStep := slices.Index(lines, linedUp(onDisk.Steps)[0])
	if firstStep < 0 {
		return fmt.Errorf("the window doesn't show %q's steps: %q", onDisk.Name, lines)
	}
	for _, tag := range []string{first, second} {
		if !slices.Contains(onDisk.Tags, tag) {
			return fmt.Errorf("checkout.feature doesn't tag %q with %s", onDisk.Name, tag)
		}
		if !slices.Contains(shown.Tags, tag) {
			return fmt.Errorf("%q has the tags %q, without %s", onDisk.Name, shown.Tags, tag)
		}
		above := slices.IndexFunc(lines[:firstStep], func(line string) bool {
			return slices.Contains(strings.Fields(line), tag)
		})
		if above < 0 {
			return fmt.Errorf("%s isn't shown above %q's steps: %q", tag, onDisk.Name, lines)
		}
	}
	return nil
}

func (s *state) setupShown() error {
	_, shown, onDisk, err := s.shownScenario()
	if err != nil {
		return err
	}
	script := filepath.Join("capabilities", "checkout", "scripts", slug.Of(onDisk.Name)+".sh")
	want, err := s.ReadFile(script)
	if err != nil {
		return err
	}
	if shown.Setup != want {
		return fmt.Errorf("%q shows the setup %q, not %s: %q", onDisk.Name, shown.Setup,
			script, want)
	}
	return nil
}

// noSetup removes every setup file a Checkout scenario has.
func (s *state) noSetup(name string) error {
	s.scenario = name
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	if _, err := s.checkoutScenario(name); err != nil {
		return err
	}
	scripts := filepath.Join(s.Capabilities(), "checkout", "scripts")
	for _, extension := range []string{".sh", ".md", ".teardown.sh"} {
		err := os.Remove(filepath.Join(scripts, slug.Of(name)+extension))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func (s *state) noSetupShown(text string) error {
	window, shown, onDisk, err := s.shownScenario()
	if err != nil {
		return err
	}
	if shown.Setup != "" {
		return fmt.Errorf("%q has no setup files, yet shows the setup %q", onDisk.Name, shown.Setup)
	}
	return world.Expect(visibleLines(window), text, "the window")
}

// storyOnDisk is the As a, I want and So that lines of checkout.feature's description.
func (s *state) storyOnDisk() ([]string, error) {
	text, err := s.ReadFile(filepath.Join("capabilities", "checkout", "checkout.feature"))
	if err != nil {
		return nil, err
	}
	story := make([]string, 0, 3)
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		for _, prefix := range []string{"As a", "I want", "So that"} {
			if strings.HasPrefix(trimmed, prefix+" ") {
				story = append(story, trimmed)
			}
		}
	}
	if len(story) != 3 {
		return nil, fmt.Errorf("checkout.feature's story is %q", story)
	}
	return story, nil
}

// aboutBelowStory checks that about.md's text and images follow the story, and that each
// image it shows is on disk, so the diagram is shown rather than a placeholder.
func (s *state) aboutBelowStory() error {
	about, err := s.ReadFile(filepath.Join("capabilities", "checkout", "about.md"))
	if err != nil {
		return fmt.Errorf("the sample shop needs checkout/about.md with its flow diagram: %w", err)
	}
	images := regexp.MustCompile(markdownImage).FindAllStringSubmatch(about, -1)
	if len(images) == 0 {
		return errors.New("checkout/about.md shows no image, so no flow diagram")
	}
	window, capability, err := s.selectedCapability()
	if err != nil {
		return err
	}
	for _, image := range images {
		path := filepath.Join(s.Capabilities(), "checkout", filepath.FromSlash(image[1]))
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("checkout/about.md's image %s: %w", image[1], err)
		}
		if !strings.Contains(capability.About, image[1]) {
			return fmt.Errorf("the about.md shown doesn't show %s: %q", image[1], capability.About)
		}
	}
	return scenariosThenText(visibleLines(window), capability, plainLine(about))
}

// scenariosThenText expects text below the capability's last scenario, so a long about.md
// doesn't push the scenarios off the screen.
func scenariosThenText(lines []string, capability app.Capability, text string) error {
	if len(capability.Scenarios) == 0 {
		return fmt.Errorf("%s lists no scenarios", capability.Name)
	}
	last := capability.Scenarios[len(capability.Scenarios)-1].Name
	at := indexContaining(lines, last, 0)
	if at < 0 {
		return fmt.Errorf("the window doesn't show %q: %q", last, lines)
	}
	if text != "" && indexContaining(lines, text, at+1) < 0 {
		return fmt.Errorf("the window doesn't show %q below the scenarios: %q", text, lines)
	}
	return nil
}

// plainLine is about.md's first line of text without markdown marks, which reads the same
// once rendered; "" when there's none.
func plainLine(markdown string) string {
	for _, line := range strings.Split(markdown, "\n") {
		text := strings.TrimSpace(strings.TrimLeft(line, "#"))
		if text != "" && !strings.ContainsAny(text, "*_`[]!<>|") {
			return text
		}
	}
	return ""
}

func storyThenText(lines []string, story []string, text string) error {
	if len(story) != 3 {
		return fmt.Errorf("the story shown is %q", story)
	}
	last := indexContaining(lines, story[2], 0)
	if last < 0 {
		return fmt.Errorf("the window doesn't show %q: %q", story[2], lines)
	}
	if text != "" && indexContaining(lines, text, last+1) < 0 {
		return fmt.Errorf("the window doesn't show %q below the story: %q", text, lines)
	}
	return nil
}

func (s *state) aboutImageMissing() error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	if err := s.WriteFile("capabilities/checkout/about.md", missingImageAbout); err != nil {
		return err
	}
	err := os.Remove(filepath.Join(s.Capabilities(), "checkout", "images", "flow.svg"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	s.textBeforeImage, s.textAfterImage = textBeforeImage, textAfterImage
	return nil
}

func (s *state) restOfStoryShown() error {
	window, capability, err := s.selectedCapability()
	if err != nil {
		return err
	}
	story, err := s.storyOnDisk()
	if err != nil {
		return err
	}
	if !slices.Equal(capability.Story, story) {
		return fmt.Errorf("%s's story is %q, not %q", capability.Name, capability.Story, story)
	}
	for _, text := range []string{s.textBeforeImage, s.textAfterImage} {
		if !strings.Contains(capability.About, text) {
			return fmt.Errorf("the about.md shown lacks %q: %q", text, capability.About)
		}
	}
	lines := visibleLines(window)
	if err := storyThenText(lines, story, s.textBeforeImage); err != nil {
		return err
	}
	return world.Expect(lines, s.textAfterImage, "the window")
}

// placeholderShown looks for a line naming the image, other than its markdown, between the
// paragraphs written on either side of it.
func (s *state) placeholderShown(path string) error {
	window, err := s.Front()
	if err != nil {
		return err
	}
	lines := visibleLines(window)
	before := indexContaining(lines, s.textBeforeImage, 0)
	if before < 0 {
		return fmt.Errorf("the window doesn't show %q: %q", s.textBeforeImage, lines)
	}
	placeholder := slices.IndexFunc(lines[before+1:], func(line string) bool {
		markup := strings.Contains(line, "](") || strings.Contains(line, "<img")
		return strings.Contains(line, path) && !markup
	})
	if placeholder < 0 {
		return fmt.Errorf("no placeholder names %s after %q: %q", path, s.textBeforeImage, lines)
	}
	if indexContaining(lines, s.textAfterImage, before+1+placeholder+1) < 0 {
		return fmt.Errorf("the placeholder for %s isn't followed by %q: %q", path,
			s.textAfterImage, lines)
	}
	return nil
}
