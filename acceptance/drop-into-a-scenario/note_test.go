package dropintoascenario

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"givens/acceptance/world"
	"givens/internal/runs"
)

// noteSteps cover a run note written in Markdown, kept whole as note.md.
func noteSteps(s *state) []world.Step {
	return []world.Step{
		world.Def(`^they write a note in Markdown, with a heading, a list and the image `+
			`"([^"]*)"$`, s.writeMarkdownNote),
		world.Def(`^the run line keeps the note's heading as its note$`, s.lineKeepsHeading),
		world.Def(`^the whole note is kept as note\.md in the run's folder$`, s.noteKept),
		world.Def(`^the run, in the scenario's run history, shows the whole note$`,
			s.historyShowsNote),
	}
}

// markdownNoteHeading is the heading writeMarkdownNote's note starts with.
const markdownNoteHeading = "Retry button"

func (s *state) writeMarkdownNote(image string) error {
	session, err := s.session()
	if err != nil {
		return err
	}
	s.note = "## " + markdownNoteHeading + "\n\nOn small screens:\n\n- it's below the fold\n" +
		"- the banner covers it\n\n![The declined card](" + image + ")\n"
	return session.SetRunNote(s.note)
}

func (s *state) lineKeepsHeading() error {
	lines, err := s.runLines(s.runKey)
	if err != nil {
		return err
	}
	fields := lines[len(lines)-1]
	if note := fields[len(fields)-1]; note != markdownNoteHeading {
		return fmt.Errorf("the run line's note is %q, not %q", note, markdownNoteHeading)
	}
	return nil
}

// runFolder is the newest run's folder, from its line's key and time.
func (s *state) runFolder() (string, error) {
	lines, err := s.runLines(s.runKey)
	if err != nil {
		return "", err
	}
	at, err := time.Parse(time.RFC3339, lines[len(lines)-1][0])
	if err != nil {
		return "", err
	}
	return runs.FilesDir(s.Capabilities(), s.runKey, at), nil
}

func (s *state) noteKept() error {
	folder, err := s.runFolder()
	if err != nil {
		return err
	}
	content, err := os.ReadFile(filepath.Join(folder, runs.NoteFileName))
	if err != nil {
		return fmt.Errorf("the run's folder has no note.md: %w", err)
	}
	if string(content) != s.note {
		return fmt.Errorf("note.md holds %q, not the note written", content)
	}
	return nil
}

// historyShowsNote selects the scenario: its newest run carries the whole note, and the
// folder its image link resolves against.
func (s *state) historyShowsNote() error {
	if err := s.Window.Select(s.scenario); err != nil {
		return err
	}
	folder, err := s.runFolder()
	if err != nil {
		return err
	}
	for _, window := range s.App.Snapshot().Windows {
		if window.ID != s.Window.ID() || len(window.Runs) == 0 {
			continue
		}
		newest := window.Runs[0]
		if newest.NoteMarkdown != s.note || newest.Folder != folder {
			return fmt.Errorf("the newest run shows note %q in %q", newest.NoteMarkdown,
				newest.Folder)
		}
		return nil
	}
	return fmt.Errorf("the scenario's page lists no runs")
}
