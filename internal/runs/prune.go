package runs

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"givens/internal/assert"
)

// What the ref keeps. It holds the newest runs only, so it can't grow without end: past
// either limit the oldest runs go, until both are at PruneToPercent or less, so it isn't
// trimmed on every run. The limits live in the ref's own limits file, so every device trims
// the same way. See "Old runs are dropped" in capabilities/keep-runs-off-main.
const (
	// LimitsFileName is the limits file at the root of the ref's copy.
	LimitsFileName = "limits"
	// RunsKeptDefault is how many runs the ref keeps when its limits file doesn't say.
	RunsKeptDefault = 1000
	// MegabytesKeptDefault is how many MB of runs the ref keeps when its limits file doesn't
	// say. GitHub asks for repositories under 1 GB, ideally; this leaves most of that for code.
	// Ref: https://docs.github.com/en/repositories/working-with-files/managing-large-files
	MegabytesKeptDefault = 250
	// PruneToPercent is how far a trim goes below each limit.
	PruneToPercent = 80
	// limitsBytesMax bounds the limits file: two short lines.
	limitsBytesMax = 1024
	// limitMax bounds either limit.
	limitMax = 1_000_000
)

// Limits bound what the ref keeps: the newest Runs runs, or Megabytes MB of them, whichever
// is fewer. A run's size is its line and its files folder.
type Limits struct {
	Runs      int
	Megabytes int
}

// DefaultLimits are the limits of a ref whose limits file doesn't say.
func DefaultLimits() Limits {
	limits := Limits{Runs: RunsKeptDefault, Megabytes: MegabytesKeptDefault}
	assert.That(limits.Runs > 0, "the default keeps runs")
	assert.That(limits.Megabytes > 0, "the default keeps bytes")
	return limits
}

// ReadLimits reads root/limits: a "runs N" line and a "megabytes N" line. A missing file, or
// a missing line, gives the default.
func ReadLimits(root string) (Limits, error) {
	assert.That(filepath.IsAbs(root), "root is absolute")
	limits := DefaultLimits()
	path := filepath.Join(root, LimitsFileName)
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return limits, nil
	}
	if err != nil {
		return Limits{}, err
	}
	if info.Size() > limitsBytesMax {
		return Limits{}, fmt.Errorf("%s is %d bytes, more than %d", path, info.Size(),
			limitsBytesMax)
	}
	text, err := os.ReadFile(path)
	if err != nil {
		return Limits{}, err
	}
	for _, line := range strings.Split(string(text), "\n") {
		name, value, _ := strings.Cut(strings.TrimSpace(line), " ")
		number, err := strconv.Atoi(strings.TrimSpace(value))
		if name == "" {
			continue
		}
		if err != nil || number < 1 || number > limitMax {
			return Limits{}, fmt.Errorf("%s: %q isn't a limit from 1 to %d", path, line, limitMax)
		}
		switch name {
		case "runs":
			limits.Runs = number
		case "megabytes":
			limits.Megabytes = number
		default:
			return Limits{}, fmt.Errorf("%s: no limit is called %q", path, name)
		}
	}
	return limits, nil
}

// WriteLimits writes root/limits.
func WriteLimits(root string, limits Limits) error {
	assert.That(limits.Runs > 0 && limits.Runs <= limitMax, "a run limit is in range")
	assert.That(limits.Megabytes > 0 && limits.Megabytes <= limitMax, "a MB limit is in range")
	text := fmt.Sprintf("runs %d\nmegabytes %d\n", limits.Runs, limits.Megabytes)
	return os.WriteFile(filepath.Join(root, LimitsFileName), []byte(text), 0o644)
}

// logLine is one line of a day's log, with what a trim weighs it by.
type logLine struct {
	file  string // the day's log it's in
	text  string
	run   Run
	read  bool  // false for a line that can't be read; it's kept and not counted
	bytes int64 // the line and its files folder
}

