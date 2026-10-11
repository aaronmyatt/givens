package app

import (
	"strings"
	"unicode/utf8"

	"givens/internal/assert"
)

// Helper is one shell function documented with a `# @helper` comment in a `_shared/` folder,
// as the sidebar's "Shared" section lists it. See capabilities/share-setup-between-scenarios.
type Helper struct {
	Name        string   // "seed_shopper"
	Signature   string   // "seed_shopper [email]", from the `# @helper` line
	Description string   // the comment lines under the `# @helper` line
	Capability  string   // the capability whose _shared folder holds it, or "" for _shared
	DefinedIn   string   // relative to the capabilities folder: "_shared/users.sh"
	Line        int      // the line of its `name() {`, counting from 1
	Callers     []string // setup scripts that call it, relative to the capabilities folder
	Overrides   string   // "Overrides _shared/users.sh" when it shadows a shared helper, or ""
	Warning     string   // why it's flagged, such as a second definition at the same level
}

// RequiredCommand is one command a scenario's setup needs, found from the `need` lines in its
// script, the setups it builds on and the helpers it calls.
type RequiredCommand struct {
	Command   string // "bb"
	Hint      string // the install hint from the need line: "brew install borkdude/brew/babashka"
	Installed bool   // whether the command is on PATH
}

// HelperProblem is one line the editor marks in a helper file, from `bash -n`.
// Ref: https://www.gnu.org/software/bash/manual/bash.html#The-Set-Builtin (-n reads without
// running)
type HelperProblem struct {
	Line    int    // as bash reports it, counting from 1
	Message string // bash's own words, such as "syntax error: unexpected end of file"
}

// HelperEdit is what the window shows while a helper file is open in its editor.
type HelperEdit struct {
	Path     string          // the helper file, absolute
	Line     int             // the line the editor opened at: the helper's `name() {` line
	Callers  []string        // scripts listed beside the editor, relative to capabilities
	Problems []HelperProblem // lines marked with a syntax error since the last save
}

// Helpers is the "Shared" section of the sidebar: every documented helper in the project's
// _shared folders, in file order. Needed by every scenario in
// capabilities/share-setup-between-scenarios that checks the Shared list ("Two helpers with the
// same name", "Two shared files define the same helper", "Add a helper").
func (w *Window) Helpers() []Helper {
	assert.That(w != nil, "helpers runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	return nil
}

// RequiredCommands lists the commands a scenario's setup needs, whether each is installed, and
// its install hint. Needed by "See what a scenario needs installed".
func (w *Window) RequiredCommands(scenario string) ([]RequiredCommand, error) {
	assert.That(w != nil, "required commands runs on a window")
	assert.That(len(scenario) <= NameLengthMax, "the name fits NameLengthMax")
	return nil, ErrNotImplemented
}

// EditingHelper is the helper file open in the editor, where it opened, the scripts that call
// it and the syntax errors marked in it. Its Path is "" when no helper is open. Needed by
// "Edit a helper" and "Save a helper with a syntax error".
func (w *Window) EditingHelper() HelperEdit {
	assert.That(w != nil, "editing helper runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	return HelperEdit{}
}

// NewHelper creates <capability>/_shared/<name>.sh holding a `# @helper` comment and an empty
// function called name, and opens it in the editor. capability is a capability's name, such as
// "Checkout". Needed by "Add a helper".
func (w *Window) NewHelper(capability string, name string) error {
	assert.That(w != nil, "new helper runs on a window")
	assert.That(capability != "", "a helper belongs to a capability")
	assert.That(name != "", "a helper has a name")
	assert.That(!strings.ContainsAny(name, "/ \n"), "a helper's name is a shell function name")
	return ErrNotImplemented
}

// SaveHelper saves text as the helper file open in the editor, even when bash can't parse it,
// then marks the lines `bash -n` reports. Needed by "Save a helper with a syntax error".
func (w *Window) SaveHelper(text string) error {
	assert.That(w != nil, "save helper runs on a window")
	assert.That(utf8.ValidString(text), "a helper file is text")
	return ErrNotImplemented
}
