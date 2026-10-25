// The big file's helpers: code that two or more of big.go's stories use, kept apart so that
// big.go reads as the stories alone. Each helper says what it does, not which story it serves;
// a function only one story uses belongs in that story's section of big.go. See
// tasks/one-big-file/plan.md.
//
// TestBigFile checks this file holds no Feature, Rule or Scenario headings. If this file grows
// past big.go, that's the tripwire: move a group that belongs together to a file of its own,
// and say why in big.go's header.

package app

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"givens/internal/app/model"
	"givens/internal/assert"
	"givens/internal/project"
	"givens/internal/runs"
	"givens/internal/slug"
	"givens/internal/spec"
)

// repoDepthMax bounds the walk up from a capabilities folder to its git repo's root.
const repoDepthMax = 64

// repoRoot is the root of the git repository holding capabilities, or, outside one, the
// folder holding capabilities. The kit moves there before running anything.
func repoRoot(capabilities string) string {
	assert.That(filepath.IsAbs(capabilities), "capabilities is absolute")
	assert.That(repoDepthMax > 0, "the walk up has a limit")
	dir := filepath.Dir(capabilities)
	for depth := 0; depth < repoDepthMax; depth++ {
		// .git is a folder in a clone and a file in a worktree.
		// Ref: https://git-scm.com/docs/gitrepository-layout
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return filepath.Dir(capabilities)
}

// remember adds folder to one of the app's folder lists: trusted, untrusted, recents.
func (a *App) remember(name string, folder string) error {
	assert.That(name != "", "a list has a file")
	assert.That(filepath.IsAbs(folder), "folders are absolute")
	folders, err := a.folders(name)
	if err != nil || slices.Contains(folders, folder) {
		return err
	}
	if len(folders) >= TrustedMax {
		return fmt.Errorf("Givens already keeps %d folders, the most it keeps", TrustedMax)
	}
	return a.writeFolders(name, append(folders, folder))
}

// forget takes folder out of one of the app's folder lists.
func (a *App) forget(name string, folder string) error {
	assert.That(name != "", "a list has a file")
	assert.That(filepath.IsAbs(folder), "folders are absolute")
	folders, err := a.folders(name)
	if err != nil || !slices.Contains(folders, folder) {
		return err
	}
	kept := slices.DeleteFunc(folders, func(other string) bool { return other == folder })
	return a.writeFolders(name, kept)
}

// writeFolders writes a folder list, one absolute path per line.
func (a *App) writeFolders(name string, folders []string) error {
	assert.That(name != "", "a list has a file")
	assert.That(len(folders) <= TrustedMax, "lists stay bounded")
	text := strings.Join(folders, "\n") + "\n"
	return os.WriteFile(filepath.Join(a.config.AppDataDir, name), []byte(text), 0o644)
}

// folders reads one of the app's folder lists, one absolute path per line.
func (a *App) folders(name string) ([]string, error) {
	assert.That(a != nil, "folders runs on an app")
	assert.That(a.config.AppDataDir != "", "lists live in the app's data folder")
	text, err := os.ReadFile(filepath.Join(a.config.AppDataDir, name))
	if errors.Is(err, os.ErrNotExist) {
		return make([]string, 0, 1), nil
	}
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(text)), "\n")
	if len(lines) > TrustedMax {
		return nil, fmt.Errorf("%s holds %d folders, more than %d", name, len(lines),
			TrustedMax)
	}
	folders := make([]string, 0, len(lines))
	for _, line := range lines {
		if filepath.IsAbs(line) {
			folders = append(folders, line)
		}
	}
	return folders, nil
}

