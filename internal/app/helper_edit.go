package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"givens/internal/assert"
	"givens/internal/helpers"
)

// Limits on checking a helper file.
const (
	// syntaxCheckTimeout bounds one `bash -n`, which only reads the file.
	syntaxCheckTimeout = 5 * time.Second
	// helperProblemsMax bounds the lines marked in one file.
	helperProblemsMax = 64
	// OfferEdit opens the selected helper in the editor.
	OfferEdit = "Edit"
)

// bashError matches one of bash's syntax reports: "x.sh: line 9: syntax error: …".
const bashErrorPattern = `line (\d+): (.+)`

var bashError = regexp.MustCompile(bashErrorPattern) //tiger:allow-global compiled once

// selectHelper selects a helper in the Shared list, if one has that name.
func (w *Window) selectHelper(name string) bool {
	assert.That(w != nil, "select helper runs on a window")
	assert.That(name != "", "a helper has a name")
	helper, ok := w.findHelper(name)
	if !ok {
		return false
	}
	w.selected, w.selectedCapability, w.selectedScenario = name, "", ""
	w.selectedHelper = helper.DefinedIn + ":" + helper.Name
	return true
}

// editHelper opens the selected helper's file in the editor, at its `name() {` line.
func (w *Window) editHelper() error {
	assert.That(w != nil, "edit helper runs on a window")
	assert.That(w.selectedHelper != "", "a helper is selected")
	helper, ok := w.findHelper(w.selectedHelper)
	if !ok {
		return fmt.Errorf("%s: %w", w.selectedHelper, ErrNotFound)
	}
	editor, err := w.Edit(helper.DefinedIn)
	if err != nil {
		return err
	}
	editor.line = helper.Line
	return nil
}

// EditingHelper is the helper file open in the editor, where it opened, the scripts that call
// its helpers and the syntax errors marked in it. Its Path is "" when no helper is open.
// Needed by "Edit a helper" and "Save a helper with a syntax error".
func (w *Window) EditingHelper() HelperEdit {
	assert.That(w != nil, "editing helper runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	if w.editor == nil || filepath.Base(filepath.Dir(w.editor.path)) != sharedFolder {
		return HelperEdit{Callers: []string{}, Problems: []HelperProblem{}}
	}
	relative, err := filepath.Rel(w.path, w.editor.path)
	if err != nil {
		return HelperEdit{Callers: []string{}, Problems: []HelperProblem{}}
	}
	callers := make([]string, 0, 8)
	for _, helper := range w.Helpers() {
		if helper.DefinedIn == filepath.ToSlash(relative) &&
			(w.editor.line == 0 || helper.Line == w.editor.line) {
			callers = append(callers, helper.Callers...)
		}
	}
	return HelperEdit{
		Path: w.editor.path, Line: w.editor.line, Callers: callers,
		Problems: append([]HelperProblem{}, w.editor.helperProblems...),
	}
}

// NewHelper creates <capability>/_shared/<name>.sh holding a `# @helper` comment and an empty
// function called name, and opens it in the editor. capability is a capability's name, such as
// "Checkout". Needed by "Add a helper".
func (w *Window) NewHelper(capability string, name string) error {
	assert.That(w != nil, "new helper runs on a window")
	assert.That(capability != "", "a helper belongs to a capability")
	assert.That(!strings.ContainsAny(name, "/ \n"), "a helper's name is a shell function name")
	if err := w.writable(); err != nil {
		return err
	}
	if !regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`).MatchString(name) {
		return fmt.Errorf("%q isn't a shell function name", name)
	}
	owner, ok := w.findCapability(capability)
	if !ok {
		return fmt.Errorf("%s: %w", capability, ErrNotFound)
	}
	path := filepath.Join(owner.Dir, sharedFolder, name+".sh")
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists", filepath.Base(path))
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	text := "# shellcheck shell=bash\n\n# @helper " + name + "\n" +
		"# What " + name + " sets up, for the scripts that call it.\n" +
		name + "() {\n  :\n}\n"
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return err
	}
	if err := w.reload(); err != nil {
		return err
	}
	return w.openHelperFile(path, name)
}

// openHelperFile opens a helper file in the editor at name's definition.
func (w *Window) openHelperFile(path string, name string) error {
	assert.That(filepath.IsAbs(path), "the helper file is absolute")
	assert.That(name != "", "a helper has a name")
	relative, err := filepath.Rel(w.path, path)
	if err != nil {
		return err
	}
	editor, err := w.Edit(relative)
	if err != nil {
		return err
	}
	for index, line := range strings.Split(editor.text, "\n") {
		if strings.HasPrefix(line, name+"()") {
			editor.line = index + 1
		}
	}
	return nil
}

// SaveHelper saves text as the helper file open in the editor, even when bash can't parse it,
// then marks the lines `bash -n` reports. Needed by "Save a helper with a syntax error".
// Ref: https://www.gnu.org/software/bash/manual/bash.html#The-Set-Builtin (-n reads only)
func (w *Window) SaveHelper(text string) error {
	assert.That(w != nil, "save helper runs on a window")
	assert.That(utf8.ValidString(text), "a helper file is text")
	if err := w.writable(); err != nil {
		return err
	}
	if w.editor == nil || filepath.Base(filepath.Dir(w.editor.path)) != sharedFolder {
		return errors.New("no helper file is open in the editor")
	}
	if len(text) > helpers.FileSizeBytesMax {
		return fmt.Errorf("a helper file is at most %d bytes", helpers.FileSizeBytesMax)
	}
	if err := os.WriteFile(w.editor.path, []byte(text), 0o644); err != nil {
		return err
	}
	w.editor.text, w.editor.opened = text, text
	problems, err := syntaxProblems(w.editor.path)
	if err != nil {
		return err
	}
	w.editor.helperProblems = problems
	return w.reload()
}

// syntaxProblems runs `bash -n` on a file and reads the lines it reports.
func syntaxProblems(path string) ([]HelperProblem, error) {
	assert.That(filepath.IsAbs(path), "the file is absolute")
	assert.That(syntaxCheckTimeout > 0, "the check has a limit")
	ctx, cancel := context.WithTimeout(context.Background(), syntaxCheckTimeout)
	defer cancel()
	output, err := exec.CommandContext(ctx, "bash", "-n", path).CombinedOutput()
	var exit *exec.ExitError
	if err != nil && !errors.As(err, &exit) {
		return nil, err
	}
	problems := make([]HelperProblem, 0, 2)
	for _, match := range bashError.FindAllStringSubmatch(string(output), helperProblemsMax) {
		line, convertErr := strconv.Atoi(match[1])
		if convertErr == nil {
			problems = append(problems, HelperProblem{Line: line, Message: match[2]})
		}
	}
	return problems, nil
}
