// Package helpers reads the shell functions in a capabilities folder's _shared folders: where
// each is defined, its `# @helper` documentation, which helpers a script calls (and the helpers
// those call), and the `need` and `from` lines that setup depends on.
//
// It reads bash by pattern, not by parsing it: a helper is a line `name() {` at the start of a
// line, ending at the next line that is just `}`. That's the shape GIVENS.md asks for, and what
// `bash -n` can't tell Givens without running anything.
// Ref: https://www.gnu.org/software/bash/manual/html_node/Shell-Functions.html
package helpers

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"givens/internal/assert"
)

// Limits. Tiger Style: put a limit on everything.
// Ref: https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md#safety
const (
	// FileSizeBytesMax bounds one helper file or setup script.
	FileSizeBytesMax = 1 << 20
	// HelpersMax bounds the functions one capability can see.
	HelpersMax = 4096
	// KitFile is the kit, which isn't a helper file.
	KitFile = "givens.sh"
)

// Patterns, compiled once.
var (
	definition = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)\s*\(\)\s*\{`) //tiger:allow-global
	word       = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)                //tiger:allow-global
	helperDoc  = regexp.MustCompile(`^#\s*@helper\s+(\S+.*)$`)               //tiger:allow-global
)

// Helper is one shell function in a _shared folder.
type Helper struct {
	Name        string
	Signature   string // the `# @helper` line's text, like "seed_shopper [email]"; "" if none
	Description string // the comment lines under `# @helper`, joined with spaces
	Capability  string // the capability slug whose _shared holds it, or "" for _shared
	File        string // relative to the capabilities folder: "_shared/users.sh"
	Line        int    // the `name() {` line, counting from 1
	Body        string // the lines between `name() {` and its closing `}`
}

// Documented is true for a helper with a `# @helper` comment, which the Shared list shows.
func (h Helper) Documented() bool {
	assert.That(h.Name != "", "a helper has a name")
	assert.That(h.File != "", "a helper has a file")
	return h.Signature != ""
}

// InFolder reads every function in root/<folder>/_shared/*.sh but the kit, in file name order
// then line order. folder is "" for root/_shared. capability names what Helper.Capability says.
func InFolder(root string, folder string, capability string) ([]Helper, error) {
	assert.That(filepath.IsAbs(root), "root is absolute")
	assert.That(!strings.Contains(folder, "/"), "folder is a capability's folder or \"\"")
	// Glob returns names in lexical order. Ref: https://pkg.go.dev/path/filepath#Glob
	files, err := filepath.Glob(filepath.Join(root, folder, "_shared", "*.sh"))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	found := make([]Helper, 0, 16)
	for _, path := range files {
		if filepath.Base(path) == KitFile {
			continue
		}
		read, err := inFile(root, path, capability)
		if err != nil {
			return nil, err
		}
		found = append(found, read...)
		if len(found) > HelpersMax {
			return nil, fmt.Errorf("more than %d helpers in %s", HelpersMax, folder)
		}
	}
	return found, nil
}

// inFile reads the functions in one file.
func inFile(root string, path string, capability string) ([]Helper, error) {
	assert.That(filepath.IsAbs(path), "path is absolute")
	assert.That(strings.HasPrefix(path, root), "the file is in the capabilities folder")
	text, err := ReadScript(path)
	if err != nil {
		return nil, err
	}
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(text, "\n")
	found := make([]Helper, 0, 8)
	for index, line := range lines {
		match := definition.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		signature, description := docAbove(lines, index)
		found = append(found, Helper{
			Name: match[1], Signature: signature, Description: description,
			Capability: capability, File: filepath.ToSlash(relative), Line: index + 1,
			Body: bodyBelow(lines, index),
		})
	}
	return found, nil
}

// docAbove reads the comment block right above a definition: its `# @helper` line, and the
// comment lines after it.
func docAbove(lines []string, definitionIndex int) (string, string) {
	assert.That(definitionIndex >= 0, "the definition is a line")
	assert.That(definitionIndex < len(lines), "the definition is in the file")
	start := definitionIndex
	for start > 0 && strings.HasPrefix(strings.TrimSpace(lines[start-1]), "#") { // bound: start
		start--
	}
	signature, description := "", make([]string, 0, 4)
	for _, line := range lines[start:definitionIndex] {
		trimmed := strings.TrimSpace(line)
		if match := helperDoc.FindStringSubmatch(trimmed); match != nil {
			signature = strings.TrimSpace(match[1])
			continue
		}
		if signature != "" {
			description = append(description, strings.TrimSpace(strings.TrimPrefix(trimmed, "#")))
		}
	}
	return signature, strings.Join(description, " ")
}

// bodyBelow is the text between a definition line and the next line that's just `}`.
func bodyBelow(lines []string, definitionIndex int) string {
	assert.That(definitionIndex >= 0, "the definition is a line")
	assert.That(definitionIndex < len(lines), "the definition is in the file")
	body := make([]string, 0, 8)
	for _, line := range lines[definitionIndex+1:] {
		if strings.TrimRight(line, " \t") == "}" {
			break
		}
		body = append(body, line)
	}
	return strings.Join(body, "\n")
}

// ReadScript reads a script or helper file, within FileSizeBytesMax.
func ReadScript(path string) (string, error) {
	assert.That(filepath.IsAbs(path), "path is absolute")
	assert.That(FileSizeBytesMax > 0, "scripts have a size limit")
	text, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if len(text) > FileSizeBytesMax {
		return "", fmt.Errorf("%s is larger than %d bytes", path, FileSizeBytesMax)
	}
	return string(text), nil
}

// Visible lists the helpers a capability's scripts can call: _shared's, then the capability's
// own, which win over a shared helper with the same name. The map is by name.
func Visible(shared []Helper, own []Helper) map[string]Helper {
	assert.That(len(shared) <= HelpersMax, "shared helpers stay bounded")
	assert.That(len(own) <= HelpersMax, "own helpers stay bounded")
	visible := make(map[string]Helper, len(shared)+len(own))
	for _, helper := range shared {
		if _, seen := visible[helper.Name]; !seen {
			visible[helper.Name] = helper
		}
	}
	for _, helper := range own {
		if existing, seen := visible[helper.Name]; !seen || existing.Capability == "" {
			visible[helper.Name] = helper
		}
	}
	return visible
}

// Called lists, sorted, the visible helpers text calls, and the helpers those call, and so on.
// Comment lines don't count.
func Called(text string, visible map[string]Helper) []string {
	assert.That(len(text) <= FileSizeBytesMax, "text stays bounded")
	assert.That(len(visible) <= 2*HelpersMax, "visible helpers stay bounded")
	called := make(map[string]bool, 8)
	queue := callsIn(text, visible)
	// bound: each helper joins the queue at most once, and there are len(visible) of them.
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		if called[name] {
			continue
		}
		called[name] = true
		queue = append(queue, callsIn(visible[name].Body, visible)...)
	}
	names := make([]string, 0, len(called))
	for name := range called {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// callsIn lists the visible helper names that appear as words in text's code lines.
func callsIn(text string, visible map[string]Helper) []string {
	assert.That(len(text) <= FileSizeBytesMax, "text stays bounded")
	assert.That(visible != nil, "there's a helper map")
	names := make([]string, 0, 4)
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		for _, token := range word.FindAllString(line, -1) {
			if _, ok := visible[token]; ok {
				names = append(names, token)
			}
		}
	}
	return names
}
