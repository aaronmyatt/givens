// Package termedit edits a file in the developer's own editor, as `git commit` and `kubectl
// edit` do: the text of an app.Editor goes into a temporary file, the editor runs on it, and
// what comes back is saved only when it's valid. A .feature file that doesn't parse opens
// again with its problems at the top as "# givens:" lines. See
// capabilities/work-in-the-terminal.
// Ref: https://git-scm.com/docs/git-var#Documentation/git-var.txt-GITEDITOR
// Ref: https://kubernetes.io/docs/reference/kubectl/generated/kubectl_edit/
package termedit

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"givens/internal/app"
	"givens/internal/assert"
)

// Limits. Tiger Style: put a limit on everything.
const (
	// opensMax bounds how often one edit reopens the editor over problems.
	opensMax = 20
	// problemPrefix starts each line about a problem at the top of a reopened file.
	problemPrefix = "# givens: "
)

// Outcome is how an edit ended.
type Outcome int

// Outcomes of an edit.
const (
	// Saved: the editor's text was valid and is in the file.
	Saved Outcome = iota
	// NoChanges: the editor closed with the text as it was.
	NoChanges
	// Abandoned: the editor closed on a reopened file with its problems still there. The
	// edit is kept in Kept.
	Abandoned
	// ChangedOnDisk: another tool saved the file meanwhile, so nothing was written. The edit
	// is kept in Kept.
	ChangedOnDisk
)

// Edit is one file being edited in the developer's editor.
type Edit struct {
	editor *app.Editor
	file   string // the temporary file the editor opens
	shown  string // what the file held when the editor last opened it
	opens  int    // how often the editor has opened
}

// Begin writes the editor's text to a temporary file named after the file, so the editor
// colours it. Ref: https://pkg.go.dev/os#CreateTemp
func Begin(editor *app.Editor) (*Edit, error) {
	assert.That(editor != nil, "an editor holds the file")
	assert.That(filepath.IsAbs(editor.Path()), "the file is absolute")
	file, err := os.CreateTemp("", "givens-*-"+filepath.Base(editor.Path()))
	if err != nil {
		return nil, err
	}
	edit := &Edit{editor: editor, file: file.Name(), shown: editor.Text()}
	_, err = file.WriteString(edit.shown)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	return edit, err
}

// File is the temporary file the editor opens.
func (e *Edit) File() string {
	assert.That(e != nil, "file runs on an edit")
	assert.That(filepath.IsAbs(e.file), "the file is absolute")
	return e.file
}

// Command is the developer's editor on the temporary file: $VISUAL, else $EDITOR, else vi,
// through sh -c so EDITOR="code -w" works. Editors Givens knows open at the scenario's line;
// see lineArgs. The caller runs it with the terminal, or hands it to Bubble Tea's
// ExecProcess.
func (e *Edit) Command() *exec.Cmd {
	assert.That(e != nil, "command runs on an edit")
	assert.That(e.opens < opensMax, "the editor opens a bounded number of times")
	e.opens++
	args := []string{"-c", `${VISUAL:-${EDITOR:-vi}} "$@"`, "sh"}
	line := e.editor.Line()
	if e.opens > 1 {
		line = 0 // problems are at the top when it opens again
	}
	// Ref: https://pubs.opengroup.org/onlinepubs/9799919799/utilities/sh.html
	return exec.Command("sh", append(args, lineArgs(editorName(), line, e.file)...)...)
}

// editorName is the editor command's own name, like "vim" for "/usr/bin/vim -p".
func editorName() string {
	command := os.Getenv("VISUAL")
	if command == "" {
		command = os.Getenv("EDITOR")
	}
	fields := strings.Fields(command)
	assert.That(len(fields) <= len(command), "fields come from the command")
	if len(fields) == 0 {
		return "vi"
	}
	assert.That(len(fields[0]) > 0, "a command has a name")
	return filepath.Base(fields[0])
}

// lineArgs are the arguments that open file at line in the editor named, as each editor
// takes a line: +N for vi and its kin, file:N for Sublime Text, Zed and Helix, -g file:N for
// VS Code and its forks, -l N for TextMate. Any other editor, or line 0, gets the file alone.
// Ref: https://vimhelp.org/starting.txt.html#-+
// Ref: https://www.sublimetext.com/docs/command_line.html
// Ref: https://code.visualstudio.com/docs/configure/command-line
// Ref: https://zed.dev/docs/command-line-interface
// Ref: https://docs.helix-editor.com/usage.html
// Ref: https://macromates.com/manual/en/using_textmate_from_terminal
func lineArgs(name string, line int, file string) []string {
	assert.That(name != "", "an editor has a name")
	assert.That(!strings.Contains(name, "/"), "a name, not a path")
	at := strconv.Itoa(line)
	if line <= 0 {
		return []string{file}
	}
	switch name {
	case "vi", "vim", "nvim", "nano", "emacs", "emacsclient", "micro", "kak", "mg":
		return []string{"+" + at, file}
	case "subl", "sublime_text", "zed", "hx", "helix":
		return []string{file + ":" + at}
	case "code", "code-insiders", "codium", "cursor", "windsurf":
		return []string{"-g", file + ":" + at}
	case "mate":
		return []string{"-l", at, file}
	}
	return []string{file}
}

