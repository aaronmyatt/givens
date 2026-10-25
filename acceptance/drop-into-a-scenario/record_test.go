package dropintoascenario

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"givens/internal/app"
	"givens/internal/runs"
)

const gitTimeout = 20 * time.Second

// quickSetup is a setup that finishes at once, for scenarios about what happens after it.
func quickSetup() string {
	return script(`ok "seeded shopper"`)
}

// droppedInAllPassed drops in to a quick setup and marks every check, Thens "pass".
func (s *state) droppedInAllPassed(name string) error {
	if err := s.writeSetup(name, quickSetup()); err != nil {
		return err
	}
	if err := s.prepare(); err != nil {
		return err
	}
	key, err := s.runKeyOf(name)
	if err != nil {
		return err
	}
	s.runKey, s.scenario = key, name
	session, err := s.dropInTo(name, app.SessionReady)
	if err != nil {
		return err
	}
	return markAll(session)
}

func (s *state) addNote(note string) error {
	session, err := s.session()
	if err != nil {
		return err
	}
	s.note = note
	return session.SetRunNote(note)
}

// todaysRuns is today's run file, by the world's fake clock.
func (s *state) todaysRuns() string {
	return filepath.Join("capabilities", "runs", s.Clock.Now().Format(time.DateOnly)+".log")
}

// runLines returns the tab-separated fields of each line in today's run file for key.
func (s *state) runLines(key string) ([][]string, error) {
	text, err := s.ReadFile(s.todaysRuns())
	if err != nil {
		return nil, fmt.Errorf("no run file for today: %w", err)
	}
	found := make([][]string, 0, 4)
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		fields := strings.Split(line, "\t")
		if hasField(fields, key) {
			found = append(found, fields)
		}
	}
	if len(found) == 0 {
		return nil, fmt.Errorf("%s has no line for %s:\n%s", s.todaysRuns(), key, text)
	}
	return found, nil
}

// hasField reports whether one of fields is exactly want.
func hasField(fields []string, want string) bool {
	for _, field := range fields {
		if strings.TrimSpace(field) == want {
			return true
		}
	}
	return false
}

func (s *state) runLineWithNote(key string) error {
	lines, err := s.runLines(key)
	if err != nil {
		return err
	}
	fields := lines[len(lines)-1]
	if !hasField(fields, "pass") && !hasField(fields, "passed") {
		return fmt.Errorf("the run line for %s doesn't record a pass: %q", key, fields)
	}
	if !hasField(fields, s.note) {
		return fmt.Errorf("the run line for %s doesn't hold the note %q: %q", key, s.note,
			fields)
	}
	return nil
}

// droppedInFirstMarked drops in to a quick setup and marks only the first Then step "pass".
func (s *state) droppedInFirstMarked(name string) error {
	if err := s.writeSetup(name, quickSetup()); err != nil {
		return err
	}
	if err := s.prepare(); err != nil {
		return err
	}
	session, err := s.dropInTo(name, app.SessionReady)
	if err != nil {
		return err
	}
	first, err := firstOfKind(session.Checklist(), "then")
	if err != nil {
		return err
	}
	return session.Mark(first.Text, "pass")
}

func (s *state) nothingInRuns() error {
	runs, err := filepath.Glob(filepath.Join(s.Capabilities(), "runs", "*"))
	if err != nil {
		return err
	}
	for _, path := range runs {
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.TrimSpace(string(content)) != "" {
			return fmt.Errorf("%s was written:\n%s", path, content)
		}
	}
	return nil
}

// git runs git in the sample shop, away from this machine's own git settings.
// Ref: https://git-scm.com/docs/git#Documentation/git.txt-codeGITCONFIGGLOBALcode
func (s *state) git(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, "git", args...)
	command.Dir = s.Project
	command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	output, err := command.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, output)
	}
	return string(output), nil
}

// gitInit makes the sample shop a repository whose user.name is user, with everything
// committed on main.
// Ref: https://git-scm.com/docs/git-init and https://git-scm.com/docs/git-config
func (s *state) gitInit(user string) error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	commands := [][]string{
		{"init", "--quiet", "--initial-branch=main"},
		{"config", "user.name", user},
		{"config", "user.email", strings.ToLower(user) + "@example.com"},
		{"config", "commit.gpgsign", "false"},
		{"add", "--all"},
		{"commit", "--quiet", "--message", "Sample shop"},
	}
	for _, args := range commands {
		if _, err := s.git(args...); err != nil {
			return err
		}
	}
	return nil
}

func (s *state) namedDeveloperAllPassed(user string, name string) error {
	if err := s.writeSetup(name, quickSetup()); err != nil {
		return err
	}
	if err := s.gitInit(user); err != nil {
		return err
	}
	return s.droppedInAllPassed(name)
}

func (s *state) runLineIncludes(text string) error {
	lines, err := s.runLines(s.runKey)
	if err != nil {
		return err
	}
	if fields := lines[len(lines)-1]; !hasField(fields, text) {
		return fmt.Errorf("the run line for %s doesn't include %q: %q", s.runKey, text, fields)
	}
	return nil
}

// attachFile writes a stand-in screenshot outside the project, as a tester's own file would
// be, and attaches it to the Drop in.
func (s *state) attachFile(name string) error {
	session, err := s.session()
	if err != nil {
		return err
	}
	s.attached = filepath.Join(s.Dir, "desktop", name)
	if err := os.MkdirAll(filepath.Dir(s.attached), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(s.attached, []byte("proof of "+name), 0o644); err != nil {
		return err
	}
	return session.Attach(s.attached)
}

// copiedIntoRunFolder finds the run's folder from its line's key and time, and checks it
// holds a copy of the attached file, the original left where it was.
func (s *state) copiedIntoRunFolder(name string) error {
	lines, err := s.runLines(s.runKey)
	if err != nil {
		return err
	}
	at, err := time.Parse(time.RFC3339, lines[len(lines)-1][0])
	if err != nil {
		return err
	}
	copied := filepath.Join(runs.FilesDir(s.Capabilities(), s.runKey, at), name)
	content, err := os.ReadFile(copied)
	if err != nil {
		return fmt.Errorf("%s isn't in the run's folder: %w", name, err)
	}
	if string(content) != "proof of "+name {
		return fmt.Errorf("%s holds %q, not the attached file", copied, content)
	}
	if _, err := os.Stat(s.attached); err != nil {
		return fmt.Errorf("the tester's own %s was moved: %w", name, err)
	}
	return nil
}

// historyListsFile selects the scenario and checks its newest run lists the file.
func (s *state) historyListsFile(name string) error {
	if err := s.Window.Select(s.scenario); err != nil {
		return err
	}
	for _, window := range s.App.Snapshot().Windows {
		if window.ID != s.Window.ID() || len(window.Runs) == 0 {
			continue
		}
		for _, path := range window.Runs[0].Files {
			if filepath.Base(path) == name {
				return nil
			}
		}
		return fmt.Errorf("the newest run lists %q, not %s", window.Runs[0].Files, name)
	}
	return fmt.Errorf("the scenario's page lists no runs")
}