// tagViews lists the tags that are views, without their @, in the order first seen. @error
// and @timeout have meanings of their own, so they aren't views.
func (w *Window) tagViews() []string {
	assert.That(w != nil, "tag views run on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	views := make([]string, 0, 8)
	if w.project == nil {
		return views
	}
	for _, capability := range w.project.Capabilities {
		for _, scenario := range capability.Scenarios {
			for _, tag := range scenario.Tags {
				name := strings.TrimPrefix(tag, "@")
				if isViewTag(tag) && !slices.Contains(views, name) {
					views = append(views, name)
				}
			}
		}
	}
	return views
}

// isViewTag is true for a tag that's a view: any but @error and @timeout:….
func isViewTag(tag string) bool {
	assert.That(strings.HasPrefix(tag, "@"), "a tag starts with @")
	assert.That(len(tag) <= NameLengthMax, "a tag fits NameLengthMax")
	return tag != project.TagError && !strings.HasPrefix(tag, "@timeout")
}

// SearchResults is the ⌘K panel's query, filter and grouped results, or the zero value while
// search is closed. A query that matches nothing, or a tag expression that doesn't parse,
// has no groups. Needed by every scenario in capabilities/search-everything and the search
// scenarios in capabilities/tags.
func (w *Window) SearchResults() SearchResults {
	assert.That(w != nil, "search results run on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	if w.search == nil {
		return SearchResults{Groups: []SearchGroup{}}
	}
	if memo := w.searchMemo; memo != nil && memo.query == w.search.query &&
		memo.filter == w.search.filter {
		return memo.results
	}
	results := SearchResults{
		Query: w.search.query, Filter: w.search.filter, Groups: make([]SearchGroup, 0, 5),
	}
	defer func() {
		w.searchMemo = &searchMemo{query: results.Query, filter: results.Filter, results: results}
	}()
	read := readQuery(w.search.query)
	if w.project == nil || read.problem != "" || (read.words == "" && !read.tagged) {
		return results
	}
	for _, group := range w.searchGroups(read) {
		if len(group.Results) > 0 && (results.Filter == "" || results.Filter == group.Kind) {
			results.Groups = append(results.Groups, group)
		}
	}
	return results
}

// uniqueSteps is a scenario's steps, each text once, so a step it repeats counts as one use.
func uniqueSteps(scenario project.Scenario) []project.Step {
	assert.That(scenario.Name != "", "the scenario has a name")
	assert.That(len(scenario.Steps) <= HelpersMax, "steps stay bounded")
	steps := make([]project.Step, 0, len(scenario.Steps))
	seen := make(map[string]bool, len(scenario.Steps))
	for _, step := range scenario.Steps {
		if !seen[step.Text] {
			seen[step.Text] = true
			steps = append(steps, step)
		}
	}
	return steps
}

// Helpers is the "Shared" section of the sidebar: every documented helper in the project's
// _shared folders, _shared's first, in file order. A helper defined twice in one folder is
// flagged; a capability's helper that shadows a shared one says so.
func (w *Window) Helpers() []model.Helper {
	assert.That(w != nil, "helpers runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	w.refresh()
	if w.project == nil {
		return []model.Helper{}
	}
	// Kept until a file changes: finding callers reads every script once per helper.
	if w.helpers != nil {
		return append([]model.Helper{}, w.helpers...)
	}
	shared, err := HelpersInFolder(w.path, "", "")
	if err != nil {
		return []model.Helper{}
	}
	listed := w.listHelpers(shared, "", nil)
	for _, capability := range w.project.Capabilities {
		own, err := HelpersInFolder(w.path, capability.Slug, capability.Slug)
		if err != nil {
			continue
		}
		listed = append(listed, w.listHelpers(own, capability.Name, shared)...)
	}
	w.helpers = listed
	return append([]model.Helper{}, listed...)
}

// Limits on checking a helper file.
const (
	// syntaxCheckTimeout bounds one `bash -n`, which only reads the file.
	syntaxCheckTimeout = 5 * time.Second
	// helperProblemsMax bounds the lines marked in one file.
	helperProblemsMax = 64
)

// bashError matches one of bash's syntax reports: "x.sh: line 9: syntax error: …".
const bashErrorPattern = `line (\d+): (.+)`

var bashError = regexp.MustCompile(bashErrorPattern) //tiger:allow-global compiled once

// RequiredCommands lists the commands a scenario's setup needs, whether each is installed, and
// its install hint: the `need` lines in its script, the setups it builds on, and every helper
// those call. Needed by "See what a scenario needs installed" and by Drop in's check before
// anything runs.
func (w *Window) RequiredCommands(scenario string) ([]model.RequiredCommand, error) {
	assert.That(w != nil, "required commands runs on a window")
	assert.That(len(scenario) <= NameLengthMax, "the name fits NameLengthMax")
	found, ok := w.findScenario(scenario)
	if !ok {
		return nil, fmt.Errorf("%s: %w", scenario, ErrNotFound)
	}
	texts, err := w.setupTexts(found)
	if err != nil {
		return nil, err
	}
	all := ""
	for _, text := range texts {
		all += text + "\n"
	}
	needs := NeedLines(all)
	required := make([]model.RequiredCommand, 0, len(needs)+1)
	if chrome, uses := chromeRequired(all); uses {
		required = append(required, chrome)
	}
	for _, need := range needs {
		// LookPath searches PATH as a shell would. Ref: https://pkg.go.dev/os/exec#LookPath
		_, missing := exec.LookPath(need.Command)
		required = append(required, model.RequiredCommand{
			Command: need.Command, Hint: need.Hint, Installed: missing == nil,
		})
	}
	return required, nil
}

// From internal/helpers, which reads the shell functions in a capabilities folder's _shared
// folders: where each is defined and its `# @helper` documentation, and the `from` lines a setup
// builds on. Copy, Drop in, Search, Share setup and Write stories read scripts with it. Which
// helpers a script calls, and its `need` lines, are Share setup's, at the end of its section in
// big.go; the browser lines are Mix shell's.
//
// It reads bash by pattern, not by parsing it: a helper is a line `name() {` at the start of a
// line, ending at the next line that is just `}`. That's the shape GIVENS.md asks for, and what
// `bash -n` can't tell Givens without running anything.
// Ref: https://www.gnu.org/software/bash/manual/html_node/Shell-Functions.html

// Limits. Tiger Style: put a limit on everything.
// Ref: https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md#safety
const (
	// ScriptSizeBytesMax bounds one helper file or setup script.
	ScriptSizeBytesMax = 1 << 20
	// HelpersMax bounds the functions one capability can see.
	HelpersMax = 4096
	// KitFileName is the kit's file name; the kit isn't a helper file.
	KitFileName = "givens.sh"
)

// Patterns, compiled once.
//
//tiger:allow-global compiled once
var (
	helperDefinition = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)\s*\(\)\s*\{`)
	helperDoc        = regexp.MustCompile(`^#\s*@helper\s+(\S+.*)$`)
)

// HelperDefinition is one shell function in a _shared folder.
type HelperDefinition struct {
	Name        string
	Signature   string // the `# @helper` line's text, like "seed_shopper [email]"; "" if none
	Description string // the comment lines under `# @helper`, joined with spaces
	Capability  string // the capability slug whose _shared holds it, or "" for _shared
	File        string // relative to the capabilities folder: "_shared/users.sh"
	Line        int    // the `name() {` line, counting from 1
	Body        string // the lines between `name() {` and its closing `}`
}

// Documented is true for a helper with a `# @helper` comment, which the Shared list shows.
func (h HelperDefinition) Documented() bool {
	assert.That(h.Name != "", "a helper has a name")
	assert.That(h.File != "", "a helper has a file")
	return h.Signature != ""
}

// HelpersInFolder reads every function in root/<folder>/_shared/*.sh but the kit, in file name
// order then line order. folder is "" for root/_shared. capability names what
// HelperDefinition.Capability says.
func HelpersInFolder(root string, folder string, capability string) ([]HelperDefinition, error) {
	assert.That(filepath.IsAbs(root), "root is absolute")
	assert.That(!strings.Contains(folder, "/"), "folder is a capability's folder or \"\"")
	// Glob returns names in lexical order. Ref: https://pkg.go.dev/path/filepath#Glob
	files, err := filepath.Glob(filepath.Join(root, folder, "_shared", "*.sh"))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	found := make([]HelperDefinition, 0, 16)
	for _, path := range files {
		if filepath.Base(path) == KitFileName {
			continue
		}
		read, err := helpersInFile(root, path, capability)
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

// helpersInFile reads the functions in one file.
func helpersInFile(root string, path string, capability string) ([]HelperDefinition, error) {
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
	found := make([]HelperDefinition, 0, 8)
	for index, line := range lines {
		match := helperDefinition.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		signature, description := docAbove(lines, index)
		found = append(found, HelperDefinition{
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

// ReadScript reads a script or helper file, within ScriptSizeBytesMax.
func ReadScript(path string) (string, error) {
	assert.That(filepath.IsAbs(path), "path is absolute")
	assert.That(ScriptSizeBytesMax > 0, "scripts have a size limit")
	text, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if len(text) > ScriptSizeBytesMax {
		return "", fmt.Errorf("%s is larger than %d bytes", path, ScriptSizeBytesMax)
	}
	return string(text), nil
}

// The pattern for the kit's from lines.
// Ref: capabilities/the-kit/about.md, in the Givens repository
const fromPattern = `(?m)^\s*from\s+([^\s/]+/[^\s/]+)((?:[ \t]+[A-Za-z_]\w*=\S*)*)[ \t]*$`

var fromLineRegexp = regexp.MustCompile(fromPattern) //tiger:allow-global compiled once

// FromLine is one `from <capability>/<slug> [column=value …]` line.
type FromLine struct {
	Key    string   // "checkout/pay-with-saved-card"
	Values []string // "card=stolen", in order
	Line   string   // the line as written, trimmed
}

// FromLines lists the from lines in a setup script, in order.
func FromLines(text string) []FromLine {
	assert.That(len(text) <= ScriptSizeBytesMax, "a script stays bounded")
	matches := fromLineRegexp.FindAllStringSubmatch(text, -1)
	froms := make([]FromLine, 0, len(matches))
	for _, match := range matches {
		froms = append(froms, FromLine{
			Key: match[1], Values: strings.Fields(match[2]),
			Line: strings.TrimSpace(match[0]),
		})
	}
	assert.That(len(froms) == len(matches), "each match is a from line")
	return froms
}

// Scenario statuses, as the library shows them.
const (
	StatusNeverRun      = "Never run"
	StatusLastRunFailed = "Last run failed"
	statusVerified      = "Verified"
	statusUnverified    = "Unverified: "
)

// statusOf says where a scenario stands, by the app's clock. Answers are kept until a file in
// the capabilities folder changes (runs/ included) or the day turns over, since the sidebar,
// the capability page and the views all ask, and each answer reads every run file and
// fingerprints the setup.
func (w *Window) statusOf(scenario project.Scenario) string {
	assert.That(w.project != nil, "status is read in a loaded project")
	assert.That(scenario.Capability != "", "the scenario belongs to a capability")
	now := w.app.config.Clock.Now()
	if day := now.Format(time.DateOnly); day != w.statusDay || w.statuses == nil {
		w.statuses, w.statusDay = make(map[string]string, 16), day
	}
	key := scenario.Capability + "/" + scenario.Name
	if status, kept := w.statuses[key]; kept {
		return status
	}
	status := StatusOf(w.project, scenario, now)
	w.statuses[key] = status
	return status
}

// StatusOf says where a scenario stands: verified while its latest run passed and nothing its
// setup depends on has changed since. An outline sums up its rows. The givens command uses it
// too. See capabilities/views and capabilities/share-setup-between-scenarios.
func StatusOf(loaded *project.Project, scenario project.Scenario, now time.Time) string {
	assert.That(loaded != nil, "status is read in a loaded project")
	assert.That(!now.IsZero(), "now is a time")
	if scenario.Slug == "" {
		return StatusNeverRun
	}
	if scenario.Outline {
		return outlineStatus(loaded, scenario, now)
	}
	return statusFor(loaded, scenario, scenario.Key(), nil, now)
}

// statusFor says where one run key, with its row's values, stands.
func statusFor(loaded *project.Project, scenario project.Scenario, key string,
	values []runs.Pair, now time.Time) string {
	assert.That(loaded != nil, "status is read in a loaded project")
	assert.That(key != "", "a run key is given")
	recorded, err := runs.Read(loaded.Root)
	if err != nil {
		return StatusNeverRun
	}
	latest, ok := runs.Latest(recorded, key, values)
	if !ok {
		return StatusNeverRun
	}
	current, err := FingerprintOf(loaded, scenario, values)
	if err != nil {
		return StatusNeverRun
	}
	if changed := FingerprintChanged(latest.Fingerprint, current); changed != "" {
		return statusUnverified + changed + " changed since the last run"
	}
	if latest.Result != runs.ResultPass {
		return StatusLastRunFailed
	}
	return verified(latest, now)
}

// verified says when, and by whom, a passing run was recorded: "Verified today by Sam".
func verified(run runs.Run, now time.Time) string {
	assert.That(run.Result == runs.ResultPass, "only a passing run verifies")
	assert.That(!now.IsZero(), "now is a time")
	day := "on " + run.Time.Format(time.DateOnly)
	switch run.Time.In(now.Location()).Format(time.DateOnly) {
	case now.Format(time.DateOnly):
		day = "today"
	case now.AddDate(0, 0, -1).Format(time.DateOnly):
		day = "yesterday"
	}
	status := statusVerified + " " + day
	if run.Author != "" {
		status += " by " + run.Author
	}
	return status
}

// From internal/fingerprint: FingerprintOf, the way in to a scenario's setup fingerprint,
// which statusFor and Drop in's RecordRun call, with the part names and hash. Working out each
// part, and saying which changed, is Share setup's, in its section of big.go.

// Part names that aren't helper names.
const (
	FingerprintPartSteps = "steps"
	FingerprintPartSetup = "setup"
	fromPrefix           = "from:"
)

// FingerprintOf works out the fingerprint of scenario, one of loaded's, for the row whose values
// are row (nil for a scenario with no Examples).
func FingerprintOf(loaded *project.Project, scenario project.Scenario,
	row []runs.Pair) ([]runs.Pair, error) {
	assert.That(loaded != nil, "the project is loaded")
	assert.That(scenario.Capability != "", "the scenario belongs to a capability")
	parts := []runs.Pair{{Name: FingerprintPartSteps, Value: hash(stepsText(scenario, row))}}
	own, err := setupText(scenario)
	if err != nil {
		return nil, err
	}
	parts = append(parts, runs.Pair{Name: FingerprintPartSetup, Value: hash(own)})
	chain, err := fromChain(loaded, scenario)
	if err != nil {
		return nil, err
	}
	for _, built := range chain {
		text, err := setupText(built)
		if err != nil {
			return nil, err
		}
		parts = append(parts, runs.Pair{Name: fromPrefix + built.Key(), Value: hash(text)})
	}
	called, err := calledHelpers(loaded, append([]project.Scenario{scenario}, chain...))
	if err != nil {
		return nil, err
	}
	return append(parts, called...), nil
}

// hash is a short code for text: the first 8 hex digits of its SHA-256.
// Ref: https://pkg.go.dev/crypto/sha256
func hash(text string) string {
	assert.That(len(text) < 1<<30, "hashed text stays bounded")
	sum := sha256.Sum256([]byte(text))
	code := hex.EncodeToString(sum[:4])
	assert.That(len(code) == 8, "a code is 8 hex digits")
	return code
}

// aboutImage matches a markdown image: ![alt](path).
const aboutImagePattern = `!\[([^\]]*)\]\(([^)\s]+)\)`

var aboutImage = regexp.MustCompile(aboutImagePattern) //tiger:allow-global compiled once

// withImagePlaceholders replaces each image about.md shows that isn't on disk with a line
// naming it, so the rest of the story still reads: "Image not found: images/flow.svg".
func withImagePlaceholders(about string, dir string) string {
	assert.That(filepath.IsAbs(dir), "the capability's folder is absolute")
	assert.That(len(about) <= model.EditorTextBytesMax, "about.md stays bounded")
	return aboutImage.ReplaceAllStringFunc(about, func(found string) string {
		path := aboutImage.FindStringSubmatch(found)[2]
		if strings.Contains(path, "://") {
			return found
		}
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(path))); err == nil {
			return found
		}
		return "Image not found: " + path
	})
}

// MessageTimeoutTag is the warning on a scenario whose @timeout tag can't be read.
const MessageTimeoutTag = "@timeout must look like @timeout:90s or @timeout:5m"

// scenarioWarning says what's wrong with a scenario as written, or "".
func scenarioWarning(capability project.Capability, scenario project.Scenario) string {
	assert.That(scenario.Capability == capability.Slug, "the scenario is in the capability")
	assert.That(scenario.StillRunning >= 0, "a timeout is never negative")
	if clash := slugClash(capability, scenario); clash != "" {
		return clash
	}
	if scenario.StillRunning == 0 {
		return MessageTimeoutTag
	}
	return examplesWarning(scenario)
}

// slugClash is the warning on a scenario whose slug another scenario in its capability
// makes too, or "". Runs and setup files are named by slug, so neither scenario can use them.
func slugClash(capability project.Capability, scenario project.Scenario) string {
	assert.That(scenario.Capability == capability.Slug, "the scenario is in the capability")
	assert.That(len(capability.Scenarios) <= spec.ScenariosPerFeatureMax, "scenarios bounded")
	for _, other := range capability.Scenarios {
		if other.Slug == scenario.Slug && other.Line != scenario.Line {
			return "Both names make the slug " + scenario.Slug
		}
	}
	return ""
}

// capabilitiesFolderName is the folder Open looks for inside a project folder.
const capabilitiesFolderName = "capabilities"

// findCapabilities resolves what Open was given to a capabilities folder: the folder itself
// when it holds capabilities or is named "capabilities", else its "capabilities" subfolder.
// found is false when neither is there; path is then the absolute folder given.
func findCapabilities(path string) (string, bool, error) {
	assert.That(path != "", "a path is given")
	assert.That(len(path) <= PathLengthMax, "the path fits PathLengthMax")
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", false, err
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", false, err
	}
	if !info.IsDir() {
		return "", false, errors.New(absolute + " isn't a folder")
	}
	inside := filepath.Join(absolute, capabilitiesFolderName)
	if info, err := os.Stat(inside); err == nil && info.IsDir() {
		return inside, true, nil
	}
	if filepath.Base(absolute) == capabilitiesFolderName {
		return absolute, true, nil
	}
	loaded, err := project.Load(absolute)
	if err != nil {
		return "", false, err
	}
	return absolute, len(loaded.Capabilities) > 0, nil
}

// reload reads the window's capabilities folder again and starts watching it. A window with
// no capabilities folder has no project.
func (w *Window) reload() error {
	assert.That(w != nil, "reload runs on a window")
	assert.That(w.kind == WindowProject, "only a project window has a project")
	if !w.found {
		w.project, w.watcher = nil, nil
		return nil
	}
	w.restoreKit()
	// The watcher's first look comes before the load, so a write in between is seen next time.
	watcher, err := NewWatcher(w.path, time.Now())
	if err != nil {
		return err
	}
	loaded, err := project.Load(w.path)
	if err != nil {
		return err
	}
	w.app.withoutTrashed(loaded)
	w.project, w.watcher, w.statuses, w.helpers = loaded, watcher, nil, nil
	w.histories, w.index, w.searchMemo = nil, nil, nil
	return nil
}

// refresh reloads the project when another tool changed its files. Givens keeps no copy of
// its own, so every read of a window starts here, and looks at the files each time: a step
// that writes a file and reads the window at once must see the write. While a snapshot is
// being built it looks once, not once per part. A folder that can't be read keeps showing what
// was last read.
func (w *Window) refresh() {
	assert.That(w != nil, "refresh runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	w.pollRunsSync()
	if w.watcher == nil || w.holdRefresh {
		return
	}
	changed, err := w.watcher.Look(time.Now())
	if err != nil || !changed {
		return
	}
	w.statuses, w.helpers, w.histories, w.index, w.searchMemo = nil, nil, nil, nil, nil
	w.restoreKit()
	if loaded, err := project.Load(w.path); err == nil {
		w.app.withoutTrashed(loaded)
		w.project = loaded
	}
}

// findCapability finds a capability by its name or slug.
func (w *Window) findCapability(name string) (project.Capability, bool) {
	assert.That(w != nil, "find runs on a window")
	assert.That(name != "", "a capability has a name")
	w.refresh()
	if w.project == nil {
		return project.Capability{}, false
	}
	for _, capability := range w.project.Capabilities {
		if capability.Name == name || capability.Slug == name {
			return capability, true
		}
	}
	return project.Capability{}, false
}

// findScenario finds a scenario by its name, "Capability/Scenario", or its run key. An exact
// name wins over a slug, so two names with the same slug can still each be found.
func (w *Window) findScenario(name string) (project.Scenario, bool) {
	assert.That(w != nil, "find runs on a window")
	assert.That(name != "", "a scenario has a name")
	w.refresh()
	if w.project == nil {
		return project.Scenario{}, false
	}
	if capabilityName, scenarioName, ok := strings.Cut(name, "/"); ok && scenarioName != "" {
		if found, ok := w.findIn(capabilityName, scenarioName); ok {
			return found, true
		}
	}
	return w.findIn("", name)
}

// findIn finds a scenario by exact name, then by slug, in one capability or, for "", in any.
func (w *Window) findIn(capabilityName string, name string) (project.Scenario, bool) {
	assert.That(w.project != nil, "find in runs on a loaded project")
	assert.That(name != "", "a scenario has a name")
	want := slug.Of(name)
	for _, bySlug := range []bool{false, true} {
		for _, capability := range w.project.Capabilities {
			if capabilityName != "" && capability.Name != capabilityName &&
				capability.Slug != capabilityName {
				continue
			}
			for _, scenario := range capability.Scenarios {
				if (!bySlug && scenario.Name == name) || (bySlug && scenario.Slug == want) {
					return scenario, true
				}
			}
		}
	}
	return project.Scenario{}, false
}

// writable refuses a change in a branch's window. The UI doesn't offer one there; this stops
// a Needs call or a CLI one that asks anyway.
func (w *Window) writable() error {
	assert.That(w != nil, "writable runs on a window")
	assert.That(w.branch == "" || w.branchOrigin != "", "a branch's window knows its origin")
	if w.branch != "" {
		return fmt.Errorf("%s @ %s: %w", filepath.Base(w.branchOrigin), w.branch, ErrReadOnly)
	}
	return nil
}

// Edit opens a scenario, by name, at its block in its .feature file, or a file, by its path
// relative to the capabilities folder ("checkout/about.md"), in the editor. The editor holds
// the file's whole text, comments and all, so saving writes back only what was typed.
// Needed by the editing, saving and preview scenarios in capabilities/write-stories-and-scenarios.
func (w *Window) Edit(target string) (*Editor, error) {
	assert.That(w != nil, "edit runs on a window")
	assert.That(target != "", "edit names a scenario or a file")
	assert.That(!filepath.IsAbs(target), "a file is named relative to the capabilities folder")
	if err := w.writable(); err != nil {
		return nil, err
	}
	if w.project == nil {
		return nil, fmt.Errorf("%s: %w", target, ErrNotFound)
	}
	path := filepath.Join(w.path, target)
	if scenario, ok := w.findScenario(target); ok {
		capability, _ := w.findCapability(scenario.Capability)
		path = capability.FeaturePath
	}
	if !strings.HasPrefix(path, w.path+string(filepath.Separator)) {
		return nil, fmt.Errorf("%s is outside the capabilities folder: %w", target, ErrNotFound)
	}
	text, err := readEditable(path)
	if err != nil {
		return nil, err
	}
	w.editor = &Editor{window: w, path: path, text: text, opened: text, problems: []string{},
		typing: -1, marks: []model.EditorMark{}}
	if scenario, ok := w.findScenario(target); ok {
		w.editor.scenario = scenario.Name
		w.editor.line = int(scenario.Line)
		w.editor.cursor = blockEnd(strings.Split(text, "\n"), int(scenario.Line)-1)
	}
	return w.editor, nil
}

// readEditable reads a file the editor can hold: text, within EditorTextBytesMax.
func readEditable(path string) (string, error) {
	assert.That(filepath.IsAbs(path), "the path is absolute")
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("%s: %w", filepath.Base(path), ErrNotFound)
	}
	if err != nil {
		return "", err
	}
	if len(content) > model.EditorTextBytesMax || !utf8.Valid(content) {
		return "", fmt.Errorf("%s isn't a text file Givens can edit", filepath.Base(path))
	}
	assert.That(len(content) <= model.EditorTextBytesMax, "editable text stays bounded")
	return string(content), nil
}

// writeNew writes a new file, making its folder, unless one is already there.
func writeNew(path string, text string) error {
	assert.That(filepath.IsAbs(path), "the path is absolute")
	assert.That(utf8.ValidString(text), "the file is text")
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(text), 0o644)
}

// trash moves a path to the Trash. Under the Mac app the move happens a moment later, so
// until it's gone the path is pending and loads leave it out.
func (a *App) trash(path string) error {
	assert.That(a != nil, "trash runs on an app")
	assert.That(filepath.IsAbs(path), "a trashed path is absolute")
	if err := a.config.Trash.Move(path); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		if a.pendingTrash == nil {
			a.pendingTrash = make(map[string]bool, 4)
		}
		a.pendingTrash[path] = true
	}
	return nil
}

// withoutTrashed leaves out of a loaded project what's waiting to go to the Trash, and
// forgets paths the Trash now has.
func (a *App) withoutTrashed(loaded *project.Project) {
	assert.That(a != nil, "without trashed runs on an app")
	assert.That(loaded != nil, "a project was loaded")
	for path := range a.pendingTrash {
		if _, err := os.Stat(path); err != nil {
			delete(a.pendingTrash, path)
		}
	}
	if len(a.pendingTrash) == 0 {
		return
	}
	kept := loaded.Capabilities[:0]
	for _, capability := range loaded.Capabilities {
		if a.pendingTrash[capability.Dir] {
			continue
		}
		for index := range capability.Scenarios {
			scenario := &capability.Scenarios[index]
			for _, path := range []*string{&scenario.Setup, &scenario.SetupCards,
				&scenario.Teardown} {
				if a.pendingTrash[*path] {
					*path = ""
				}
			}
		}
		kept = append(kept, capability)
	}
	loaded.Capabilities = kept
}

// From internal/markdown, which reads a setup done by hand, scripts/<slug>.md: the text before
// the first numbered item, and each top-level numbered item as one step, with its fenced code
// blocks, inline code, links and images. It reads only what setups use, not all of CommonMark.
// Drop in's cards and runBlocksIn read it, and `givens run` (cmd/givens/run.go) prints it.
// Drop in renders a step's text with RenderedMarkdown, in its section of big.go, and `givens
// run` with PlainMarkdown, in Use Givens from the terminal's.
// Ref: https://spec.commonmark.org/0.31.2/#ordered-list
// Ref: https://spec.commonmark.org/0.31.2/#fenced-code-blocks

// Limits. Tiger Style: put a limit on everything.
const (
	// MarkdownSetupBytesMax bounds one setup file.
	MarkdownSetupBytesMax = 1 << 20
	// MarkdownStepsMax bounds the numbered items in one file.
	MarkdownStepsMax = 256
)

// Patterns, compiled once.
const (
	markdownItemPattern   = `^(\d+)\.\s+(.*)$`
	markdownLinkPattern   = `(!?)\[([^\]]*)\]\(([^)\s]*)\)`
	markdownInlinePattern = "`([^`]+)`"
	// A fence's info string: its language, then any other words, like "run".
	// Ref: https://spec.commonmark.org/0.31.2/#info-string
	markdownFencePattern = "^\\s*(```+|~~~+)\\s*([A-Za-z0-9_+-]*)((?:[ \\t]+[^\\s`]+)*)\\s*$"
)

var (
	markdownItem   = regexp.MustCompile(markdownItemPattern)   //tiger:allow-global compiled once
	markdownLink   = regexp.MustCompile(markdownLinkPattern)   //tiger:allow-global compiled once
	markdownInline = regexp.MustCompile(markdownInlinePattern) //tiger:allow-global compiled once
	markdownFence  = regexp.MustCompile(markdownFencePattern)  //tiger:allow-global compiled once
)

// FencedBlock is one fenced code block.
type FencedBlock struct {
	Language string // the info string's first word: "bash", "js", or ""
	Run      bool   // the info string's next word is "run": Givens runs the block itself
	Code     string // exactly the code: no fences, no list indent
	Closed   bool   // false when the fence never closes
	Line     int    // the opening fence's line, counting from 1
}

// MarkdownStep is one top-level numbered item.
type MarkdownStep struct {
	Number int
	Text   string        // the item's markdown, list number taken off, code blocks left out
	Blocks []FencedBlock // its fenced code blocks, in order
	Links  []string      // link targets, in order
	Images []string      // image paths, in order
	Inline []string      // inline code spans, in order
}

// MarkdownSetup is a whole markdown setup.
type MarkdownSetup struct {
	Intro string // the text before the first numbered item, trimmed
	Steps []MarkdownStep
}

// ParseMarkdownSetup reads a markdown setup.
func ParseMarkdownSetup(text string) (MarkdownSetup, error) {
	assert.That(len(text) <= MarkdownSetupBytesMax*2, "the text stays bounded")
	if len(text) > MarkdownSetupBytesMax {
		return MarkdownSetup{}, fmt.Errorf("a markdown setup is at most %d bytes",
			MarkdownSetupBytesMax)
	}
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	setup := MarkdownSetup{Steps: make([]MarkdownStep, 0, 8)}
	intro := make([]string, 0, 4)
	var current *MarkdownStep
	var open *FencedBlock
	indent := 0
	for index, line := range lines {
		if open != nil {
			open, indent = closeOrAdd(current, open, line, indent)
			continue
		}
		if match := markdownItem.FindStringSubmatch(line); match != nil {
			if len(setup.Steps) >= MarkdownStepsMax {
				return MarkdownSetup{}, fmt.Errorf("a markdown setup has at most %d steps",
					MarkdownStepsMax)
			}
			setup.Steps = append(setup.Steps, MarkdownStep{Number: len(setup.Steps) + 1,
				Text: match[2]})
			current = &setup.Steps[len(setup.Steps)-1]
			continue
		}
		if current == nil {
			intro = append(intro, line)
			continue
		}
		if match := markdownFence.FindStringSubmatch(line); match != nil {
			words := strings.Fields(match[3])
			current.Blocks = append(current.Blocks, FencedBlock{Language: match[2],
				Run: len(words) > 0 && words[0] == "run", Line: index + 1})
			open, indent = &current.Blocks[len(current.Blocks)-1], leadingSpaces(line)
			continue
		}
		current.Text += "\n" + strings.TrimSpace(line)
	}
	setup.Intro = strings.TrimSpace(strings.Join(intro, "\n"))
	for index := range setup.Steps {
		finishMarkdownStep(&setup.Steps[index])
	}
	assert.That(len(setup.Steps) <= MarkdownStepsMax, "steps stay bounded")
	return setup, nil
}

// closeOrAdd ends an open code block at its closing fence, or adds a line to it with the
// fence's indent taken off. It returns the block still open, or nil.
func closeOrAdd(step *MarkdownStep, open *FencedBlock, line string,
	indent int) (*FencedBlock, int) {
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
	assert.That(len(line) <= MarkdownSetupBytesMax, "a line stays bounded")
	count := len(line) - len(strings.TrimLeft(line, " "))
	assert.That(count >= 0 && count <= len(line), "spaces are part of the line")
	return count
}

// finishMarkdownStep trims a step's text and reads its links, images and inline code.
func finishMarkdownStep(step *MarkdownStep) {
	assert.That(step != nil, "there's a step to finish")
	assert.That(step.Number > 0, "steps count from 1")
	step.Text = strings.TrimSpace(step.Text)
	for index := range step.Blocks {
		step.Blocks[index].Code = strings.TrimRight(step.Blocks[index].Code, "\n")
	}
	for _, match := range markdownLink.FindAllStringSubmatch(step.Text, -1) {
		if match[1] == "!" {
			step.Images = append(step.Images, match[3])
		} else {
			step.Links = append(step.Links, match[3])
		}
	}
	for _, match := range markdownInline.FindAllStringSubmatch(step.Text, -1) {
		step.Inline = append(step.Inline, match[1])
	}
}

// runBlocksIn reads a markdown setup's run blocks' code, a js block as the in_browser line it
// runs as, so Drop in's checks read them as a script's lines. Empty when nothing runs.
func runBlocksIn(path string) []string {
	assert.That(path == "" || filepath.IsAbs(path), "a setup file is absolute")
	assert.That(MarkdownStepsMax > 0, "steps have a limit")
	texts := make([]string, 0, 2)
	if path == "" {
		return texts
	}
	text, err := ReadScript(path)
	if err != nil {
		return texts
	}
	parsed, err := ParseMarkdownSetup(text)
	if err != nil {
		return texts
	}
	for _, step := range parsed.Steps {
		for _, block := range step.Blocks {
			switch runLanguage(block) {
			case "bash":
				texts = append(texts, block.Code)
			case "js":
				texts = append(texts, "in_browser js")
			}
		}
	}
	return texts
}

// scratchDir is the session's folder of its own, made on first use: argument files and run
// blocks' scripts. removeArguments removes it.
func (s *Session) scratchDir() (string, error) {
	assert.That(s != nil, "scratch dir runs on a session")
	assert.That(s.window != nil, "a session belongs to a window")
	if s.argumentsDir == "" {
		// Ref: https://pkg.go.dev/os#MkdirTemp
		dir, err := os.MkdirTemp("", "givens-arguments-")
		if err != nil {
			return "", err
		}
		s.argumentsDir = dir
	}
	return s.argumentsDir, nil
}

// checklistOf is what the developer does once setup is done: first_you lines from the setup
// script, then the When and Then steps in the order they're written. And and But take the
// kind before them.
func checklistOf(scenario project.Scenario, row []runs.Pair) []model.CheckItem {
	assert.That(scenario.Name != "", "the scenario has a name")
	assert.That(len(scenario.Steps) <= HelpersMax, "steps stay bounded")
	items := make([]model.CheckItem, 0, len(scenario.Steps)+2)
	if scenario.Setup != "" {
		if text, err := ReadScript(scenario.Setup); err == nil {
			for _, match := range firstYouLine.FindAllStringSubmatch(text, -1) {
				items = append(items, model.CheckItem{Text: "First, " + unquote(match[1]),
					Kind: "given"})
			}
		}
	}
	kind := "given"
	for _, step := range scenario.Steps {
		switch step.Keyword {
		case "Given", "When", "Then":
			kind = strings.ToLower(step.Keyword)
		}
		if kind != "given" {
			text := step.Keyword + " " + fillPlaceholders(step.Text, row)
			items = append(items, model.CheckItem{Text: text, Kind: kind})
		}
	}
	return items
}

// fail ends the session with a message and no checklist.
func (s *Session) fail(message string) {
	assert.That(message != "", "a failure says what failed")
	assert.That(s.status != SessionReady, "a finished setup doesn't fail afterwards")
	s.status, s.message, s.checklist = SessionFailed, message, nil
}

// restart drops in again to the same scenario and row: "Try again" and "Reset state".
func (s *Session) restart(skipTeardown bool) error {
	assert.That(s != nil, "restart runs on a session")
	assert.That(s.window != nil, "a session belongs to a window")
	scenario, ok := s.window.findScenario(s.scenario.Capability + "/" + s.scenario.Name)
	if !ok {
		return fmt.Errorf("%s: %w", s.scenario.Name, ErrNotFound)
	}
	_, err := s.window.startSession(scenario, outlineRow{values: s.row, examples: s.examples},
		skipTeardown)
	return err
}

// KitSourceLine is the first line of every setup script: it sources the kit.
const KitSourceLine = `source "$(dirname "${BASH_SOURCE[0]}")/../../_shared/givens.sh"`

// resume ends a wait at a card, adding the time waited so it doesn't count as slow.
func (s *Session) resume() {
	assert.That(s != nil, "resume runs on a session")
	assert.That(!s.started.IsZero(), "the session started")
	if s.status == SessionWaiting {
		s.waited += s.window.app.config.Clock.Now().Sub(s.waitingSince)
		s.status = SessionRunning
	}
}

// editFile opens a file inside the capabilities folder in the editor.
func (w *Window) editFile(path string) error {
	assert.That(w != nil, "edit file runs on a window")
	assert.That(path == "" || filepath.IsAbs(path), "the path is absolute")
	if path == "" {
		return ErrNotFound
	}
	relative, err := filepath.Rel(w.path, path)
	if err != nil {
		return err
	}
	_, err = w.Edit(relative)
	return err
}

// outlineRow is one Examples row: its block's slug and its values in column order. The zero
// value is "not an outline row".
type outlineRow struct {
	examples string
	values   []runs.Pair
}

// rowsOf lists an outline's rows, block by block, in file order.
func rowsOf(scenario project.Scenario) []outlineRow {
	assert.That(scenario.Name != "", "the scenario has a name")
	assert.That(len(scenario.Examples) <= project.HelperFilesMax, "blocks stay bounded")
	rows := make([]outlineRow, 0, 4)
	for _, block := range scenario.Examples {
		for _, cells := range block.Rows {
			values := make([]runs.Pair, 0, len(block.Header))
			for column, name := range block.Header {
				if column < len(cells) {
					values = append(values, runs.Pair{Name: name, Value: cells[column]})
				}
			}
			rows = append(rows, outlineRow{examples: block.Slug, values: values})
		}
	}
	return rows
}

// rowStatus says where one outline row stands, as StatusOf does for a scenario.
func rowStatus(loaded *project.Project, scenario project.Scenario, row outlineRow,
	now time.Time) string {
	assert.That(loaded != nil, "status is read in a loaded project")
	assert.That(scenario.Outline, "only an outline has rows")
	return statusFor(loaded, scenario, runs.Key(scenario.Key(), row.examples), row.values, now)
}

// rowName says a row as a person would: its first value, like "expired".
func rowName(row []runs.Pair) string {
	assert.That(len(row) <= runs.FieldsMax, "a row stays bounded")
	if len(row) == 0 {
		return ""
	}
	assert.That(row[0].Name != "", "a value has a column")
	return row[0].Value
}

// removeArguments removes the session's argument files, if it wrote any.
func (s *Session) removeArguments() {
	assert.That(s != nil, "arguments are removed for a session")
	assert.That(s.argumentsDir == "" || strings.Contains(s.argumentsDir, "givens-arguments-"),
		"only an arguments folder is removed")
	if s.argumentsDir != "" {
		_ = os.RemoveAll(s.argumentsDir)
	}
}
