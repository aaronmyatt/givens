package usegivensfromtheterminal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// skillPath is where Claude Code looks for a project's skill named givens.
// Ref: https://code.claude.com/docs/en/skills
const skillPath = ".claude/skills/givens/SKILL.md"

// ownParagraph is the sample shop's CLAUDE.md before givens skill runs: a project's own notes,
// which the Givens paragraph goes after. Ref: https://code.claude.com/docs/en/memory
const ownParagraph = "# Sample shop\n\nRun npm test before pushing.\n"

// noSkill removes any givens skill from the copied shop, so the command has to write it.
func (s *state) noSkill() error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	return os.RemoveAll(filepath.Join(s.Project, filepath.Dir(skillPath)))
}

// skillWritten expects an Agent Skill: front matter between "---" lines with the name and a
// description, then a body that points to what the scenario's comment says it tells an agent.
// Ref: https://agentskills.io/specification
func (s *state) skillWritten(name string) error {
	if s.ExitCode != 0 {
		return fmt.Errorf("givens skill exited %d:\n%s", s.ExitCode, s.Output)
	}
	text, err := s.ReadFile(skillPath)
	if err != nil {
		return fmt.Errorf("givens skill didn't write %s: %w", skillPath, err)
	}
	front, body, ok := strings.Cut(strings.TrimPrefix(text, "---\n"), "\n---\n")
	if !strings.HasPrefix(text, "---\n") || !ok {
		return fmt.Errorf("%s doesn't start with front matter:\n%s", skillPath, text)
	}
	if !hasField(front, "name", name) || !hasField(front, "description", "") {
		return fmt.Errorf("%s's front matter lacks name %q or a description:\n%s",
			skillPath, name, front)
	}
	mentions := []string{"capabilities/GIVENS.md", "givens list", "givens run", "givens check",
		"givens todo", "givens done"}
	for _, mention := range mentions {
		if !strings.Contains(body, mention) {
			return fmt.Errorf("%s doesn't mention %q:\n%s", skillPath, mention, body)
		}
	}
	return nil
}

// hasField says whether front has a "key: value" line; an empty want takes any value but none.
func hasField(front, key, want string) bool {
	for _, line := range strings.Split(front, "\n") {
		value, ok := strings.CutPrefix(line, key+":")
		value = strings.TrimSpace(value)
		if ok && value != "" && (want == "" || value == want) {
			return true
		}
	}
	return false
}

// claudeHasOwn gives the copied shop a CLAUDE.md of its own, which givens skill must keep.
func (s *state) claudeHasOwn() error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.Project, "CLAUDE.md"), []byte(ownParagraph), 0o644)
}

// claudeHasParagraph expects CLAUDE.md to start with ownParagraph, unchanged, then a heading
// line, then a paragraph that points an agent to the skill.
func (s *state) claudeHasParagraph(heading string) error {
	text, err := s.ReadFile("CLAUDE.md")
	if err != nil {
		return err
	}
	added, ok := strings.CutPrefix(text, ownParagraph)
	if !ok {
		return fmt.Errorf("CLAUDE.md doesn't start with its own paragraph any more:\n%s", text)
	}
	if !strings.HasPrefix(strings.TrimLeft(added, "\n"), heading+"\n") {
		return fmt.Errorf("CLAUDE.md has no %q line after its own paragraph:\n%s", heading, text)
	}
	if !strings.Contains(added, skillPath) {
		return fmt.Errorf("CLAUDE.md's %s paragraph doesn't point to %s:\n%s", heading,
			skillPath, added)
	}
	return nil
}

// againChangesNothing runs command a second time and expects it to succeed and leave the
// skill and CLAUDE.md as the first run wrote them.
func (s *state) againChangesNothing(command string) error {
	files := []string{skillPath, "CLAUDE.md"}
	before := make([]string, 0, len(files))
	for _, file := range files {
		text, err := s.ReadFile(file)
		if err != nil {
			return err
		}
		before = append(before, text)
	}
	if err := s.Run(command); err != nil {
		return err
	}
	if s.ExitCode != 0 {
		return fmt.Errorf("%s exited %d the second time:\n%s", command, s.ExitCode, s.Output)
	}
	for i, file := range files {
		after, err := s.ReadFile(file)
		if err != nil {
			return err
		}
		if after != before[i] {
			return fmt.Errorf("%s changed %s the second time", command, file)
		}
	}
	return nil
}