// Prune trims root/runs to limits, when either is passed: it keeps the newest runs, with their
// files, until both are at PruneToPercent or less, and each scenario's newest run line
// whatever its age, so none goes back to "Never run". It says whether anything went.
func Prune(root string, limits Limits) (bool, error) {
	assert.That(filepath.IsAbs(root), "root is absolute")
	assert.That(limits.Runs > 0 && limits.Megabytes > 0, "limits keep something")
	lines, err := readLogLines(root)
	if err != nil {
		return false, err
	}
	count, total := 0, int64(0)
	for _, line := range lines {
		if line.read {
			count, total = count+1, total+line.bytes
		}
	}
	bytesMax := int64(limits.Megabytes) << 20
	if count <= limits.Runs && total <= bytesMax {
		return false, nil
	}
	kept := keptLines(lines, limits.Runs*PruneToPercent/100, bytesMax*PruneToPercent/100)
	assert.That(len(kept) <= len(lines), "a trim keeps only lines there are")
	return rewriteLogs(root, lines, kept)
}

// readLogLines reads every line of root's day logs, weighed.
func readLogLines(root string) ([]logLine, error) {
	assert.That(filepath.IsAbs(root), "root is absolute")
	paths, err := filepath.Glob(filepath.Join(Dir(root), "*.log"))
	if err != nil {
		return nil, err
	}
	if len(paths) > RunFilesMax {
		return nil, fmt.Errorf("runs/ holds %d files, more than %d", len(paths), RunFilesMax)
	}
	lines := make([]logLine, 0, len(paths)*8)
	for _, path := range paths {
		texts, err := ReadLines(path)
		if err != nil {
			return nil, err
		}
		for _, text := range texts {
			line := logLine{file: path, text: text, bytes: int64(len(text) + 1)}
			if run, err := Parse(text); err == nil {
				line.run, line.read = run, true
				line.bytes += folderBytes(FilesDir(root, run.Key, run.Time))
			}
			lines = append(lines, line)
		}
	}
	return lines, nil
}

// ReadLines reads a day's log as its lines, blank ones left out.
// Ref: https://pkg.go.dev/bufio#Scanner.Buffer
func ReadLines(path string) ([]string, error) {
	assert.That(strings.HasSuffix(path, ".log"), "path is a run file")
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 4096), LineLengthBytesMax+1)
	texts := make([]string, 0, 16)
	// bound: Scan stops at the end of the file, which holds one day of runs.
	for scanner.Scan() {
		if text := scanner.Text(); strings.TrimSpace(text) != "" {
			texts = append(texts, text)
		}
	}
	if err := scanner.Err(); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	assert.That(len(texts) >= 0, "lines are counted")
	return texts, file.Close()
}

// folderBytes is the size of a run's files folder: one level, as CopyFiles makes it.
func folderBytes(folder string) int64 {
	assert.That(filepath.IsAbs(folder), "the folder is absolute")
	entries, err := os.ReadDir(folder)
	if err != nil {
		return 0
	}
	total := int64(0)
	for _, entry := range entries {
		if info, err := entry.Info(); err == nil && info.Mode().IsRegular() {
			total += info.Size()
		}
	}
	assert.That(total >= 0, "sizes add up")
	return total
}

