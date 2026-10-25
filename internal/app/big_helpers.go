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
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"givens/internal/assert"
	"givens/internal/browser"
	"givens/internal/fingerprint"
	"givens/internal/helpers"
	"givens/internal/project"
	"givens/internal/runs"
	"givens/internal/slug"
	"givens/internal/spec"
	"givens/internal/watch"
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
	assert.That(len(scenario.Steps) <= helpers.HelpersMax, "steps stay bounded")
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
func (w *Window) Helpers() []Helper {
	assert.That(w != nil, "helpers runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	w.refresh()
	if w.project == nil {
		return []Helper{}
	}
	// Kept until a file changes: finding callers reads every script once per helper.
	if w.helpers != nil {
		return append([]Helper{}, w.helpers...)
	}
	shared, err := helpers.InFolder(w.path, "", "")
	if err != nil {
		return []Helper{}
	}
	listed := w.listHelpers(shared, "", nil)
	for _, capability := range w.project.Capabilities {
		own, err := helpers.InFolder(w.path, capability.Slug, capability.Slug)
		if err != nil {
			continue
		}
		listed = append(listed, w.listHelpers(own, capability.Name, shared)...)
	}
	w.helpers = listed
	return append([]Helper{}, listed...)
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

// ChromeHint is the install hint for a setup with browser lines.
const ChromeHint = "brew install --cask google-chrome"

// RequiredCommands lists the commands a scenario's setup needs, whether each is installed, and
// its install hint: the `need` lines in its script, the setups it builds on, and every helper
// those call. Needed by "See what a scenario needs installed" and by Drop in's check before
// anything runs.
func (w *Window) RequiredCommands(scenario string) ([]RequiredCommand, error) {
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
	needs := helpers.Needs(all)
	required := make([]RequiredCommand, 0, len(needs)+1)
	// A browser line needs Chrome, as a need line needs its command.
	if helpers.UsesBrowser(all) {
		_, found := browser.Chrome()
		required = append(required, RequiredCommand{
			Command: "Chrome", Hint: ChromeHint, Installed: found,
		})
	}
	for _, need := range needs {
		// LookPath searches PATH as a shell would. Ref: https://pkg.go.dev/os/exec#LookPath
		_, missing := exec.LookPath(need.Command)
		required = append(required, RequiredCommand{
			Command: need.Command, Hint: need.Hint, Installed: missing == nil,
		})
	}
	return required, nil
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
	current, err := fingerprint.Of(loaded, scenario, values)
	if err != nil {
		return StatusNeverRun
	}
	if changed := fingerprint.Changed(latest.Fingerprint, current); changed != "" {
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

// aboutImage matches a markdown image: ![alt](path).
const aboutImagePattern = `!\[([^\]]*)\]\(([^)\s]+)\)`

var aboutImage = regexp.MustCompile(aboutImagePattern) //tiger:allow-global compiled once

// withImagePlaceholders replaces each image about.md shows that isn't on disk with a line
// naming it, so the rest of the story still reads: "Image not found: images/flow.svg".
func withImagePlaceholders(about string, dir string) string {
	assert.That(filepath.IsAbs(dir), "the capability's folder is absolute")
	assert.That(len(about) <= EditorTextBytesMax, "about.md stays bounded")
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
	watcher, err := watch.New(w.path, time.Now())
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
		typing: -1, marks: []EditorMark{}}
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
	if len(content) > EditorTextBytesMax || !utf8.Valid(content) {
		return "", fmt.Errorf("%s isn't a text file Givens can edit", filepath.Base(path))
	}
	assert.That(len(content) <= EditorTextBytesMax, "editable text stays bounded")
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
