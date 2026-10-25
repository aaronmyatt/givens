package model

// Helper is one shell function documented with a `# @helper` comment in a `_shared/` folder,
// as the sidebar's "Shared" section lists it. See capabilities/share-setup-between-scenarios.
type Helper struct {
	Name        string `json:"name"`        // "seed_shopper"
	Signature   string `json:"signature"`   // "seed_shopper [email]", from the `# @helper` line
	Description string `json:"description"` // the comment lines under the `# @helper` line
	// The capability whose _shared folder holds it, or "" for _shared.
	Capability string `json:"capability"`
	// Relative to the capabilities folder: "_shared/users.sh".
	DefinedIn string `json:"definedIn"`
	Line      int    `json:"line"` // the line of its `name() {`, counting from 1
	// Setup scripts that call it, relative to the capabilities folder.
	Callers []string `json:"callers"`
	// "Overrides _shared/users.sh" when it shadows a shared helper, or "".
	Overrides string `json:"overrides"`
	// Why it's flagged, such as a second definition at the same level.
	Warning string `json:"warning"`
	// The function as written, from its `name() {` line to its closing brace.
	Code string `json:"code"`
}

// RequiredCommand is one command a scenario's setup needs, found from the `need` lines in its
// script, the setups it builds on and the helpers it calls.
type RequiredCommand struct {
	Command string `json:"command"` // "bb"
	// The install hint from the need line: "brew install borkdude/brew/babashka".
	Hint      string `json:"hint"`
	Installed bool   `json:"installed"` // whether the command is on PATH
}

// HelperProblem is one line the editor marks in a helper file, from `bash -n`.
// Ref: https://www.gnu.org/software/bash/manual/bash.html#The-Set-Builtin (-n reads without
// running)
type HelperProblem struct {
	Line int `json:"line"` // as bash reports it, counting from 1
	// Bash's own words, such as "syntax error: unexpected end of file".
	Message string `json:"message"`
}

// HelperEdit is what the window shows while a helper file is open in its editor.
type HelperEdit struct {
	Path string `json:"path"` // the helper file, absolute
	// The line the editor opened at: the helper's `name() {` line.
	Line int `json:"line"`
	// Scripts listed beside the editor, relative to capabilities.
	Callers []string `json:"callers"`
	// Lines marked with a syntax error since the last save.
	Problems []HelperProblem `json:"problems"`
}