// keptLines picks which lines a trim keeps, by index: lines that can't be read, the newest
// runs while both counts stay within runsMax and bytesMax, and each run key and row's newest.
// A line in the map stays; true keeps its files too.
func keptLines(lines []logLine, runsMax int, bytesMax int64) map[int]bool {
	assert.That(runsMax >= 0, "a run limit isn't negative")
	assert.That(bytesMax >= 0, "a byte limit isn't negative")
	order := make([]int, 0, len(lines))
	kept := make(map[int]bool, len(lines))
	for index, line := range lines {
		if line.read {
			order = append(order, index)
		} else {
			kept[index] = true
		}
	}
	// Newest first; ties broken by key, then text, so every device picks the same runs.
	// Ref: https://pkg.go.dev/sort#Slice
	sort.Slice(order, func(a, b int) bool {
		first, second := lines[order[a]], lines[order[b]]
		if !first.run.Time.Equal(second.run.Time) {
			return first.run.Time.After(second.run.Time)
		}
		if first.run.Key != second.run.Key {
			return first.run.Key < second.run.Key
		}
		return first.text < second.text
	})
	count, total := 0, int64(0)
	for _, index := range order {
		if count+1 > runsMax || total+lines[index].bytes > bytesMax {
			break
		}
		kept[index], count, total = true, count+1, total+lines[index].bytes
	}
	newest := make(map[string]bool, len(order))
	for _, index := range order {
		if row := rowKey(lines[index].run); !newest[row] {
			newest[row] = true
			if !kept[index] {
				kept[index] = false // the line stays; its files go
			}
		}
	}
	return kept
}

// rowKey names a run key and its row's values, the unit whose newest run is always kept.
func rowKey(run Run) string {
	assert.That(run.Key != "", "a run has a key")
	assert.That(len(run.Values) <= FieldsMax, "a row stays bounded")
	key := run.Key
	for _, value := range run.Values {
		key += "\t" + value.Name + "=" + value.Value
	}
	return key
}

// rewriteLogs writes each day's log with only its kept lines, removes a log left empty, and
// removes the files folders of runs kept without their files. It says whether anything went.
func rewriteLogs(root string, lines []logLine, kept map[int]bool) (bool, error) {
	assert.That(filepath.IsAbs(root), "root is absolute")
	assert.That(len(kept) <= len(lines), "only lines are kept")
	byFile := make(map[string][]string, 8)
	files := make([]string, 0, 8)
	changed := false
	for index, line := range lines {
		if _, seen := byFile[line.file]; !seen {
			files, byFile[line.file] = append(files, line.file), []string{}
		}
		withFiles, stays := kept[index]
		if stays {
			byFile[line.file] = append(byFile[line.file], line.text)
		}
		folder := ""
		if line.read {
			folder = FilesDir(root, line.run.Key, line.run.Time)
		}
		if _, err := os.Stat(folder); folder != "" && !withFiles && err == nil {
			if err := removeFolder(root, folder); err != nil {
				return false, err
			}
			changed = true
		}
		changed = changed || !stays
	}
	for _, file := range files {
		if err := writeLog(file, byFile[file]); err != nil {
			return false, err
		}
	}
	return changed, nil
}

// writeLog writes a day's log, or removes it when no line is left.
func writeLog(path string, texts []string) error {
	assert.That(strings.HasSuffix(path, ".log"), "path is a run file")
	assert.That(len(texts) <= RunFilesMax*LineLengthBytesMax, "lines stay bounded")
	if len(texts) == 0 {
		return os.Remove(path)
	}
	return os.WriteFile(path, []byte(strings.Join(texts, "\n")+"\n"), 0o644)
}

// removeFolder removes a run's files folder, then each parent left empty, up to runs/files.
func removeFolder(root string, folder string) error {
	assert.That(filepath.IsAbs(folder), "the folder is absolute")
	stop := filepath.Join(Dir(root), "files")
	assert.That(strings.HasPrefix(folder, stop+string(filepath.Separator)), "it's a run's folder")
	if err := os.RemoveAll(folder); err != nil {
		return err
	}
	parent := filepath.Dir(folder)
	// bound: each step goes one folder up, and stops at runs/files.
	for parent != stop && strings.HasPrefix(parent, stop) {
		// Remove fails on a folder that isn't empty, which ends the walk.
		// Ref: https://pkg.go.dev/os#Remove
		if os.Remove(parent) != nil {
			return nil
		}
		parent = filepath.Dir(parent)
	}
	return nil
}
