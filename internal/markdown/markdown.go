// Package markdown reads a setup done by hand, scripts/<slug>.md: the text before the first
// numbered item, and each top-level numbered item as one step, with its fenced code blocks,
// inline code, links and images. It reads only what setups use, not all of CommonMark.
// Ref: https://spec.commonmark.org/0.31.2/#ordered-list
// Ref: https://spec.commonmark.org/0.31.2/#fenced-code-blocks
package markdown

import (
	"fmt"
	"regexp"
	"strings"

	"givens/internal/assert"
)

// Limits. Tiger Style: put a limit on everything.
const (
	// TextBytesMax bounds one setup file.
	TextBytesMax = 1 << 20
	// StepsMax bounds the numbered items in one file.
	StepsMax = 256
)

// Patterns, compiled once.
const (
	itemPattern   = `^(\d+)\.\s+(.*)$`
	linkPattern   = `(!?)\[([^\]]*)\]\(([^)\s]*)\)`
	inlinePattern = "`([^`]+)`"
	// A fence's info string: its language, then any other words, like "run".
	// Ref: https://spec.commonmark.org/0.31.2/#info-string
	fencePattern = "^\\s*(```+|~~~+)\\s*([A-Za-z0-9_+-]*)((?:[ \\t]+[^\\s`]+)*)\\s*$"
)

var (
	item   = regexp.MustCompile(itemPattern)   //tiger:allow-global compiled once
	link   = regexp.MustCompile(linkPattern)   //tiger:allow-global compiled once
	inline = regexp.MustCompile(inlinePattern) //tiger:allow-global compiled once
	fence  = regexp.MustCompile(fencePattern)  //tiger:allow-global compiled once
)

// Block is one fenced code block.
type Block struct {
	Language string // the info string's first word: "bash", "js", or ""
	Run      bool   // the info string's next word is "run": Givens runs the block itself
	Code     string // exactly the code: no fences, no list indent
	Closed   bool   // false when the fence never closes
	Line     int    // the opening fence's line, counting from 1
}

// Step is one top-level numbered item.
type Step struct {
	Number int
	Text   string   // the item's markdown, list number taken off, code blocks left out
	Blocks []Block  // its fenced code blocks, in order
	Links  []string // link targets, in order
	Images []string // image paths, in order
	Inline []string // inline code spans, in order
}

// Setup is a whole markdown setup.
type Setup struct {
	Intro string // the text before the first numbered item, trimmed
	Steps []Step
}

// Parse reads a markdown setup.
func Parse(text string) (Setup, error) {
	assert.That(len(text) <= TextBytesMax*2, "the text stays bounded")
	if len(text) > TextBytesMax {
		return Setup{}, fmt.Errorf("a markdown setup is at most %d bytes", TextBytesMax)
	}
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	setup := Setup{Steps: make([]Step, 0, 8)}
	intro := make([]string, 0, 4)
	var current *Step
	var open *Block
	indent := 0
	for index, line := range lines {
		if open != nil {
			open, indent = closeOrAdd(current, open, line, indent)
			continue
		}
		if match := item.FindStringSubmatch(line); match != nil {
			if len(setup.Steps) >= StepsMax {
				return Setup{}, fmt.Errorf("a markdown setup has at most %d steps", StepsMax)
			}
			setup.Steps = append(setup.Steps, Step{Number: len(setup.Steps) + 1,
				Text: match[2]})
			current = &setup.Steps[len(setup.Steps)-1]
			continue
		}
		if current == nil {
			intro = append(intro, line)
			continue
		}
		if match := fence.FindStringSubmatch(line); match != nil {
			words := strings.Fields(match[3])
			current.Blocks = append(current.Blocks, Block{Language: match[2],
				Run: len(words) > 0 && words[0] == "run", Line: index + 1})
			open, indent = &current.Blocks[len(current.Blocks)-1], leadingSpaces(line)
			continue
		}
		current.Text += "\n" + strings.TrimSpace(line)
	}
	setup.Intro = strings.TrimSpace(strings.Join(intro, "\n"))
	for index := range setup.Steps {
		finish(&setup.Steps[index])
	}
	assert.That(len(setup.Steps) <= StepsMax, "steps stay bounded")
	return setup, nil
}

// closeOrAdd ends an open code block at its closing fence, or adds a line to it with the
// fence's indent taken off. It returns the block still open, or nil.
func closeOrAdd(step *Step, open *Block, line string, indent int) (*Block, int) {
	assert.That(step != nil, "a code block is inside a step")
	assert.That(open != nil, "a block is open")
	// A closing fence is three or more backticks or tildes, and nothing else.
	if bare := strings.TrimSpace(line); len(bare) >= 3 && strings.Trim(bare, "`~") == "" {
		open.Closed = true
		return nil, 0
	}
	trimmed := strings.TrimLeft(line, " ")
	if leadingSpaces(line) >= indent {
		trimmed = line[indent:]
	}
	if open.Code == "" {
		open.Code = trimmed
	} else {
		open.Code += "\n" + trimmed
	}
	return open, indent
}

// leadingSpaces counts the spaces a line starts with.
func leadingSpaces(line string) int {
	assert.That(len(line) <= TextBytesMax, "a line stays bounded")
	count := len(line) - len(strings.TrimLeft(line, " "))
	assert.That(count >= 0 && count <= len(line), "spaces are part of the line")
	return count
}

// finish trims a step's text and reads its links, images and inline code.
func finish(step *Step) {
	assert.That(step != nil, "there's a step to finish")
	assert.That(step.Number > 0, "steps count from 1")
	step.Text = strings.TrimSpace(step.Text)
	for index := range step.Blocks {
		step.Blocks[index].Code = strings.TrimRight(step.Blocks[index].Code, "\n")
	}
	for _, match := range link.FindAllStringSubmatch(step.Text, -1) {
		if match[1] == "!" {
			step.Images = append(step.Images, match[3])
		} else {
			step.Links = append(step.Links, match[3])
		}
	}
	for _, match := range inline.FindAllStringSubmatch(step.Text, -1) {
		step.Inline = append(step.Inline, match[1])
	}
}

// Plain is markdown as a terminal shows it: links as their text with the target after it,
// images as "Image: <alt> (<path>)", inline code and bold without their marks.
func Plain(text string) string {
	assert.That(len(text) <= TextBytesMax, "the text stays bounded")
	plain := link.ReplaceAllStringFunc(text, func(found string) string {
		match := link.FindStringSubmatch(found)
		if match[1] == "!" {
			return "Image: " + match[2] + " (" + match[3] + ")"
		}
		return match[2] + " (" + match[3] + ")"
	})
	plain = strings.ReplaceAll(strings.ReplaceAll(plain, "`", ""), "**", "")
	assert.That(!strings.Contains(plain, "`"), "no inline code marks are left")
	return plain
}

// Rendered is markdown as the Drop in page shows it: links as their text (their targets are
// in Step.Links), images left out (they're in Step.Images), and no code or bold marks.
func Rendered(text string) string {
	assert.That(len(text) <= TextBytesMax, "the text stays bounded")
	shown := link.ReplaceAllStringFunc(text, func(found string) string {
		match := link.FindStringSubmatch(found)
		if match[1] == "!" {
			return ""
		}
		return match[2]
	})
	shown = strings.ReplaceAll(strings.ReplaceAll(shown, "`", ""), "**", "")
	assert.That(!strings.Contains(shown, "]("), "no link markup is left")
	return strings.TrimSpace(shown)
}