// Finish reads the file back once the editor closed, and says what happens next: reopen
// (true) when the text has problems, which are now at its top; or the outcome.
func (e *Edit) Finish() (Outcome, bool, error) {
	assert.That(e != nil, "finish runs on an edit")
	assert.That(e.opens > 0, "the editor ran")
	content, err := os.ReadFile(e.file)
	if err != nil {
		return NoChanges, false, err
	}
	text := withoutProblems(string(content))
	if string(content) == e.shown {
		if e.opens > 1 {
			return Abandoned, false, nil
		}
		return NoChanges, false, os.Remove(e.file)
	}
	if err := e.editor.SetText(text); err != nil {
		return NoChanges, false, err
	}
	if problems := e.editor.Problems(); len(problems) > 0 && e.opens < opensMax {
		e.shown = withProblems(text, problems)
		return NoChanges, true, os.WriteFile(e.file, []byte(e.shown), 0o644)
	}
	if e.editor.ChangedOnDisk() {
		return ChangedOnDisk, false, os.WriteFile(e.file, []byte(text), 0o644)
	}
	if err := e.editor.Save(); err != nil {
		return NoChanges, false, err
	}
	return Saved, false, os.Remove(e.file)
}

// withProblems puts each problem above the text as a "# givens:" line.
func withProblems(text string, problems []app.EditorMark) string {
	assert.That(len(problems) > 0, "there are problems to show")
	assert.That(len(problems) <= opensMax*8, "problems stay bounded")
	var lines strings.Builder
	for _, problem := range problems {
		lines.WriteString(fmt.Sprintf("%sline %d: %s\n", problemPrefix, problem.Line,
			problem.Message))
	}
	return lines.String() + text
}

// withoutProblems takes the "# givens:" lines off the top of a file.
func withoutProblems(text string) string {
	assert.That(problemPrefix != "", "problem lines are marked")
	assert.That(len(text) <= app.EditorTextBytesMax*2, "the text stays bounded")
	// bound: each pass takes one line off text.
	for strings.HasPrefix(text, problemPrefix) {
		_, rest, found := strings.Cut(text, "\n")
		if !found {
			return ""
		}
		text = rest
	}
	return text
}

// Run edits in the developer's editor with the terminal as its stdin, stdout and stderr,
// reopening it over problems, and says how it ended.
func Run(editor *app.Editor) (Outcome, *Edit, error) {
	assert.That(editor != nil, "an editor holds the file")
	assert.That(opensMax > 0, "the editor opens at least once")
	edit, err := Begin(editor)
	if err != nil {
		return NoChanges, nil, err
	}
	for range opensMax {
		command := edit.Command()
		command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
		if err := command.Run(); err != nil {
			return NoChanges, edit, fmt.Errorf("the editor didn't finish: %w", err)
		}
		outcome, again, err := edit.Finish()
		if err != nil || !again {
			return outcome, edit, err
		}
	}
	return Abandoned, edit, nil
}

// Template opens text in the developer's editor in a temporary file called name, with the
// terminal, and returns what it holds when the editor closes, with lines starting with "#"
// taken out: a form to fill in, as `git commit` gives. The file is removed after.
func Template(name string, text string) (string, error) {
	assert.That(name != "" && !strings.Contains(name, "/"), "a template has a file name")
	assert.That(len(text) <= app.EditorTextBytesMax, "a template stays bounded")
	file, err := os.CreateTemp("", "givens-*-"+name)
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(file.Name()) }()
	_, err = file.WriteString(text)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return "", err
	}
	command := exec.Command("sh", "-c", `${VISUAL:-${EDITOR:-vi}} "$@"`, "sh", file.Name())
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := command.Run(); err != nil {
		return "", fmt.Errorf("the editor didn't finish: %w", err)
	}
	filled, err := os.ReadFile(file.Name())
	if err != nil {
		return "", err
	}
	kept := make([]string, 0, 8)
	for _, line := range strings.Split(string(filled), "\n") {
		if !strings.HasPrefix(line, "#") {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n"), nil
}
